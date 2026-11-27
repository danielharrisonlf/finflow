package mcp

import (
	"context"
	"encoding/json"

	"finflow/internal/domain"
)

// ───────────────────────── Tools: divisão (Casa) e empresa (v3 fase 2) ─────────────────────────
// Operam no espaço do servidor MCP (`finflow mcp --espaco <slug>` / FINFLOW_SPACE ou o cabeçalho
// X-FinFlow-Space no MCP via HTTP).

func spacesTools() []toolDef {
	return []toolDef{
		{Name: "list_members", Description: "Lista os membros do espaço (espaço Casa: pessoas que dividem as despesas), com id, nome e cor.", InputSchema: objSchema(map[string]any{})},
		{Name: "get_member_balances", Description: "Divisão de despesas do mês no espaço Casa: quanto cada membro pagou, a parte dele (despesas confirmadas com quem pagou; sem divisão = partes iguais), saldo (> 0 tem a receber) e o mínimo de acertos sugeridos. Centavos.", InputSchema: objSchema(periodProps())},
		{Name: "get_business_metrics", Description: "Métricas do espaço Empresa no mês: MRR (receitas recorrentes ativas no fim do mês), ARR, novo MRR, MRR perdido, churn (%), queima média dos 3 meses anteriores, caixa e runway (meses). Centavos.", InputSchema: objSchema(periodProps())},
		{Name: "get_business_dre", Description: "DRE anual do espaço Empresa, mês a mês e total: receita, deduções, receita líquida, custos, lucro bruto, despesas operacionais, resultado operacional, financeiro, outros, resultado líquido e margem (%). Centavos.", InputSchema: objSchema(map[string]any{
			"year": map[string]any{"type": "integer", "description": "ano; padrão: ano atual"},
		})},
	}
}

// callSpacesTool executa as tools de spacesTools; handled=false para nomes de outras tools.
func (s *Server) callSpacesTool(ctx context.Context, name string, rawArgs json.RawMessage) (data any, handled bool, err error) {
	switch name {
	case "list_members":
		data, err = s.backend.Members(ctx)
	case "get_member_balances":
		var a struct{ Month, Year *int }
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			data, err = s.backend.MemberBalances(ctx, domain.PeriodQuery{Month: a.Month, Year: a.Year})
		}
	case "get_business_metrics":
		var a struct{ Month, Year *int }
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			data, err = s.backend.BusinessMetrics(ctx, domain.PeriodQuery{Month: a.Month, Year: a.Year})
		}
	case "get_business_dre":
		var a struct {
			Year *int `json:"year"`
		}
		if err = json.Unmarshal(rawArgs, &a); err == nil {
			data, err = s.backend.BusinessDRE(ctx, a.Year)
		}
	default:
		return nil, false, nil
	}
	return data, true, err
}
