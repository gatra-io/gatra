const GATRA_PROXY_URL = "http://localhost:8080/v1/refund";

async function executeAgentToolCall(token, amount, currency, stepDescription) {
  const payload = { amount, currency };

  console.log(`\n🤖 [Agent Decision] ${stepDescription}`);
  console.log(`   Payload -> Amount: $${amount.toFixed(2)} ${currency}`);

  const startTime = performance.now();

  try {
    const res = await fetch(GATRA_PROXY_URL, {
      method: "POST",
      headers: {
        "X-Capability-Token": token,
        "Content-Type": "application/json",
      },
      body: JSON.stringify(payload),
    });

    const latencyMs = (performance.now() - startTime).toFixed(2);
    const data = await res.json();

    if (res.ok) {
      console.log(`   🟢 [GATRA APPROVED] HTTP ${res.status} (${latencyMs}ms)`);
      console.log(`      Downstream Response:`, data);
    } else {
      console.log(`   🔴 [GATRA INTERCEPTED & BLOCKED] HTTP ${res.status} (${latencyMs}ms)`);
      console.log(`      Security Gate:`, data);
    }
  } catch (err) {
    console.error(`   ❌ Request failed:`, err);
  }
}

async function main() {
  const token = process.argv[2] || process.env.GATRA_TOKEN;

  if (!token) {
    console.error("❌ Missing capability token!");
    console.error("   Usage: node agent_simulation.js <YOUR_ACTIVE_TOKEN>");
    process.exit(1);
  }

  console.log("==================================================");
  console.log("🤖 Simulating Live LLM Agent Execution Session (Node.js)");
  console.log("==================================================");
  console.log(`🎟️ Active Capability Token:\n   ${token.substring(0, 45)}...`);

  // Scenario 1: Approved valid tool invocation
  await executeAgentToolCall(token, 25.0, "USD", "Refunding $25.00 USD for customer ticket #1042");

  // Scenario 2: CEL Policy Rejection (Forbidden currency)
  await executeAgentToolCall(token, 15.0, "EUR", "Refunding €15.00 EUR for European user ticket #1043");

  // Scenario 3: Single-call limit breach ($60 > $50 max per call)
  await executeAgentToolCall(token, 60.0, "USD", "Attempting high-value refund of $60.00 USD");

  // Scenario 4: Cumulative trajectory boundary breach ($40 x 5 > $200 limit)
  console.log("\n--------------------------------------------------");
  console.log("🔄 Testing Stateful Cumulative Trajectory Boundary");
  console.log("--------------------------------------------------");

  for (let i = 1; i <= 5; i++) {
    await executeAgentToolCall(token, 40.0, "USD", `Cumulative Refund #${i} ($40.00 USD)`);
  }
}

main();