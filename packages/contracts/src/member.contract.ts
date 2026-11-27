import { z } from "zod";
import {
  centsSchema,
  dateStringSchema,
  hexColorSchema,
  idSchema,
  positiveCentsSchema,
} from "./common";

/** Pessoas de um espaço Casa para dividir despesas (v3 fase 2 §4). */
export const createMemberSchema = z.object({
  name: z.string().trim().min(1).max(60),
  color: hexColorSchema.optional(),
});
export type CreateMemberInput = z.infer<typeof createMemberSchema>;

export const updateMemberSchema = createMemberSchema.partial();
export type UpdateMemberInput = z.infer<typeof updateMemberSchema>;

export const memberSchema = z.object({
  id: idSchema,
  name: z.string(),
  color: z.string(),
});
export type Member = z.infer<typeof memberSchema>;

/** Parte de um membro numa despesa; a soma das partes é 100. */
export const transactionSplitSchema = z.object({
  memberId: idSchema,
  sharePct: z.number().min(0).max(100),
});
export type TransactionSplit = z.infer<typeof transactionSplitSchema>;

/** GET /api/members/balances: balance > 0 = tem a receber. */
export const memberBalancesSchema = z.object({
  members: z.array(
    z.object({
      memberId: idSchema,
      name: z.string(),
      paidCents: centsSchema,
      owedCents: centsSchema,
      balanceCents: centsSchema,
    }),
  ),
  settlements: z.array(
    z.object({ fromMemberId: idSchema, toMemberId: idSchema, amountCents: centsSchema }),
  ),
});
export type MemberBalances = z.infer<typeof memberBalancesSchema>;
export type MemberBalance = MemberBalances["members"][number];
export type SuggestedSettlement = MemberBalances["settlements"][number];

/** POST /api/members/settle: transferência se as duas contas vierem, senão só o registro. */
export const settleMembersSchema = z.object({
  fromMemberId: idSchema,
  toMemberId: idSchema,
  amountCents: positiveCentsSchema,
  date: dateStringSchema.optional(),
  fromAccountId: idSchema.optional(),
  toAccountId: idSchema.optional(),
});
export type SettleMembersInput = z.infer<typeof settleMembersSchema>;
