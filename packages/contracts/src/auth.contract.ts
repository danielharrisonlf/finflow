import { z } from "zod";

export const loginSchema = z.object({
  password: z.string().min(1, "informe a senha"),
});
export type LoginInput = z.infer<typeof loginSchema>;

export const loginResponseSchema = z.object({
  token: z.string(),
  expiresAt: z.string(),
});
export type LoginResponse = z.infer<typeof loginResponseSchema>;

export const authStatusSchema = z.object({
  required: z.boolean(),
  authenticated: z.boolean(),
});
export type AuthStatus = z.infer<typeof authStatusSchema>;
