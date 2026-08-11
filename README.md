# GATRA: Zero-Trust Security Proxy & Control Plane for AI Agents

> **Automated, cryptographic runtime enforcement for Model Context Protocol (MCP) servers, tool calls, and LLM agent trajectories.**

---

## Overview

**GATRA** is an enterprise-grade, high-performance security reverse proxy designed to govern autonomous AI agents. By placing GATRA between agent orchestrators (LangChain, CrewAI, AutoGen, custom MCP clients) and downstream APIs or MCP tools, DevSecOps teams can enforce strict rate limits, Common Expression Language (CEL) safety policies, stateful trajectory boundaries, and ephemeral task-level constraints without altering agent code.

```text
┌─────────────────┐       ┌───────────────────────────────┐       ┌─────────────────┐
│                 │  HTTP │    GATRA Security Proxy       │  HTTP │                 │
│   LLM Agent /   ├──────>│  • Ed25519 Token Auth         ├──────>│   Downstream    │
│  Orchestrator   │       │  • Stateful Trajectory Caps   │       │   MCP Tool /    │
│                 │       │  • Schema Auto-Discovery      │       │   API Target    │
└─────────────────┘       └───────────────────────────────┘       └─────────────────┘
```

---

## Key Features

- **Zero-Trust Asymmetric Authentication:** Mints short-lived capability tokens signed with Ed25519 private keys. The proxy validates tokens statelessly using public keys.
- **Stateful Trajectory Boundaries:** Enforces cumulative consumption caps across multiple calls in a single session (e.g., limit spending to $200 total per trajectory).
- **Schema Auto-Discovery (`gatra discover`):** Automatically scans MCP tool definitions (`tools/list`) or OpenAPI schemas to draft zero-effort baseline security policies.
- **Ephemeral Task Directives (`X-Gatra-Directive`):** Enforces dynamic, per-task constraints passed in request headers while guaranteeing the **Monotonic Restriction Principle** (ephemeral directives can only tighten, never relax, global policy ceilings).
- **CEL Safety Engine:** Evaluates rich Google Common Expression Language (CEL) logic against incoming JSON payloads to block injection attacks, forbidden parameters, or path traversals.
- **Dry-Run / Audit Mode (`--dry-run`):** Log violations and emit Prometheus metrics without blocking live production traffic (`HTTP 200` permitted).
- **Prometheus Telemetry & Admin API:** Real-time metrics export (`/metrics`) and embedded dynamic hot-reloading policy store (`/admin/api/policies`).

---

## Quickstart

### 1. Build the Binary

```bash
go build -o bin/gatra ./cmd/gatra
```

### 2. Auto-Discover Policy from MCP Tools

Generate a baseline `policy.json` directly from an MCP tool manifest:

```bash
./bin/gatra discover -s examples/policies/sample_mcp_tools.json -o policy.json
```

### 3. Mint Keypair and Capability Token

Generate an Ed25519 keypair and signed session token:

```bash
./bin/gatra gen-keys -t session_101 -p "demo/tool" --json
```

### 4. Start GATRA Proxy

```bash
./bin/gatra start -c policy.json -k "<YOUR_BASE64_PUBLIC_KEY>"
```

---

## Native SDKs

GATRA provides lightweight, zero-dependency SDKs for seamless orchestrator integration:

### Python SDK (`gatra-sdk`)

```bash
pip install gatra-sdk
```

```python
from gatra import GatraTokenIssuer, GatraClient

# 1. Mint Capability Token
issuer = GatraTokenIssuer(private_key_base64="<YOUR_PRIVATE_KEY>")
token = issuer.mint_token(trajectory_id="session_101", tool_pattern="*")

# 2. Execute tool calls through proxy
client = GatraClient(proxy_url="http://localhost:8080", capability_token=token)
status, response, latency = client.execute_tool("/v1/action", {"amount": 25.00, "currency": "USD"})
```

### TypeScript / JavaScript SDK (`@gatra/sdk`)

```bash
npm install @gatra/sdk
```

```typescript
import { GatraTokenIssuer, GatraClient } from '@gatra/sdk';

const issuer = new GatraTokenIssuer('<YOUR_PRIVATE_KEY>');
const token = issuer.mintToken('session_101', '*');

const client = new GatraClient('http://localhost:8080', token);
const { status, data } = await client.executeTool('/v1/action', { amount: 25.00, currency: 'USD' });
```

---

## Ephemeral Task Directives

Orchestrators can pass task-specific guardrails dynamically using the `X-Gatra-Directive` header:

```bash
curl -X POST http://localhost:8080/v1/action \
  -H "X-Capability-Token: <YOUR_TOKEN>" \
  -H "X-Gatra-Directive: {\"max_per_call\": 30.00, \"condition\": \"payload.currency == 'USD'\"}" \
  -H "Content-Type: application/json" \
  -d '{"amount": 25.00, "currency": "USD"}'
```

---

## Repository Structure

```text
gatra/
├── cmd/gatra/              # Cobra CLI binaries (start, gen-keys, discover, validate-config)
├── internal/               # Core data plane proxy, CEL engine, and state accumulators
├── sdks/                   # Native client SDKs (python, typescript)
├── examples/               # Node.js & Python agent simulation recipes
├── tests/                  # End-to-end integration and ephemeral constraint test suites
├── policy.json             # Starter policy configuration
└── Makefile                # Cross-platform developer workflow build file
```

---

## License

Distributed under the [MIT License](LICENSE).