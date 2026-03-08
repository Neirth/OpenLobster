// Test to verify POST requests are being sent to GraphQL endpoint

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

  try {
    const response = await fetch("/graphql", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        query,
        variables,
      }),
    });

    console.log("POST request sent to /graphql");
    console.log("Response status:", response.status);
    const data = await response.json();
    console.log("Response data:", data);
    return data;
  } catch (error) {
    console.error("GraphQL mutation failed:", error);
    throw error;
  }
}

// Call this from browser console to test
export { testGraphQLMutation };
