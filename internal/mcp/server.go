package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"finflow/internal/client"
	"finflow/internal/domain"
)

type Server struct {
	backend client.Backend
	in      io.Reader
	out     io.Writer
	errOut  io.Writer
}

func NewServer(backend client.Backend) *Server {
	return &Server{
		backend: backend,
		in:      os.Stdin,
		out:     os.Stdout,
		errOut:  os.Stderr,
	}
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type toolResult struct {
	Content []textContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func okResult(data any) toolResult {
	if s, ok := data.(string); ok {
		return toolResult{Content: []textContent{{Type: "text", Text: s}}}
	}
	b, _ := json.MarshalIndent(data, "", "  ")
	return toolResult{Content: []textContent{{Type: "text", Text: string(b)}}}
}

func errResult(msg string) toolResult {
	return toolResult{
		Content: []textContent{{Type: "text", Text: "erro: " + msg}},
		IsError: true,
	}
}

func (s *Server) Run(ctx context.Context, modeLabel string) error {
	fmt.Fprintf(s.errOut, "finflow-mcp conectado (%s)\n", modeLabel)
	scanner := bufio.NewScanner(s.in)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	enc := json.NewEncoder(s.out)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			// Pela especificação JSON-RPC, erro de parse responde com "id": null.
			if err := enc.Encode(rpcResponse{
				JSONRPC: "2.0",
				ID:      json.RawMessage("null"),
				Error:   &rpcError{Code: -32700, Message: "JSON inválido"},
			}); err != nil {
				return err
			}
			continue
		}
		// Notifications não têm id e não devem receber resposta.
		if len(req.ID) == 0 || string(req.ID) == "null" {
			continue
		}

		resp := s.handle(ctx, req)
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// supportedProtocolVersions lista as versões do protocolo MCP aceitas, da mais nova para a mais antiga.
var supportedProtocolVersions = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

// negotiateProtocolVersion devolve a versão pedida pelo cliente quando suportada; senão, a mais recente suportada.
func negotiateProtocolVersion(requested string) string {
	for _, v := range supportedProtocolVersions {
		if v == requested {
			return v
		}
	}
	return supportedProtocolVersions[0]
}

func (s *Server) handle(ctx context.Context, req rpcRequest) rpcResponse {
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		proto := negotiateProtocolVersion(p.ProtocolVersion)
		return rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": proto,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "finflow", "version": "0.1.0"},
			},
		}
	case "ping":
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{}}
	case "tools/list":
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": allTools()}}
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return rpcResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &rpcError{Code: -32602, Message: "parâmetros inválidos"},
			}
		}
		res := s.callTool(ctx, p.Name, p.Arguments)
		return rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: res}
	default:
		return rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32601, Message: "método não suportado: " + req.Method},
		}
	}
}

func objSchema(props map[string]any, required ...string) map[string]any {
	s := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func periodProps() map[string]any {
	return map[string]any{
		"month": map[string]any{"type": "integer", "minimum": 1, "maximum": 12, "description": "mês (1-12); padrão: mês atual"},
		"year":  map[string]any{"type": "integer", "description": "ano; padrão: ano atual"},
	}
}

func simpleTxProps() map[string]any {
	return map[string]any{
		"amountCents": map[string]any{"type": "integer", "minimum": 1, "description": "valor em centavos (R$ 45,90 = 4590)"},
		"description": map[string]any{"type": "string", "minLength": 1},
		"account":     map[string]any{"type": "string", "description": "nome ou id da conta; padrão: primeira conta"},
		"category":    map[string]any{"type": "string", "description": "nome da categoria"},
		"date":        map[string]any{"type": "string", "pattern": `^\d{4}-\d{2}-\d{2}$`, "description": "YYYY-MM-DD; padrão: hoje"},
		"pending":     map[string]any{"type": "boolean", "description": "true = ainda não paga/recebida"},
	}
}

func allTools() []toolDef {
	txListProps := periodProps()
	txListProps["search"] = map[string]any{"type": "string", "description": "busca na descrição"}
	txListProps["type"] = map[string]any{"type": "string", "enum": []string{"INCOME", "EXPENSE", "TRANSFER", "CARD_EXPENSE", "INVOICE_PAYMENT"}}
	txListProps["status"] = map[string]any{"type": "string", "enum": []string{"PENDING", "CONFIRMED"}}
	txListProps["page"] = map[string]any{"type": "integer", "minimum": 1}

	exportProps := periodProps()
	exportProps["format"] = map[string]any{"type": "string", "enum": []string{"csv", "ofx"}}

	tools := []toolDef{
		{Name: "get_summary", Description: "Resumo financeiro: saldo consolidado, receitas/despesas/balanço do mês, faturas em aberto e pendências. Valores em centavos.", InputSchema: objSchema(periodProps())},
		{Name: "get_cashflow", Description: "Fluxo de caixa diário do mês (entradas vs. saídas), em centavos.", InputSchema: objSchema(periodProps())},
		{Name: "get_category_spending", Description: "Despesas do mês agrupadas por categoria, com percentuais.", InputSchema: objSchema(periodProps())},
		{Name: "get_upcoming_bills", Description: "Contas pendentes e faturas de cartão vencendo nos próximos 7 dias.", InputSchema: objSchema(map[string]any{})},
		{Name: "list_accounts", Description: "Lista as contas com saldo atual (centavos).", InputSchema: objSchema(map[string]any{})},
		{Name: "list_cards", Description: "Lista cartões de crédito com limite usado/disponível e fatura atual.", InputSchema: objSchema(map[string]any{})},
		{Name: "list_card_invoices", Description: "Lista as faturas de um cartão (status OPEN/CLOSED/PAID, totais e vencimentos).", InputSchema: objSchema(map[string]any{
			"card": map[string]any{"type": "string", "description": "nome (parcial) ou id do cartão"},
		}, "card")},
		{Name: "list_transactions", Description: "Extrato com filtros por período, tipo, status e busca textual.", InputSchema: objSchema(txListProps)},
		{Name: "list_categories", Description: "Lista categorias de receita e despesa.", InputSchema: objSchema(map[string]any{
			"kind": map[string]any{"type": "string", "enum": []string{"INCOME", "EXPENSE"}},
		})},
		{Name: "list_budgets", Description: "Orçamentos por categoria com gasto do mês, percentual e status (OK/WARNING/DANGER).", InputSchema: objSchema(periodProps())},
		{Name: "list_goals", Description: "Metas financeiras com progresso e aporte mensal necessário.", InputSchema: objSchema(map[string]any{})},
		{Name: "create_expense", Description: "Lança uma despesa em uma conta. Valor em centavos.", InputSchema: objSchema(simpleTxProps(), "amountCents", "description")},
		{Name: "create_income", Description: "Lança uma receita em uma conta. Valor em centavos.", InputSchema: objSchema(simpleTxProps(), "amountCents", "description")},
		{Name: "create_card_expense", Description: "Compra no cartão de crédito, à vista ou parcelada; cada parcela cai na fatura do mês correto.", InputSchema: objSchema(map[string]any{
			"card":         map[string]any{"type": "string", "description": "nome ou id do cartão"},
			"description":  map[string]any{"type": "string", "minLength": 1},
			"totalCents":   map[string]any{"type": "integer", "minimum": 1, "description": "valor TOTAL da compra em centavos"},
			"installments": map[string]any{"type": "integer", "minimum": 1, "maximum": 48, "description": "parcelas; padrão 1"},
			"category":     map[string]any{"type": "string"},
			"date":         map[string]any{"type": "string", "pattern": `^\d{4}-\d{2}-\d{2}$`},
		}, "card", "description", "totalCents")},
		{Name: "transfer", Description: "Transfere entre contas próprias (débito e crédito atômicos).", InputSchema: objSchema(map[string]any{
			"from":        map[string]any{"type": "string", "description": "conta de origem (nome ou id)"},
			"to":          map[string]any{"type": "string", "description": "conta de destino (nome ou id)"},
			"amountCents": map[string]any{"type": "integer", "minimum": 1},
			"date":        map[string]any{"type": "string", "pattern": `^\d{4}-\d{2}-\d{2}$`},
		}, "from", "to", "amountCents")},
		{Name: "pay_invoice", Description: "Paga a fatura de um cartão debitando de uma conta.", InputSchema: objSchema(map[string]any{
			"card":    map[string]any{"type": "string", "description": "nome ou id do cartão"},
			"month":   map[string]any{"type": "integer", "minimum": 1, "maximum": 12, "description": "mês da fatura"},
			"year":    map[string]any{"type": "integer", "description": "ano da fatura"},
			"account": map[string]any{"type": "string", "description": "conta para débito (nome ou id)"},
		}, "card", "month", "year", "account")},
		{Name: "set_budget", Description: "Define (ou atualiza) o teto mensal de uma categoria de despesa.", InputSchema: objSchema(map[string]any{
			"category":    map[string]any{"type": "string", "description": "nome da categoria de despesa"},
			"amountCents": map[string]any{"type": "integer", "minimum": 1},
		}, "category", "amountCents")},
		{Name: "contribute_goal", Description: "Aporta (positivo) ou resgata (negativo) de uma meta financeira.", InputSchema: objSchema(map[string]any{
			"goal":        map[string]any{"type": "string", "description": "nome ou id da meta"},
			"amountCents": map[string]any{"type": "integer", "description": "centavos; negativo = resgate"},
			"note":        map[string]any{"type": "string"},
		}, "goal", "amountCents")},
		{Name: "export_transactions", Description: "Exporta o extrato filtrado em CSV ou OFX e devolve o conteúdo do arquivo.", InputSchema: objSchema(exportProps, "format")},
		{Name: "get_insights", Description: "Visão geral do mês: ritmo de gastos vs. mês anterior, limite mensal, performance (entradas − saídas fixas − diário), economia, gasto por tipo (Fixo/Cortável/Variável), patrimônio, reserva de emergência e onde cortar. Centavos.", InputSchema: objSchema(periodProps())},
		{Name: "get_projection", Description: "Saldo projetado dia a dia (recorrências, faturas, pendências e gasto diário estimado pelo limite mensal), menor saldo em 30 dias, próxima entrada, quanto dá para guardar agora e resumo de 12 meses. Centavos.", InputSchema: objSchema(map[string]any{
			"days": map[string]any{"type": "integer", "minimum": 1, "maximum": 365, "description": "dias devolvidos na lista diária; padrão 30"},
		})},
		{Name: "get_year_overview", Description: "Resumo dos 12 meses do ano pelo método da planilha diária: entradas, saídas fixas, diário, performance, economia, taxa de economia e saldo no fim de cada mês.", InputSchema: objSchema(map[string]any{
			"year": map[string]any{"type": "integer", "description": "ano; padrão: ano atual"},
		})},
		{Name: "list_recurring", Description: "Lista contas fixas e receitas recorrentes com a próxima ocorrência ainda não lançada.", InputSchema: objSchema(map[string]any{})},
		{Name: "create_recurring", Description: "Cadastra uma conta fixa (EXPENSE) ou receita recorrente (INCOME) mensal. Valor em centavos.", InputSchema: objSchema(map[string]any{
			"description": map[string]any{"type": "string", "minLength": 1},
			"kind":        map[string]any{"type": "string", "enum": []string{"INCOME", "EXPENSE"}},
			"amountCents": map[string]any{"type": "integer", "minimum": 1, "description": "valor em centavos"},
			"dayOfMonth":  map[string]any{"type": "integer", "minimum": 1, "maximum": 31, "description": "dia do mês (meses curtos usam o último dia)"},
			"account":     map[string]any{"type": "string", "description": "nome ou id da conta usada ao lançar"},
			"category":    map[string]any{"type": "string", "description": "nome da categoria (do mesmo tipo)"},
			"startDate":   map[string]any{"type": "string", "pattern": `^\d{4}-\d{2}-\d{2}$`, "description": "YYYY-MM-DD; padrão: hoje"},
			"endDate":     map[string]any{"type": "string", "pattern": `^\d{4}-\d{2}-\d{2}$`},
		}, "description", "kind", "amountCents", "dayOfMonth")},
	}
	tools = append(tools, insightsV3Tools()...)
	tools = append(tools, wealthTools()...)
	return append(tools, spacesTools()...)
}

func (s *Server) accountByRef(ctx context.Context, ref string) (domain.Account, error) {
	accounts, err := s.backend.Accounts(ctx, false)
	if err != nil {
		return domain.Account{}, err
	}
	found, ok := client.ResolveByName(accounts, ref, func(a domain.Account) string { return a.ID }, func(a domain.Account) string { return a.Name })
	if !ok {
		names := make([]string, len(accounts))
		for i, a := range accounts {
			names[i] = a.Name
		}
		return domain.Account{}, fmt.Errorf(`conta %q não encontrada ou ambígua. Contas: %s`, ref, strings.Join(names, ", "))
	}
	return found, nil
}

func (s *Server) cardByRef(ctx context.Context, ref string) (domain.Card, error) {
	cards, err := s.backend.Cards(ctx, false)
	if err != nil {
		return domain.Card{}, err
	}
	found, ok := client.ResolveByName(cards, ref, func(c domain.Card) string { return c.ID }, func(c domain.Card) string { return c.Name })
	if !ok {
		names := make([]string, len(cards))
		for i, c := range cards {
			names[i] = c.Name
		}
		return domain.Card{}, fmt.Errorf(`cartão %q não encontrado ou ambíguo. Cartões: %s`, ref, strings.Join(names, ", "))
	}
	return found, nil
}

func (s *Server) categoryIDByRef(ctx context.Context, ref string, kind domain.CategoryKind) (*string, error) {
	if strings.TrimSpace(ref) == "" {
		return nil, nil
	}
	cats, err := s.backend.Categories(ctx, &kind)
	if err != nil {
		return nil, err
	}
	found, ok := client.ResolveByName(cats, ref, func(c domain.Category) string { return c.ID }, func(c domain.Category) string { return c.Name })
	if !ok {
		names := make([]string, len(cats))
		for i, c := range cats {
			names[i] = c.Name
		}
		return nil, fmt.Errorf(`categoria %q não encontrada. Disponíveis: %s`, ref, strings.Join(names, ", "))
	}
	id := found.ID
	return &id, nil
}

func (s *Server) callTool(ctx context.Context, name string, rawArgs json.RawMessage) toolResult {
	if len(rawArgs) == 0 || string(rawArgs) == "null" {
		rawArgs = []byte("{}")
	}
	if def, ok := findTool(name); ok {
		normalized, msg := validateAndNormalizeArgs(def.InputSchema, rawArgs)
		if msg != "" {
			return errResult("argumentos inválidos: " + msg)
		}
		rawArgs = normalized
	}
	var err error
	var data any

	switch name {
	case "get_summary":
		var a struct{ Month, Year *int }
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			data, err = s.backend.DashboardSummary(ctx, domain.PeriodQuery{Month: a.Month, Year: a.Year})
		}
	case "get_cashflow":
		var a struct{ Month, Year *int }
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			data, err = s.backend.DashboardCashflow(ctx, domain.PeriodQuery{Month: a.Month, Year: a.Year})
		}
	case "get_category_spending":
		var a struct{ Month, Year *int }
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			data, err = s.backend.DashboardCategories(ctx, domain.PeriodQuery{Month: a.Month, Year: a.Year})
		}
	case "get_upcoming_bills":
		data, err = s.backend.DashboardUpcoming(ctx)
	case "list_accounts":
		data, err = s.backend.Accounts(ctx, false)
	case "list_cards":
		data, err = s.backend.Cards(ctx, false)
	case "list_card_invoices":
		var a struct {
			Card string `json:"card"`
		}
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			var found domain.Card
			if found, err = s.cardByRef(ctx, a.Card); err == nil {
				data, err = s.backend.Invoices(ctx, found.ID)
			}
		}
	case "list_transactions":
		var a struct {
			Month  *int                      `json:"month"`
			Year   *int                      `json:"year"`
			Search *string                   `json:"search"`
			Type   *domain.TransactionType   `json:"type"`
			Status *domain.TransactionStatus `json:"status"`
			Page   int                       `json:"page"`
		}
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			data, err = s.backend.Transactions(ctx, domain.ListTransactionsQuery{
				Month:  a.Month,
				Year:   a.Year,
				Search: a.Search,
				Type:   a.Type,
				Status: a.Status,
				Page:   a.Page,
			})
		}
	case "list_categories":
		var a struct {
			Kind *domain.CategoryKind `json:"kind"`
		}
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			data, err = s.backend.Categories(ctx, a.Kind)
		}
	case "list_budgets":
		var a struct{ Month, Year *int }
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			data, err = s.backend.Budgets(ctx, domain.PeriodQuery{Month: a.Month, Year: a.Year})
		}
	case "list_goals":
		data, err = s.backend.Goals(ctx)
	case "create_expense", "create_income":
		txType := domain.TxExpense
		catKind := domain.CategoryExpense
		if name == "create_income" {
			txType = domain.TxIncome
			catKind = domain.CategoryIncome
		}
		var a struct {
			AmountCents int64  `json:"amountCents"`
			Description string `json:"description"`
			Account     string `json:"account"`
			Category    string `json:"category"`
			Date        string `json:"date"`
			Pending     bool   `json:"pending"`
		}
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			var acc domain.Account
			if a.Account != "" {
				acc, err = s.accountByRef(ctx, a.Account)
			} else {
				var accounts []domain.Account
				accounts, err = s.backend.Accounts(ctx, false)
				if err == nil {
					if len(accounts) == 0 {
						return errResult("nenhuma conta cadastrada")
					}
					acc = accounts[0]
					for _, x := range accounts {
						if x.Type == domain.AccountChecking {
							acc = x
							break
						}
					}
				}
			}
			if err == nil {
				var catID *string
				catID, err = s.categoryIDByRef(ctx, a.Category, catKind)
				if err == nil {
					date := a.Date
					if date == "" {
						date = domain.TodayString()
					}
					st := domain.TxConfirmed
					if a.Pending {
						st = domain.TxPending
					}
					notes := "lançado via MCP"
					data, err = s.backend.CreateTransaction(ctx, domain.CreateTransactionInput{
						Type:        txType,
						Description: a.Description,
						AmountCents: a.AmountCents,
						Date:        date,
						Status:      st,
						AccountID:   acc.ID,
						CategoryID:  catID,
						Notes:       &notes,
					})
				}
			}
		}
	case "create_card_expense":
		var a struct {
			Card         string `json:"card"`
			Description  string `json:"description"`
			TotalCents   int64  `json:"totalCents"`
			Installments int    `json:"installments"`
			Category     string `json:"category"`
			Date         string `json:"date"`
		}
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			var found domain.Card
			if found, err = s.cardByRef(ctx, a.Card); err == nil {
				var catID *string
				if catID, err = s.categoryIDByRef(ctx, a.Category, domain.CategoryExpense); err == nil {
					date := a.Date
					if date == "" {
						date = domain.TodayString()
					}
					inst := a.Installments
					if inst == 0 {
						inst = 1
					}
					notes := "lançado via MCP"
					var res domain.CardExpenseResult
					res, err = s.backend.AddCardExpense(ctx, found.ID, domain.CreateCardExpenseInput{
						CardID:       found.ID,
						Description:  a.Description,
						TotalCents:   a.TotalCents,
						Date:         date,
						CategoryID:   catID,
						Installments: inst,
						Notes:        &notes,
					})
					if err == nil {
						data = map[string]any{"card": found.Name, "created": res.Created}
					}
				}
			}
		}
	case "transfer":
		var a struct {
			From        string `json:"from"`
			To          string `json:"to"`
			AmountCents int64  `json:"amountCents"`
			Date        string `json:"date"`
		}
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			var origin, dest domain.Account
			if origin, err = s.accountByRef(ctx, a.From); err == nil {
				if dest, err = s.accountByRef(ctx, a.To); err == nil {
					date := a.Date
					if date == "" {
						date = domain.TodayString()
					}
					var res domain.TransferResult
					res, err = s.backend.Transfer(ctx, domain.TransferInput{
						FromAccountID: origin.ID,
						ToAccountID:   dest.ID,
						AmountCents:   a.AmountCents,
						Date:          date,
					})
					if err == nil {
						data = map[string]any{"from": origin.Name, "to": dest.Name, "transferGroupId": res.TransferGroupID}
					}
				}
			}
		}
	case "pay_invoice":
		var a struct {
			Card    string `json:"card"`
			Month   int    `json:"month"`
			Year    int    `json:"year"`
			Account string `json:"account"`
		}
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			var found domain.Card
			if found, err = s.cardByRef(ctx, a.Card); err == nil {
				var invoices []domain.Invoice
				if invoices, err = s.backend.Invoices(ctx, found.ID); err == nil {
					var target *domain.Invoice
					labels := make([]string, len(invoices))
					for i := range invoices {
						labels[i] = fmt.Sprintf("%d/%d", invoices[i].Month, invoices[i].Year)
						if invoices[i].Month == a.Month && invoices[i].Year == a.Year {
							target = &invoices[i]
						}
					}
					if target == nil {
						return errResult(fmt.Sprintf("fatura %d/%d não existe para %s. Faturas: %s", a.Month, a.Year, found.Name, strings.Join(labels, ", ")))
					}
					var payer domain.Account
					if payer, err = s.accountByRef(ctx, a.Account); err == nil {
						data, err = s.backend.PayInvoice(ctx, found.ID, target.ID, domain.PayInvoiceInput{AccountID: payer.ID})
					}
				}
			}
		}
	case "set_budget":
		var a struct {
			Category    string `json:"category"`
			AmountCents int64  `json:"amountCents"`
		}
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			var catID *string
			if catID, err = s.categoryIDByRef(ctx, a.Category, domain.CategoryExpense); err == nil {
				if catID == nil {
					return errResult(fmt.Sprintf(`categoria %q não encontrada`, a.Category))
				}
				data, err = s.backend.UpsertBudget(ctx, domain.UpsertBudgetInput{
					CategoryID:  *catID,
					AmountCents: a.AmountCents,
				})
			}
		}
	case "contribute_goal":
		var a struct {
			Goal        string  `json:"goal"`
			AmountCents int64   `json:"amountCents"`
			Note        *string `json:"note"`
		}
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			var goals []domain.Goal
			if goals, err = s.backend.Goals(ctx); err == nil {
				found, ok := client.ResolveByName(goals, a.Goal, func(g domain.Goal) string { return g.ID }, func(g domain.Goal) string { return g.Name })
				if !ok {
					names := make([]string, len(goals))
					for i, g := range goals {
						names[i] = g.Name
					}
					return errResult(fmt.Sprintf(`meta %q não encontrada. Metas: %s`, a.Goal, strings.Join(names, ", ")))
				}
				data, err = s.backend.ContributeGoal(ctx, found.ID, domain.ContributeGoalInput{
					AmountCents: a.AmountCents,
					Note:        a.Note,
				})
			}
		}
	case "export_transactions":
		var a struct {
			Format string `json:"format"`
			Month  *int   `json:"month"`
			Year   *int   `json:"year"`
		}
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			data, err = s.backend.ExportTransactions(ctx, a.Format, domain.ListTransactionsQuery{
				Month: a.Month,
				Year:  a.Year,
			})
		}
	case "get_insights":
		var a struct{ Month, Year *int }
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			data, err = s.backend.InsightsOverview(ctx, domain.PeriodQuery{Month: a.Month, Year: a.Year})
		}
	case "get_projection":
		var a struct {
			Days int `json:"days"`
		}
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			if a.Days == 0 {
				a.Days = 30
			}
			data, err = s.backend.InsightsProjection(ctx, a.Days)
		}
	case "get_year_overview":
		var a struct {
			Year *int `json:"year"`
		}
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			data, err = s.backend.InsightsYear(ctx, a.Year)
		}
	case "list_recurring":
		data, err = s.backend.Recurring(ctx)
	case "create_recurring":
		var a struct {
			Description string              `json:"description"`
			Kind        domain.CategoryKind `json:"kind"`
			AmountCents int64               `json:"amountCents"`
			DayOfMonth  int                 `json:"dayOfMonth"`
			Account     string              `json:"account"`
			Category    string              `json:"category"`
			StartDate   *string             `json:"startDate"`
			EndDate     *string             `json:"endDate"`
		}
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			in := domain.CreateRecurringInput{
				Description: a.Description,
				Kind:        a.Kind,
				AmountCents: a.AmountCents,
				DayOfMonth:  a.DayOfMonth,
				StartDate:   a.StartDate,
				EndDate:     a.EndDate,
			}
			if strings.TrimSpace(a.Account) != "" {
				var acc domain.Account
				if acc, err = s.accountByRef(ctx, a.Account); err == nil {
					in.AccountID = &acc.ID
				}
			}
			if err == nil {
				if in.CategoryID, err = s.categoryIDByRef(ctx, a.Category, a.Kind); err == nil {
					data, err = s.backend.CreateRecurring(ctx, in)
				}
			}
		}
	default:
		var handled bool
		if data, handled, err = s.callInsightsV3Tool(ctx, name, rawArgs); !handled {
			if data, handled, err = s.callWealthTool(ctx, name, rawArgs); !handled {
				if data, handled, err = s.callSpacesTool(ctx, name, rawArgs); !handled {
					return errResult("tool desconhecida: " + name)
				}
			}
		}
	}

	if err != nil {
		return errResult(err.Error())
	}
	return okResult(data)
}
