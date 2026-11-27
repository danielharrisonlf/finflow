package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"finflow/internal/domain"
	"finflow/internal/store"
)

// ───────────────────────── Telegram API Client ─────────────────────────

type TelegramAPIClient struct {
	HTTP *http.Client
}

func NewTelegramAPIClient(httpClient *http.Client) *TelegramAPIClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &TelegramAPIClient{HTTP: httpClient}
}

func (c *TelegramAPIClient) IsConfigured() bool {
	return strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")) != ""
}

func (c *TelegramAPIClient) DefaultChatID() string {
	return strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID"))
}

func (c *TelegramAPIClient) baseURL() string {
	override := strings.TrimSpace(os.Getenv("TELEGRAM_API_BASE"))
	if override == "" {
		override = "https://api.telegram.org"
	}
	token := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	return fmt.Sprintf("%s/bot%s", strings.TrimRight(override, "/"), token)
}

func (c *TelegramAPIClient) SendMessage(ctx context.Context, text, chatID string) bool {
	target := chatID
	if target == "" {
		target = c.DefaultChatID()
	}
	if !c.IsConfigured() || target == "" {
		return false
	}
	body, err := json.Marshal(map[string]string{
		"chat_id":    target,
		"text":       text,
		"parse_mode": "HTML",
	})
	if err != nil {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL()+"/sendMessage", bytes.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		log.Printf("telegram indisponível: %v", err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("telegram sendMessage falhou: HTTP %d", resp.StatusCode)
		return false
	}
	return true
}

// ───────────────────────── Notifications ─────────────────────────

type NotificationsService struct {
	store    *store.Store
	telegram *TelegramAPIClient
}

func NewNotificationsService(st *store.Store, tg *TelegramAPIClient) *NotificationsService {
	return &NotificationsService{store: st, telegram: tg}
}

func (n *NotificationsService) NotifyBudgetThresholdCrossed(ctx context.Context, categoryID string, addedCents int64) {
	if !n.telegram.IsConfigured() || n.telegram.DefaultChatID() == "" {
		return
	}
	month, year := domain.CurrentMonthYear()
	snap, err := n.store.BudgetSnapshot(ctx, categoryID, year, month)
	if err != nil || snap == nil || snap.BudgetCents <= 0 {
		return
	}
	toPercent := func(c int64) float64 {
		return (float64(c) / float64(snap.BudgetCents)) * 100
	}
	absAdded := addedCents
	if absAdded < 0 {
		absAdded = -absAdded
	}
	beforeCents := snap.SpentCents - absAdded
	if beforeCents < 0 {
		beforeCents = 0
	}
	after := toPercent(snap.SpentCents)
	before := toPercent(beforeCents)
	threshold := domain.CrossedThreshold(before, after)
	if threshold == 0 {
		return
	}
	emoji := "⚠️"
	headline := "Orçamento em atenção"
	if threshold == 90 {
		emoji = "🚨"
		headline = "Orçamento quase estourado"
	}
	msg := strings.Join([]string{
		fmt.Sprintf("%s <b>%s</b>", emoji, headline),
		fmt.Sprintf("Categoria: <b>%s</b>", html.EscapeString(snap.CategoryName)),
		// math.Round arredonda .5 para cima (como toFixed(0) do JS); %.0f arredondaria para o par.
		fmt.Sprintf("Gasto no mês: <b>%s</b> de %s (%d%%)", domain.FormatBRL(snap.SpentCents), domain.FormatBRL(snap.BudgetCents), int64(math.Round(after))),
	}, "\n")
	n.telegram.SendMessage(ctx, msg, "")
}

// ───────────────────────── Telegram Parser & Bot ─────────────────────────

type ParsedTelegramMessage struct {
	Kind         string // "help" | "balance" | "summary" | "statement" | "expense" | "unknown"
	AmountCents  int64
	Description  string
	CategoryName string
	Raw          string
}

var (
	tgCmdSplitRe = regexp.MustCompile(`[\s@]`)
	tgExpenseRe  = regexp.MustCompile(`(?s)^([\d.,]+)\s+(.+)$`)
	tgHashtagRe  = regexp.MustCompile(`#([\p{L}\p{N}_-]+)`)
	tgMultiSpRe  = regexp.MustCompile(`\s{2,}`)
)

// normalizeJSSpaces troca por ' ' os espaços que o \s do JS reconhece e o \s do RE2 (só ASCII) não:
// VT (U+000B), NBSP (U+00A0), U+1680, U+2000–U+200A, U+2028/2029, U+202F, U+205F, U+3000 e BOM (U+FEFF).
// Assim "45,90<NBSP>almoço" continua sendo despesa, como no parser original.
func normalizeJSSpaces(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == 0x0b, r == 0xa0, r == 0x1680, r >= 0x2000 && r <= 0x200a,
			r == 0x2028, r == 0x2029, r == 0x202f, r == 0x205f, r == 0x3000, r == 0xfeff:
			return ' '
		}
		return r
	}, s)
}

func ParseTelegramMessage(raw string) ParsedTelegramMessage {
	text := strings.TrimSpace(normalizeJSSpaces(raw))
	if text == "" {
		return ParsedTelegramMessage{Kind: "unknown", Raw: raw}
	}
	cmd := strings.ToLower(tgCmdSplitRe.Split(text, 2)[0])
	switch cmd {
	case "/start", "/ajuda", "/help":
		return ParsedTelegramMessage{Kind: "help"}
	case "/saldo":
		return ParsedTelegramMessage{Kind: "balance"}
	case "/resumo":
		return ParsedTelegramMessage{Kind: "summary"}
	case "/extrato":
		return ParsedTelegramMessage{Kind: "statement"}
	}
	if strings.HasPrefix(text, "/") {
		return ParsedTelegramMessage{Kind: "unknown", Raw: raw}
	}

	m := tgExpenseRe.FindStringSubmatch(text)
	if len(m) != 3 {
		return ParsedTelegramMessage{Kind: "unknown", Raw: raw}
	}
	amountCents, ok := domain.ParseBRLToCents(m[1])
	if !ok {
		return ParsedTelegramMessage{Kind: "unknown", Raw: raw}
	}

	desc := strings.TrimSpace(m[2])
	var categoryName string
	if hm := tgHashtagRe.FindStringSubmatch(desc); len(hm) == 2 {
		categoryName = strings.NewReplacer("_", " ", "-", " ").Replace(hm[1])
		desc = strings.Replace(desc, hm[0], "", 1)
		desc = strings.TrimSpace(tgMultiSpRe.ReplaceAllString(desc, " "))
	}
	if desc == "" {
		return ParsedTelegramMessage{Kind: "unknown", Raw: raw}
	}
	return ParsedTelegramMessage{
		Kind:         "expense",
		AmountCents:  amountCents,
		Description:  desc,
		CategoryName: categoryName,
	}
}

const telegramHelpText = "<b>FinFlow Bot</b> — seu controle financeiro no Telegram\n\n" +
	"<b>Comandos</b>\n" +
	"/saldo — saldo das contas\n" +
	"/resumo — receitas, despesas e balanço do mês\n" +
	"/extrato — últimos lançamentos\n" +
	"/ajuda — esta mensagem\n\n" +
	"<b>Lançamento rápido de despesa</b>\n" +
	"Envie: <code>45,90 almoço #alimentacao</code>\n" +
	"(valor + descrição + categoria opcional via #hashtag)"

type TelegramService struct {
	svcs *Services
}

func NewTelegramService(svcs *Services) *TelegramService {
	return &TelegramService{svcs: svcs}
}

func (t *TelegramService) Status() domain.TelegramStatus {
	return domain.TelegramStatus{
		Configured: t.svcs.TelegramAPI.IsConfigured(),
		ChatLinked: t.svcs.TelegramAPI.DefaultChatID() != "",
	}
}

func (t *TelegramService) HandleUpdate(ctx context.Context, update domain.TelegramUpdate) error {
	// Como no TS, só texto vazio é ignorado; "   " recebe a resposta padrão (/ajuda).
	if update.Message == nil || update.Message.Text == "" {
		return nil
	}
	chatID := strconv.FormatInt(update.Message.Chat.ID, 10)
	linked := t.svcs.TelegramAPI.DefaultChatID()
	if linked == "" {
		t.svcs.TelegramAPI.SendMessage(
			ctx,
			fmt.Sprintf("Chat ainda não vinculado. Defina <code>TELEGRAM_CHAT_ID=%s</code> no servidor para ativar o bot.", chatID),
			chatID,
		)
		return nil
	}
	if chatID != linked {
		log.Printf("telegram update ignorado de chat não vinculado: %s", chatID)
		return nil
	}
	reply, err := t.BuildReply(ctx, update.Message.Text)
	if err != nil {
		// Erro de entrada do usuário vira resposta no chat; o webhook nunca falha por isso
		// (um não-2xx faria o Telegram reenviar o update indefinidamente).
		var appErr *AppError
		if !errors.As(err, &appErr) || appErr.Status >= http.StatusInternalServerError {
			return err
		}
		reply = "⚠️ Não consegui concluir: " + html.EscapeString(appErr.Error())
	}
	t.svcs.TelegramAPI.SendMessage(ctx, reply, chatID)
	return nil
}

func (t *TelegramService) BuildReply(ctx context.Context, text string) (string, error) {
	parsed := ParseTelegramMessage(text)
	switch parsed.Kind {
	case "help":
		return telegramHelpText, nil
	case "balance":
		return t.balanceReply(ctx)
	case "summary":
		return t.summaryReply(ctx)
	case "statement":
		return t.statementReply(ctx)
	case "expense":
		return t.expenseReply(ctx, parsed.AmountCents, parsed.Description, parsed.CategoryName)
	default:
		return "Não entendi 🤔 — envie /ajuda para ver os comandos e o formato de lançamento rápido.", nil
	}
}

func (t *TelegramService) balanceReply(ctx context.Context) (string, error) {
	accounts, err := t.svcs.ListAccounts(ctx, false)
	if err != nil {
		return "", err
	}
	if len(accounts) == 0 {
		return "Nenhuma conta cadastrada ainda.", nil
	}
	var total int64
	lines := []string{"<b>💰 Saldo das contas</b>"}
	for _, a := range accounts {
		total += a.BalanceCents
		lines = append(lines, fmt.Sprintf("• %s: <b>%s</b>", html.EscapeString(a.Name), domain.FormatBRL(a.BalanceCents)))
	}
	lines = append(lines, "", fmt.Sprintf("Total: <b>%s</b>", domain.FormatBRL(total)))
	return strings.Join(lines, "\n"), nil
}

func (t *TelegramService) summaryReply(ctx context.Context) (string, error) {
	summary, err := t.svcs.DashboardSummary(ctx, domain.PeriodQuery{})
	if err != nil {
		return "", err
	}
	categories, err := t.svcs.DashboardCategories(ctx, domain.PeriodQuery{})
	if err != nil {
		return "", err
	}
	lines := []string{
		fmt.Sprintf("<b>📊 Resumo %02d/%d</b>", summary.Month, summary.Year),
		fmt.Sprintf("Receitas: <b>%s</b>", domain.FormatBRL(summary.MonthIncomeCents)),
		fmt.Sprintf("Despesas: <b>%s</b>", domain.FormatBRL(summary.MonthExpenseCents)),
		fmt.Sprintf("Balanço: <b>%s</b>", domain.FormatBRL(summary.MonthNetCents)),
		fmt.Sprintf("Faturas em aberto: %s", domain.FormatBRL(summary.OpenInvoicesCents)),
	}
	if len(categories) > 0 {
		lines = append(lines, "", "<b>Maiores gastos</b>")
		limit := len(categories)
		if limit > 3 {
			limit = 3
		}
		for _, c := range categories[:limit] {
			lines = append(lines, fmt.Sprintf("• %s: %s (%v%%)", html.EscapeString(c.Name), domain.FormatBRL(c.TotalCents), c.Percent))
		}
	}
	return strings.Join(lines, "\n"), nil
}

func (t *TelegramService) statementReply(ctx context.Context) (string, error) {
	page, err := t.svcs.ListTransactions(ctx, domain.ListTransactionsQuery{Page: 1, PageSize: 10})
	if err != nil {
		return "", err
	}
	if len(page.Items) == 0 {
		return "Nenhum lançamento ainda.", nil
	}
	lines := []string{"<b>🧾 Últimos lançamentos</b>"}
	for _, item := range page.Items {
		arrow := "🔴"
		if item.AmountCents >= 0 {
			arrow = "🟢"
		}
		dayMonth := item.Date
		if len(item.Date) >= 10 {
			dayMonth = item.Date[8:10] + "/" + item.Date[5:7]
		}
		lines = append(lines, fmt.Sprintf("%s %s %s — <b>%s</b>", arrow, dayMonth, html.EscapeString(item.Description), domain.FormatBRL(item.AmountCents)))
	}
	return strings.Join(lines, "\n"), nil
}

func (t *TelegramService) expenseReply(ctx context.Context, amountCents int64, description, categoryName string) (string, error) {
	accounts, err := t.svcs.ListAccounts(ctx, false)
	if err != nil {
		return "", err
	}
	if len(accounts) == 0 {
		return "Nenhuma conta cadastrada — crie uma conta no app antes de lançar despesas.", nil
	}
	target := accounts[0]
	for _, a := range accounts {
		if a.Type == domain.AccountChecking {
			target = a
			break
		}
	}
	var categoryID *string
	categoryNote := ""
	if categoryName != "" {
		cat, err := t.svcs.Store.FindCategoryByName(ctx, categoryName, domain.CategoryExpense)
		if err != nil {
			return "", err
		}
		if cat != nil {
			id := cat.ID
			categoryID = &id
			categoryNote = fmt.Sprintf(" em <b>%s</b>", html.EscapeString(cat.Name))
		} else {
			categoryNote = fmt.Sprintf(` (categoria %q não encontrada — ficou sem categoria)`, html.EscapeString(categoryName))
		}
	}
	if categoryID == nil {
		// Sem categoria: aplica as regras de automação aqui para poder citar a categoria na resposta.
		suggested, err := t.svcs.SuggestCategory(ctx, description, domain.CategoryExpense)
		if err != nil {
			return "", err
		}
		if suggested != nil {
			cat, err := t.svcs.Store.FindCategoryByID(ctx, *suggested)
			if err != nil {
				return "", err
			}
			if cat != nil {
				categoryID = suggested
				if categoryName != "" {
					categoryNote = fmt.Sprintf(` (categoria %q não encontrada — classificada em <b>%s</b> pelas regras)`, html.EscapeString(categoryName), html.EscapeString(cat.Name))
				} else {
					categoryNote = fmt.Sprintf(" em <b>%s</b>", html.EscapeString(cat.Name))
				}
			}
		}
	}
	description = truncateRunes(description, telegramMaxDescription)
	notes := "lançado via Telegram"
	if _, err := t.svcs.CreateTransaction(ctx, domain.CreateTransactionInput{
		Type:        domain.TxExpense,
		Description: description,
		AmountCents: amountCents,
		Date:        domain.TodayString(),
		Status:      domain.TxConfirmed,
		AccountID:   target.ID,
		CategoryID:  categoryID,
		Notes:       &notes,
	}); err != nil {
		return "", err
	}
	return fmt.Sprintf("✅ Despesa de <b>%s</b> (%s)%s lançada na conta <b>%s</b>.", domain.FormatBRL(amountCents), html.EscapeString(description), categoryNote, html.EscapeString(target.Name)), nil
}

// telegramMaxDescription é o limite de descrição de transação (validação: 1–120 caracteres).
const telegramMaxDescription = 120

// truncateRunes corta s em no máximo max runas (sem quebrar caracteres UTF-8).
func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:max]))
}
