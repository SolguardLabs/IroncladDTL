package main

import "sort"

type ExposureLine struct {
	Vault              string `json:"vault"`
	Commitment        string `json:"commitment"`
	Sub               string `json:"sub"`
	Beneficiary       string `json:"beneficiary"`
	State             string `json:"state"`
	Claimable         int64  `json:"claimable"`
	Pending           int64  `json:"pending"`
	Penalized         int64  `json:"penalized"`
	DeliveredBps      int64  `json:"delivered_bps"`
	DueEpoch          int64  `json:"due_epoch"`
	Overdue           bool   `json:"overdue"`
	ParentReleased    bool   `json:"parent_released"`
	CompositeMismatch bool   `json:"composite_mismatch"`
}

type VaultRisk struct {
	Vault                  string `json:"vault"`
	Reserve                int64  `json:"reserve"`
	TechnicalLocked        int64  `json:"technical_locked"`
	EconomicClaims         int64  `json:"economic_claims"`
	ClaimableNow           int64  `json:"claimable_now"`
	PendingClaims          int64  `json:"pending_claims"`
	Penalized              int64  `json:"penalized"`
	Fees                   int64  `json:"fees"`
	Insolvency             int64  `json:"insolvency"`
	EconomicShortfall      int64  `json:"economic_shortfall"`
	CoverageBps            int64  `json:"coverage_bps"`
	ParentReleaseMismatch  bool   `json:"parent_release_mismatch"`
	ReleasedWithOpenSubIDs []string `json:"released_with_open_sub_ids"`
}

type RiskReport struct {
	Vaults           []VaultRisk    `json:"vaults"`
	Exposures        []ExposureLine `json:"exposures"`
	TotalClaims      int64          `json:"total_claims"`
	TotalReserve     int64          `json:"total_reserve"`
	TotalShortfall   int64          `json:"total_shortfall"`
	ReleasedMismatch int64          `json:"released_mismatch"`
}

func BuildRiskReport(ledger *Ledger) RiskReport {
	ledger.RefreshDerivedVaultClaims()
	exposures := BuildExposureLines(ledger)
	vaultMap := map[string]*VaultRisk{}
	for _, vaultID := range ledger.SortedVaultIDs() {
		vault := ledger.Vaults[vaultID]
		vaultMap[vaultID] = &VaultRisk{
			Vault:           vaultID,
			Reserve:         vault.Reserve,
			TechnicalLocked: vault.Locked,
			Fees:            vault.Fees,
			Insolvency:      vault.Insolvency,
		}
	}
	for _, line := range exposures {
		vaultRisk := vaultMap[line.Vault]
		if vaultRisk == nil {
			continue
		}
		claim := line.Claimable + line.Pending
		vaultRisk.EconomicClaims += claim
		vaultRisk.ClaimableNow += line.Claimable
		vaultRisk.PendingClaims += line.Pending
		vaultRisk.Penalized += line.Penalized
		if line.CompositeMismatch {
			vaultRisk.ParentReleaseMismatch = true
			vaultRisk.ReleasedWithOpenSubIDs = append(vaultRisk.ReleasedWithOpenSubIDs, line.Sub)
		}
	}
	report := RiskReport{Vaults: []VaultRisk{}, Exposures: exposures}
	for _, vaultID := range ledger.SortedVaultIDs() {
		risk := vaultMap[vaultID]
		if risk.EconomicClaims > risk.Reserve {
			risk.EconomicShortfall = risk.EconomicClaims - risk.Reserve
		}
		risk.CoverageBps = RatioBps(risk.Reserve, risk.EconomicClaims)
		sort.Strings(risk.ReleasedWithOpenSubIDs)
		report.TotalClaims += risk.EconomicClaims
		report.TotalReserve += risk.Reserve
		report.TotalShortfall += risk.EconomicShortfall
		if risk.ParentReleaseMismatch {
			report.ReleasedMismatch++
		}
		report.Vaults = append(report.Vaults, *risk)
	}
	return report
}

func BuildExposureLines(ledger *Ledger) []ExposureLine {
	lines := []ExposureLine{}
	for _, subID := range ledger.SortedSubIDs() {
		sub := ledger.Subs[subID]
		if sub == nil {
			continue
		}
		commitment := ledger.Commitments[sub.ParentID]
		parentReleased := false
		if commitment != nil {
			parent := ledger.Locks[commitment.ParentLockID]
			parentReleased = parent != nil && parent.State == StateReleased
		}
		line := ExposureLine{
			Vault:           sub.VaultID,
			Commitment:      sub.ParentID,
			Sub:             sub.ID,
			Beneficiary:     sub.Beneficiary,
			State:           sub.State,
			Penalized:       sub.Penalty,
			DeliveredBps:    DeliveryRatio(sub.Delivered, sub.Amount),
			DueEpoch:        sub.DueEpoch,
			Overdue:         ledger.Epoch > sub.DueEpoch+ledger.Policy.GraceEpochs,
			ParentReleased:  parentReleased,
		}
		remaining := RemainingAmount(sub.Amount, sub.Claimed, sub.Penalty)
		switch sub.State {
		case StateClaimable:
			line.Claimable = remaining
		case StatePending, StateDelivered:
			line.Pending = remaining
		}
		line.CompositeMismatch = line.ParentReleased && (line.Pending > 0 || line.Claimable > 0)
		lines = append(lines, line)
	}
	return lines
}

func ParentReleasedWithOpenSubs(ledger *Ledger) []string {
	var ids []string
	for _, commitmentID := range ledger.SortedCommitmentIDs() {
		commitment := ledger.Commitments[commitmentID]
		if commitment == nil || !commitment.Composite {
			continue
		}
		parent := ledger.Locks[commitment.ParentLockID]
		if parent == nil || parent.State != StateReleased {
			continue
		}
		for _, subID := range commitment.SubIDs {
			sub := ledger.Subs[subID]
			if sub == nil {
				continue
			}
			if IsOpenSubState(sub.State) {
				ids = append(ids, commitment.ID)
				break
			}
		}
	}
	sort.Strings(ids)
	return ids
}

func CommitmentOutstanding(ledger *Ledger, commitment *Commitment) int64 {
	if commitment == nil {
		return 0
	}
	var total int64
	for _, subID := range commitment.SubIDs {
		sub := ledger.Subs[subID]
		if sub == nil {
			continue
		}
		if IsOpenSubState(sub.State) {
			total += RemainingAmount(sub.Amount, sub.Claimed, sub.Penalty)
		}
	}
	return total
}

func CommitmentTerminal(ledger *Ledger, commitment *Commitment) bool {
	if commitment == nil {
		return false
	}
	for _, subID := range commitment.SubIDs {
		sub := ledger.Subs[subID]
		if sub == nil {
			return false
		}
		if IsOpenSubState(sub.State) {
			return false
		}
	}
	return true
}

func ClaimsByBeneficiary(ledger *Ledger) map[string]int64 {
	claims := map[string]int64{}
	for _, sub := range ledger.Subs {
		if sub == nil {
			continue
		}
		if IsOpenSubState(sub.State) {
			claims[sub.Beneficiary] += RemainingAmount(sub.Amount, sub.Claimed, sub.Penalty)
		}
	}
	return claims
}

func ReleasedBySponsor(ledger *Ledger) map[string]int64 {
	released := map[string]int64{}
	for _, commitment := range ledger.Commitments {
		if commitment == nil {
			continue
		}
		released[commitment.Sponsor] += commitment.ReleasedToSponsor
	}
	return released
}

func OverdueSubs(ledger *Ledger) []string {
	ids := []string{}
	for _, subID := range ledger.SortedSubIDs() {
		sub := ledger.Subs[subID]
		if sub == nil {
			continue
		}
		if IsOpenSubState(sub.State) && ledger.Epoch > sub.DueEpoch+ledger.Policy.GraceEpochs {
			ids = append(ids, sub.ID)
		}
	}
	return ids
}

func ClaimableSubs(ledger *Ledger) []string {
	ids := []string{}
	for _, subID := range ledger.SortedSubIDs() {
		sub := ledger.Subs[subID]
		if sub != nil && sub.State == StateClaimable {
			ids = append(ids, sub.ID)
		}
	}
	return ids
}

func PendingSubs(ledger *Ledger) []string {
	ids := []string{}
	for _, subID := range ledger.SortedSubIDs() {
		sub := ledger.Subs[subID]
		if sub == nil {
			continue
		}
		if sub.State == StatePending || sub.State == StateDelivered {
			ids = append(ids, sub.ID)
		}
	}
	return ids
}

func LockedParentAmount(ledger *Ledger) int64 {
	var total int64
	for _, commitment := range ledger.Commitments {
		if commitment == nil {
			continue
		}
		lock := ledger.Locks[commitment.ParentLockID]
		if lock == nil {
			continue
		}
		if lock.State == StateLocked {
			total += lock.Amount - lock.Released
		}
	}
	return total
}

func LockedSubAmount(ledger *Ledger) int64 {
	var total int64
	for _, sub := range ledger.Subs {
		if sub == nil {
			continue
		}
		lock := ledger.Locks[sub.SubLockID]
		if lock == nil {
			continue
		}
		if lock.State == StateLocked || lock.State == StateClaimable {
			total += RemainingAmount(lock.Amount, lock.Claimed, lock.Penalty)
		}
	}
	return total
}

func EconomicShortfall(ledger *Ledger) int64 {
	risk := BuildRiskReport(ledger)
	return risk.TotalShortfall
}

func ExposureByVault(ledger *Ledger, vaultID string) int64 {
	var total int64
	for _, line := range BuildExposureLines(ledger) {
		if line.Vault == vaultID {
			total += line.Pending + line.Claimable
		}
	}
	return total
}
