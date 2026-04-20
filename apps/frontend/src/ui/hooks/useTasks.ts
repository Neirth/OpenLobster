// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { createQuery } from '@tanstack/solid-query';
import type { GraphQLClient } from 'graphql-request';
import { TASKS_QUERY } from '../graphql/queries/index';
import type { Task } from '../types/index';

interface TasksQueryResult {
  tasks: Task[];
}

/**
 * Fetches scheduled tasks with a 3-second polling interval.
 * Refetches on mount and window focus so the tasks view is up to date.
 *
 * @param client - GraphQL client instance
 * @returns solid-query result containing Task[] or undefined while loading
 */
export function useTasks(client: GraphQLClient) {
  return createQuery<Task[]>(() => ({
    queryKey: ['tasks'],
    queryFn: async () => {
      const data = await client.request<TasksQueryResult>(TASKS_QUERY);
      return data.tasks;
    },
    refetchInterval: 3_000,
    refetchOnMount: 'always',
    refetchOnWindowFocus: true,
  }));
}
