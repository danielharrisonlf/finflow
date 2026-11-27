package service

import (
	"context"
	"fmt"

	"finflow/internal/domain"
	"finflow/internal/store"
)

// ───────────────────────── Empréstimos e financiamentos ─────────────────────────

// loanState junta a linha do empréstimo com as parcelas pagas (base + pagamentos lançados).
type loanState struct {
	row      store.LoanRow
	payments []string // datas dos pagamentos lançados (ordenadas)
}

func (l loanState) paid() int {
	return min(l.row.TotalInstallments, l.row.BasePaid+len(l.payments))
}

func (l loanState) schedule() domain.LoanSchedule {
	return domain.LoanSchedule{
		InstallmentCents:  l.row.InstallmentCents,
		RateBP:            l.row.RateBP,
		TotalInstallments: l.row.TotalInstallments,
		PaidInstallments:  l.paid(),
		FirstDueDate:      l.row.FirstDueDate,
		Active:            l.row.Active,
	}
}

// paidAt é quantas parcelas estavam pagas na data (as informadas no cadastro contam desde sempre).
func (l loanState) paidAt(date string) int {
	n := l.row.BasePaid
	for _, d := range l.payments {
		if d <= date {
			n++
		}
	}
	return min(l.row.TotalInstallments, n)
}

func (l loanState) view() domain.Loan {
	sch := l.schedule()
	r := l.row
	remaining := sch.Remaining()
	outstanding := domain.LoanOutstandingCents(r.InstallmentCents, r.RateBP, remaining)
	return domain.Loan{
		ID:                          r.ID,
		Name:                        r.Name,
		Lender:                      r.Lender,
		Kind:                        r.Kind,
		PrincipalCents:              r.PrincipalCents,
		MonthlyRatePct:              domain.RateBPToPct(r.RateBP),
		InstallmentCents:            r.InstallmentCents,
		TotalInstallments:           r.TotalInstallments,
		PaidInstallments:            sch.PaidInstallments,
		RemainingInstallments:       remaining,
		FirstDueDate:                r.FirstDueDate,
		NextDueDate:                 domain.LoanNextDueDate(r.FirstDueDate, sch.PaidInstallments, r.TotalInstallments, r.Active),
		OutstandingCents:            outstanding,
		TotalInterestRemainingCents: max(0, r.InstallmentCents*int64(remaining)-outstanding),
		AccountID:                   r.AccountID,
		CategoryID:                  r.CategoryID,
		Active:                      r.Active,
		Notes:                       r.Notes,
	}
}

// loanStates carrega todos os empréstimos com seus pagamentos.
func (s *Services) loanStates(ctx context.Context) ([]loanState, error) {
	rows, err := s.Store.ListLoans(ctx)
	if err != nil {
		return nil, err
	}
	payments, err := s.Store.LoanPaymentDates(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]loanState, 0, len(rows))
	for _, r := range rows {
		out = append(out, loanState{row: r, payments: payments[r.ID]})
	}
	return out, nil
}

func (s *Services) loanStateByID(ctx context.Context, id string) (loanState, error) {
	r, err := s.Store.FindLoanByID(ctx, id)
	if err != nil {
		return loanState{}, err
	}
	if r == nil {
		return loanState{}, NotFound("empréstimo não encontrado")
	}
	payments, err := s.Store.LoanPaymentDates(ctx)
	if err != nil {
		return loanState{}, err
	}
	return loanState{row: *r, payments: payments[r.ID]}, nil
}

// ListLoans lista os empréstimos com parcelas pagas, próximo vencimento e saldo devedor.
func (s *Services) ListLoans(ctx context.Context) ([]domain.Loan, error) {
	states, err := s.loanStates(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Loan, 0, len(states))
	for _, l := range states {
		out = append(out, l.view())
	}
	return out, nil
}

func (s *Services) GetLoan(ctx context.Context, id string) (domain.Loan, error) {
	l, err := s.loanStateByID(ctx, id)
	if err != nil {
		return domain.Loan{}, err
	}
	return l.view(), nil
}

// validateLoanRefs confere conta (existente e ativa) e categoria (existente e de despesa).
func (s *Services) validateLoanRefs(ctx context.Context, accountID, categoryID *string) error {
	if accountID != nil {
		if err := s.requireActiveAccount(ctx, *accountID); err != nil {
			return err
		}
	}
	if categoryID != nil {
		cat, err := s.Store.FindCategoryByID(ctx, *categoryID)
		if err != nil {
			return err
		}
		if cat == nil {
			return NotFound("categoria não encontrada")
		}
		if cat.Kind != domain.CategoryExpense {
			return BadRequest("a categoria do empréstimo deve ser de despesa")
		}
	}
	return nil
}

func (s *Services) CreateLoan(ctx context.Context, in domain.CreateLoanInput) (domain.Loan, error) {
	if issues := domain.ValidateCreateLoan(in); len(issues) > 0 {
		return domain.Loan{}, ValidationFailed(issues)
	}
	if err := s.validateLoanRefs(ctx, in.AccountID, in.CategoryID); err != nil {
		return domain.Loan{}, err
	}
	rateBP := domain.RatePctToBP(in.MonthlyRatePct)
	installment := domain.PriceInstallment(in.PrincipalCents, rateBP, in.TotalInstallments)
	if in.InstallmentCents != nil {
		installment = *in.InstallmentCents
	}
	id, err := s.Store.CreateLoan(ctx, store.LoanRow{
		Name:              in.Name,
		Lender:            in.Lender,
		Kind:              in.Kind,
		PrincipalCents:    in.PrincipalCents,
		RateBP:            rateBP,
		InstallmentCents:  installment,
		TotalInstallments: in.TotalInstallments,
		BasePaid:          in.PaidInstallments,
		FirstDueDate:      in.FirstDueDate,
		AccountID:         in.AccountID,
		CategoryID:        in.CategoryID,
		Active:            true,
		Notes:             in.Notes,
	})
	if err != nil {
		return domain.Loan{}, err
	}
	return s.GetLoan(ctx, id)
}

func (s *Services) UpdateLoan(ctx context.Context, id string, in domain.UpdateLoanInput) (domain.Loan, error) {
	if issues := domain.ValidateUpdateLoan(in); len(issues) > 0 {
		return domain.Loan{}, ValidationFailed(issues)
	}
	l, err := s.loanStateByID(ctx, id)
	if err != nil {
		return domain.Loan{}, err
	}
	r := &l.row
	if in.Name.Set {
		r.Name = in.Name.Value
	}
	if in.Lender.Set {
		r.Lender = optionalString(in.Lender)
	}
	if in.Kind.Set {
		r.Kind = in.Kind.Value
	}
	if in.PrincipalCents.Set {
		r.PrincipalCents = in.PrincipalCents.Value
	}
	if in.MonthlyRatePct.Set {
		r.RateBP = domain.RatePctToBP(in.MonthlyRatePct.Value)
	}
	if in.InstallmentCents.Set {
		r.InstallmentCents = in.InstallmentCents.Value
	}
	if in.TotalInstallments.Set {
		r.TotalInstallments = in.TotalInstallments.Value
	}
	if in.FirstDueDate.Set {
		r.FirstDueDate = in.FirstDueDate.Value
	}
	if in.AccountID.Set {
		r.AccountID = optionalString(in.AccountID)
	}
	if in.CategoryID.Set {
		r.CategoryID = optionalString(in.CategoryID)
	}
	if in.Active.Set {
		r.Active = in.Active.Value
	}
	if in.Notes.Set {
		r.Notes = optionalString(in.Notes)
	}
	// paidInstallments é o total pago; os pagamentos lançados no app já contam, então só a
	// diferença fica como "pagas fora do FinFlow".
	launched := len(l.payments)
	if in.PaidInstallments.Set {
		if in.PaidInstallments.Value < launched {
			return domain.Loan{}, ValidationFailed([]domain.ValidationIssue{{Path: "paidInstallments", Message: fmt.Sprintf("já existem %d pagamentos lançados para este empréstimo", launched)}})
		}
		r.BasePaid = in.PaidInstallments.Value - launched
	}
	if r.BasePaid+launched > r.TotalInstallments {
		return domain.Loan{}, ValidationFailed([]domain.ValidationIssue{{Path: "paidInstallments", Message: "parcelas pagas deve ser entre 0 e o total"}})
	}
	var accountID, categoryID *string
	if in.AccountID.Set {
		accountID = r.AccountID
	}
	if in.CategoryID.Set {
		categoryID = r.CategoryID
	}
	if err := s.validateLoanRefs(ctx, accountID, categoryID); err != nil {
		return domain.Loan{}, err
	}
	if err := s.Store.UpdateLoan(ctx, r); err != nil {
		return domain.Loan{}, err
	}
	return s.GetLoan(ctx, id)
}

// DeleteLoan apaga o empréstimo; os pagamentos lançados continuam como despesas comuns.
func (s *Services) DeleteLoan(ctx context.Context, id string) error {
	if _, err := s.loanStateByID(ctx, id); err != nil {
		return err
	}
	return s.Store.DeleteLoan(ctx, id)
}

// loanCategoryID devolve a categoria do pagamento: a do empréstimo ou a padrão
// "Empréstimos e financiamentos" (FIXED), criada se tiver sido apagada.
func (s *Services) loanCategoryID(ctx context.Context, r store.LoanRow) (*string, error) {
	if r.CategoryID != nil {
		return r.CategoryID, nil
	}
	cat, err := s.Store.FindCategoryByName(ctx, domain.LoanCategoryName, domain.CategoryExpense)
	if err != nil {
		return nil, err
	}
	if cat == nil {
		fixed := domain.SpendFixed
		cat, err = s.Store.CreateCategory(ctx, domain.CreateCategoryInput{
			Name: domain.LoanCategoryName, Kind: domain.CategoryExpense, Icon: "landmark", Color: "#ef4444", SpendType: &fixed,
		})
		if err != nil {
			return nil, err
		}
	}
	id := cat.ID
	return &id, nil
}

// PayLoan lança o pagamento da próxima parcela como despesa confirmada ligada ao empréstimo.
func (s *Services) PayLoan(ctx context.Context, id string, in domain.PayLoanInput) (domain.Transaction, error) {
	if issues := domain.ValidatePayLoan(in); len(issues) > 0 {
		return domain.Transaction{}, ValidationFailed(issues)
	}
	l, err := s.loanStateByID(ctx, id)
	if err != nil {
		return domain.Transaction{}, err
	}
	if !l.row.Active {
		return domain.Transaction{}, BadRequest("empréstimo inativo")
	}
	paid := l.paid()
	if paid >= l.row.TotalInstallments {
		return domain.Transaction{}, Conflict("todas as parcelas deste empréstimo já foram pagas")
	}
	accountID := l.row.AccountID
	if in.AccountID != nil {
		accountID = in.AccountID
	}
	if accountID == nil {
		return domain.Transaction{}, ValidationFailed([]domain.ValidationIssue{{Path: "accountId", Message: "informe a conta para pagar a parcela"}})
	}
	if err := s.requireActiveAccount(ctx, *accountID); err != nil {
		return domain.Transaction{}, err
	}
	categoryID, err := s.loanCategoryID(ctx, l.row)
	if err != nil {
		return domain.Transaction{}, err
	}
	amount := l.row.InstallmentCents
	if in.AmountCents != nil {
		amount = *in.AmountCents
	}
	date := dateOrToday(in.Date)
	desc := fmt.Sprintf("%s — parcela %d/%d", l.row.Name, paid+1, l.row.TotalInstallments)
	created, err := s.Store.CreateLoanPayment(ctx, l.row.ID, desc, amount, date, *accountID, categoryID)
	if err != nil {
		return domain.Transaction{}, err
	}
	if categoryID != nil && domain.IsInCurrentMonth(date) {
		catID := *categoryID
		go s.Notifications.NotifyBudgetThresholdCrossed(context.Background(), catID, amount)
	}
	return *created, nil
}

// DebtStrategy simula a quitação dos empréstimos ativos (avalanche, bola de neve e só parcelas)
// com extraCents por mês além das parcelas.
func (s *Services) DebtStrategy(ctx context.Context, extraCents int64) (domain.DebtStrategy, error) {
	if extraCents < 0 || extraCents > domain.MaxParsedCents {
		return domain.DebtStrategy{}, ValidationFailed([]domain.ValidationIssue{{Path: "extraCents", Message: "extra deve ser um valor em centavos >= 0"}})
	}
	states, err := s.loanStates(ctx)
	if err != nil {
		return domain.DebtStrategy{}, err
	}
	var debts []domain.DebtInput
	for _, l := range states {
		v := l.view()
		if !v.Active || v.NextDueDate == nil || v.OutstandingCents <= 0 {
			continue
		}
		debts = append(debts, domain.DebtInput{
			ID:               v.ID,
			BalanceCents:     v.OutstandingCents,
			RateBP:           l.row.RateBP,
			InstallmentCents: v.InstallmentCents,
			Remaining:        v.RemainingInstallments,
			NextDueDate:      *v.NextDueDate,
		})
	}
	return domain.SimulateDebtStrategies(debts, extraCents), nil
}

// ───────────────────────── Integração com projeção e compromissos ─────────────────────────

// loanProjectionInputs devolve as parcelas em aberto dos empréstimos ativos (eventos "LOAN") e a
// soma das parcelas (pagas ou não) por mês de vencimento, que consome o limite mensal.
func (s *Services) loanProjectionInputs(ctx context.Context) ([]domain.ProjectionLoan, map[string]int64, error) {
	states, err := s.loanStates(ctx)
	if err != nil {
		return nil, nil, err
	}
	var loans []domain.ProjectionLoan
	byMonth := make(map[string]int64)
	for _, l := range states {
		if !l.row.Active {
			continue
		}
		sch := l.schedule()
		for k := 1; k <= l.row.TotalInstallments; k++ {
			byMonth[domain.MonthKey(domain.LoanDueDate(l.row.FirstDueDate, k))] += l.row.InstallmentCents
		}
		if dates := sch.UnpaidDueDates(); len(dates) > 0 {
			loans = append(loans, domain.ProjectionLoan{
				Description:      fmt.Sprintf("Parcela %s", l.row.Name),
				InstallmentCents: l.row.InstallmentCents,
				DueDates:         dates,
			})
		}
	}
	return loans, byMonth, nil
}

// addLoanCommitments soma as parcelas de empréstimos nos meses (loansCents) e lista as do mês
// consultado (kind "LOAN"); a parcela é realizada quando o número dela já está pago.
func (s *Services) addLoanCommitments(ctx context.Context, out *domain.Commitments, year, month int) error {
	states, err := s.loanStates(ctx)
	if err != nil {
		return err
	}
	for _, l := range states {
		if !l.row.Active {
			continue
		}
		sch := l.schedule()
		for i := range out.Months {
			if sch.InstallmentIn(out.Months[i].Year, out.Months[i].Month) > 0 {
				out.Months[i].LoansCents += l.row.InstallmentCents
			}
		}
		k := sch.InstallmentIn(year, month)
		if k == 0 {
			continue
		}
		num, tot, id := k, l.row.TotalInstallments, l.row.ID
		out.Items = append(out.Items, domain.CommitmentItem{
			Kind:              "LOAN",
			Direction:         "PAY",
			Description:       l.row.Name,
			DueDate:           domain.LoanDueDate(l.row.FirstDueDate, k),
			AmountCents:       l.row.InstallmentCents,
			Realized:          k <= sch.PaidInstallments,
			InstallmentNumber: &num,
			InstallmentTotal:  &tot,
			LoanID:            &id,
		})
	}
	return nil
}
