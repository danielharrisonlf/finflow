import {
  type AuthStatus,
  type LoginInput,
  type LoginResponse,
  loginSchema,
} from "@finflow/contracts";
import { Body, Controller, Get, HttpCode, Post, Req } from "@nestjs/common";
import { ApiOperation, ApiTags } from "@nestjs/swagger";
import type { Request } from "express";
import { ZodValidationPipe } from "../../common/zod-validation.pipe";
import { extractRequestToken } from "./auth.guard";
import { AuthService } from "./auth.service";
import { Public } from "./public.decorator";

@ApiTags("auth")
@Controller("auth")
export class AuthController {
  constructor(private readonly service: AuthService) {}

  @Public()
  @Get("status")
  @ApiOperation({ summary: "Indica se a API exige login e se o token atual é válido" })
  status(@Req() req: Request): AuthStatus {
    return this.service.status(extractRequestToken(req));
  }

  @Public()
  @Post("login")
  @HttpCode(200)
  @ApiOperation({ summary: "Autentica com a senha e retorna um token de sessão assinado" })
  login(@Body(new ZodValidationPipe(loginSchema)) input: LoginInput): LoginResponse {
    return this.service.login(input.password);
  }
}
