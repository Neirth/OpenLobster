import { createGraphqlClient } from "./client";
import { getStoredToken } from "@/stores/authStore";

import { GRAPHQL_ENDPOINT } from "./constants";

export const client = createGraphqlClient(GRAPHQL_ENDPOINT, getStoredToken);
