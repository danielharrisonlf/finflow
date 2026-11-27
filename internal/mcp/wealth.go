package mcp

import (
	"context"
	"encoding/json"
)

// ───────────────────────── Tools: investimentos, dívidas e patrimônio (v3 §13) ─────────────────────────

func wealthTools() []toolDef {
	return []toolDef{
		{Name: "list_investments", Description: "Lista as posições de investimento com valor investido, valor atual, rentabilidade (centavos e %), conta vinculada e data da última atualização de valor.", InputSchema: objSchema(map[string]any{
			"includeArchived": map[string]any{"type": "boolean", "description": "inclui posições arquivadas; padrão false"},
		})},
		{Name: "get_investment_summary", Description: "Resumo dos investimentos: total investido e atual, rentabilidade, alocação por tipo e por instituição e evolução dos últimos 12 meses (snapshots). Centavos.", InputSchema: objSchema(map[string]any{})},
		{Name: "list_loans", Description: "Lista empréstimos e financiamentos com taxa (% a.m.), parcela, parcelas pagas/restantes, próximo vencimento, saldo devedor (valor presente, tabela Price) e juros restantes. Centavos.", InputSchema: objSchema(map[string]any{})},
		{Name: "get_debt_strategy", Description: "Compara a quitação das dívidas ativas: avalanche (maior taxa primeiro), bola de neve (menor saldo primeiro) e só parcelas, com meses, juros totais e data de quitação de cada uma.", InputSchema: objSchema(map[string]any{
			"extraCents": map[string]any{"type": "integer", "minimum": 0, "description": "valor extra por mês além das parcelas, em centavos; padrão 0"},
		})},
		{Name: "get_net_worth_history", Description: "Evolução do patrimônio no fim de cada mês: caixa (contas não-investimento), investimentos, dívidas (faturas vencidas não pagas + saldo devedor de empréstimos) e patrimônio líquido. Centavos.", InputSchema: objSchema(map[string]any{
			"months": map[string]any{"type": "integer", "minimum": 1, "maximum": 60, "description": "meses devolvidos, terminando no mês atual; padrão 12"},
		})},
	}
}

// callWealthTool executa as tools de wealthTools; handled=false para nomes de outras tools.
func (s *Server) callWealthTool(ctx context.Context, name string, rawArgs json.RawMessage) (data any, handled bool, err error) {
	switch name {
	case "list_investments":
		var a struct {
			IncludeArchived bool `json:"includeArchived"`
		}
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			data, err = s.backend.Investments(ctx, a.IncludeArchived)
		}
	case "get_investment_summary":
		data, err = s.backend.InvestmentSummary(ctx)
	case "list_loans":
		data, err = s.backend.Loans(ctx)
	case "get_debt_strategy":
		var a struct {
			ExtraCents int64 `json:"extraCents"`
		}
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			data, err = s.backend.DebtStrategy(ctx, a.ExtraCents)
		}
	case "get_net_worth_history":
		var a struct {
			Months int `json:"months"`
		}
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			data, err = s.backend.NetWorthHistory(ctx, a.Months)
		}
	default:
		return nil, false, nil
	}
	return data, true, err
}
