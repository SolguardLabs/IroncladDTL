package main

import (
	"encoding/json"
	"io"
	"sort"
)

type AmountEntry struct {
	ID     string `json:"id"`
	Amount int64  `json:"amount"`
}

type AccountReport struct {
	ID               string        `json:"id"`
	Label            string        `json:"label"`
	Reserves         []AmountEntry `json:"reserves"`
	ExternalIn       int64         `json:"external_in"`
	ExternalOut      int64         `json:"external_out"`
	ClaimsReceived   int64         `json:"claims_received"`
	SponsorReleases  int64         `json:"sponsor_releases"`
	PenaltiesPaid    int64         `json:"penalties_paid"`
	RejectedAttempts int64         `json:"rejected_attempts"`
}

type VaultReport struct {
	ID             string `json:"id"`
	Asset          string `json:"asset"`
	Reserve        int64  `json:"reserve"`
	Locked         int64  `json:"locked"`
	Fees           int64  `json:"fees"`
	Insolvency     int64  `json:"insolvency"`
	ClaimShortfall int64  `json:"claim_shortfall"`
	OpenClaims     int64  `json:"open_claims"`
	FeeAccount     string `json:"fee_account,omitempty"`
}

type CommitmentReport struct {
	ID                  string   `json:"id"`
	Vault               string   `json:"vault"`
	Sponsor             string   `json:"sponsor"`
	Amount              int64    `json:"amount"`
	Delivered           int64    `json:"delivered"`
	Claimed             int64    `json:"claimed"`
	ReleasedToSponsor   int64    `json:"released_to_sponsor"`
	PenaltyAccrued      int64    `json:"penalty_accrued"`
	FeeAccrued          int64    `json:"fee_accrued"`
	DueEpoch            int64    `json:"due_epoch"`
	ClosedEpoch         int64    `json:"closed_epoch,omitempty"`
	Status              string   `json:"status"`
	Composite           bool     `json:"composite"`
	ReleaseThresholdBps int64    `json:"release_threshold_bps"`
	ParentLockID        string   `json:"parent_lock_id"`
	SubIDs              []string `json:"sub_ids"`
	Outstanding         int64    `json:"outstanding"`
	Terminal            bool     `json:"terminal"`
}

type SubReport struct {
	ID           string `json:"id"`
	ParentID     string `json:"parent_id"`
	Vault        string `json:"vault"`
	Sponsor      string `json:"sponsor"`
	Beneficiary  string `json:"beneficiary"`
	Label        string `json:"label,omitempty"`
	Amount       int64  `json:"amount"`
	Delivered    int64  `json:"delivered"`
	Claimed      int64  `json:"claimed"`
	Penalty      int64  `json:"penalty"`
	DueEpoch     int64  `json:"due_epoch"`
	State        string `json:"state"`
	SubLockID    string `json:"sub_lock_id"`
	Remaining    int64  `json:"remaining"`
	DeliveredBps int64  `json:"delivered_bps"`
}

type LockReport struct {
	ID            string `json:"id"`
	Commitment    string `json:"commitment"`
	Sub           string `json:"sub,omitempty"`
	Scope         string `json:"scope"`
	Vault         string `json:"vault"`
	Account       string `json:"account"`
	Beneficiary   string `json:"beneficiary,omitempty"`
	Amount        int64  `json:"amount"`
	Released      int64  `json:"released"`
	Claimed       int64  `json:"claimed"`
	Penalty       int64  `json:"penalty"`
	State         string `json:"state"`
	CreatedEpoch  int64  `json:"created_epoch"`
	DueEpoch      int64  `json:"due_epoch"`
	ReleasedEpoch int64  `json:"released_epoch,omitempty"`
}

type MetricsReport struct {
	TotalReserve         int64 `json:"total_reserve"`
	TotalAccountReserves int64 `json:"total_account_reserves"`
	TotalEconomicClaims  int64 `json:"total_economic_claims"`
	EconomicShortfall    int64 `json:"economic_shortfall"`
	RejectedOperations   int64 `json:"rejected_operations"`
	Commitments          int64 `json:"commitments"`
	SubCommitments       int64 `json:"subcommitments"`
	Locks                int64 `json:"locks"`
	ClaimableSubs        int64 `json:"claimable_subs"`
	PendingSubs          int64 `json:"pending_subs"`
	DefaultedSubs        int64 `json:"defaulted_subs"`
	SponsorWithdrawals   int64 `json:"sponsor_withdrawals"`
	SponsorReleases      int64 `json:"sponsor_releases"`
	BeneficiaryClaims    int64 `json:"beneficiary_claims"`
}

type Report struct {
	Name        string              `json:"name"`
	Epoch       int64               `json:"epoch"`
	Policy      PolicyConfig        `json:"policy"`
	Accounts    []AccountReport     `json:"accounts"`
	Vaults      []VaultReport       `json:"vaults"`
	Commitments []CommitmentReport  `json:"commitments"`
	Subs        []SubReport         `json:"subs"`
	Locks       []LockReport        `json:"locks"`
	Metrics     MetricsReport       `json:"metrics"`
	Risk        RiskReport          `json:"risk"`
	Audit       AuditReport         `json:"audit"`
	Timeline    []TimelineBucket    `json:"timeline"`
	Planner     []PlannerAction     `json:"planner"`
	Rejected    []RejectedOperation `json:"rejected"`
	Events      []Event             `json:"events,omitempty"`
}

func BuildReport(ledger *Ledger, includeEvents bool) Report {
	ledger.RefreshDerivedVaultClaims()
	risk := BuildRiskReport(ledger)
	audit := BuildAuditReport(ledger, risk)
	report := Report{
		Name:        ledger.Name,
		Epoch:       ledger.Epoch,
		Policy:      ledger.Policy,
		Accounts:    BuildAccountReports(ledger),
		Vaults:      BuildVaultReports(ledger),
		Commitments: BuildCommitmentReports(ledger),
		Subs:        BuildSubReports(ledger),
		Locks:       BuildLockReports(ledger),
		Risk:        risk,
		Audit:       audit,
		Timeline:    BuildTimeline(ledger),
		Planner:     BuildPlanner(ledger, risk),
		Rejected:    append([]RejectedOperation{}, ledger.Rejected...),
	}
	report.Metrics = BuildMetrics(ledger, risk, audit)
	if includeEvents {
		report.Events = append([]Event{}, ledger.Events...)
	}
	return report
}

func BuildAccountReports(ledger *Ledger) []AccountReport {
	out := []AccountReport{}
	for _, accountID := range ledger.SortedAccountIDs() {
		account := ledger.Accounts[accountID]
		if account == nil {
			continue
		}
		out = append(out, AccountReport{
			ID:               account.ID,
			Label:            account.Label,
			Reserves:         amountEntries(account.Reserves),
			ExternalIn:       account.ExternalIn,
			ExternalOut:      account.ExternalOut,
			ClaimsReceived:   account.ClaimsReceived,
			SponsorReleases:  account.SponsorReleases,
			PenaltiesPaid:    account.PenaltiesPaid,
			RejectedAttempts: account.RejectedAttempts,
		})
	}
	return out
}

func BuildVaultReports(ledger *Ledger) []VaultReport {
	out := []VaultReport{}
	for _, vaultID := range ledger.SortedVaultIDs() {
		vault := ledger.Vaults[vaultID]
		if vault == nil {
			continue
		}
		out = append(out, VaultReport{
			ID:             vault.ID,
			Asset:          vault.Asset,
			Reserve:        vault.Reserve,
			Locked:         vault.Locked,
			Fees:           vault.Fees,
			Insolvency:     vault.Insolvency,
			ClaimShortfall: vault.ClaimShortfall,
			OpenClaims:     vault.OpenClaims,
			FeeAccount:     vault.FeeAccount,
		})
	}
	return out
}

func BuildCommitmentReports(ledger *Ledger) []CommitmentReport {
	out := []CommitmentReport{}
	for _, commitmentID := range ledger.SortedCommitmentIDs() {
		commitment := ledger.Commitments[commitmentID]
		if commitment == nil {
			continue
		}
		subIDs := append([]string{}, commitment.SubIDs...)
		sort.Strings(subIDs)
		out = append(out, CommitmentReport{
			ID:                  commitment.ID,
			Vault:               commitment.VaultID,
			Sponsor:             commitment.Sponsor,
			Amount:              commitment.Amount,
			Delivered:           ledger.DeliveredForCommitment(commitment),
			Claimed:             commitment.Claimed,
			ReleasedToSponsor:   commitment.ReleasedToSponsor,
			PenaltyAccrued:      commitment.PenaltyAccrued,
			FeeAccrued:          commitment.FeeAccrued,
			DueEpoch:            commitment.DueEpoch,
			ClosedEpoch:         commitment.ClosedEpoch,
			Status:              commitment.Status,
			Composite:           commitment.Composite,
			ReleaseThresholdBps: commitment.ReleaseThresholdBps,
			ParentLockID:        commitment.ParentLockID,
			SubIDs:              subIDs,
			Outstanding:         CommitmentOutstanding(ledger, commitment),
			Terminal:            CommitmentTerminal(ledger, commitment),
		})
	}
	return out
}

func BuildSubReports(ledger *Ledger) []SubReport {
	out := []SubReport{}
	for _, subID := range ledger.SortedSubIDs() {
		sub := ledger.Subs[subID]
		if sub == nil {
			continue
		}
		out = append(out, SubReport{
			ID:           sub.ID,
			ParentID:     sub.ParentID,
			Vault:        sub.VaultID,
			Sponsor:      sub.Sponsor,
			Beneficiary:  sub.Beneficiary,
			Label:        sub.Label,
			Amount:       sub.Amount,
			Delivered:    sub.Delivered,
			Claimed:      sub.Claimed,
			Penalty:      sub.Penalty,
			DueEpoch:     sub.DueEpoch,
			State:        sub.State,
			SubLockID:    sub.SubLockID,
			Remaining:    RemainingAmount(sub.Amount, sub.Claimed, sub.Penalty),
			DeliveredBps: DeliveryRatio(sub.Delivered, sub.Amount),
		})
	}
	return out
}

func BuildLockReports(ledger *Ledger) []LockReport {
	out := []LockReport{}
	for _, lockID := range ledger.SortedLockIDs() {
		lock := ledger.Locks[lockID]
		if lock == nil {
			continue
		}
		out = append(out, LockReport{
			ID:            lock.ID,
			Commitment:    lock.CommitmentID,
			Sub:           lock.SubID,
			Scope:         lock.Scope,
			Vault:         lock.VaultID,
			Account:       lock.Account,
			Beneficiary:   lock.Beneficiary,
			Amount:        lock.Amount,
			Released:      lock.Released,
			Claimed:       lock.Claimed,
			Penalty:       lock.Penalty,
			State:         lock.State,
			CreatedEpoch:  lock.CreatedEpoch,
			DueEpoch:      lock.DueEpoch,
			ReleasedEpoch: lock.ReleasedEpoch,
		})
	}
	return out
}

func BuildMetrics(ledger *Ledger, risk RiskReport, audit AuditReport) MetricsReport {
	metrics := MetricsReport{
		TotalEconomicClaims: risk.TotalClaims,
		EconomicShortfall:   risk.TotalShortfall,
		RejectedOperations:  int64(len(ledger.Rejected)),
		Commitments:         int64(len(ledger.Commitments)),
		SubCommitments:      int64(len(ledger.Subs)),
		Locks:               int64(len(ledger.Locks)),
		ClaimableSubs:       audit.ClaimableSubs,
		PendingSubs:         audit.PendingSubs,
		DefaultedSubs:       audit.DefaultedSubs,
	}
	for _, vault := range ledger.Vaults {
		if vault != nil {
			metrics.TotalReserve += vault.Reserve
		}
	}
	for _, account := range ledger.Accounts {
		if account == nil {
			continue
		}
		metrics.TotalAccountReserves += SumAmountMap(account.Reserves)
		metrics.SponsorWithdrawals += account.ExternalOut
		metrics.SponsorReleases += account.SponsorReleases
		metrics.BeneficiaryClaims += account.ClaimsReceived
	}
	return metrics
}

func amountEntries(values map[string]int64) []AmountEntry {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]AmountEntry, 0, len(keys))
	for _, key := range keys {
		out = append(out, AmountEntry{ID: key, Amount: values[key]})
	}
	return out
}

func WriteReport(writer io.Writer, report Report) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func WriteCompactReport(writer io.Writer, report Report) error {
	encoder := json.NewEncoder(writer)
	return encoder.Encode(report)
}
