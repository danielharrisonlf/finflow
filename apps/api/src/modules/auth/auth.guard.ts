import { CanActivate, ExecutionContext, Injectable, UnauthorizedException } from "@nestjs/common";
import { Reflector } from "@nestjs/core";
import type { Request } from "express";
import { AuthService } from "./auth.service";
import { IS_PUBLIC_KEY } from "./public.decorator";

export function extractRequestToken(req: Request): string | undefined {
  const authHeader = req.headers.authorization;
  if (typeof authHeader === "string" && authHeader.toLowerCase().startsWith("bearer ")) {
    return authHeader.slice(7).trim();
  }
  const apiToken = req.headers["x-api-token"];
  if (typeof apiToken === "string" && apiToken.trim()) {
    return apiToken.trim();
  }
  const queryToken = req.query?.token;
  if (typeof queryToken === "string" && queryToken.trim()) {
    return queryToken.trim();
  }
  return undefined;
}

@Injectable()
export class AuthGuard implements CanActivate {
  constructor(
    private readonly auth: AuthService,
    private readonly reflector: Reflector,
  ) {}

  canActivate(context: ExecutionContext): boolean {
    if (!this.auth.isRequired) return true;

    const isPublic = this.reflector.getAllAndOverride<boolean>(IS_PUBLIC_KEY, [
      context.getHandler(),
      context.getClass(),
    ]);
    if (isPublic) return true;

    const req = context.switchToHttp().getRequest<Request>();
    const token = extractRequestToken(req);
    if (!this.auth.verifyToken(token)) {
      throw new UnauthorizedException("autenticação necessária");
    }
    return true;
  }
}
