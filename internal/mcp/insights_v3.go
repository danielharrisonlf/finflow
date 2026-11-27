package mcp

import (
	"context"
	"encoding/json"
	"strings"

	"finflow/internal/domain"
)

// ───────────────────────── Tools: tags, assinaturas, alertas, "posso comprar?" e fluxo de caixa (v3 §13) ─────────────────────────

func insightsV3Tools() []toolDef {
	cashflowProps := periodProps()
	cashflowProps["months"] = map[string]any{"type": "integer", "minimum": 1, "maximum": 12, "description": "meses devolvidos, terminando no mês informado (padrão: atual); padrão 3"}
	cashflowProps["basis"] = map[string]any{"type": "string", "enum": []string{"CASH", "ACCRUAL"}, "description": "CASH (padrão): compra no cartão conta no vencimento da fatura; ACCRUAL: na data da compra"}

	return []toolDef{
		{Name: "list_tags", Description: "Lista as tags (etiquetas livres dos lançamentos) com cor e quantidade de lançamentos marcados.", InputSchema: objSchema(map[string]any{})},
		{Name: "get_subscriptions", Description: "Assinaturas detectadas no histórico (cobranças semanais, mensais ou anuais de valor estável): valor médio e último, próxima cobrança prevista, variação de preço e se já são acompanhadas como conta fixa; mais o total mensal normalizado. Centavos.", InputSchema: objSchema(map[string]any{})},
		{Name: "get_alerts", Description: "Alertas e dicas do mês: orçamentos estourando, ritmo acima do limite ou do mês passado, saldo projetado negativo, contas/faturas/parcelas vencendo em até 3 dias, assinaturas novas ou mais caras, reserva baixa, metas atrasadas, pendências antigas e lançamentos sem categoria.", InputSchema: objSchema(map[string]any{})},
		{Name: "can_afford", Description: "Simula uma compra sobre a projeção de saldo e responde se cabe (YES / CAREFUL / NO), com o menor saldo projetado depois da compra, o impacto no limite mensal, quando caberia e quanto sobraria da reserva. Com cartão, as parcelas caem nas faturas pelo ciclo do cartão. Centavos.", InputSchema: objSchema(map[string]any{
			"amountCents":  map[string]any{"type": "integer", "minimum": 1, "description": "valor TOTAL da compra em centavos"},
			"date":         map[string]any{"type": "string", "pattern": `^\d{4}-\d{2}-\d{2}$`, "description": "data da compra YYYY-MM-DD; padrão: hoje"},
			"installments": map[string]any{"type": "integer", "minimum": 1, "maximum": 48, "description": "parcelas (só com cartão); padrão 1"},
			"card":         map[string]any{"type": "string", "description": "nome ou id do cartão; omitido = sai do saldo disponível"},
			"description":  map[string]any{"type": "string", "description": "o que vai comprar"},
		}, "amountCents")},
		{Name: "get_cashflow_basis", Description: "Fluxo de caixa mensal por regime de caixa (CASH) ou competência (ACCRUAL): receitas, contas fixas (recorrências e empréstimos), parcelas, dia a dia, resultado e variação vs. mês anterior, mais para onde foi o dinheiro no último mês. Centavos.", InputSchema: objSchema(cashflowProps)},
	}
}

// callInsightsV3Tool executa as tools de insightsV3Tools; handled=false para nomes de outras tools.
func (s *Server) callInsightsV3Tool(ctx context.Context, name string, rawArgs json.RawMessage) (data any, handled bool, err error) {
	switch name {
	case "list_tags":
		data, err = s.backend.Tags(ctx)
	case "get_subscriptions":
		data, err = s.backend.Subscriptions(ctx)
	case "get_alerts":
		data, err = s.backend.Alerts(ctx)
	case "can_afford":
		var a struct {
			AmountCents  int64   `json:"amountCents"`
			Date         *string `json:"date"`
			Installments *int    `json:"installments"`
			Card         string  `json:"card"`
			Description  *string `json:"description"`
		}
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			in := domain.CanAffordInput{AmountCents: a.AmountCents, Date: a.Date, Installments: a.Installments, Description: a.Description}
			if strings.TrimSpace(a.Card) != "" {
				var card domain.Card
				if card, err = s.cardByRef(ctx, a.Card); err == nil {
					in.CardID = &card.ID
				}
			}
			if err == nil {
				data, err = s.backend.CanAfford(ctx, in)
			}
		}
	case "get_cashflow_basis":
		var a struct {
			Months int    `json:"months"`
			Basis  string `json:"basis"`
			Month  *int   `json:"month"`
			Year   *int   `json:"year"`
		}
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			data, err = s.backend.InsightsCashflow(ctx, domain.CashflowQuery{Months: a.Months, Basis: a.Basis, Month: a.Month, Year: a.Year})
		}
	default:
		return nil, false, nil
	}
	return data, true, err
}
