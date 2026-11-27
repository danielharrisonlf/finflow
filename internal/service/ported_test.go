package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"finflow/internal/db"
	"finflow/internal/domain"
	"finflow/internal/store"
	"finflow/internal/testutil"
)

// Casos portados de parser.test.ts, telegram.service.test.ts e auth.service.test.ts,
// mais regressões das correções de validação, fuso e alertas.

func TestParseTelegramMessageCases(t *testing.T) {
	cases := []struct {
		in   string
		want ParsedTelegramMessage
	}{
		{"/saldo", ParsedTelegramMessage{Kind: "balance"}},
		{"/saldo@finflow_bot", ParsedTelegramMessage{Kind: "balance"}},
		{"/resumo", ParsedTelegramMessage{Kind: "summary"}},
		{"/extrato", ParsedTelegramMessage{Kind: "statement"}},
		{"/ajuda", ParsedTelegramMessage{Kind: "help"}},
		{"/start", ParsedTelegramMessage{Kind: "help"}},
		{"45,90 almoço no shopping", ParsedTelegramMessage{Kind: "expense", AmountCents: 4590, Description: "almoço no shopping"}},
		{"1.250,00 passagem aérea #viagem", ParsedTelegramMessage{Kind: "expense", AmountCents: 125000, Description: "passagem aérea", CategoryName: "viagem"}},
		{"50 uber", ParsedTelegramMessage{Kind: "expense", AmountCents: 5000, Description: "uber"}},
		{"30 pastel #comida_de_rua", ParsedTelegramMessage{Kind: "expense", AmountCents: 3000, Description: "pastel", CategoryName: "comida de rua"}},
		// NBSP (U+00A0) entre valor e descrição: o \s do JS aceita, o do RE2 não.
		{"45,90\xc2\xa0almoço", ParsedTelegramMessage{Kind: "expense", AmountCents: 4590, Description: "almoço"}},
		// THIN SPACE (U+2009) e BOM (U+FEFF) no início.
		{"\xef\xbb\xbf12,30\xe2\x80\x89café", ParsedTelegramMessage{Kind: "expense", AmountCents: 1230, Description: "café"}},
		{"/naoexiste", ParsedTelegramMessage{Kind: "unknown", Raw: "/naoexiste"}},
		{"comprei um lanche", ParsedTelegramMessage{Kind: "unknown", Raw: "comprei um lanche"}},
		{"0,00 nada", ParsedTelegramMessage{Kind: "unknown", Raw: "0,00 nada"}},
	}
	for _, tc := range cases {
		if got := ParseTelegramMessage(tc.in); got != tc.want {
			t.Errorf("ParseTelegramMessage(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
	}
}

func TestAppErrorIncludesIssues(t *testing.T) {
	cases := []struct {
		err  *AppError
		want string
	}{
		{NotFound("conta não encontrada"), "conta não encontrada"},
		{ValidationFailed([]domain.ValidationIssue{
			{Path: "date", Message: "data deve estar no formato YYYY-MM-DD"},
			{Path: "amountCents", Message: "valor deve ser positivo"},
		}), "dados inválidos (date: data deve estar no formato YYYY-MM-DD; amountCents: valor deve ser positivo)"},
		{ValidationFailed([]domain.ValidationIssue{{Message: "sem caminho"}}), "dados inválidos (sem caminho)"},
	}
	for _, tc := range cases {
		if got := tc.err.Error(); got != tc.want {
			t.Errorf("Error() = %q, want %q", got, tc.want)
		}
	}
}

func TestAuthServiceCases(t *testing.T) {
	isoMillis := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$`)
	clearEnv := func(t *testing.T) {
		t.Setenv("FINFLOW_PASSWORD", "")
		t.Setenv("FINFLOW_AUTH_SECRET", "")
		t.Setenv("FINFLOW_API_TOKEN", "")
	}

	t.Run("modo aberto sem FINFLOW_PASSWORD", func(t *testing.T) {
		clearEnv(t)
		auth := NewAuthService()
		if got := auth.Status(""); got != (domain.AuthStatus{Required: false, Authenticated: true}) {
			t.Fatalf("Status = %+v", got)
		}
		// zod min(1): só a string vazia é rejeitada; espaços são uma senha.
		if _, err := auth.Login("   "); err != nil {
			t.Fatalf("Login(espaços) em modo aberto: %v", err)
		}
		var appErr *AppError
		if _, err := auth.Login(""); !errors.As(err, &appErr) || appErr.Status != http.StatusBadRequest {
			t.Fatalf("Login(\"\") = %v, want 400", err)
		}
	})

	t.Run("senha exigida emite token assinado", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("FINFLOW_PASSWORD", "segredo-forte")
		auth := NewAuthService()
		if got := auth.Status(""); got != (domain.AuthStatus{Required: true, Authenticated: false}) {
			t.Fatalf("Status sem token = %+v", got)
		}
		var appErr *AppError
		if _, err := auth.Login("errada"); !errors.As(err, &appErr) || appErr.Status != http.StatusUnauthorized {
			t.Fatalf("Login(errada) = %v, want 401", err)
		}
		if _, err := auth.Login("   "); !errors.As(err, &appErr) || appErr.Status != http.StatusUnauthorized {
			t.Fatalf("Login(espaços) = %v, want 401", err)
		}
		res, err := auth.Login("segredo-forte")
		if err != nil {
			t.Fatal(err)
		}
		if n := len(strings.Split(res.Token, ".")); n != 3 {
			t.Fatalf("token com %d partes", n)
		}
		if !isoMillis.MatchString(res.ExpiresAt) {
			t.Fatalf("expiresAt %q não está no formato toISOString()", res.ExpiresAt)
		}
		exp, err := time.Parse(time.RFC3339, res.ExpiresAt)
		if err != nil || !exp.After(time.Now()) {
			t.Fatalf("expiresAt %q deveria estar no futuro (%v)", res.ExpiresAt, err)
		}
		if got := auth.Status(res.Token); got != (domain.AuthStatus{Required: true, Authenticated: true}) {
			t.Fatalf("Status com token = %+v", got)
		}
	})

	t.Run("rejeita token adulterado, expirado ou malformado", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("FINFLOW_PASSWORD", "minha-senha")
		t.Setenv("FINFLOW_API_TOKEN", "token-cli-mcp")
		auth := NewAuthService()
		res, err := auth.Login("minha-senha")
		if err != nil {
			t.Fatal(err)
		}
		parts := strings.Split(res.Token, ".")
		expired := time.Now().Add(-time.Second).UnixMilli()
		cases := []struct {
			name  string
			token string
			want  bool
		}{
			{"token válido", res.Token, true},
			{"adulterado (token+x)", res.Token + "x", false},
			{"expirado", strings.Join([]string{itoa(expired), "nonce", "assinatura"}, "."), false},
			{"expirado com assinatura reaproveitada", strings.Join([]string{itoa(expired), parts[1], parts[2]}, "."), false},
			{"duas partes", parts[0] + "." + parts[1], false},
			{"quatro partes", res.Token + ".extra", false},
			{"parte vazia", parts[0] + ".." + parts[2], false},
			{"token estático", "token-cli-mcp", true},
			{"senha direta", "minha-senha", true},
		}
		for _, tc := range cases {
			if got := auth.VerifyToken(tc.token); got != tc.want {
				t.Errorf("%s: VerifyToken = %v, want %v", tc.name, got, tc.want)
			}
		}
	})
}

func itoa(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func assertValidation(t *testing.T, err error, path string) {
	t.Helper()
	var appErr *AppError
	if !errors.As(err, &appErr) || appErr.Status != http.StatusBadRequest {
		t.Fatalf("erro = %v, want 400", err)
	}
	for _, is := range appErr.Issues {
		if is.Path == path {
			return
		}
	}
	t.Fatalf("issues = %v, want path %q", appErr.Issues, path)
}

// O serviço valida antes de tocar no banco: CLI e MCP locais não passam pela camada HTTP.
func TestServiceValidatesQueriesWithoutHTTP(t *testing.T) {
	ctx := context.Background()
	svcs := &Services{}
	m13 := 13
	y1999 := 1999
	foo := domain.TransactionType("foo")
	cases := []struct {
		name string
		call func() error
		path string
	}{
		{"ListBudgets mês 13", func() error { _, err := svcs.ListBudgets(ctx, domain.PeriodQuery{Month: &m13}); return err }, "month"},
		{"DashboardSummary ano 1999", func() error {
			_, err := svcs.DashboardSummary(ctx, domain.PeriodQuery{Year: &y1999})
			return err
		}, "year"},
		{"DashboardCashflow mês 13", func() error { _, err := svcs.DashboardCashflow(ctx, domain.PeriodQuery{Month: &m13}); return err }, "month"},
		{"DashboardCategories mês 13", func() error {
			_, err := svcs.DashboardCategories(ctx, domain.PeriodQuery{Month: &m13})
			return err
		}, "month"},
		{"ListTransactions tipo foo", func() error {
			_, err := svcs.ListTransactions(ctx, domain.ListTransactionsQuery{Type: &foo})
			return err
		}, "type"},
		{"ListTransactions pageSize 500", func() error {
			_, err := svcs.ListTransactions(ctx, domain.ListTransactionsQuery{PageSize: 500})
			return err
		}, "pageSize"},
		{"ExportTransactions mês 13", func() error {
			_, _, _, err := svcs.ExportTransactions(ctx, "csv", domain.ListTransactionsQuery{Month: &m13})
			return err
		}, "month"},
		{"AddCardExpense com 0 parcelas explícito", func() error {
			_, err := svcs.AddCardExpense(ctx, "11111111-1111-4111-8111-111111111111", domain.CreateCardExpenseInput{
				Description: "tv", TotalCents: 1000, Date: "2026-10-30", Installments: 0,
			})
			return err
		}, "installments"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { assertValidation(t, tc.call(), tc.path) })
	}
}

// ───────────────────────── Telegram com banco real ─────────────────────────

type sentMessage struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

// fakeTelegram sobe um servidor que imita a Bot API e registra cada sendMessage.
func fakeTelegram(t *testing.T, linkedChat string) <-chan sentMessage {
	t.Helper()
	ch := make(chan sentMessage, 32)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var msg sentMessage
		_ = json.Unmarshal(body, &msg)
		ch <- msg
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	t.Setenv("TELEGRAM_CHAT_ID", linkedChat)
	t.Setenv("TELEGRAM_API_BASE", srv.URL)
	return ch
}

func drain(ch <-chan sentMessage) []sentMessage {
	var out []sentMessage
	for {
		select {
		case m := <-ch:
			out = append(out, m)
		default:
			return out
		}
	}
}

func waitMessage(t *testing.T, ch <-chan sentMessage, timeout time.Duration) (sentMessage, bool) {
	t.Helper()
	select {
	case m := <-ch:
		return m, true
	case <-time.After(timeout):
		return sentMessage{}, false
	}
}

func tgUpdate(text string, chatID int64) domain.TelegramUpdate {
	return domain.TelegramUpdate{UpdateID: 1, Message: &domain.TelegramMessage{MessageID: 1, Text: text, Chat: domain.TelegramChat{ID: chatID}}}
}

func TestTelegramServiceCases(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		ctx := context.Background()
		sent := fakeTelegram(t, "42")
		svcs := New(store.New(database), nil)

		acc, err := svcs.CreateAccount(ctx, domain.CreateAccountInput{Name: "Nubank", Type: domain.AccountChecking, InitialBalanceCents: 123456})
		if err != nil {
			t.Fatal(err)
		}
		cat, err := svcs.CreateCategory(ctx, domain.CreateCategoryInput{Name: "alimentacao", Kind: domain.CategoryExpense, Icon: "utensils", Color: "#f43f5e"})
		if err != nil {
			t.Fatal(err)
		}
		listTx := func() []domain.Transaction {
			t.Helper()
			page, err := svcs.ListTransactions(ctx, domain.ListTransactionsQuery{})
			if err != nil {
				t.Fatal(err)
			}
			return page.Items
		}

		t.Run("ignora chat não vinculado: sem envio e sem lançamento", func(t *testing.T) {
			if err := svcs.Telegram.HandleUpdate(ctx, tgUpdate("45,90 almoço", 999)); err != nil {
				t.Fatal(err)
			}
			if msgs := drain(sent); len(msgs) != 0 {
				t.Fatalf("mensagens enviadas = %+v", msgs)
			}
			if items := listTx(); len(items) != 0 {
				t.Fatalf("lançamentos = %+v", items)
			}
		})

		t.Run("/saldo responde com contas, R$ e chat 42", func(t *testing.T) {
			if err := svcs.Telegram.HandleUpdate(ctx, tgUpdate("/saldo", 42)); err != nil {
				t.Fatal(err)
			}
			msgs := drain(sent)
			if len(msgs) != 1 {
				t.Fatalf("mensagens = %+v", msgs)
			}
			if !strings.Contains(msgs[0].Text, "Nubank") || !strings.Contains(msgs[0].Text, "R$") || msgs[0].ChatID != "42" {
				t.Fatalf("resposta = %+v", msgs[0])
			}
		})

		t.Run("escapa HTML de texto do usuário (parse_mode=HTML)", func(t *testing.T) {
			if err := svcs.Telegram.HandleUpdate(ctx, tgUpdate("12,00 pão <b>&</b> café", 42)); err != nil {
				t.Fatal(err)
			}
			msgs := drain(sent)
			if len(msgs) != 1 {
				t.Fatalf("mensagens = %+v", msgs)
			}
			if !strings.Contains(msgs[0].Text, "pão &lt;b&gt;&amp;&lt;/b&gt; café") {
				t.Fatalf("descrição não escapada: %q", msgs[0].Text)
			}
			for _, tx := range listTx() {
				if err := svcs.DeleteTransaction(ctx, tx.ID, ""); err != nil {
					t.Fatal(err)
				}
			}
		})

		t.Run("fallback e texto só de espaços pedem /ajuda", func(t *testing.T) {
			for _, text := range []string{"bom dia", "   "} {
				if err := svcs.Telegram.HandleUpdate(ctx, tgUpdate(text, 42)); err != nil {
					t.Fatal(err)
				}
				msgs := drain(sent)
				if len(msgs) != 1 || !strings.Contains(msgs[0].Text, "/ajuda") {
					t.Fatalf("%q -> %+v", text, msgs)
				}
			}
			if err := svcs.Telegram.HandleUpdate(ctx, tgUpdate("", 42)); err != nil {
				t.Fatal(err)
			}
			if msgs := drain(sent); len(msgs) != 0 {
				t.Fatalf("texto vazio não deveria responder: %+v", msgs)
			}
		})

		t.Run("despesa rápida resolve categoria EXPENSE pela hashtag", func(t *testing.T) {
			if err := svcs.Telegram.HandleUpdate(ctx, tgUpdate("45,90 almoço #alimentacao", 42)); err != nil {
				t.Fatal(err)
			}
			msgs := drain(sent)
			if len(msgs) != 1 || !strings.Contains(msgs[0].Text, "✅") {
				t.Fatalf("resposta = %+v", msgs)
			}
			items := listTx()
			if len(items) != 1 {
				t.Fatalf("lançamentos = %+v", items)
			}
			got := items[0]
			if got.Type != domain.TxExpense || got.AmountCents != -4590 || got.Description != "almoço" ||
				got.AccountID == nil || *got.AccountID != acc.ID || got.CategoryID == nil || *got.CategoryID != cat.ID {
				t.Fatalf("lançamento = %+v", got)
			}
		})

		t.Run("descrição longa é truncada em 120 caracteres e ainda responde", func(t *testing.T) {
			long := strings.Repeat("á", 150)
			if err := svcs.Telegram.HandleUpdate(ctx, tgUpdate("10 "+long, 42)); err != nil {
				t.Fatalf("webhook não pode falhar por entrada do usuário: %v", err)
			}
			msgs := drain(sent)
			if len(msgs) != 1 || !strings.Contains(msgs[0].Text, "✅") {
				t.Fatalf("resposta = %+v", msgs)
			}
			for _, it := range listTx() {
				if it.AmountCents == -1000 {
					if n := utf8.RuneCountInString(it.Description); n != 120 {
						t.Fatalf("descrição com %d caracteres", n)
					}
					return
				}
			}
			t.Fatal("lançamento de R$ 10,00 não encontrado")
		})
	})
}

// Alertas de orçamento só para lançamentos do mês corrente; percentual arredonda .5 para cima.
func TestBudgetAlertOnlyForCurrentMonth(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		ctx := context.Background()
		sent := fakeTelegram(t, "42")
		svcs := New(store.New(database), nil)

		acc, err := svcs.CreateAccount(ctx, domain.CreateAccountInput{Name: "Conta", Type: domain.AccountChecking})
		if err != nil {
			t.Fatal(err)
		}
		cat, err := svcs.CreateCategory(ctx, domain.CreateCategoryInput{Name: "Mercado", Kind: domain.CategoryExpense, Icon: "cart", Color: "#22c55e"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svcs.UpsertBudget(ctx, domain.UpsertBudgetInput{CategoryID: cat.ID, AmountCents: 10000}); err != nil {
			t.Fatal(err)
		}
		expense := func(date string, cents int64) {
			t.Helper()
			catID := cat.ID
			if _, err := svcs.CreateTransaction(ctx, domain.CreateTransactionInput{
				Type: domain.TxExpense, Description: "compra", AmountCents: cents, Date: date,
				AccountID: acc.ID, CategoryID: &catID,
			}); err != nil {
				t.Fatal(err)
			}
		}
		today := domain.TodayString()

		// 70,50 de 100,00 → 70.5% cruza 70%; toFixed(0) do JS mostra "71%".
		expense(today, 7050)
		msg, ok := waitMessage(t, sent, 5*time.Second)
		if !ok {
			t.Fatal("alerta de 70% não enviado")
		}
		if !strings.Contains(msg.Text, "(71%)") {
			t.Fatalf("alerta = %q, want (71%%)", msg.Text)
		}

		// Lançamentos em outros meses não podem repetir o alerta do mês corrente.
		expense(domain.AddMonthsToDateString(today, -1), 2000)
		expense(domain.AddMonthsToDateString(today, 1), 2000)
		if msg, ok := waitMessage(t, sent, 500*time.Millisecond); ok {
			t.Fatalf("alerta indevido para outro mês: %q", msg.Text)
		}
	})
}

// Pagar a mesma fatura duas vezes resulta em 409; o sentinel do store (pagamento concorrente
// detectado dentro da transação) também vira o mesmo conflito.
func TestPayInvoiceTwiceIsConflict(t *testing.T) {
	testutil.ForEachDialect(t, func(t *testing.T, database *db.DB) {
		ctx := context.Background()
		t.Setenv("TELEGRAM_BOT_TOKEN", "")
		st := store.New(database)
		svcs := New(st, nil)

		acc, err := svcs.CreateAccount(ctx, domain.CreateAccountInput{Name: "Conta", Type: domain.AccountChecking, InitialBalanceCents: 100000})
		if err != nil {
			t.Fatal(err)
		}
		card, err := svcs.CreateCard(ctx, domain.CreateCardInput{Name: "Cartão", Brand: domain.BrandVisa, LimitCents: 500000, ClosingDay: 10, DueDay: 17})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svcs.AddCardExpense(ctx, card.ID, domain.CreateCardExpenseInput{
			Description: "tv", TotalCents: 12000, Date: "2026-10-05", Installments: 1,
		}); err != nil {
			t.Fatal(err)
		}
		invoices, err := svcs.ListInvoices(ctx, card.ID)
		if err != nil || len(invoices) != 1 {
			t.Fatalf("faturas = %+v (%v)", invoices, err)
		}
		invID := invoices[0].ID
		date := "2026-10-20"
		paid, err := svcs.PayInvoice(ctx, card.ID, invID, domain.PayInvoiceInput{AccountID: acc.ID, Date: &date})
		if err != nil || paid.Status != domain.InvoicePaid {
			t.Fatalf("1º pagamento = %+v (%v)", paid, err)
		}

		var appErr *AppError
		_, err = svcs.PayInvoice(ctx, card.ID, invID, domain.PayInvoiceInput{AccountID: acc.ID, Date: &date})
		if !errors.As(err, &appErr) || appErr.Status != http.StatusConflict {
			t.Fatalf("2º pagamento = %v, want 409", err)
		}

		// Caminho da corrida: a checagem otimista passou, mas o store achou a fatura já paga.
		err = st.PayInvoice(ctx, invID, acc.ID, date, "duplicado", 12000)
		if !errors.Is(err, store.ErrConflict) {
			t.Fatalf("store.PayInvoice repetido = %v, want ErrConflict", err)
		}
	})
}
