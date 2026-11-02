import { UnauthorizedException } from "@nestjs/common";
import { afterEach, describe, expect, it } from "vitest";
import { AuthService } from "./auth.service";

describe("AuthService", () => {
  const service = new AuthService();

  afterEach(() => {
    delete process.env.FINFLOW_PASSWORD;
    delete process.env.FINFLOW_AUTH_SECRET;
    delete process.env.FINFLOW_API_TOKEN;
  });

  it("fica em modo aberto quando FINFLOW_PASSWORD não está definida", () => {
    expect(service.isRequired).toBe(false);
    expect(service.verifyToken(undefined)).toBe(true);
    expect(service.status(undefined)).toEqual({ required: false, authenticated: true });
  });

  it("exige senha correta e emite token assinado válido quando FINFLOW_PASSWORD está definida", () => {
    process.env.FINFLOW_PASSWORD = "segredo-forte";
    expect(service.isRequired).toBe(true);
    expect(service.verifyToken(undefined)).toBe(false);

    expect(() => service.login("errada")).toThrow(UnauthorizedException);

    const { token, expiresAt } = service.login("segredo-forte");
    expect(token.split(".")).toHaveLength(3);
    expect(new Date(expiresAt).getTime()).toBeGreaterThan(Date.now());
    expect(service.verifyToken(token)).toBe(true);
    expect(service.status(token)).toEqual({ required: true, authenticated: true });
  });

  it("rejeita token adulterado ou expirado e aceita FINFLOW_API_TOKEN estático", () => {
    process.env.FINFLOW_PASSWORD = "minha-senha";
    process.env.FINFLOW_API_TOKEN = "token-cli-mcp";

    const { token } = service.login("minha-senha");
    expect(service.verifyToken(`${token}x`)).toBe(false);

    const expiredPayload = `${Date.now() - 1000}.nonce`;
    expect(service.verifyToken(`${expiredPayload}.assinatura`)).toBe(false);

    expect(service.verifyToken("token-cli-mcp")).toBe(true);
    expect(service.verifyToken("minha-senha")).toBe(true);
  });
});
