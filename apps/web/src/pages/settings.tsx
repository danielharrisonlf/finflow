import type { Settings } from "@finflow/contracts";
import { zodResolver } from "@hookform/resolvers/zod";
import { Check, HardDrive, Settings as SettingsIcon } from "lucide-react";
import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { ApiAiSection } from "../components/api-ai-section";
import { BrowserDataSection } from "../components/browser/browser-data-section";
import { Button } from "../components/ui/button";
import { Card } from "../components/ui/card";
import { Empty, LoadingRows } from "../components/ui/empty";
import { Field, Input } from "../components/ui/field";
import { BAND_META } from "../lib/bands";
import { HOSTED_APP_URL } from "../lib/browser-runtime";
import { centsToInput, parseBRLInput, parseSignedBRLInput } from "../lib/format";
import { useSettings, useUpdateSettings } from "../lib/queries";
import { isBrowserRuntime } from "../runtime/client";

const intIn = (min: number, max: number) => (v: string) => {
  const n = Number(v);
  return v.trim() !== "" && Number.isInteger(n) && n >= min && n <= max;
};
const pctIn = (v: string) => {
  const n = Number(v.replace(",", "."));
  return v.trim() !== "" && Number.isFinite(n) && n >= 0 && n <= 100;
};
const toNumber = (v: string): number => Number(v.replace(",", "."));
const signedMoney = z
  .string()
  .refine((v) => parseSignedBRLInput(v) !== null, "valor inválido — use 1.000,00 ou -500,00");

const settingsFormSchema = z
  .object({
    monthlyLimit: z
      .string()
      .refine((v) => v.trim() === "" || parseBRLInput(v) !== null, "valor inválido — use 4.000,00"),
    reserveMonths: z.string().refine(intIn(1, 36), "entre 1 e 36 meses"),
    hasDependents: z.boolean(),
    refillRate: z.string().refine(pctIn, "entre 0 e 100"),
    darkGreen: signedMoney,
    lightGreen: signedMoney,
    yellow: signedMoney,
    lightRed: signedMoney,
    planEnabled: z.boolean(),
    fixedPct: z.string(),
    cuttablePct: z.string(),
    variablePct: z.string(),
  })
  .superRefine((v, ctx) => {
    const bands = [v.darkGreen, v.lightGreen, v.yellow, v.lightRed].map(parseSignedBRLInput);
    if (bands.every((b) => b !== null)) {
      const [dg, lg, y, lr] = bands as number[];
      if (!(dg! > lg! && lg! > y! && y! > lr!)) {
        ctx.addIssue({
          code: "custom",
          message: "cada faixa precisa começar abaixo da anterior",
          path: ["lightRed"],
        });
      }
    }
    if (v.planEnabled) {
      const pcts = [v.fixedPct, v.cuttablePct, v.variablePct];
      if (!pcts.every(pctIn)) {
        ctx.addIssue({
          code: "custom",
          message: "use valores entre 0 e 100",
          path: ["variablePct"],
        });
      } else if (Math.abs(pcts.map(toNumber).reduce((a, b) => a + b, 0) - 100) > 0.01) {
        ctx.addIssue({ code: "custom", message: "a soma precisa dar 100%", path: ["variablePct"] });
      }
    }
  });
type SettingsForm = z.infer<typeof settingsFormSchema>;

function toForm(s: Settings): SettingsForm {
  return {
    monthlyLimit: s.monthlyLimitCents === null ? "" : centsToInput(s.monthlyLimitCents),
    reserveMonths: String(s.reserveMonths),
    hasDependents: s.hasDependents,
    refillRate: String(s.reserveRefillRatePct),
    darkGreen: centsToInput(s.balanceBands.darkGreen),
    lightGreen: centsToInput(s.balanceBands.lightGreen),
    yellow: centsToInput(s.balanceBands.yellow),
    lightRed: centsToInput(s.balanceBands.lightRed),
    planEnabled: s.spendPlan !== null,
    fixedPct: String(s.spendPlan?.fixedPct ?? 50),
    cuttablePct: String(s.spendPlan?.cuttablePct ?? 20),
    variablePct: String(s.spendPlan?.variablePct ?? 30),
  };
}

function Section({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children: React.ReactNode;
}) {
  return (
    <Card className="grid gap-4 p-5 md:grid-cols-[minmax(0,2fr)_minmax(0,3fr)] md:gap-8">
      <div>
        <h2 className="text-sm font-semibold text-app-text">{title}</h2>
        <p className="mt-1 text-sm text-app-muted">{description}</p>
      </div>
      <div className="flex flex-col gap-4">{children}</div>
    </Card>
  );
}

export function SettingsPage() {
  const settings = useSettings();
  const update = useUpdateSettings();
  const form = useForm<SettingsForm>({ resolver: zodResolver(settingsFormSchema) });
  const errors = form.formState.errors;
  const planEnabled = form.watch("planEnabled");
  const hasDependents = form.watch("hasDependents");
  const planSum = form
    .watch(["fixedPct", "cuttablePct", "variablePct"])
    .map((v) => toNumber(v ?? "0") || 0)
    .reduce((a, b) => a + b, 0);

  useEffect(() => {
    if (settings.data) form.reset(toForm(settings.data));
  }, [settings.data, form]);

  const submit = form.handleSubmit(async (v) => {
    update.reset();
    await update.mutateAsync({
      monthlyLimitCents: v.monthlyLimit.trim() === "" ? null : parseBRLInput(v.monthlyLimit),
      reserveMonths: Number(v.reserveMonths),
      hasDependents: v.hasDependents,
      reserveRefillRatePct: toNumber(v.refillRate),
      balanceBands: {
        darkGreen: parseSignedBRLInput(v.darkGreen) ?? 0,
        lightGreen: parseSignedBRLInput(v.lightGreen) ?? 0,
        yellow: parseSignedBRLInput(v.yellow) ?? 0,
        lightRed: parseSignedBRLInput(v.lightRed) ?? 0,
      },
      spendPlan: v.planEnabled
        ? {
            fixedPct: toNumber(v.fixedPct),
            cuttablePct: toNumber(v.cuttablePct),
            variablePct: toNumber(v.variablePct),
          }
        : null,
    });
  });

  return (
    <div className="space-y-4">
      <header>
        <h1 className="text-xl font-semibold tracking-tight">Configurações</h1>
        <p className="text-sm text-app-muted">Limites, reserva e faixas usadas nas análises</p>
      </header>

      {settings.isLoading ? (
        <Card>
          <LoadingRows rows={5} />
        </Card>
      ) : !settings.data ? (
        <Card>
          <Empty
            icon={SettingsIcon}
            title="Não foi possível carregar as configurações"
            hint={settings.error?.message ?? "Tente novamente em instantes."}
          />
        </Card>
      ) : (
        <form onSubmit={submit} className="space-y-4" data-testid="form-settings" noValidate>
          <Section
            title="Limite mensal de gastos"
            description="Quanto você quer gastar por mês, somando tudo. Usado no ritmo do mês, no valor diário disponível e na projeção dos gastos do dia a dia."
          >
            <Field
              label="Limite mensal (deixe vazio para não usar)"
              error={errors.monthlyLimit?.message}
            >
              <Input
                inputMode="decimal"
                placeholder="4.000,00"
                {...form.register("monthlyLimit")}
              />
            </Field>
          </Section>

          <Section
            title="Reserva de emergência"
            description="A meta sugerida é o seu custo de vida médio multiplicado pelos meses abaixo. Com dependentes, usamos 12 meses."
          >
            <div className="grid grid-cols-2 gap-3">
              <Field label="Meses de custo de vida" error={errors.reserveMonths?.message}>
                <Input inputMode="numeric" {...form.register("reserveMonths")} />
              </Field>
              <Field label="Reposição após usar (%)" error={errors.refillRate?.message}>
                <Input inputMode="decimal" {...form.register("refillRate")} />
              </Field>
            </div>
            <label className="flex min-h-11 cursor-pointer items-center gap-2.5 rounded-(--radius-control) border border-app-border/80 px-3 text-sm text-app-text">
              <input
                type="checkbox"
                className="h-4 w-4 accent-emerald-500"
                {...form.register("hasDependents")}
              />
              Tenho dependentes (meta de 12 meses)
            </label>
            {hasDependents ? (
              <p className="text-xs text-app-muted">
                Com dependentes, a meta sugerida usa 12 meses no lugar do número acima.
              </p>
            ) : null}
            <p className="text-xs text-app-muted">
              Se precisar usar a reserva, reponha o valor com um acréscimo (padrão +10%) para
              recompor o que a inflação comeu.
            </p>
          </Section>

          <Section
            title="Faixas de saldo"
            description="Cores do saldo projetado dia a dia. Cada valor é o piso da faixa: abaixo do último, o saldo fica vermelho."
          >
            <div className="grid grid-cols-2 gap-3">
              {(
                [
                  ["darkGreen", "DARK_GREEN"],
                  ["lightGreen", "LIGHT_GREEN"],
                  ["yellow", "YELLOW"],
                  ["lightRed", "LIGHT_RED"],
                ] as const
              ).map(([key, band]) => (
                <Field
                  key={key}
                  label={`${BAND_META[band].label} a partir de`}
                  error={errors[key]?.message}
                >
                  <div className="relative">
                    <span
                      className="absolute top-1/2 left-3 h-3 w-3 -translate-y-1/2 rounded-sm"
                      style={{ backgroundColor: BAND_META[band].color }}
                      aria-hidden
                    />
                    <Input inputMode="decimal" className="pl-8" {...form.register(key)} />
                  </div>
                </Field>
              ))}
            </div>
          </Section>

          <Section
            title="Seu plano de gastos"
            description="Como você quer dividir as despesas entre Fixo, Cortável e Variável. A visão geral compara o mês real com este plano."
          >
            <label className="flex min-h-11 cursor-pointer items-center gap-2.5 rounded-(--radius-control) border border-app-border/80 px-3 text-sm text-app-text">
              <input
                type="checkbox"
                className="h-4 w-4 accent-emerald-500"
                {...form.register("planEnabled")}
              />
              Usar um plano de distribuição
            </label>
            {planEnabled ? (
              <>
                <div className="grid grid-cols-3 gap-3">
                  <Field label="Fixo (%)">
                    <Input inputMode="decimal" {...form.register("fixedPct")} />
                  </Field>
                  <Field label="Cortável (%)">
                    <Input inputMode="decimal" {...form.register("cuttablePct")} />
                  </Field>
                  <Field label="Variável (%)">
                    <Input inputMode="decimal" {...form.register("variablePct")} />
                  </Field>
                </div>
                <p
                  className={`tnum text-xs ${Math.abs(planSum - 100) > 0.01 ? "text-expense" : "text-app-muted"}`}
                  aria-live="polite"
                >
                  Soma: {Math.round(planSum * 10) / 10}%
                  {errors.variablePct?.message ? ` · ${errors.variablePct.message}` : ""}
                </p>
              </>
            ) : null}
          </Section>

          {/* modo navegador: os dados ficam no navegador e têm a seção própria, fora do formulário */}
          {isBrowserRuntime ? null : (
            <Card className="flex gap-3 p-5" data-testid="local-data">
              <HardDrive size={20} aria-hidden className="mt-0.5 shrink-0 text-app-muted" />
              <div>
                <h2 className="text-sm font-semibold text-app-text">Dados locais</h2>
                <p className="mt-1 text-sm text-app-muted">
                  Seus dados ficam num arquivo SQLite neste computador, gravado pelo próprio
                  FinFlow. Nada é enviado para servidores externos. Para fazer backup, copie esse
                  arquivo ou exporte a planilha anual em Planilhas.
                </p>
                <p className="mt-2 text-sm text-app-muted" data-testid="use-in-browser">
                  Usar no navegador: o FinFlow também abre sem instalar, no celular ou em outro
                  computador, em{" "}
                  <a
                    href={HOSTED_APP_URL}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="font-medium text-income underline-offset-4 hover:underline"
                  >
                    finflow-landing-umber.vercel.app/app
                  </a>
                  . Lá os dados ficam no navegador; para levá-los, use o backup (.db) deste arquivo.
                </p>
              </div>
            </Card>
          )}

          <div className="flex flex-wrap items-center gap-3">
            <Button type="submit" disabled={update.isPending} className="h-12 px-6">
              {update.isPending ? "Salvando…" : "Salvar configurações"}
            </Button>
            <p role="status" className="text-sm">
              {update.isSuccess ? (
                <span className="flex items-center gap-1.5 text-income">
                  <Check size={15} aria-hidden /> Configurações salvas
                </span>
              ) : update.error ? (
                <span className="text-expense">{update.error.message}</span>
              ) : null}
            </p>
          </div>
        </form>
      )}

      {/* fora do formulário: não interfere no envio das configurações */}
      {isBrowserRuntime ? <BrowserDataSection /> : null}
      <ApiAiSection />
    </div>
  );
}
