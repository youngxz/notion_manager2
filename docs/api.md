# API Usage

[← Back to README](../README.md)

## Standard request

`/v1/messages` accepts Anthropic Messages API payloads.

```bash
curl http://localhost:3000/v1/messages \
  -H "x-api-key: <api_key>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "opus-4.6",
    "max_tokens": 1024,
    "messages": [
      { "role": "user", "content": "Describe the main components of this project." }
    ]
  }'
```

If `model` is omitted, the service falls back to `proxy.default_model`.

## Search overrides

```bash
curl http://localhost:3000/v1/messages \
  -H "x-api-key: <api_key>" \
  -H "Content-Type: application/json" \
  -H "X-Web-Search: true" \
  -H "X-Workspace-Search: false" \
  -d '{
    "model": "sonnet-4.6",
    "messages": [
      { "role": "user", "content": "Search for recent information about Go 1.25." }
    ]
  }'
```

## File uploads

Supported media types:

- `image/png`
- `image/jpeg`
- `image/gif`
- `image/webp`
- `application/pdf`
- `text/csv`

```bash
curl http://localhost:3000/v1/messages \
  -H "x-api-key: <api_key>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "sonnet-4.6",
    "max_tokens": 600,
    "messages": [{
      "role": "user",
      "content": [
        {
          "type": "document",
          "source": {
            "type": "base64",
            "media_type": "application/pdf",
            "data": "<base64>"
          }
        },
        {
          "type": "text",
          "text": "Summarize this PDF."
        }
      ]
    }]
  }'
```

## Research mode

```bash
curl -N http://localhost:3000/v1/messages \
  -H "x-api-key: <api_key>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "researcher",
    "stream": true,
    "max_tokens": 16000,
    "thinking": { "type": "enabled", "budget_tokens": 50000 },
    "messages": [
      { "role": "user", "content": "Map common architectural patterns used by Notion AI proxy tools." }
    ]
  }'
```

Research mode is single-turn, ignores file uploads, ignores custom tools, and runs with a longer timeout path.
