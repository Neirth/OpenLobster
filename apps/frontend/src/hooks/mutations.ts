// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { createMutation } from "@tanstack/solid-query";
import type { GraphQLClient } from "graphql-request";
import {
  UPDATE_CONFIG_MUTATION,
  ADD_MCP_SERVER_MUTATION,
  REMOVE_MCP_SERVER_MUTATION,
  ADD_TASK_MUTATION,
  REMOVE_TASK_MUTATION,
} from "../graphql/mutations";

/**
 * Hook to update agent configuration via GraphQL mutation
 */
export function useUpdateConfig(client: GraphQLClient) {
  return createMutation(() => ({
    mutationFn: async (variables: Record<string, any>) => {
      console.log("Sending updateConfig mutation with variables:", variables);
      const result = await client.request(UPDATE_CONFIG_MUTATION, variables);
      return result;
    },
    onError: (error: any) => {
      console.error("updateConfig mutation failed:", error);
    },
    onSuccess: (data: any) => {
      console.log("updateConfig mutation succeeded:", data);
    },
  }));
}

/**
 * Hook to add a new MCP server via GraphQL mutation
 */
export function useAddMCPServer(client: GraphQLClient) {
  return createMutation(() => ({
    mutationFn: async (variables: Record<string, any>) => {
      console.log("Sending addMCPServer mutation with variables:", variables);
      const result = await client.request(ADD_MCP_SERVER_MUTATION, variables);
      return result;
    },
    onError: (error: any) => {
      console.error("addMCPServer mutation failed:", error);
    },
    onSuccess: (data: any) => {
      console.log("addMCPServer mutation succeeded:", data);
    },
  }));
}

/**
 * Hook to remove an MCP server via GraphQL mutation
 */
export function useRemoveMCPServer(client: GraphQLClient) {
  return createMutation(() => ({
    mutationFn: async (variables: Record<string, any>) => {
      console.log("Sending removeMCPServer mutation with variables:", variables);
      const result = await client.request(REMOVE_MCP_SERVER_MUTATION, variables);
      return result;
    },
    onError: (error: any) => {
      console.error("removeMCPServer mutation failed:", error);
    },
    onSuccess: (data: any) => {
      console.log("removeMCPServer mutation succeeded:", data);
    },
  }));
}

/**
 * Hook to add a new task via GraphQL mutation
 */
export function useAddTask(client: GraphQLClient) {
  return createMutation(() => ({
    mutationFn: async (variables: Record<string, any>) => {
      console.log("Sending addTask mutation with variables:", variables);
      const result = await client.request(ADD_TASK_MUTATION, variables);
      return result;
    },
    onError: (error: any) => {
      console.error("addTask mutation failed:", error);
    },
    onSuccess: (data: any) => {
      console.log("addTask mutation succeeded:", data);
    },
  }));
}

/**
 * Hook to remove a task via GraphQL mutation
 */
export function useRemoveTask(client: GraphQLClient) {
  return createMutation(() => ({
    mutationFn: async (variables: Record<string, any>) => {
      console.log("Sending removeTask mutation with variables:", variables);
      const result = await client.request(REMOVE_TASK_MUTATION, variables);
      return result;
    },
    onError: (error: any) => {
      console.error("removeTask mutation failed:", error);
    },
    onSuccess: (data: any) => {
      console.log("removeTask mutation succeeded:", data);
    },
  }));
}
