import * as crypto from 'node:crypto';
import { Buffer } from 'node:buffer';

export interface CapabilityClaims {
  traj_id: string;
  tool: string;
  exp: number;
}

export class GatraTokenIssuer {
  private privateKey: crypto.KeyObject;

  constructor(privateKeyBase64: string) {
    const rawKey = Buffer.from(privateKeyBase64.trim(), 'base64');
    const pkcs8Header = Buffer.from('302e020100300506032b657004220420', 'hex');
    this.privateKey = crypto.createPrivateKey({
      key: Buffer.concat([pkcs8Header, rawKey]),
      format: 'der',
      type: 'pkcs8',
    });
  }

  mintToken(trajectoryId: string, toolPattern: string = '*', ttlSeconds: number = 86400): string {
    const claims: CapabilityClaims = {
      traj_id: trajectoryId,
      tool: toolPattern,
      exp: Math.floor(Date.now() / 1000) + ttlSeconds,
    };

    const payloadBytes = Buffer.from(JSON.stringify(claims));
    const signature = crypto.sign(null, payloadBytes, this.privateKey);

    const signedPayload = {
      payload: claims,
      signature: signature.toString('base64'),
    };

    return Buffer.from(JSON.stringify(signedPayload)).toString('base64');
  }
}

export class GatraClient {
  private proxyUrl: string;
  private capabilityToken: string;

  constructor(proxyUrl: string = 'http://localhost:8080', capabilityToken: string = '') {
    this.proxyUrl = proxyUrl.replace(/\/$/, '');
    this.capabilityToken = capabilityToken;
  }

  async executeTool(endpoint: string, payload: Record<string, unknown>): Promise<{ status: number; data: unknown; latencyMs: number }> {
    const url = endpoint.startsWith('/') ? `${this.proxyUrl}${endpoint}` : `${this.proxyUrl}/${endpoint}`;
    const startTime = performance.now();

    try {
      const res = await fetch(url, {
        method: 'POST',
        headers: {
          'X-Capability-Token': this.capabilityToken,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(payload),
      });

      const latencyMs = parseFloat((performance.now() - startTime).toFixed(2));
      const data = await res.json();
      return { status: res.status, data, latencyMs };
    } catch (err) {
      const latencyMs = parseFloat((performance.now() - startTime).toFixed(2));
      return { status: 500, data: { error: String(err) }, latencyMs };
    }
  }
}