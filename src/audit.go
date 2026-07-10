package main

import "sort"

type Check struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Details string `json:"details,omitempty"`
}

type AuditReport struct {
	Checks             []Check  `json:"checks"`
	Warnings           []string `json:"warnings"`
	OpenCommitments    int64    `json:"open_commitments"`
	ClosedCommitments  int64    `json:"closed_commitments"`
	ClaimableSubs      int64    `json:"claimable_subs"`
	PendingSubs        int64    `json:"pending_subs"`
	DefaultedSubs      int64    `json:"defaulted_subs"`
	ReleasedParents    int64    `json:"released_parents"`
	RejectedOperations int64    `json:"rejected_operations"`
	HasCriticalFinding bool     `json:"has_critical_finding"`
}

func BuildAuditReport(ledger *Ledger, risk RiskReport) AuditReport {
	report := AuditReport{
		Checks:             []Check{},
		Warnings:           []string{},
		RejectedOperations: int64(len(ledger.Rejected)),
	}
	for _, commitment := range ledger.Commitments {
		if commitment == nil {
			continue
		}
		if commitment.Status == StateClosed {
			report.ClosedCommitments++
		} else {
			report.OpenCommitments++
		}
		parent := ledger.Locks[commitment.ParentLockID]
		if parent != nil && parent.State == StateReleased {
			report.ReleasedParents++
		}
	}
	for _, sub := range ledger.Subs {
		if sub == nil {
			continue
		}
		switch sub.State {
		case StateClaimable:
			report.ClaimableSubs++
		case StatePending, StateDelivered:
			report.PendingSubs++
		case StateDefaulted:
			report.DefaultedSubs++
		}
	}
	report.Checks = append(report.Checks, CheckNoNegativeAccounts(ledger))
	report.Checks = append(report.Checks, CheckNoNegativeVaults(ledger))
	report.Checks = append(report.Checks, CheckNoReleasedParentWithOpenSubs(ledger))
	report.Checks = append(report.Checks, CheckEconomicClaimsCovered(risk))
	report.Checks = append(report.Checks, CheckCommitmentTotals(ledger))
	for _, check := range report.Checks {
		if !check.OK {
			report.HasCriticalFinding = true
			report.Warnings = append(report.Warnings, check.Details)
		}
	}
	report.Warnings = append(report.Warnings, ledger.Warnings...)
	sort.Strings(report.Warnings)
	return report
}

func CheckNoNegativeAccounts(ledger *Ledger) Check {
	for _, accountID := range ledger.SortedAccountIDs() {
		account := ledger.Accounts[accountID]
		if account == nil {
			continue
		}
		for asset, amount := range account.Reserves {
			if amount < 0 {
				return Check{Name: "no_negative_accounts", OK: false, Details: accountID + ":" + asset}
			}
		}
	}
	return Check{Name: "no_negative_accounts", OK: true}
}

func CheckNoNegativeVaults(ledger *Ledger) Check {
	for _, vaultID := range ledger.SortedVaultIDs() {
		vault := ledger.Vaults[vaultID]
		if vault == nil {
			continue
		}
		if vault.Reserve < 0 || vault.Locked < 0 || vault.Fees < 0 {
			return Check{Name: "no_negative_vaults", OK: false, Details: vaultID}
		}
	}
	return Check{Name: "no_negative_vaults", OK: true}
}

func CheckNoReleasedParentWithOpenSubs(ledger *Ledger) Check {
	ids := ParentReleasedWithOpenSubs(ledger)
	if len(ids) == 0 {
		return Check{Name: "no_released_parent_with_open_subs", OK: true}
	}
	return Check{Name: "no_released_parent_with_open_subs", OK: false, Details: joinStrings(ids, ",")}
}

func CheckEconomicClaimsCovered(risk RiskReport) Check {
	if risk.TotalShortfall == 0 {
		return Check{Name: "economic_claims_covered", OK: true}
	}
	return Check{Name: "economic_claims_covered", OK: false, Details: "shortfall=" + int64toa(risk.TotalShortfall)}
}

func CheckCommitmentTotals(ledger *Ledger) Check {
	for _, commitmentID := range ledger.SortedCommitmentIDs() {
		commitment := ledger.Commitments[commitmentID]
		if commitment == nil {
			continue
		}
		var total int64
		for _, subID := range commitment.SubIDs {
			sub := ledger.Subs[subID]
			if sub != nil {
				total += sub.Amount
			}
		}
		if total != commitment.Amount {
			return Check{Name: "commitment_totals", OK: false, Details: commitmentID}
		}
	}
	return Check{Name: "commitment_totals", OK: true}
}

func joinStrings(values []string, sep string) string {
	if len(values) == 0 {
		return ""
	}
	out := values[0]
	for _, value := range values[1:] {
		out += sep + value
	}
	return out
}

func int64toa(value int64) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var digits [32]byte
	i := len(digits)
	for value > 0 {
		i--
		digits[i] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		i--
		digits[i] = '-'
	}
	return string(digits[i:])
}

type TimelineBucket struct {
	Epoch      int64    `json:"epoch"`
	Events     int64    `json:"events"`
	Commitment []string `json:"commitments"`
}

func BuildTimeline(ledger *Ledger) []TimelineBucket {
	buckets := map[int64]*TimelineBucket{}
	for _, event := range ledger.Events {
		bucket := buckets[event.Epoch]
		if bucket == nil {
			bucket = &TimelineBucket{Epoch: event.Epoch, Commitment: []string{}}
			buckets[event.Epoch] = bucket
		}
		bucket.Events++
		if event.Commitment != "" && !containsString(bucket.Commitment, event.Commitment) {
			bucket.Commitment = append(bucket.Commitment, event.Commitment)
		}
	}
	epochs := make([]int64, 0, len(buckets))
	for epoch := range buckets {
		epochs = append(epochs, epoch)
	}
	sort.Slice(epochs, func(i, j int) bool { return epochs[i] < epochs[j] })
	out := []TimelineBucket{}
	for _, epoch := range epochs {
		bucket := buckets[epoch]
		sort.Strings(bucket.Commitment)
		out = append(out, *bucket)
	}
	return out
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

type PlannerAction struct {
	Priority   int    `json:"priority"`
	Type       string `json:"type"`
	Target     string `json:"target"`
	Reason     string `json:"reason"`
	Amount     int64  `json:"amount,omitempty"`
	Executable bool   `json:"executable"`
}

func BuildPlanner(ledger *Ledger, risk RiskReport) []PlannerAction {
	actions := []PlannerAction{}
	for _, line := range risk.Exposures {
		if line.CompositeMismatch {
			actions = append(actions, PlannerAction{
				Priority:   1,
				Type:       "freeze_sponsor_withdrawals",
				Target:     line.Commitment,
				Reason:     "parent lock released while subclaim remains open",
				Amount:     line.Pending + line.Claimable,
				Executable: false,
			})
		}
		if line.Overdue && line.Pending > 0 {
			actions = append(actions, PlannerAction{
				Priority:   2,
				Type:       "penalize_subcommitment",
				Target:     line.Sub,
				Reason:     "subcommitment overdue with pending delivery",
				Amount:     line.Pending,
				Executable: true,
			})
		}
		if line.Claimable > 0 {
			actions = append(actions, PlannerAction{
				Priority:   3,
				Type:       "pay_claim",
				Target:     line.Sub,
				Reason:     "beneficiary has claimable delivery",
				Amount:     line.Claimable,
				Executable: true,
			})
		}
	}
	for _, vault := range risk.Vaults {
		if vault.EconomicShortfall > 0 {
			actions = append(actions, PlannerAction{
				Priority:   1,
				Type:       "recapitalize_vault",
				Target:     vault.Vault,
				Reason:     "economic claims exceed reserve",
				Amount:     vault.EconomicShortfall,
				Executable: false,
			})
		}
	}
	sort.SliceStable(actions, func(i, j int) bool {
		if actions[i].Priority == actions[j].Priority {
			return actions[i].Target < actions[j].Target
		}
		return actions[i].Priority < actions[j].Priority
	})
	return actions
}
