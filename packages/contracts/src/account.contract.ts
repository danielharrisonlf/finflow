import { z } from "zod";
import {
  centsSchema,
  dateStringSchema,
  hexColorSchema,
  idSchema,
  positiveCentsSchema,
} from "./common";

export const accountTypeSchema = z.enum(["CHECKING", "SAVINGS", "WALLET", "SALARY", "INVESTMENT"]);
export type AccountType = z.infer<typeof accountTypeSchema>;

export const createAccountSchema = z.object({
  name: z.string().min(1).max(60),
  type: accountTypeSchema,
  initialBalanceCents: centsSchema.default(0),
  color: hexColorSchema.optional(),
});
export type CreateAccountInput = z.infer<typeof createAccountSchema>;

export const updateAccountSchema = createAccountSchema.partial().extend({
  archived: z.boolean().optional(),
});
export type UpdateAccountInput = z.infer<typeof updateAccountSchema>;

export const accountSchema = z.object({
  id: idSchema,
  name: z.string(),
  type: accountTypeSchema,
  initialBalanceCents: centsSchema,
  /** Saldo atual = saldo inicial + transações confirmadas. */
  balanceCents: centsSchema,
  color: z.string().nullable(),
  archived: z.boolean(),
});
export type Account = z.infer<typeof accountSchema>;

export const transferSchema = z
  .object({
    fromAccountId: idSchema,
    toAccountId: idSchema,
    amountCents: positiveCentsSchema,
    date: dateStringSchema,
    description: z.string().max(120).optional(),
  })
  .refine((v) => v.fromAccountId !== v.toAccountId, {
    message: "conta de origem e destino devem ser diferentes",
    path: ["toAccountId"],
  });
export type TransferInput = z.infer<typeof transferSchema>;
