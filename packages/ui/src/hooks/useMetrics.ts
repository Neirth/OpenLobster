// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { createQuery } from "@tanstack/solid-query";
import type { GraphQLClient } from "graphql-request";
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore – no type declarations for parse-prometheus-text-format
import parsePrometheusTextFormat from "parse-prometheus-text-format";
import type { Metrics } from "../types/index";

/** Single sample returned by parse-prometheus-text-format */
interface PromSample {
  value: string;
  labels?: Record<string, string>;
}

/** Top-level metric family returned by parse-prometheus-text-format */
interface PromFamily {
  name: string;
  type: string;
  help: string;
  metrics: PromSample[];
}

/**
 * Extracts the numeric value of the first sample in a metric family.
 * Returns 0 when the family is missing or the value is not a finite number.
 */
function valueOf(families: PromFamily[], metricName: string): number {
  const family = families.find((f) => f.name === metricName);
  if (!family || family.metrics.length === 0) return 0;
  const n = parseFloat(family.metrics[0].value);
  return Number.isFinite(n) ? n : 0;
}

/**
 * Fetches system-wide metrics from the /metrics OpenTelemetry Prometheus
 * endpoint with a 5-second polling interval.
 *
 * Only polls when the page is visible to avoid wasteful background requests.
 *
 * The endpoint exports data in Prometheus text exposition format.
 * parse-prometheus-text-format is used to parse the scrape response into
 * metric families, which are then mapped to the Metrics domain type.
 *
 * The metrics URL is derived from the GraphQL client URL by replacing the
 * path segment: /graphql -> /metrics.
 *
 * @param client - GraphQL client instance (used to derive the base URL)
 * @returns solid-query result containing Metrics or undefined while loading
 */
export function useMetrics(client: GraphQLClient) {
  const metricsUrl = (client as unknown as { url: string }).url.replace(
    /\/graphql$/,
    "/metrics",
  );

  return createQuery<Metrics>(() => ({
    queryKey: ["metrics"],
    queryFn: async () => {
      const res = await fetch(metricsUrl, {
        headers: { Accept: "text/plain" },
      });
      if (!res.ok) throw new Error(`metrics fetch failed: ${res.status}`);
      const text = await res.text();
      const families: PromFamily[] = parsePrometheusTextFormat(
        text,
      ) as PromFamily[];

      return {
        uptime: valueOf(families, "openlobster_uptime_seconds"),
        messagesReceived: valueOf(
          families,
          "openlobster_messages_received_total",
        ),
        messagesSent: valueOf(families, "openlobster_messages_sent_total"),
        activeSessions: valueOf(families, "openlobster_active_sessions"),
        memoryNodes: valueOf(families, "openlobster_memory_nodes"),
        memoryEdges: valueOf(families, "openlobster_memory_edges"),
        mcpTools: valueOf(families, "openlobster_mcp_tools"),
        tasksPending: valueOf(families, "openlobster_tasks_pending"),
        tasksRunning: valueOf(families, "openlobster_tasks_running"),
        tasksDone: valueOf(families, "openlobster_tasks_done_total"),
        errorsTotal: valueOf(families, "openlobster_errors_total"),
      } satisfies Metrics;
    },
    refetchInterval: (query) => {
      if (
        typeof document !== "undefined" &&
        document.visibilityState === "hidden"
      ) {
        return false;
      }
      return 5_000;
    },
    enabled:
      typeof document !== "undefined"
        ? document.visibilityState === "visible"
        : true,
  }));
}
