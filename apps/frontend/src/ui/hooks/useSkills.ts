// Copyright (c) OpenLobster contributors. See LICENSE for details.

import { createQuery } from '@tanstack/solid-query';
import type { GraphQLClient } from 'graphql-request';
import { SKILLS_QUERY } from '../graphql/queries/index';
import type { Skill } from '../types/index';

interface SkillsQueryResult {
  skills: Skill[];
}

/**
 * Fetches available skills with a 30-second polling interval.
 * Skills rarely change, so a long interval is appropriate.
 *
 * @param client - GraphQL client instance
 * @returns solid-query result containing Skill[] or undefined while loading
 */
export function useSkills(client: GraphQLClient) {
  return createQuery<Skill[]>(() => ({
    queryKey: ['skills'],
    queryFn: async () => {
      const data = await client.request<SkillsQueryResult>(SKILLS_QUERY);
      return data.skills;
    },
    refetchInterval: 30_000,
  }));
}
