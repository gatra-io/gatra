# gatra-sdk: Zero-Trust AI Agent Security SDK (Python)

> **Official Python client SDK for GATRA — the Zero-Trust Security Proxy & Control Plane for AI Agents, Model Context Protocol (MCP) servers, and LLM tool calls.**

---

## Overview

`gatra-sdk` provides native, zero-dependency Python utilities for orchestrating autonomous AI agents behind a **GATRA Security Proxy**. It enables Python agent frameworks (LangChain, LlamaIndex, CrewAI, AutoGen, or custom MCP clients) to mint local Ed25519 capability tokens and route tool calls through GATRA's cryptographic policy engine.

```text
┌─────────────────┐       ┌───────────────────────────────┐       ┌─────────────────┐
│                 │  HTTP │    GATRA Security Proxy       │  HTTP │                 │
│   LLM Agent /   ├──────>│  • Ed25519 Token Auth         ├──────>│   Downstream    │
│  Orchestrator   │       │  • Stateful Trajectory Caps   │       │   MCP Tool /    │
│  (gatra-sdk)    │       │  • Schema Auto-Discovery      │       │   API Target    │
└─────────────────┘       └───────────────────────────────┘       └─────────────────┘
```

---

## Key Features

- **Asymmetric Token Minting:** Mint short-lived, Ed25519-signed capability tokens locally without contacting a central authorization server.
- **Proxy Client Wrapper:** Executing requests through `GatraClient` automatically injects security headers (`X-Capability-Token`, `X-Gatra-Directive`).
- **Ephemeral Task Directives:** Pass runtime, per-task guardrails directly in requests while preserving GATRA's Monotonic Restriction Principle.
- **Zero Heavy Dependencies:** Pure Python implementation using Standard Library and `cryptography` for lightning-fast capability token minting.

---

## Installation

```bash
pip install gatra-sdk
```

---

## Quickstart

> **Prerequisite:** Make sure the GATRA proxy binary is running locally on port `8080`:
> ```bash
> ./bin/gatra start -c policy.json -k "<YOUR_BASE64_PUBLIC_KEY>" --port 8080 --target http://localhost:3000
> ```

### 1. Basic Token Minting & Tool Execution

```python
from gatra import GatraTokenIssuer, GatraClient

# Step 1: Initialize Token Issuer with your base64-encoded Ed25519 private key
issuer = GatraTokenIssuer(private_key_base64="YOUR_BASE64_PRIVATE_KEY")

# Step 2: Mint a capability token bound to a specific trajectory/session
capability_token = issuer.mint_token(
    trajectory_id="session_101",
    tool_pattern="*"
)

# Step 3: Initialize GATRA Client pointing to your security proxy instance at localhost:8080
client = GatraClient(
    proxy_url="http://localhost:8080",
    capability_token=capability_token
)

# Step 4: Execute a tool call safely through GATRA Proxy
status, response, latency_ms = client.execute_tool(
    path="/v1/action",
    payload={
        "amount": 25.00,
        "currency": "USD"
    }
)

print(f"[HTTP {status}] Executed in {latency_ms}ms:", response)
```

---

## Ephemeral Task Directives

Orchestrators can dynamically inject tighter guardrails for a specific execution step without altering global proxy policies:

```python
import json

# Define an ephemeral constraint for this specific invocation
ephemeral_directive = json.dumps({
    "max_per_call": 30.00,
    "condition": "payload.currency == 'USD'"
})

# Execute request with directive attached
status, response, latency_ms = client.execute_tool(
    path="/v1/action",
    payload={"amount": 25.00, "currency": "USD"},
    directive=ephemeral_directive
)
```

---

## API Reference

### `GatraTokenIssuer`
- **`__init__(private_key_base64: str)`** — Initializes issuer with an Ed25519 private key.
- **`mint_token(trajectory_id: str, tool_pattern: str, ttl_seconds: int = 3600) -> str`** — Signs and returns a compact Ed25519 capability token.

### `GatraClient`
- **`__init__(proxy_url: str, capability_token: str = None)`** — Initializes proxy client targeting a GATRA gateway.
- **`execute_tool(path: str, payload: dict, directive: str = None) -> tuple[int, dict, float]`** — Dispatch HTTP POST requests with automatically managed security headers. Returns `(status_code, response_json, latency_ms)`.

---

## Resources

- **Core Repository:** [github.com/gatra-io/gatra](https://github.com/gatra-io/gatra)
- **TypeScript / JavaScript SDK (`gatra-sdk`):** [npmjs.com/package/gatra-sdk](https://www.npmjs.com/package/gatra-sdk)

---

## License

Distributed under the [MIT License](https://github.com/gatra-io/gatra/blob/main/LICENSE).