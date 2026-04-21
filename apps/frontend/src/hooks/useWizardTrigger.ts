// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { createEffect } from "solid-js";
import { useNavigate } from "@solidjs/router";
import type { CreateQueryResult } from "@tanstack/solid-query";
import type { AppConfig } from "@/types";

/**
 * useWizardTrigger monitors the configuration status and redirects to the /wizard
 * route if the initial setup has not been completed.
 * 
 * @param configQuery - The solid-query result containing the application config.
 */
export function useWizardTrigger(configQuery: CreateQueryResult<AppConfig, Error>) {
  const navigate = useNavigate();

  createEffect(() => {
    // Only redirect if config is loaded and wizardCompleted is explicitly false.
    // Errors (e.g. backend down) are handled by the query's loading state.
    if (configQuery.data && configQuery.data.wizardCompleted === false) {
      navigate("/wizard", { replace: true });
    }
  });
}
