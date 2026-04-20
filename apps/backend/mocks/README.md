# Mockoon Mocks

This directory contains Mockoon configuration files for mocking external services during development and testing.

## Usage

1. Install Mockoon: https://mockoon.com/download/
2. Open Mockoon and import one of the `.json` files
3. Start the mock server
4. Update your configuration to point to `localhost:<port>`

## Available Mocks

| Service  | Port | File |
|----------|------|------|
| Telegram Bot API | 3001 | telegram.json |
| Discord API | 3002 | discord.json |
| OpenAI API | 3003 | openai.json |
| Ollama API | 3004 | ollama.json |
| OpenRouter API | 3005 | openrouter.json |
| Twilio API | 3006 | twilio.json |
| WhatsApp Business API | 3007 | whatsapp.json |

## Example Configuration

```yaml
messaging:
  telegram:
    api_url: "http://localhost:3001"
  discord:
    api_url: "http://localhost:3002"

ai:
  openai:
    endpoint: "http://localhost:3003/v1"
  ollama:
    endpoint: "http://localhost:3004"
  openrouter:
    endpoint: "http://localhost:3005/v1"
```

## Running Multiple Mocks

You can run multiple mock servers simultaneously on different ports. Each file is independent.
