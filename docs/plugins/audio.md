# Audio Plugins
Audio plugins provide Text-to-Speech (TTS) and audio processing capabilities.
## Identification
- **Type**: `audio`
## Core Methods

### `get_info`
Returns general plugin metadata plus audio capabilities.
- **`properties`**:
    - `supports_tts` (bool): If true, `tts` is available.
    - `supports_stt` (bool): If true, `stt` is available.

---

## Exported Functions

### `tts`
Converts text to speech.

**Request (`params`):**
```json
{
  "text": "Hello, world!"
}
```

### `configure`
Update settings.
