# llsed

A high-performance HTTP proxy for translating between different LLM API formats (OpenAI, Anthropic, Bedrock, etc.) using lazy-loaded JSON-RPC transformation hooks.

## Overview

`llsed` acts as a transparent proxy that sits between your application and LLM API providers. It transforms requests and responses on-the-fly using a tranforms JSON which can use templated strings to do transforms and external JSON-RPC services where they can't, allowing you to:

- Use OpenAI-formatted clients with Claude/Anthropic APIs
- Normalize different API formats to a common interface
- Add custom pre/post-processing without modifying application code
- Hot-swap transformation logic without restarting the proxy
- Scale transformation services independently

## Architecture
```
Client Request
    ↓
llsed (proxy)
    ↓
Pre-transform (JSON-RPC) ← optional, lazy-loaded
    ↓
Target API (OpenAI/Anthropic/etc)
    ↓
Post-transform (JSON-RPC) ← optional, lazy-loaded
    ↓
Client Response
```

## Installation
```bash
go build -o llsed llsed.go
```

Or run directly:
```bash
go run llsed.go [flags]
```

## Usage
```bash
llsed --host 0.0.0.0 \
       --port 8080 \
       --map_file config.json \
       --server https://api.openai.com
```

### Flags

- `--host` - Host to bind to (default: `0.0.0.0`)
- `--port` - Port to listen on (default: `8080`)
- `--map_file` - Path to transformation configuration file (default: `config.json`)
- `--server` - Target API server URL (default: `https://api.openai.com`)

## Configuration

Create a `config.json` file defining transformation rules:
```json
{
  "rules": [
    {
      "tag": "claude_to_openai",
      "from": "anthropic",
      "to": "openai",
      "params": {
        "model_map": {
          "claude-3-opus": "gpt-4",
          "claude-3-sonnet": "gpt-3.5-turbo"
        }
      },
      "pre": "http://localhost:9001",
      "post": "http://localhost:9002"
    }
  ]
}
```

### Configuration Fields

- `tag` - Identifier for this rule
- `from` - Source API format
- `to` - Target API format
- `params` - Optional parameters (currently unused, reserved for future use)
- `pre` - JSON-RPC endpoint for request transformation (optional)
- `post` - JSON-RPC endpoint for response transformation (optional)

## JSON-RPC Transformation Services

Transformation services receive and return JSON via JSON-RPC 2.0.

### Request Format
```json
{
  "jsonrpc": "2.0",
  "method": "transform",
  "params": {
    "model": "claude-3-opus",
    "messages": [...]
  },
  "id": 1
}
```

### Response Format
```json
{
  "jsonrpc": "2.0",
  "result": {
    "model": "gpt-4",
    "messages": [...]
  },
  "id": 1
}
```

### Example Transformation Service (Python)
```python
from flask import Flask, request, jsonify

app = Flask(__name__)

@app.route('/', methods=['POST'])
def transform():
    data = request.json
    params = data['params']
    
    # Transform Anthropic format to OpenAI format
    transformed = {
        "model": params.get("model", "").replace("claude-3-opus", "gpt-4"),
        "messages": [
            {"role": msg["role"], "content": msg["content"][0]["text"]}
            for msg in params.get("messages", [])
        ]
    }
    
    return jsonify({
        "jsonrpc": "2.0",
        "result": transformed,
        "id": data["id"]
    })

if __name__ == '__main__':
    app.run(port=9001)
```

### Example Transformation Service (Node.js)
```javascript
const express = require('express');
const app = express();

app.use(express.json());

app.post('/', (req, res) => {
  const { params, id } = req.body;
  
  // Transform OpenAI response back to Anthropic format
  const transformed = {
    content: [
      {
        type: "text",
        text: params.choices[0].message.content
      }
    ],
    role: params.choices[0].message.role,
    model: params.model
  };
  
  res.json({
    jsonrpc: "2.0",
    result: transformed,
    id: id
  });
});

app.listen(9002, () => console.log('Post-transform service on :9002'));
```

## Example: Using OpenAI Client with Claude API

1. Start your transformation services:
```bash
# Pre-transform: OpenAI → Anthropic format
python pre_transform.py

# Post-transform: Anthropic → OpenAI format  
node post_transform.js
```

2. Start llsed:
```bash
llsed --server https://api.anthropic.com --port 8080
```

3. Point your OpenAI client at llsed:
```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="your-anthropic-key"
)

response = client.chat.completions.create(
    model="claude-3-opus",
    messages=[{"role": "user", "content": "Hello!"}]
)
```

## Use Cases

### API Format Translation
Use OpenAI SDKs/tools with Anthropic Claude, or vice versa.

### Multi-Provider Abstraction
Write code once, swap providers by changing `--server` flag.

### Request/Response Augmentation
- Add logging/metrics without touching application code
- Inject system prompts
- Filter/sanitize inputs
- Cache responses
- Rate limiting

### A/B Testing
Route requests to different providers based on rules.

### Development/Testing
Mock API responses for testing without hitting real APIs.

## Performance Considerations

- **Lazy Loading**: JSON-RPC services only called if configured
- **Connection Pooling**: HTTP client reuses connections
- **Minimal Overhead**: Simple proxy logic, no heavy processing in main path
- **Latency**: ~1-5ms overhead for transformation calls (negligible vs API latency)

## Roadmap

- [ ] Rule matching based on request content/headers
- [ ] Streaming support (SSE)
- [ ] Unix socket support for RPC calls
- [ ] Metrics endpoint (Prometheus)
- [ ] Request/response logging
- [ ] Configuration hot-reload
- [ ] Multiple rule matching
- [ ] Built-in common transformations (no RPC needed)
- [ ] Circuit breaker for RPC services

## Contributing

Pull requests welcome! Please ensure:
- Code is formatted with `go fmt`
- Changes include tests
- README is updated for new features

## License

MIT

## Credits

Inspired by the need for a clean separation of concerns between API proxying and protocol translation logic.
