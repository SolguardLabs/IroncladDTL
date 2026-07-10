package main

import (
	"fmt"
	"sort"
)

func (l *Ledger) AddAccount(config AccountConfig) error {
	if err := validateID("account", config.ID); err != nil {
		return err
	}
	if _, exists := l.Accounts[config.ID]; exists {
		return NewValidationError("account", "duplicate account "+config.ID)
	}
	reserves := AmountMapClone(config.Reserves)
	if reserves == nil {
		reserves = map[string]int64{}
	}
	for asset, amount := range reserves {
		if err := validateID("asset", asset); err != nil {
			return err
		}
		if _, err := MustAmount(amount, "reserve"); err != nil {
			return err
		}
	}
	label := config.Label
	if label == "" {
		label = config.ID
	}
	l.Accounts[config.ID] = &Account{ID: config.ID, Label: label, Reserves: reserves}
	l.Emit(Event{Type: "account_created", Status: StateApplied, Account: config.ID})
	return nil
}

func (l *Ledger) AddVault(config VaultConfig) error {
	if err := validateID("vault", config.ID); err != nil {
		return err
	}
	if err := validateID("asset", config.Asset); err != nil {
		return err
	}
	if _, exists := l.Vaults[config.ID]; exists {
		return NewValidationError("vault", "duplicate vault "+config.ID)
	}
	if _, err := MustAmount(config.Reserve, "reserve"); err != nil {
		return err
	}
	l.Vaults[config.ID] = &Vault{
		ID:         config.ID,
		Asset:      config.Asset,
		Reserve:    config.Reserve,
		FeeAccount: config.FeeAccount,
	}
	l.Emit(Event{Type: "vault_created", Status: StateApplied, Vault: config.ID, Amount: config.Reserve})
	return nil
}

func (l *Ledger) ApplyOperation(index int, op Operation) error {
	switch op.Type {
	case "advance":
		return l.Advance(op.Delta)
	case "deposit":
		return l.Deposit(op.Account, op.Vault, op.Amount, op.ID)
	case "withdraw":
		return l.Withdraw(op.Account, op.Vault, op.Amount, op.ID)
	case "open":
		return l.OpenCommitment(op)
	case "deliver":
		return l.Deliver(op)
	case "settle":
		return l.SettleCommitment(op.Commitment)
	case "claim":
		return l.Claim(op)
	case "penalize":
		return l.Penalize(op)
	case "close":
		return l.CloseCommitment(op.Commitment)
	case "sweep":
		return l.SweepFees(op.Vault, op.Account)
	default:
		return NewValidationError("operation", fmt.Sprintf("unknown operation at index %d: %s", index, op.Type))
	}
}

func (l *Ledger) Emit(event Event) {
	l.seq++
	event.Seq = l.seq
	event.Epoch = l.Epoch
	if event.Status == "" {
		event.Status = StateApplied
	}
	l.Events = append(l.Events, event)
}

func (l *Ledger) RecordRejected(index int, op Operation, err error) {
	l.Rejected = append(l.Rejected, RejectedOperation{
		Index:   index,
		Type:    op.Type,
		ID:      op.ID,
		Code:    ErrorCode(err),
		Field:   ErrorField(err),
		Message: ErrorMessage(err),
	})
	if op.Account != "" {
		if account := l.Accounts[op.Account]; account != nil {
			account.RejectedAttempts++
		}
	}
	l.Emit(Event{
		Type:   "operation_rejected",
		Status: StateRejected,
		ID:     op.ID,
		Amount: op.Amount,
		Details: map[string]string{
			"operation": op.Type,
			"code":      ErrorCode(err),
			"message":   ErrorMessage(err),
		},
	})
}

func (l *Ledger) Account(id string) (*Account, error) {
	account := l.Accounts[id]
	if account == nil {
		return nil, NewNotFoundError("account", id)
	}
	return account, nil
}

func (l *Ledger) Vault(id string) (*Vault, error) {
	vault := l.Vaults[id]
	if vault == nil {
		return nil, NewNotFoundError("vault", id)
	}
	return vault, nil
}

func (l *Ledger) Commitment(id string) (*Commitment, error) {
	commitment := l.Commitments[id]
	if commitment == nil {
		return nil, NewNotFoundError("commitment", id)
	}
	return commitment, nil
}

func (l *Ledger) Sub(id string) (*SubCommitment, error) {
	sub := l.Subs[id]
	if sub == nil {
		return nil, NewNotFoundError("sub", id)
	}
	return sub, nil
}

func (l *Ledger) Lock(id string) (*Lock, error) {
	lock := l.Locks[id]
	if lock == nil {
		return nil, NewNotFoundError("lock", id)
	}
	return lock, nil
}

func (l *Ledger) Advance(delta int64) error {
	if delta <= 0 {
		return NewValidationError("delta", "advance delta must be positive")
	}
	l.Epoch += delta
	l.Emit(Event{Type: "epoch_advanced", Status: StateApplied, Amount: delta})
	return nil
}

func (l *Ledger) Deposit(accountID string, vaultID string, amount int64, id string) error {
	account, err := l.Account(accountID)
	if err != nil {
		return err
	}
	vault, err := l.Vault(vaultID)
	if err != nil {
		return err
	}
	if _, err := MustAmount(amount, "amount"); err != nil {
		return err
	}
	if err := AmountMapSub(account.Reserves, vault.Asset, amount); err != nil {
		return err
	}
	vault.Reserve += amount
	account.ExternalIn += amount
	l.Emit(Event{Type: "deposit", ID: id, Account: accountID, Vault: vaultID, Amount: amount})
	return nil
}

func (l *Ledger) Withdraw(accountID string, vaultID string, amount int64, id string) error {
	account, err := l.Account(accountID)
	if err != nil {
		return err
	}
	vault, err := l.Vault(vaultID)
	if err != nil {
		return err
	}
	if _, err := MustAmount(amount, "amount"); err != nil {
		return err
	}
	if err := AmountMapSub(account.Reserves, vault.Asset, amount); err != nil {
		return err
	}
	account.ExternalOut += amount
	l.Emit(Event{Type: "withdraw", ID: id, Account: accountID, Vault: vaultID, Amount: amount})
	return nil
}

func (l *Ledger) OpenCommitment(op Operation) error {
	if op.ID == "" {
		return NewValidationError("commitment", "open operation requires id")
	}
	if _, exists := l.Commitments[op.ID]; exists {
		return NewValidationError("commitment", "duplicate commitment "+op.ID)
	}
	sponsor, err := l.Account(op.Account)
	if err != nil {
		return err
	}
	vault, err := l.Vault(op.Vault)
	if err != nil {
		return err
	}
	if op.Amount < l.Policy.MinLockAmount {
		return NewValidationError("amount", "amount below minimum lock")
	}
	if op.Amount > l.Policy.MaxCommitmentAmount {
		return NewValidationError("amount", "amount above maximum commitment")
	}
	if err := AmountMapSub(sponsor.Reserves, vault.Asset, op.Amount); err != nil {
		return err
	}
	vault.Reserve += op.Amount
	vault.Locked += op.Amount

	threshold := NormalizeBps(op.ReleaseThresholdBps, l.Policy.DefaultReleaseThresholdBps)
	due := op.DueEpoch
	if due == 0 {
		due = l.Epoch + 1
	}
	parentLockID := op.ID + ":parent"
	commitment := &Commitment{
		ID:                  op.ID,
		VaultID:             op.Vault,
		Sponsor:             op.Account,
		Amount:              op.Amount,
		DueEpoch:            due,
		Status:              StatePending,
		Composite:           len(op.Beneficiaries) > 1,
		ReleaseThresholdBps: threshold,
		ParentLockID:        parentLockID,
		SubIDs:              []string{},
	}
	parentLock := &Lock{
		ID:           parentLockID,
		CommitmentID: op.ID,
		Scope:        ScopeParent,
		VaultID:      op.Vault,
		Account:      op.Account,
		Amount:       op.Amount,
		State:        StateLocked,
		CreatedEpoch: l.Epoch,
		DueEpoch:     due,
	}
	l.Commitments[op.ID] = commitment
	l.Locks[parentLockID] = parentLock

	beneficiaries, err := l.normalizeBeneficiaries(op)
	if err != nil {
		return err
	}
	for _, beneficiary := range beneficiaries {
		if err := l.createSubCommitment(commitment, beneficiary, due); err != nil {
			return err
		}
	}
	l.Emit(Event{Type: "commitment_opened", ID: op.ID, Account: op.Account, Vault: op.Vault, Commitment: op.ID, Amount: op.Amount})
	return nil
}

func (l *Ledger) normalizeBeneficiaries(op Operation) ([]BeneficiaryConfig, error) {
	if len(op.Beneficiaries) == 0 {
		if op.Beneficiary == "" {
			return nil, NewValidationError("beneficiary", "open operation requires at least one beneficiary")
		}
		return []BeneficiaryConfig{{
			ID:          op.ID + ":sub:0",
			Beneficiary: op.Beneficiary,
			Amount:      op.Amount,
			DueEpoch:    op.DueEpoch,
		}}, nil
	}
	var total int64
	seen := map[string]bool{}
	for i, beneficiary := range op.Beneficiaries {
		if beneficiary.ID == "" {
			beneficiary.ID = fmt.Sprintf("%s:sub:%d", op.ID, i)
			op.Beneficiaries[i].ID = beneficiary.ID
		}
		if beneficiary.Beneficiary == "" {
			return nil, NewValidationError("beneficiary", "beneficiary account missing")
		}
		if seen[beneficiary.ID] {
			return nil, NewValidationError("beneficiary", "duplicate sub id "+beneficiary.ID)
		}
		seen[beneficiary.ID] = true
		if _, err := l.Account(beneficiary.Beneficiary); err != nil {
			return nil, err
		}
		if beneficiary.Amount <= 0 {
			return nil, NewValidationError("beneficiary", "beneficiary amount must be positive")
		}
		total += beneficiary.Amount
	}
	if total != op.Amount {
		return nil, NewValidationError("beneficiary", fmt.Sprintf("beneficiary total %d does not match amount %d", total, op.Amount))
	}
	return op.Beneficiaries, nil
}

func (l *Ledger) createSubCommitment(parent *Commitment, config BeneficiaryConfig, parentDue int64) error {
	if _, exists := l.Subs[config.ID]; exists {
		return NewValidationError("sub", "duplicate sub "+config.ID)
	}
	due := config.DueEpoch
	if due == 0 {
		due = parentDue
	}
	subLockID := config.ID + ":lock"
	sub := &SubCommitment{
		ID:          config.ID,
		ParentID:    parent.ID,
		VaultID:     parent.VaultID,
		Sponsor:     parent.Sponsor,
		Beneficiary: config.Beneficiary,
		Label:       config.Label,
		Amount:      config.Amount,
		DueEpoch:    due,
		State:       StatePending,
		SubLockID:   subLockID,
	}
	lock := &Lock{
		ID:           subLockID,
		CommitmentID: parent.ID,
		SubID:        sub.ID,
		Scope:        ScopeSub,
		VaultID:      parent.VaultID,
		Account:      parent.Sponsor,
		Beneficiary:  config.Beneficiary,
		Amount:       config.Amount,
		State:        StateLocked,
		CreatedEpoch: l.Epoch,
		DueEpoch:     due,
	}
	l.Subs[sub.ID] = sub
	l.Locks[lock.ID] = lock
	parent.SubIDs = append(parent.SubIDs, sub.ID)
	l.Emit(Event{Type: "subcommitment_opened", ID: sub.ID, Commitment: parent.ID, Sub: sub.ID, Amount: sub.Amount})
	return nil
}

func (l *Ledger) Deliver(op Operation) error {
	sub, err := l.resolveSubForOperation(op)
	if err != nil {
		return err
	}
	if sub.State == StateClaimed || sub.State == StateDefaulted || sub.State == StateCancelled {
		return NewConflictError("sub", "cannot deliver to terminal sub "+sub.ID)
	}
	if op.Amount <= 0 {
		return NewValidationError("amount", "delivery amount must be positive")
	}
	next := sub.Delivered + op.Amount
	if next > sub.Amount {
		return NewValidationError("amount", "delivery exceeds sub amount")
	}
	sub.Delivered = next
	sub.State = StateDelivered
	sub.LastEventSeq = l.seq + 1
	l.Emit(Event{Type: "delivery_recorded", ID: op.ID, Commitment: sub.ParentID, Sub: sub.ID, Amount: op.Amount})
	return nil
}

func (l *Ledger) resolveSubForOperation(op Operation) (*SubCommitment, error) {
	if op.Sub != "" {
		return l.Sub(op.Sub)
	}
	if op.Commitment == "" {
		return nil, NewValidationError("commitment", "operation requires commitment or sub")
	}
	commitment, err := l.Commitment(op.Commitment)
	if err != nil {
		return nil, err
	}
	if op.Beneficiary != "" {
		for _, subID := range commitment.SubIDs {
			sub := l.Subs[subID]
			if sub != nil && sub.Beneficiary == op.Beneficiary {
				return sub, nil
			}
		}
		return nil, NewNotFoundError("beneficiary", op.Beneficiary)
	}
	if len(commitment.SubIDs) == 1 {
		return l.Sub(commitment.SubIDs[0])
	}
	return nil, NewValidationError("sub", "ambiguous sub operation")
}

func (l *Ledger) SettleCommitment(id string) error {
	commitment, err := l.Commitment(id)
	if err != nil {
		return err
	}
	if commitment.Status == StateClosed || commitment.Status == StateClaimed {
		return NewConflictError("commitment", "commitment already terminal")
	}
	for _, subID := range commitment.SubIDs {
		if err := l.settleSub(commitment, subID); err != nil {
			return err
		}
	}
	l.refreshCommitmentTotals(commitment)
	if commitment.Composite {
		if err := l.releaseCompositeParentIfThresholdMet(commitment); err != nil {
			return err
		}
	}
	l.Emit(Event{Type: "commitment_settled", Commitment: commitment.ID, Amount: commitment.Delivered})
	return nil
}

func (l *Ledger) settleSub(commitment *Commitment, subID string) error {
	sub, err := l.Sub(subID)
	if err != nil {
		return err
	}
	lock, err := l.Lock(sub.SubLockID)
	if err != nil {
		return err
	}
	if sub.State == StatePending || sub.State == StateDelivered {
		if DeliverySatisfied(sub.Delivered, sub.Amount) {
			sub.State = StateClaimable
			lock.State = StateClaimable
			l.Emit(Event{Type: "sub_claimable", Commitment: commitment.ID, Sub: sub.ID, Amount: sub.Amount})
			return nil
		}
		if l.Epoch > sub.DueEpoch+l.Policy.GraceEpochs {
			return l.applyPenaltyToSub(commitment, sub, lock, l.Policy.DefaultPenaltyBps)
		}
	}
	return nil
}

func (l *Ledger) applyPenaltyToSub(commitment *Commitment, sub *SubCommitment, lock *Lock, penaltyBps int64) error {
	missing := sub.Amount - sub.Delivered
	if missing <= 0 {
		return nil
	}
	penalty, err := PenaltyForMissing(missing, penaltyBps)
	if err != nil {
		return err
	}
	if penalty > missing {
		penalty = missing
	}
	sub.Penalty += penalty
	sub.State = StateDefaulted
	lock.Penalty += penalty
	lock.State = StateDefaulted
	vault, err := l.Vault(commitment.VaultID)
	if err != nil {
		return err
	}
	vault.Fees += penalty
	if vault.Locked >= penalty {
		vault.Locked -= penalty
	} else {
		vault.Locked = 0
	}
	commitment.PenaltyAccrued += penalty
	l.Emit(Event{Type: "sub_penalized", Commitment: commitment.ID, Sub: sub.ID, Amount: penalty})
	return nil
}

func (l *Ledger) releaseCompositeParentIfThresholdMet(commitment *Commitment) error {
	parent, err := l.Lock(commitment.ParentLockID)
	if err != nil {
		return err
	}
	if parent.State != StateLocked {
		return nil
	}
	if commitment.ReleaseThresholdBps >= BasisPoints {
		return nil
	}
	delivered := l.DeliveredForCommitment(commitment)
	deliveredBps := DeliveryRatio(delivered, commitment.Amount)
	if deliveredBps < commitment.ReleaseThresholdBps {
		return nil
	}

	// Intentional CTF bug:
	// The aggregate threshold is treated as sufficient to release the full parent
	// lock. The function does not require every subcommitment to be terminal, so
	// pending beneficiary claims can remain economically backed by funds that are
	// credited back to the sponsor.
	vault, err := l.Vault(commitment.VaultID)
	if err != nil {
		return err
	}
	sponsor, err := l.Account(commitment.Sponsor)
	if err != nil {
		return err
	}
	gross := parent.Amount - parent.Released
	if gross <= 0 {
		return nil
	}
	net, fee, err := NetOfFee(gross, l.Policy.CompositeReleaseFeeBps)
	if err != nil {
		return err
	}
	if vault.Reserve < gross {
		shortfall := gross - vault.Reserve
		vault.Insolvency += shortfall
		vault.Reserve = 0
	} else {
		vault.Reserve -= gross
	}
	vault.Fees += fee
	if vault.Locked >= gross {
		vault.Locked -= gross
	} else {
		vault.Locked = 0
	}
	if err := AmountMapAdd(sponsor.Reserves, vault.Asset, net); err != nil {
		return err
	}
	parent.Released += gross
	parent.State = StateReleased
	parent.ReleasedEpoch = l.Epoch
	commitment.ReleasedToSponsor += net
	commitment.FeeAccrued += fee
	sponsor.SponsorReleases += net
	l.Emit(Event{
		Type:       "composite_parent_released",
		Status:     StateReleased,
		Account:    commitment.Sponsor,
		Vault:      commitment.VaultID,
		Commitment: commitment.ID,
		Amount:     net,
		Details: map[string]string{
			"delivered_bps": FormatBps(deliveredBps),
			"threshold_bps": FormatBps(commitment.ReleaseThresholdBps),
			"bug":           "parent released without checking pending sublocks",
		},
	})
	return nil
}

func (l *Ledger) Claim(op Operation) error {
	sub, err := l.resolveSubForOperation(op)
	if err != nil {
		return err
	}
	if sub.State != StateClaimable {
		return NewConflictError("sub", "sub is not claimable")
	}
	payout := RemainingAmount(sub.Amount, sub.Claimed, sub.Penalty)
	if op.Amount > 0 && op.Amount < payout {
		if !l.Policy.AllowPartialClaims {
			return NewConflictError("claim", "partial claims disabled")
		}
		payout = op.Amount
	}
	if payout <= 0 {
		return NewConflictError("claim", "nothing to claim")
	}
	vault, err := l.Vault(sub.VaultID)
	if err != nil {
		return err
	}
	beneficiary, err := l.Account(sub.Beneficiary)
	if err != nil {
		return err
	}
	paid := payout
	if vault.Reserve < payout {
		paid = vault.Reserve
		shortfall := payout - paid
		vault.ClaimShortfall += shortfall
		vault.Insolvency += shortfall
		l.Emit(Event{Type: "claim_shortfall", Status: StateRejected, Commitment: sub.ParentID, Sub: sub.ID, Amount: shortfall})
	}
	vault.Reserve -= paid
	if vault.Locked >= paid {
		vault.Locked -= paid
	} else {
		vault.Locked = 0
	}
	if paid > 0 {
		if err := AmountMapAdd(beneficiary.Reserves, vault.Asset, paid); err != nil {
			return err
		}
	}
	sub.Claimed += paid
	lock := l.Locks[sub.SubLockID]
	if lock != nil {
		lock.Claimed += paid
		if sub.Claimed >= RemainingAmount(sub.Amount, 0, sub.Penalty) {
			lock.State = StateClaimed
		}
	}
	beneficiary.ClaimsReceived += paid
	commitment := l.Commitments[sub.ParentID]
	if commitment != nil {
		commitment.Claimed += paid
	}
	if sub.Claimed >= RemainingAmount(sub.Amount, 0, sub.Penalty) {
		sub.State = StateClaimed
	}
	l.Emit(Event{Type: "claim_paid", Account: sub.Beneficiary, Vault: sub.VaultID, Commitment: sub.ParentID, Sub: sub.ID, Amount: paid})
	return nil
}

func (l *Ledger) Penalize(op Operation) error {
	sub, err := l.resolveSubForOperation(op)
	if err != nil {
		return err
	}
	commitment, err := l.Commitment(sub.ParentID)
	if err != nil {
		return err
	}
	lock, err := l.Lock(sub.SubLockID)
	if err != nil {
		return err
	}
	bps := op.PenaltyBps
	if bps <= 0 {
		bps = l.Policy.DefaultPenaltyBps
	}
	return l.applyPenaltyToSub(commitment, sub, lock, bps)
}

func (l *Ledger) CloseCommitment(id string) error {
	commitment, err := l.Commitment(id)
	if err != nil {
		return err
	}
	if commitment.Status == StateClosed {
		return nil
	}
	if err := l.SettleCommitment(id); err != nil {
		return err
	}
	for _, subID := range commitment.SubIDs {
		sub := l.Subs[subID]
		if sub == nil {
			continue
		}
		if sub.State == StatePending || sub.State == StateDelivered {
			return NewConflictError("commitment", "cannot close with non-terminal sub "+sub.ID)
		}
		if sub.State == StateClaimable {
			return NewConflictError("commitment", "cannot close with unclaimed sub "+sub.ID)
		}
	}
	if err := l.releaseResidualToSponsor(commitment); err != nil {
		return err
	}
	commitment.Status = StateClosed
	commitment.ClosedEpoch = l.Epoch
	parent := l.Locks[commitment.ParentLockID]
	if parent != nil && parent.State == StateLocked {
		parent.State = StateClosed
	}
	l.Emit(Event{Type: "commitment_closed", Status: StateClosed, Commitment: commitment.ID, Amount: commitment.Amount})
	return nil
}

func (l *Ledger) releaseResidualToSponsor(commitment *Commitment) error {
	vault, err := l.Vault(commitment.VaultID)
	if err != nil {
		return err
	}
	sponsor, err := l.Account(commitment.Sponsor)
	if err != nil {
		return err
	}
	totalObligations := commitment.Claimed + commitment.PenaltyAccrued + commitment.ReleasedToSponsor + commitment.FeeAccrued
	residual := commitment.Amount - totalObligations
	if residual <= 0 {
		return nil
	}
	if vault.Reserve < residual {
		residual = vault.Reserve
	}
	vault.Reserve -= residual
	if vault.Locked >= residual {
		vault.Locked -= residual
	} else {
		vault.Locked = 0
	}
	if err := AmountMapAdd(sponsor.Reserves, vault.Asset, residual); err != nil {
		return err
	}
	sponsor.SponsorReleases += residual
	commitment.ReleasedToSponsor += residual
	l.Emit(Event{Type: "residual_released", Account: commitment.Sponsor, Vault: commitment.VaultID, Commitment: commitment.ID, Amount: residual})
	return nil
}

func (l *Ledger) SweepFees(vaultID string, accountID string) error {
	vault, err := l.Vault(vaultID)
	if err != nil {
		return err
	}
	target := accountID
	if target == "" {
		target = vault.FeeAccount
	}
	if target == "" {
		return NewValidationError("account", "fee account missing")
	}
	account, err := l.Account(target)
	if err != nil {
		return err
	}
	if vault.Fees <= 0 {
		return nil
	}
	fees := vault.Fees
	vault.Fees = 0
	if vault.Reserve < fees {
		vault.Insolvency += fees - vault.Reserve
		fees = vault.Reserve
	}
	vault.Reserve -= fees
	if err := AmountMapAdd(account.Reserves, vault.Asset, fees); err != nil {
		return err
	}
	l.Emit(Event{Type: "fees_swept", Account: target, Vault: vaultID, Amount: fees})
	return nil
}

func (l *Ledger) DeliveredForCommitment(commitment *Commitment) int64 {
	var delivered int64
	for _, subID := range commitment.SubIDs {
		sub := l.Subs[subID]
		if sub == nil {
			continue
		}
		delivered += ClampAmount(sub.Delivered, 0, sub.Amount)
	}
	return delivered
}

func (l *Ledger) refreshCommitmentTotals(commitment *Commitment) {
	commitment.Delivered = l.DeliveredForCommitment(commitment)
	var claimed int64
	var penalty int64
	for _, subID := range commitment.SubIDs {
		sub := l.Subs[subID]
		if sub == nil {
			continue
		}
		claimed += sub.Claimed
		penalty += sub.Penalty
	}
	commitment.Claimed = claimed
	commitment.PenaltyAccrued = penalty
}

func (l *Ledger) RefreshDerivedVaultClaims() {
	for _, vault := range l.Vaults {
		vault.OpenClaims = 0
	}
	for _, sub := range l.Subs {
		if !IsOpenSubState(sub.State) {
			continue
		}
		claim := RemainingAmount(sub.Amount, sub.Claimed, sub.Penalty)
		if claim <= 0 {
			continue
		}
		if vault := l.Vaults[sub.VaultID]; vault != nil {
			vault.OpenClaims += claim
		}
	}
}

func (l *Ledger) SortedCommitmentIDs() []string {
	ids := make([]string, 0, len(l.Commitments))
	for id := range l.Commitments {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (l *Ledger) SortedSubIDs() []string {
	ids := make([]string, 0, len(l.Subs))
	for id := range l.Subs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (l *Ledger) SortedLockIDs() []string {
	ids := make([]string, 0, len(l.Locks))
	for id := range l.Locks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (l *Ledger) SortedVaultIDs() []string {
	ids := make([]string, 0, len(l.Vaults))
	for id := range l.Vaults {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (l *Ledger) SortedAccountIDs() []string {
	ids := make([]string, 0, len(l.Accounts))
	for id := range l.Accounts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
