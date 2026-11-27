package service

import (
	"context"
	"fmt"
	"strings"

	"finflow/internal/domain"
)

// ───────────────────────── Casa: membros e divisão de despesas (v3 fase 2 §4) ─────────────────────────

func (s *Services) ListMembers(ctx context.Context) ([]domain.Member, error) {
	return s.Store.ListMembers(ctx)
}

func (s *Services) getMember(ctx context.Context, id string) (domain.Member, error) {
	m, err := s.Store.FindMemberByID(ctx, id)
	if err != nil {
		return domain.Member{}, err
	}
	if m == nil {
		return domain.Member{}, NotFound("membro não encontrado")
	}
	return *m, nil
}

// memberNameTaken confere o nome sem diferenciar maiúsculas/acentos (ignorando o próprio membro).
func (s *Services) memberNameTaken(ctx context.Context, name, exceptID string) (bool, error) {
	members, err := s.Store.ListMembers(ctx)
	if err != nil {
		return false, err
	}
	norm := domain.NormalizeText(name)
	for _, m := range members {
		if m.ID != exceptID && domain.NormalizeText(m.Name) == norm {
			return true, nil
		}
	}
	return false, nil
}

func (s *Services) CreateMember(ctx context.Context, in domain.CreateMemberInput) (domain.Member, error) {
	if issues := domain.ValidateCreateMember(in); len(issues) > 0 {
		return domain.Member{}, ValidationFailed(issues)
	}
	name := strings.TrimSpace(in.Name)
	taken, err := s.memberNameTaken(ctx, name, "")
	if err != nil {
		return domain.Member{}, err
	}
	if taken {
		return domain.Member{}, Conflict(fmt.Sprintf("membro %q já existe", name))
	}
	color := domain.DefaultMemberColor
	if in.Color != nil {
		color = *in.Color
	}
	id, err := s.Store.CreateMember(ctx, name, color)
	if err != nil {
		return domain.Member{}, err
	}
	return s.getMember(ctx, id)
}

func (s *Services) UpdateMember(ctx context.Context, id string, in domain.UpdateMemberInput) (domain.Member, error) {
	if issues := domain.ValidateUpdateMember(in); len(issues) > 0 {
		return domain.Member{}, ValidationFailed(issues)
	}
	cur, err := s.getMember(ctx, id)
	if err != nil {
		return domain.Member{}, err
	}
	if in.Name.Set {
		name := strings.TrimSpace(in.Name.Value)
		taken, err := s.memberNameTaken(ctx, name, id)
		if err != nil {
			return domain.Member{}, err
		}
		if taken {
			return domain.Member{}, Conflict(fmt.Sprintf("membro %q já existe", name))
		}
		cur.Name = name
	}
	if in.Color.Set {
		cur.Color = in.Color.Value
	}
	if err := s.Store.UpdateMember(ctx, cur); err != nil {
		return domain.Member{}, err
	}
	return s.getMember(ctx, id)
}

// DeleteMember apaga o membro: as divisões e os acertos dele somem e as despesas que ele pagou
// ficam sem "quem pagou".
func (s *Services) DeleteMember(ctx context.Context, id string) error {
	if _, err := s.getMember(ctx, id); err != nil {
		return err
	}
	return s.Store.DeleteMember(ctx, id)
}

// requireSharing garante que quem pagou e os membros da divisão existem (404 no primeiro ausente).
func (s *Services) requireSharing(ctx context.Context, paidBy *string, splits []domain.TransactionSplit) error {
	if paidBy == nil && len(splits) == 0 {
		return nil
	}
	members, err := s.Store.ListMembers(ctx)
	if err != nil {
		return err
	}
	exists := make(map[string]bool, len(members))
	for _, m := range members {
		exists[m.ID] = true
	}
	if paidBy != nil && !exists[*paidBy] {
		return NotFound("membro (quem pagou) não encontrado")
	}
	for _, sp := range splits {
		if !exists[sp.MemberID] {
			return NotFound("membro da divisão não encontrado")
		}
	}
	return nil
}

// MemberBalances devolve, para o mês, quanto cada membro pagou, a parte dele nas despesas com
// "quem pagou" (confirmadas; sem divisão = partes iguais entre todos os membros) e os acertos
// sugeridos. Acertos registrados no mês abatem os saldos.
func (s *Services) MemberBalances(ctx context.Context, q domain.PeriodQuery) (domain.MemberBalances, error) {
	month, year, err := resolvePeriod(q)
	if err != nil {
		return domain.MemberBalances{}, err
	}
	members, err := s.Store.ListMembers(ctx)
	if err != nil {
		return domain.MemberBalances{}, err
	}
	start, end := domain.MonthRange(year, month)
	expenses, err := s.Store.SharedExpensesBetween(ctx, start, end)
	if err != nil {
		return domain.MemberBalances{}, err
	}
	settled, err := s.Store.SettlementsBetween(ctx, start, end)
	if err != nil {
		return domain.MemberBalances{}, err
	}
	records := make([]domain.SettlementRecord, 0, len(settled))
	for _, st := range settled {
		records = append(records, domain.SettlementRecord{FromMemberID: st.FromMemberID, ToMemberID: st.ToMemberID, AmountCents: st.AmountCents})
	}
	balances := domain.ComputeMemberBalances(members, expenses, records)
	return domain.MemberBalances{Members: balances, Settlements: domain.SuggestSettlements(balances)}, nil
}

// Settle registra um acerto entre membros. Com as duas contas, também lança a transferência
// entre elas (o registro de acerto guarda o grupo da transferência).
func (s *Services) Settle(ctx context.Context, in domain.SettleInput) (domain.Settlement, error) {
	if issues := domain.ValidateSettle(in); len(issues) > 0 {
		return domain.Settlement{}, ValidationFailed(issues)
	}
	from, err := s.getMember(ctx, in.FromMemberID)
	if err != nil {
		return domain.Settlement{}, err
	}
	to, err := s.getMember(ctx, in.ToMemberID)
	if err != nil {
		return domain.Settlement{}, err
	}
	date := domain.TodayString()
	if in.Date != nil {
		date = *in.Date
	}
	var groupID *string
	if in.FromAccountID != nil && in.ToAccountID != nil {
		desc := fmt.Sprintf("Acerto %s → %s", from.Name, to.Name)
		res, err := s.Transfer(ctx, domain.TransferInput{
			FromAccountID: *in.FromAccountID,
			ToAccountID:   *in.ToAccountID,
			AmountCents:   in.AmountCents,
			Date:          date,
			Description:   &desc,
		})
		if err != nil {
			return domain.Settlement{}, err
		}
		groupID = &res.TransferGroupID
	}
	id, err := s.Store.CreateSettlement(ctx, from.ID, to.ID, in.AmountCents, date, groupID)
	if err != nil {
		if groupID != nil {
			_ = s.Store.DeleteTransferGroup(ctx, *groupID)
		}
		return domain.Settlement{}, err
	}
	return domain.Settlement{
		ID:              id,
		FromMemberID:    from.ID,
		ToMemberID:      to.ID,
		AmountCents:     in.AmountCents,
		Date:            date,
		TransferGroupID: groupID,
	}, nil
}
