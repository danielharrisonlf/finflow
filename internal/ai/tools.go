package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/google/uuid"

	"finflow/internal/domain"
	"finflow/internal/service"
)

// Tipos de proposta (evento SSE `proposal`).
const (
	ProposalCreateTransaction = "CREATE_TRANSACTION"
	ProposalCreateRecurring   = "CREATE_RECURRING"
	ProposalContributeGoal    = "CONTRIBUTE_GOAL"
	ProposalAddWish           = "ADD_WISH"
)

// proposalSentMessage é o tool_result das ferramentas de escrita: nada é gravado pelo modelo.
const proposalSentMessage = "proposta enviada ao usuário para confirmação"

// Proposal é o payload do evento `proposal`; Payload é o corpo do endpoint normal que a UI chama ao confirmar.
type Proposal struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Summary string `json:"summary"`
	Payload any    `json:"payload"`
}

// intRange é uma restrição validada no binário (o modo strict não aceita minimum/maximum no schema).
type intRange struct{ min, max int64 }

type toolSpec struct {
	name        string
	description string
	label       string // rótulo pt-BR do evento `tool`
	props       map[string]any
	required    []string
	ints        map[string]intRange // limites de inteiros
	maxLen      map[string]int      // tamanho máximo (runas) de strings; mínimo é 1 quando presente
	run         func(ctx context.Context, t *toolEnv, raw json.RawMessage) (any, error)
}

// toolEnv é o contexto de execução de uma ferramenta.
type toolEnv struct {
	s    *Service
	emit func(event string, data any)
}

func strProp(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}
func intProp(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}
func boolProp(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}
func dateProp(desc string) map[string]any {
	return map[string]any{"type": "string", "format": "date", "description": desc}
}
func enumProp(desc string, values ...string) map[string]any {
	return map[string]any{"type": "string", "enum": values, "description": desc}
}

func periodToolProps() map[string]any {
	return map[string]any{
		"month": intProp("mês 1-12; padrão: mês atual"),
		"year":  intProp("ano com 4 dígitos; padrão: ano atual"),
	}
}

var periodInts = map[string]intRange{"month": {1, 12}, "year": {2000, 2100}}

type periodArgs struct {
	Month *int `json:"month"`
	Year  *int `json:"year"`
}

func (p periodArgs) query() domain.PeriodQuery {
	return domain.PeriodQuery{Month: p.Month, Year: p.Year}
}

func decodeArgs[T any](raw json.RawMessage) (T, error) {
	var out T
	err := json.Unmarshal(raw, &out)
	return out, err
}

// toolSpecs devolve as ferramentas em ordem determinística (por nome) para não invalidar o cache.
func toolSpecs() []toolSpec {
	specs := []toolSpec{
		{
			name:        "get_summary",
			description: "Resumo do mês: saldo consolidado das contas, receitas, despesas e balanço do mês, faturas em aberto e pendências. Use para perguntas gerais sobre 'como estou este mês'. Centavos.",
			label:       "Consultando seu resumo do mês",
			props:       periodToolProps(), ints: periodInts,
			run: func(ctx context.Context, t *toolEnv, raw json.RawMessage) (any, error) {
				a, err := decodeArgs[periodArgs](raw)
				if err != nil {
					return nil, err
				}
				return t.s.dataSvcs().DashboardSummary(ctx, a.query())
			},
		},
		{
			name:        "get_insights",
			description: "Visão estratégica do mês: ritmo de gastos vs. mês anterior, limite mensal e quanto ainda cabe por dia, performance (entradas − saídas fixas − diário), economia, gasto por tipo (Fixo/Cortável/Variável), patrimônio, reserva de emergência e sugestões de onde cortar. Use para 'posso gastar com X este mês?', 'estou gastando demais?' e 'onde cortar'. Centavos.",
			label:       "Analisando seus gastos do mês",
			props:       periodToolProps(), ints: periodInts,
			run: func(ctx context.Context, t *toolEnv, raw json.RawMessage) (any, error) {
				a, err := decodeArgs[periodArgs](raw)
				if err != nil {
					return nil, err
				}
				return t.s.dataSvcs().InsightsOverview(ctx, a.query())
			},
		},
		{
			name:        "get_projection",
			description: "Saldo projetado dia a dia (receitas e contas fixas recorrentes, faturas, pendências e gasto diário estimado), menor saldo nos próximos dias, próxima entrada, quanto dá para guardar agora e resumo de 12 meses. Use para perguntas sobre o futuro do saldo. Centavos.",
			label:       "Consultando sua projeção",
			props:       map[string]any{"days": intProp("quantos dias detalhar (1-365); padrão 60")},
			ints:        map[string]intRange{"days": {1, domain.ProjectionHorizonDays}},
			run: func(ctx context.Context, t *toolEnv, raw json.RawMessage) (any, error) {
				a, err := decodeArgs[struct {
					Days *int `json:"days"`
				}](raw)
				if err != nil {
					return nil, err
				}
				days := 60
				if a.Days != nil {
					days = *a.Days
				}
				p, err := t.s.dataSvcs().InsightsProjection(ctx, days)
				if err != nil {
					return nil, err
				}
				return compactProjection(p), nil
			},
		},
		{
			name:        "get_cashflow",
			description: "Fluxo de caixa diário do mês: entradas e saídas por dia. Centavos.",
			label:       "Consultando seu fluxo de caixa",
			props:       periodToolProps(), ints: periodInts,
			run: func(ctx context.Context, t *toolEnv, raw json.RawMessage) (any, error) {
				a, err := decodeArgs[periodArgs](raw)
				if err != nil {
					return nil, err
				}
				return t.s.dataSvcs().DashboardCashflow(ctx, a.query())
			},
		},
		{
			name:        "get_cashflow_basis",
			description: "Fluxo de caixa mensal dos últimos meses (receitas, contas fixas, parcelas, gasto do dia a dia, resultado e variação) e para onde foi o dinheiro no último mês. basis CASH conta compras no cartão no vencimento da fatura; ACCRUAL na data da compra. Centavos.",
			label:       "Consultando seu fluxo de caixa mensal",
			props: map[string]any{
				"months": intProp("quantidade de meses (1-12); padrão 3"),
				"basis":  enumProp("regime: CASH (padrão) ou ACCRUAL", "CASH", "ACCRUAL"),
				"month":  intProp("último mês do período (1-12); padrão: mês atual"),
				"year":   intProp("ano do último mês"),
			},
			ints: map[string]intRange{"months": {1, 12}, "month": {1, 12}, "year": {2000, 2100}},
			run: func(ctx context.Context, t *toolEnv, raw json.RawMessage) (any, error) {
				a, err := decodeArgs[struct {
					Months *int    `json:"months"`
					Basis  *string `json:"basis"`
					Month  *int    `json:"month"`
					Year   *int    `json:"year"`
				}](raw)
				if err != nil {
					return nil, err
				}
				q := domain.CashflowQuery{Basis: "CASH", Month: a.Month, Year: a.Year}
				if a.Months != nil {
					q.Months = *a.Months
				}
				if a.Basis != nil {
					q.Basis = *a.Basis
				}
				return t.s.dataSvcs().InsightsCashflow(ctx, q)
			},
		},
		{
			name:        "get_category_spending",
			description: "Despesas do mês agrupadas por categoria, com total e percentual de cada uma. Use para 'onde estou gastando mais?'. Centavos.",
			label:       "Somando seus gastos por categoria",
			props:       periodToolProps(), ints: periodInts,
			run: func(ctx context.Context, t *toolEnv, raw json.RawMessage) (any, error) {
				a, err := decodeArgs[periodArgs](raw)
				if err != nil {
					return nil, err
				}
				return t.s.dataSvcs().DashboardCategories(ctx, a.query())
			},
		},
		{
			name:        "list_budgets",
			description: "Orçamentos por categoria com gasto do mês, percentual usado e status (OK/WARNING/DANGER). Centavos.",
			label:       "Consultando seus orçamentos",
			props:       periodToolProps(), ints: periodInts,
			run: func(ctx context.Context, t *toolEnv, raw json.RawMessage) (any, error) {
				a, err := decodeArgs[periodArgs](raw)
				if err != nil {
					return nil, err
				}
				return t.s.dataSvcs().ListBudgets(ctx, a.query())
			},
		},
		{
			name:        "list_goals",
			description: "Metas financeiras (incluindo a reserva de emergência) com alvo, valor guardado, prazo, progresso e aporte mensal necessário. Também fornece o id para propose_goal_contribution. Centavos.",
			label:       "Consultando suas metas",
			props:       map[string]any{},
			run: func(ctx context.Context, t *toolEnv, _ json.RawMessage) (any, error) {
				return t.s.dataSvcs().ListGoals(ctx)
			},
		},
		{
			name:        "list_accounts",
			description: "Contas com saldo atual (centavos) e ids. Use para saber saldos por conta e para obter o accountId de uma proposta.",
			label:       "Consultando suas contas",
			props:       map[string]any{},
			run: func(ctx context.Context, t *toolEnv, _ json.RawMessage) (any, error) {
				return t.s.dataSvcs().ListAccounts(ctx, false)
			},
		},
		{
			name:        "list_cards",
			description: "Cartões de crédito com limite, limite usado e disponível, fechamento, vencimento e fatura atual. Centavos.",
			label:       "Consultando seus cartões",
			props:       map[string]any{},
			run: func(ctx context.Context, t *toolEnv, _ json.RawMessage) (any, error) {
				return t.s.dataSvcs().ListCards(ctx, false)
			},
		},
		{
			name:        "list_categories",
			description: "Categorias de receita (INCOME) e despesa (EXPENSE) com ids, para filtrar o extrato ou preencher categoryId de uma proposta.",
			label:       "Consultando suas categorias",
			props:       map[string]any{"kind": enumProp("filtra pelo tipo; padrão: todas", "INCOME", "EXPENSE")},
			run: func(ctx context.Context, t *toolEnv, raw json.RawMessage) (any, error) {
				a, err := decodeArgs[struct {
					Kind *domain.CategoryKind `json:"kind"`
				}](raw)
				if err != nil {
					return nil, err
				}
				return t.s.dataSvcs().ListCategories(ctx, a.Kind)
			},
		},
		{
			name:        "search_transactions",
			description: "Busca no extrato por texto da descrição, mês, tipo, categoria ou conta (50 por página, mais recentes primeiro). Use para perguntas sobre lançamentos específicos ('quanto gastei com Uber?').",
			label:       "Buscando no seu extrato",
			props: map[string]any{
				"search":     strProp("trecho da descrição (sem diferenciar maiúsculas)"),
				"month":      intProp("mês 1-12"),
				"year":       intProp("ano com 4 dígitos"),
				"type":       enumProp("tipo de lançamento", "INCOME", "EXPENSE", "TRANSFER", "CARD_EXPENSE", "INVOICE_PAYMENT"),
				"categoryId": strProp("id da categoria (de list_categories)"),
				"accountId":  strProp("id da conta (de list_accounts)"),
				"page":       intProp("página, a partir de 1"),
			},
			ints:   map[string]intRange{"month": {1, 12}, "year": {2000, 2100}, "page": {1, 1000}},
			maxLen: map[string]int{"search": 100, "categoryId": 36, "accountId": 36},
			run: func(ctx context.Context, t *toolEnv, raw json.RawMessage) (any, error) {
				a, err := decodeArgs[struct {
					Search     *string                 `json:"search"`
					Month      *int                    `json:"month"`
					Year       *int                    `json:"year"`
					Type       *domain.TransactionType `json:"type"`
					CategoryID *string                 `json:"categoryId"`
					AccountID  *string                 `json:"accountId"`
					Page       *int                    `json:"page"`
				}](raw)
				if err != nil {
					return nil, err
				}
				q := domain.ListTransactionsQuery{Page: 1, PageSize: 50, Search: a.Search, Month: a.Month, Year: a.Year,
					Type: a.Type, CategoryID: a.CategoryID, AccountID: a.AccountID}
				if a.Month != nil && a.Year == nil {
					_, y := domain.CurrentMonthYear()
					q.Year = &y
				}
				if a.Page != nil {
					q.Page = *a.Page
				}
				return t.s.dataSvcs().ListTransactions(ctx, q)
			},
		},
		{
			name:        "list_recurring",
			description: "Compromissos mensais: contas fixas (EXPENSE) e receitas recorrentes (INCOME) com valor, dia e próxima ocorrência ainda não lançada. Centavos.",
			label:       "Consultando seus compromissos",
			props:       map[string]any{},
			run: func(ctx context.Context, t *toolEnv, _ json.RawMessage) (any, error) {
				return t.s.dataSvcs().ListRecurring(ctx)
			},
		},
		{
			name:        "can_afford",
			description: "Simula uma compra sobre a projeção de saldo e responde se cabe: veredito YES/CAREFUL/NO, menor saldo depois da compra e quando caberia. Chame sempre que a pessoa perguntar 'posso comprar/jantar/viajar/gastar R$ X?'. amountCents é o valor total; installments para compras parceladas.",
			label:       "Simulando se cabe no seu orçamento",
			props: map[string]any{
				"amountCents":  intProp("valor total em centavos (R$ 3.000,00 = 300000)"),
				"date":         dateProp("data da compra YYYY-MM-DD; padrão: hoje"),
				"installments": intProp("número de parcelas (1-48); padrão 1"),
				"cardId":       strProp("id do cartão (de list_cards) se for no cartão de crédito"),
				"description":  strProp("o que é a compra"),
			},
			required: []string{"amountCents"},
			ints:     map[string]intRange{"amountCents": {1, 1_000_000_000_00}, "installments": {1, 48}},
			maxLen:   map[string]int{"cardId": 36, "description": 120},
			run:      runCanAfford,
		},
		{
			name:        "get_subscriptions",
			description: "Assinaturas detectadas no histórico (streaming, apps, academias...) com valor médio, cadência, próxima cobrança, aumento de preço e total mensal. Centavos.",
			label:       "Procurando suas assinaturas",
			props:       map[string]any{},
			run: func(ctx context.Context, t *toolEnv, _ json.RawMessage) (any, error) {
				return t.s.dataSvcs().Subscriptions(ctx)
			},
		},
		{
			name:        "list_investments",
			description: "Investimentos: posições com valor investido, valor atual e rentabilidade, e o resumo por tipo e instituição. Apenas leitura; não recomende investimentos. Centavos.",
			label:       "Consultando seus investimentos",
			props:       map[string]any{},
			run: func(ctx context.Context, t *toolEnv, _ json.RawMessage) (any, error) {
				items, err := t.s.dataSvcs().ListInvestments(ctx, false)
				if err != nil {
					return nil, err
				}
				summary, err := t.s.dataSvcs().InvestmentSummary(ctx)
				if err != nil {
					return nil, err
				}
				return map[string]any{"items": items, "summary": summary}, nil
			},
		},
		{
			name:        "list_loans",
			description: "Empréstimos e financiamentos com parcela, parcelas restantes, saldo devedor, juros restantes e próximo vencimento. Centavos.",
			label:       "Consultando seus empréstimos",
			props:       map[string]any{},
			run: func(ctx context.Context, t *toolEnv, _ json.RawMessage) (any, error) {
				return t.s.dataSvcs().ListLoans(ctx)
			},
		},
		{
			name:        "propose_transaction",
			description: "Prepara (não grava) um lançamento de receita ou despesa numa conta. O usuário confirma na tela. Use quando pedirem para lançar/registrar um gasto ou recebimento.",
			label:       "Preparando um lançamento para você confirmar",
			props: map[string]any{
				"type":        enumProp("INCOME = receita, EXPENSE = despesa", "INCOME", "EXPENSE"),
				"amountCents": intProp("valor em centavos, positivo"),
				"description": strProp("descrição curta do lançamento"),
				"date":        dateProp("data YYYY-MM-DD; padrão: hoje"),
				"accountId":   strProp("id da conta (de list_accounts)"),
				"categoryId":  strProp("id da categoria do mesmo tipo (de list_categories)"),
				"pending":     boolProp("true se ainda não foi pago/recebido"),
				"notes":       strProp("observação opcional"),
			},
			required: []string{"type", "amountCents", "description", "accountId"},
			ints:     map[string]intRange{"amountCents": {1, 1_000_000_000_00}},
			maxLen:   map[string]int{"description": 120, "accountId": 36, "categoryId": 36, "notes": 500},
			run:      runProposeTransaction,
		},
		{
			name:        "propose_recurring",
			description: "Prepara (não grava) uma conta fixa mensal (EXPENSE) ou receita recorrente (INCOME). O usuário confirma na tela.",
			label:       "Preparando um compromisso para você confirmar",
			props: map[string]any{
				"kind":        enumProp("INCOME = receita recorrente, EXPENSE = conta fixa", "INCOME", "EXPENSE"),
				"description": strProp("descrição (ex.: Aluguel, Salário)"),
				"amountCents": intProp("valor mensal em centavos"),
				"dayOfMonth":  intProp("dia do mês (1-31)"),
				"accountId":   strProp("id da conta (de list_accounts)"),
				"categoryId":  strProp("id da categoria do mesmo tipo (de list_categories)"),
				"startDate":   dateProp("primeira ocorrência possível YYYY-MM-DD"),
			},
			required: []string{"kind", "description", "amountCents", "dayOfMonth"},
			ints:     map[string]intRange{"amountCents": {1, 1_000_000_000_00}, "dayOfMonth": {1, 31}},
			maxLen:   map[string]int{"description": 120, "accountId": 36, "categoryId": 36},
			run:      runProposeRecurring,
		},
		{
			name:        "propose_goal_contribution",
			description: "Prepara (não grava) um aporte (valor positivo) ou resgate (valor negativo) numa meta. O usuário confirma na tela.",
			label:       "Preparando um aporte para você confirmar",
			props: map[string]any{
				"goalId":      strProp("id da meta (de list_goals)"),
				"amountCents": intProp("valor em centavos; negativo para resgate"),
				"date":        dateProp("data YYYY-MM-DD; padrão: hoje"),
				"note":        strProp("observação opcional"),
			},
			required: []string{"goalId", "amountCents"},
			ints:     map[string]intRange{"amountCents": {-1_000_000_000_00, 1_000_000_000_00}},
			maxLen:   map[string]int{"goalId": 36, "note": 200},
			run:      runProposeGoalContribution,
		},
		{
			name:        "propose_wish",
			description: "Prepara (não grava) um item na lista de desejos (compras planejadas). O usuário confirma na tela.",
			label:       "Preparando um desejo para você confirmar",
			props: map[string]any{
				"name":       strProp("nome do item"),
				"priceCents": intProp("preço em centavos"),
				"url":        strProp("link do produto (http/https)"),
				"notes":      strProp("observação opcional"),
			},
			required: []string{"name", "priceCents"},
			ints:     map[string]intRange{"priceCents": {1, 1_000_000_000_00}},
			maxLen:   map[string]int{"name": 80, "url": 500, "notes": 500},
			run:      runProposeWish,
		},
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].name < specs[j].name })
	return specs
}

func findToolSpec(name string) (toolSpec, bool) {
	for _, t := range toolSpecs() {
		if t.name == name {
			return t, true
		}
	}
	return toolSpec{}, false
}

// toolParams converte as specs para o formato da API: strict (argumentos válidos pelo schema) e
// eager_input_streaming (padrão para requisições em streaming com ferramentas do cliente — por isso a
// validação de entrada no binário em validateToolInput).
func toolParams() []anthropic.BetaToolUnionParam {
	specs := toolSpecs()
	out := make([]anthropic.BetaToolUnionParam, 0, len(specs))
	for _, spec := range specs {
		required := spec.required
		if required == nil {
			required = []string{}
		}
		tool := anthropic.BetaToolParam{
			Name:                spec.name,
			Description:         anthropic.String(spec.description),
			Strict:              anthropic.Bool(true),
			EagerInputStreaming: anthropic.Bool(true),
			InputSchema: anthropic.BetaToolInputSchemaParam{
				Properties:  spec.props,
				Required:    required,
				ExtraFields: map[string]any{"additionalProperties": false},
			},
		}
		out = append(out, anthropic.BetaToolUnionParam{OfTool: &tool})
	}
	return out
}

// toolLabel devolve o rótulo pt-BR exibido enquanto a ferramenta roda.
func toolLabel(name string) string {
	if spec, ok := findToolSpec(name); ok {
		return spec.label
	}
	return "Consultando seus dados"
}

// validateToolInput confere a entrada contra o schema da ferramenta (com eager_input_streaming a API não
// valida, e o JSON pode chegar truncado ou inválido). Devolve a mensagem do problema ("" quando válido).
func validateToolInput(spec toolSpec, raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = json.RawMessage("{}")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var args map[string]any
	if err := dec.Decode(&args); err != nil || args == nil {
		return "entrada não é um objeto JSON válido"
	}
	if dec.More() {
		return "entrada contém dados após o objeto JSON"
	}
	var problems []string
	for _, key := range spec.required {
		if v, ok := args[key]; !ok || v == nil {
			problems = append(problems, key+": obrigatório")
		}
	}
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		prop, ok := spec.props[key].(map[string]any)
		if !ok {
			problems = append(problems, key+": campo desconhecido")
			continue
		}
		val := args[key]
		if val == nil {
			continue
		}
		if msg := validateToolValue(spec, key, prop, val); msg != "" {
			problems = append(problems, key+": "+msg)
		}
	}
	return strings.Join(problems, "; ")
}

func validateToolValue(spec toolSpec, key string, prop map[string]any, val any) string {
	switch prop["type"] {
	case "string":
		s, ok := val.(string)
		if !ok {
			return "deve ser texto"
		}
		if enum, ok := prop["enum"].([]string); ok {
			for _, e := range enum {
				if s == e {
					return ""
				}
			}
			return "valor fora das opções: " + strings.Join(enum, ", ")
		}
		if prop["format"] == "date" && !domain.IsValidDateString(s) {
			return "data deve estar no formato YYYY-MM-DD"
		}
		if limit, ok := spec.maxLen[key]; ok {
			n := utf8.RuneCountInString(strings.TrimSpace(s))
			if n < 1 || n > limit {
				return fmt.Sprintf("deve ter entre 1 e %d caracteres", limit)
			}
		}
	case "integer":
		num, ok := val.(json.Number)
		if !ok {
			return "deve ser um número inteiro"
		}
		n, err := num.Int64()
		if err != nil {
			return "deve ser um número inteiro"
		}
		if r, ok := spec.ints[key]; ok && (n < r.min || n > r.max) {
			return fmt.Sprintf("deve estar entre %d e %d", r.min, r.max)
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			return "deve ser true ou false"
		}
	}
	return ""
}

// runTool valida e executa uma ferramenta, devolvendo o conteúdo do tool_result e se é erro.
func (s *Service) runTool(ctx context.Context, name string, raw json.RawMessage, emit func(string, any)) (string, bool) {
	spec, ok := findToolSpec(name)
	if !ok {
		return "ferramenta desconhecida: " + name, true
	}
	if msg := validateToolInput(spec, raw); msg != "" {
		// Mesmo formato sugerido para JSON inválido: devolve a entrada recebida para o modelo corrigir.
		b, _ := json.Marshal(map[string]string{"INVALID_INPUT": msg, "received": string(raw)})
		return string(b), true
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = json.RawMessage("{}")
	}
	data, err := spec.run(ctx, &toolEnv{s: s, emit: emit}, raw)
	if err != nil {
		return "erro: " + toolErrorMessage(err), true
	}
	if str, ok := data.(string); ok {
		return str, false
	}
	b, err := json.Marshal(data)
	if err != nil {
		return "erro: falha ao serializar o resultado", true
	}
	return string(b), false
}

func toolErrorMessage(err error) string {
	var appErr *service.AppError
	if errors.As(err, &appErr) {
		return appErr.Error()
	}
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &syntaxErr) || errors.As(err, &typeErr) {
		return "argumentos inválidos: " + err.Error()
	}
	return "falha ao consultar os dados"
}

// ───────────────────────── Projeção compacta ─────────────────────────

type compactDay struct {
	Date         string   `json:"date"`
	InCents      int64    `json:"inCents"`
	OutCents     int64    `json:"outCents"`
	BalanceCents int64    `json:"balanceCents"`
	Band         string   `json:"band"`
	Events       []string `json:"events,omitempty"`
}

// compactProjection reduz a projeção ao que o modelo precisa: dias com movimento (com os eventos
// resumidos) e os agregados. Evita mandar 365 dias de eventos detalhados.
func compactProjection(p domain.Projection) map[string]any {
	days := make([]compactDay, 0, len(p.Days))
	for i, d := range p.Days {
		if d.InCents == 0 && d.OutCents == 0 && i != 0 && i != len(p.Days)-1 {
			continue
		}
		cd := compactDay{Date: d.Date, InCents: d.InCents, OutCents: d.OutCents, BalanceCents: d.BalanceCents, Band: d.Band}
		for _, ev := range d.Events {
			if ev.Kind == domain.EventDailySpend {
				continue
			}
			cd.Events = append(cd.Events, fmt.Sprintf("%s %s %d", ev.Kind, ev.Description, ev.AmountCents))
		}
		days = append(days, cd)
	}
	return map[string]any{
		"startDate":         p.StartDate,
		"startBalanceCents": p.StartBalanceCents,
		"limitConfigured":   p.LimitConfigured,
		"lowest":            p.Lowest,
		"nextIncomeDate":    p.NextIncomeDate,
		"canSaveNowCents":   p.CanSaveNowCents,
		"months":            p.Months,
		"days":              days,
		"note":              "days lista só dias com entradas/saídas (mais o primeiro e o último); o gasto diário estimado já está em outCents",
	}
}

// ───────────────────────── can_afford ─────────────────────────

// runCanAfford usa a mesma simulação de POST /api/insights/can-afford (§8).
func runCanAfford(ctx context.Context, t *toolEnv, raw json.RawMessage) (any, error) {
	in, err := decodeArgs[domain.CanAffordInput](raw)
	if err != nil {
		return nil, err
	}
	return t.s.dataSvcs().CanAfford(ctx, in)
}

// ───────────────────────── Propostas ─────────────────────────

func issuesError(issues []domain.ValidationIssue) error {
	if len(issues) == 0 {
		return nil
	}
	return service.ValidationFailed(issues)
}

func (t *toolEnv) categoryName(ctx context.Context, id *string, kind domain.CategoryKind) (string, error) {
	if id == nil {
		return "", nil
	}
	cats, err := t.s.dataSvcs().ListCategories(ctx, &kind)
	if err != nil {
		return "", err
	}
	for _, c := range cats {
		if c.ID == *id {
			return c.Name, nil
		}
	}
	return "", service.NotFound("categoria não encontrada para este tipo; consulte list_categories")
}

func (t *toolEnv) propose(kind, summary string, payload any) string {
	t.emit("proposal", Proposal{ID: uuid.NewString(), Kind: kind, Summary: summary, Payload: payload})
	return proposalSentMessage
}

func formatDateBR(date string) string {
	if len(date) != 10 {
		return date
	}
	return date[8:10] + "/" + date[5:7] + "/" + date[0:4]
}

func runProposeTransaction(ctx context.Context, t *toolEnv, raw json.RawMessage) (any, error) {
	a, err := decodeArgs[struct {
		Type        domain.TransactionType `json:"type"`
		AmountCents int64                  `json:"amountCents"`
		Description string                 `json:"description"`
		Date        *string                `json:"date"`
		AccountID   string                 `json:"accountId"`
		CategoryID  *string                `json:"categoryId"`
		Pending     *bool                  `json:"pending"`
		Notes       *string                `json:"notes"`
	}](raw)
	if err != nil {
		return nil, err
	}
	in := domain.CreateTransactionInput{
		Type: a.Type, Description: strings.TrimSpace(a.Description), AmountCents: a.AmountCents,
		Date: domain.TodayString(), Status: domain.TxConfirmed, AccountID: a.AccountID,
		CategoryID: a.CategoryID, Notes: a.Notes,
	}
	if a.Date != nil {
		in.Date = *a.Date
	}
	if a.Pending != nil && *a.Pending {
		in.Status = domain.TxPending
	}
	if err := issuesError(domain.ValidateCreateTransaction(in)); err != nil {
		return nil, err
	}
	acc, err := t.s.dataSvcs().GetAccount(ctx, in.AccountID)
	if err != nil {
		return nil, err
	}
	kind := domain.CategoryExpense
	label := "Despesa"
	if in.Type == domain.TxIncome {
		kind = domain.CategoryIncome
		label = "Receita"
	}
	catName, err := t.categoryName(ctx, in.CategoryID, kind)
	if err != nil {
		return nil, err
	}
	summary := fmt.Sprintf("%s de %s — %s, em %s (%s)", label, domain.FormatBRL(in.AmountCents), in.Description, formatDateBR(in.Date), acc.Name)
	if catName != "" {
		summary += " · " + catName
	}
	if in.Status == domain.TxPending {
		summary += " · pendente"
	}
	return t.propose(ProposalCreateTransaction, summary, in), nil
}

func runProposeRecurring(ctx context.Context, t *toolEnv, raw json.RawMessage) (any, error) {
	in, err := decodeArgs[domain.CreateRecurringInput](raw)
	if err != nil {
		return nil, err
	}
	in.Description = strings.TrimSpace(in.Description)
	if err := issuesError(domain.ValidateCreateRecurring(in)); err != nil {
		return nil, err
	}
	accName := ""
	if in.AccountID != nil {
		acc, err := t.s.dataSvcs().GetAccount(ctx, *in.AccountID)
		if err != nil {
			return nil, err
		}
		accName = acc.Name
	}
	catName, err := t.categoryName(ctx, in.CategoryID, in.Kind)
	if err != nil {
		return nil, err
	}
	label := "Conta fixa"
	if in.Kind == domain.CategoryIncome {
		label = "Receita recorrente"
	}
	summary := fmt.Sprintf("%s: %s de %s todo dia %d", label, in.Description, domain.FormatBRL(in.AmountCents), in.DayOfMonth)
	if accName != "" {
		summary += " · " + accName
	}
	if catName != "" {
		summary += " · " + catName
	}
	return t.propose(ProposalCreateRecurring, summary, in), nil
}

// goalContributionPayload é o corpo de POST /api/goals/{goalId}/contributions mais o goalId da rota.
type goalContributionPayload struct {
	GoalID string `json:"goalId"`
	domain.ContributeGoalInput
}

func runProposeGoalContribution(ctx context.Context, t *toolEnv, raw json.RawMessage) (any, error) {
	a, err := decodeArgs[struct {
		GoalID      string  `json:"goalId"`
		AmountCents int64   `json:"amountCents"`
		Date        *string `json:"date"`
		Note        *string `json:"note"`
	}](raw)
	if err != nil {
		return nil, err
	}
	in := domain.ContributeGoalInput{AmountCents: a.AmountCents, Date: a.Date, Note: a.Note}
	if err := issuesError(domain.ValidateContributeGoal(in)); err != nil {
		return nil, err
	}
	if !domain.IsValidUUID(a.GoalID) {
		return nil, service.BadRequest("goalId deve ser o id de uma meta (consulte list_goals)")
	}
	goal, err := t.s.dataSvcs().GetGoal(ctx, a.GoalID)
	if err != nil {
		return nil, err
	}
	verb := "Aporte"
	amount := in.AmountCents
	if amount < 0 {
		verb = "Resgate"
		amount = -amount
	}
	summary := fmt.Sprintf("%s de %s na meta %s", verb, domain.FormatBRL(amount), goal.Name)
	return t.propose(ProposalContributeGoal, summary, goalContributionPayload{GoalID: goal.ID, ContributeGoalInput: in}), nil
}

func runProposeWish(_ context.Context, t *toolEnv, raw json.RawMessage) (any, error) {
	in, err := decodeArgs[domain.CreateWishItemInput](raw)
	if err != nil {
		return nil, err
	}
	in.Name = strings.TrimSpace(in.Name)
	if err := issuesError(domain.ValidateCreateWishItem(in)); err != nil {
		return nil, err
	}
	if in.URL != nil && !strings.HasPrefix(*in.URL, "http://") && !strings.HasPrefix(*in.URL, "https://") {
		return nil, service.BadRequest("url deve começar com http:// ou https://")
	}
	summary := fmt.Sprintf("Desejo: %s — %s", in.Name, domain.FormatBRL(in.PriceCents))
	return t.propose(ProposalAddWish, summary, in), nil
}
