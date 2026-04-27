---
description: "Specification for audio and speech processing plugins in OpenLobster."
icon: volume-high
---

# Audio Plugins

Audio plugins provide Text-to-Speech (TTS) and Speech-to-Text (STT) capabilities, enabling voice interactions within OpenLobster.

## Identification

- **Type**: `audio`
- **Required Functions**: `tts`, `stt`

---

## The Audio Standard

To ensure interoperability between messaging platforms, AI models, and audio processors, OpenLobster defines a **Native Audio Standard**. While plugins can support multiple formats (mp3, ogg, etc.), they MUST support the following raw format when `format` is set to `pcm`:

| Property | Value |
|----------|-------|
| **Sample Rate** | 16000 Hz |
| **Channels** | 1 (Mono) |
| **Bit Depth** | 16-bit |
| **Endianness** | Little Endian (LE) |
| **Encoding** | Signed Integer PCM |

> [!TIP]
> If a messaging plugin receives audio in a platform-specific format (like Ogg Opus for Telegram or WhatsApp), it is responsible for converting it to the Native Standard before sending it to the Core, or identifying the format correctly so the Audio plugin can handle it.

---

## Multimodal Routing Pipeline

The OpenLobster Core acts as a central router for audio data. The flow depends on the capabilities of the configured AI model:

### 1. Inbound Flow (User → Core)
1.  **Messaging Plugin**: Receives audio from the platform, encodes it in Base64, and identifies the format.
2.  **Core (`MessageHandler`)**: Receives the message and checks the AI Provider.
    *   **Native Support**: If the AI model supports audio input, the Core sends the raw/encoded bytes directly to the AI.
    *   **Fallback (STT)**: If the AI model is text-only, the Core calls the Audio Plugin's `stt` function, retrieves the text, and passes that to the AI.

### 2. Outbound Flow (Core → User)
1.  **Core (`MessageHandler`)**: Receives a text response from the AI.
2.  **Voice Trigger**: If the user initiated the conversation via voice, the Core triggers the audio pipeline.
    *   **Native Support**: If the AI model can generate audio, it returns the bytes directly.
    *   **Fallback (TTS)**: The Core calls the Audio Plugin's `tts` function with the assistant's text.
3.  **Messaging Plugin**: Receives the audio from the Core and dispatches it to the platform (using `SendVoice`).

---

## Exported Functions

### `tts` (Text-to-Speech)
Converts a string of text into audio data.

**Request Parameters:**
```json
{
  "config": { ... },
  "text": "Hello, world!",
  "voice_id": "optional-voice-id"
}
```

**Response:**
Returns an object containing the base64-encoded audio and its format.
```json
{
  "audio": "base64...",
  "format": "pcm|mp3|ogg|wav",
  "error": null
}
```

### `stt` (Speech-to-Text)
Transcribes audio data into a text string.

**Request Parameters:**
```json
{
  "config": { ... },
  "audio": "base64...",
  "format": "pcm|ogg|mp3",
  "language": "en"
}
```

**Response:**
```json
{
  "text": "The transcribed text",
  "language": "en",
  "error": null
}
```
