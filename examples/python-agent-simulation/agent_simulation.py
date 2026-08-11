import sys
import os
import time
import json
import urllib.request
import urllib.error

GATRA_PROXY_URL = "http://localhost:8080/v1/action"

def execute_agent_tool_call(token: str, amount: float, currency: str, step_description: str):
    payload = {"amount": amount, "currency": currency}
    data = json.dumps(payload).encode("utf-8")

    print(f"\n🤖 [Agent Decision] {step_description}")
    print(f"   Payload -> Amount: ${amount:.2f} {currency}")

    headers = {
        "X-Capability-Token": token,
        "Content-Type": "application/json",
    }

    req = urllib.request.Request(GATRA_PROXY_URL, data=data, headers=headers, method="POST")
    start_time = time.perf_counter()

    try:
        with urllib.request.urlopen(req) as res:
            latency_ms = (time.perf_counter() - start_time) * 1000
            resp_data = json.loads(res.read().decode("utf-8"))
            print(f"   🟢 [GATRA APPROVED] HTTP {res.status} ({latency_ms:.2f}ms)")
            print(f"      Downstream Response: {resp_data}")
    except urllib.error.HTTPError as err:
        latency_ms = (time.perf_counter() - start_time) * 1000
        try:
            resp_data = json.loads(err.read().decode("utf-8"))
        except Exception:
            resp_data = err.reason
        print(f"   🔴 [GATRA INTERCEPTED & BLOCKED] HTTP {err.code} ({latency_ms:.2f}ms)")
        print(f"      Security Gate: {resp_data}")
    except Exception as err:
        print(f"   ❌ Request failed: {err}")

def main():
    token = sys.argv[1] if len(sys.argv) > 1 else os.environ.get("GATRA_TOKEN")

    if not token:
        print("❌ Missing capability token!")
        print("   Usage: python agent_simulation.py <YOUR_ACTIVE_TOKEN>")
        sys.exit(1)

    print("==================================================")
    print("🤖 Simulating Live LLM Agent Execution Session (Python)")
    print("==================================================")
    print(f"🎟️ Active Capability Token:\n   {token[:45]}...")

    # Scenario 1: Approved valid tool invocation
    execute_agent_tool_call(token, 25.0, "USD", "Executing valid tool action $25.00 USD")

    # Scenario 2: CEL Policy Rejection (Forbidden currency)
    execute_agent_tool_call(token, 15.0, "EUR", "Attempting forbidden currency €15.00 EUR")

    # Scenario 3: Single-call limit breach ($60 > $50 max per call)
    execute_agent_tool_call(token, 60.0, "USD", "Attempting limit breach $60.00 USD (> $50 max)")

    # Scenario 4: Cumulative trajectory boundary breach ($40 x 5 > $200 limit)
    print("\n--------------------------------------------------")
    print("🔄 Testing Stateful Cumulative Trajectory Boundary")
    print("--------------------------------------------------")

    for i in range(1, 6):
        execute_agent_tool_call(token, 40.0, "USD", f"Cumulative Action #{i} ($40.00 USD)")

if __name__ == "__main__":
    main()