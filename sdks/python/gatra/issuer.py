import base64
import json
import time
from cryptography.hazmat.primitives.asymmetric import ed25519

class GatraTokenIssuer:
    """Mints cryptographically signed capability tokens using an Ed25519 private key."""
    
    def __init__(self, private_key_base64: str):
        raw_bytes = base64.b64decode(private_key_base64.strip())
        self.private_key = ed25519.Ed25519PrivateKey.from_private_bytes(raw_bytes)

    def mint_token(self, trajectory_id: str, tool_pattern: str = "*", ttl_seconds: int = 86400) -> str:
        claims = {
            "traj_id": trajectory_id,
            "tool": tool_pattern,
            "exp": int(time.time()) + ttl_seconds
        }
        
        payload_bytes = json.dumps(claims, separators=(',', ':')).encode('utf-8')
        signature_bytes = self.private_key.sign(payload_bytes)
        
        signed_token_payload = {
            "payload": claims,
            "signature": base64.b64encode(signature_bytes).decode('utf-8')
        }
        
        final_bytes = json.dumps(signed_token_payload, separators=(',', ':')).encode('utf-8')
        return base64.b64encode(final_bytes).decode('utf-8')