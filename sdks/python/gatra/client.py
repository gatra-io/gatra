import json
import time
import urllib.request
import urllib.error
from typing import Dict, Any, Tuple

class GatraClient:
    """Helper client to execute tool calls through GATRA proxy."""
    
    def __init__(self, proxy_url: str = "http://localhost:8080", capability_token: str = ""):
        self.proxy_url = proxy_url.rstrip("/")
        self.capability_token = capability_token

    def execute_tool(self, endpoint: str, payload: Dict[str, Any]) -> Tuple[int, Dict[str, Any], float]:
        url = f"{self.proxy_url}{endpoint}" if endpoint.startswith("/") else f"{self.proxy_url}/{endpoint}"
        data = json.dumps(payload).encode("utf-8")
        headers = {
            "X-Capability-Token": self.capability_token,
            "Content-Type": "application/json"
        }

        req = urllib.request.Request(url, data=data, headers=headers, method="POST")
        start_time = time.perf_counter()

        try:
            with urllib.request.urlopen(req) as res:
                latency_ms = (time.perf_counter() - start_time) * 1000
                resp_data = json.loads(res.read().decode("utf-8"))
                return res.status, resp_data, latency_ms
        except urllib.error.HTTPError as err:
            latency_ms = (time.perf_counter() - start_time) * 1000
            try:
                resp_data = json.loads(err.read().decode("utf-8"))
            except Exception:
                resp_data = {"error": err.reason}
            return err.code, resp_data, latency_ms