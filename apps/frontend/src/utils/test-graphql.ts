// Test to verify POST requests are being sent to GraphQL endpoint.
// Call from browser console; uses stored token when available so requests are authenticated.

import { getStoredToken } from "../stores/authStore";

async function testGraphQLMutation() {
  const query = `
    mutation UpdateConfig($input: UpdateConfigInput!) {
      updateConfig(input: $input) {
        agentName
        systemPrompt
        provider
        channels {
          channelId
          channelName
          enabled
        }
      }
    }
  `;

  const variables = {
    input: {
      agentName: "test-agent",
      provider: "ollama",
      systemPrompt: "You are helpful.",
    },
  };

  const headers: Record<string, string> = { "Content-Type": "application/json" };
  const token = getStoredToken();
  if (token) headers["Authorization"] = `Bearer ${token}`;

  try {
    const response = await fetch("/graphql", {
      method: "POST",
      headers,
      body: JSON.stringify({
        query,
        variables,
      }),
    });

    const data = await response.json();
    return data;
  } catch (error) {
    console.error("GraphQL mutation failed:", error);
    throw error;
  }
}

// Call this from browser console to test
export { testGraphQLMutation };
