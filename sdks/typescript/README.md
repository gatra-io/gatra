# gatra-sdk: Zero-Trust AI Agent Security SDK

> **Official Node.js & TypeScript client SDK for GATRA — the Zero-Trust Security Proxy & Control Plane for AI Agents, Model Context Protocol (MCP) servers, and LLM tool calls.**

---

## Overview

`gatra-sdk` provides native, zero-dependency TypeScript/JavaScript utilities for orchestrating autonomous AI agents behind a **GATRA Security Proxy**. It enables agent orchestrators (LangChain, CrewAI, AutoGen, or custom MCP clients) to mint local Ed25519 capability tokens and route tool calls through GATRA's cryptographic policy engine.

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
- **Zero Heavy Dependencies:** Fully typed, lightweight, and optimized for Node.js (>= 18), Bun, and modern JS runtimes.

---

## Installation

```bash
npm install gatra-sdk
```

---

## Quickstart

### 1. Basic Token Minting & Tool Execution

```typescript
import { GatraTokenIssuer, GatraClient } from 'gatra-sdk';

// Step 1: Initialize Token Issuer with your base64-encoded Ed25519 private key
const issuer = new GatraTokenIssuer('YOUR_BASE64_PRIVATE_KEY');

// Step 2: Mint a capability token bound to a specific trajectory/session
const capabilityToken = issuer.mintToken('session_101', '*');

// Step 3: Initialize GATRA Client pointing to your security proxy instance
const client = new GatraClient('http://localhost:8080', capabilityToken);

// Step 4: Execute a tool call safely through GATRA
async function runTool() {
  const { status, data, latencyMs } = await client.executeTool('/v1/action', {
    amount: 25.00,
    currency: 'USD'
  });

  console.log(`[HTTP ${status}] Executed in${latencyMs}ms:`, data);
}

runTool();
```

---

## Ephemeral Task Directives

Orchestrators can dynamically inject tighter guardrails for a specific execution step without altering global proxy policies:

```typescript
// Define an ephemeral constraint for this specific invocation
const ephemeralDirective = JSON.stringify({
  max_per_call: 30.00,
  condition: "payload.currency == 'USD'"
});

// Execute request with directive attached
const { status, data } = await client.executeTool(
  '/v1/action',
  { amount: 25.00, currency: 'USD' },
  { directive: ephemeralDirective }
);
```

---

## API Reference

### `GatraTokenIssuer`
- **`constructor(privateKeyBase64: string)`** — Initializes issuer with an Ed25519 private key.
- **`mintToken(trajectoryId: string, toolPattern: string, options?: TokenOptions): string`** — Signs and returns a compact Ed25519 capability token.

### `GatraClient`
- **`constructor(proxyUrl: string, capabilityToken?: string)`** — Initializes proxy client targeting a GATRA gateway.
- **`executeTool(path: string, payload: Record<string, any>, options?: ExecuteOptions)`** — Dispatch HTTP POST requests with automatically managed security headers.

---

## Resources

- **Core Repository:** [github.com/gatra-io/gatra](https://github.com/gatra-io/gatra)
- **Python SDK (`gatra-sdk`):** [pypi.org/project/gatra-sdk/](https://pypi.org/project/gatra-sdk/)

---

## License

Distributed under the [MIT License](https://github.com/gatra-io/gatra/blob/main/LICENSE).