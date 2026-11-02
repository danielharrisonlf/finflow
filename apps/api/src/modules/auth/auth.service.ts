import { createHash, createHmac, randomBytes, timingSafeEqual } from "node:crypto";
import type { AuthStatus, LoginResponse } from "@finflow/contracts";
import { Injectable, UnauthorizedException } from "@nestjs/common";

const SESSION_TTL_MS = 30 * 24 * 60 * 60 * 1000; // 30 dias

function sha256(value: string): Buffer {
  return createHash("sha256").update(value, "utf8").digest();
}

function safeEqual(a: string, b: string): boolean {
  return timingSafeEqual(sha256(a), sha256(b));
}

@Injectable()
export class AuthService {
  /** A autenticação fica ativa sempre que FINFLOW_PASSWORD estiver definida. */
  get isRequired(): boolean {
    return Boolean(this.expectedPassword);
  }

  private get expectedPassword(): string {
    return (process.env.FINFLOW_PASSWORD ?? "").trim();
  }

  private get signingSecret(): string {
    const custom = (process.env.FINFLOW_AUTH_SECRET ?? "").trim();
    return custom || this.expectedPassword || "finflow-local-dev-secret";
  }

  private sign(payload: string): string {
    return createHmac("sha256", this.signingSecret).update(payload, "utf8").digest("base64url");
  }

  login(password: string): LoginResponse {
    if (this.isRequired && !safeEqual(password, this.expectedPassword)) {
      throw new UnauthorizedException("senha incorreta");
    }
    const expiresAtMs = Date.now() + SESSION_TTL_MS;
    const nonce = randomBytes(12).toString("base64url");
    const payload = `${expiresAtMs}.${nonce}`;
    const signature = this.sign(payload);
    return {
      token: `${payload}.${signature}`,
      expiresAt: new Date(expiresAtMs).toISOString(),
    };
  }

  verifyToken(token: string | undefined | null): boolean {
    if (!this.isRequired) return true;
    const raw = token?.trim();
    if (!raw) return false;

    // Permite uso direto da senha ou de FINFLOW_API_TOKEN pela CLI e pelo MCP.
    if (safeEqual(raw, this.expectedPassword)) return true;
    const staticToken = (process.env.FINFLOW_API_TOKEN ?? "").trim();
    if (staticToken && safeEqual(raw, staticToken)) return true;

    const parts = raw.split(".");
    if (parts.length !== 3) return false;
    const [expiresAtStr, nonce, sig] = parts;
    if (!expiresAtStr || !nonce || !sig) return false;

    const expiresAt = Number(expiresAtStr);
    if (!Number.isFinite(expiresAt) || expiresAt <= Date.now()) return false;

    const expectedSig = this.sign(`${expiresAtStr}.${nonce}`);
    return safeEqual(sig, expectedSig);
  }

  status(token?: string | null): AuthStatus {
    return {
      required: this.isRequired,
      authenticated: this.verifyToken(token),
    };
  }
}
