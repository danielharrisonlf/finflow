import { zodResolver } from "@hookform/resolvers/zod";
import { Lock } from "lucide-react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { ThemeToggle } from "../components/landing/ThemeToggle";
import { Button } from "../components/ui/button";
import { Card } from "../components/ui/card";
import { Field, Input } from "../components/ui/field";
import { useLogin } from "../lib/queries";

const formSchema = z.object({
  password: z.string().min(1, "informe a senha"),
});

export function LoginPage() {
  const login = useLogin();
  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: { password: "" },
  });

  const submit = form.handleSubmit(async (values) => {
    try {
      await login.mutateAsync({ password: values.password });
      form.reset();
    } catch {
      // erro é exibido via login.error
    }
  });

  return (
    <div
      className="flex min-h-dvh items-center justify-center bg-app-background px-4"
      data-testid="login-page"
    >
      <div className="fixed top-4 right-4">
        <ThemeToggle />
      </div>
      <Card className="w-full max-w-sm p-6">
        <div className="mb-6 flex flex-col items-center text-center">
          <span className="flex h-12 w-12 items-center justify-center rounded-2xl bg-emerald-500/15 text-income">
            <Lock size={22} />
          </span>
          <h1 className="mt-4 text-xl font-semibold tracking-tight text-app-text">
            Entrar no FinFlow
          </h1>
          <p className="mt-1 text-sm text-app-muted">
            Digite sua senha de acesso para abrir o painel financeiro.
          </p>
        </div>

        <form onSubmit={submit} className="flex flex-col gap-4" data-testid="form-login">
          <Field label="Senha" error={form.formState.errors.password?.message}>
            <Input
              type="password"
              placeholder="Sua senha…"
              autoFocus
              autoComplete="current-password"
              {...form.register("password")}
            />
          </Field>
          {login.error ? <p className="text-sm text-expense">{login.error.message}</p> : null}
          <Button type="submit" disabled={login.isPending} className="h-11">
            {login.isPending ? "Verificando…" : "Entrar"}
          </Button>
        </form>
      </Card>
    </div>
  );
}
