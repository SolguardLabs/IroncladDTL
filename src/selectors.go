package main

import "sort"

type CommitmentSelector struct {
	ID                  string `json:"id"`
	Vault               string `json:"vault"`
	Sponsor             string `json:"sponsor"`
	Status              string `json:"status"`
	Composite           bool   `json:"composite"`
	ReleaseThresholdBps int64  `json:"release_threshold_bps"`
	Outstanding         int64  `json:"outstanding"`
	DeliveredBps        int64  `json:"delivered_bps"`
	ParentState         string `json:"parent_state"`
}

type AccountExposure struct {
	Account          string `json:"account"`
	AsSponsor        int64  `json:"as_sponsor"`
	AsBeneficiary    int64  `json:"as_beneficiary"`
	ReleasedToAccount int64 `json:"released_to_account"`
	ExternalOut      int64  `json:"external_out"`
}

func SelectCommitments(ledger *Ledger) []CommitmentSelector {
	out := []CommitmentSelector{}
	for _, id := range ledger.SortedCommitmentIDs() {
		commitment := ledger.Commitments[id]
		if commitment == nil {
			continue
		}
		parentState := ""
		if parent := ledger.Locks[commitment.ParentLockID]; parent != nil {
			parentState = parent.State
		}
		out = append(out, CommitmentSelector{
			ID:                  commitment.ID,
			Vault:               commitment.VaultID,
			Sponsor:             commitment.Sponsor,
			Status:              commitment.Status,
			Composite:           commitment.Composite,
			ReleaseThresholdBps: commitment.ReleaseThresholdBps,
			Outstanding:         CommitmentOutstanding(ledger, commitment),
			DeliveredBps:        DeliveryRatio(ledger.DeliveredForCommitment(commitment), commitment.Amount),
			ParentState:         parentState,
		})
	}
	return out
}

func SelectOpenCommitments(ledger *Ledger) []CommitmentSelector {
	all := SelectCommitments(ledger)
	out := []CommitmentSelector{}
	for _, item := range all {
		if item.Status != StateClosed {
			out = append(out, item)
		}
	}
	return out
}

func SelectCompositeCommitments(ledger *Ledger) []CommitmentSelector {
	all := SelectCommitments(ledger)
	out := []CommitmentSelector{}
	for _, item := range all {
		if item.Composite {
			out = append(out, item)
		}
	}
	return out
}

func SelectReleasedCompositeCommitments(ledger *Ledger) []CommitmentSelector {
	all := SelectCompositeCommitments(ledger)
	out := []CommitmentSelector{}
	for _, item := range all {
		if item.ParentState == StateReleased {
			out = append(out, item)
		}
	}
	return out
}

func SelectAccountExposure(ledger *Ledger) []AccountExposure {
	rows := map[string]*AccountExposure{}
	for _, accountID := range ledger.SortedAccountIDs() {
		rows[accountID] = &AccountExposure{Account: accountID}
		if account := ledger.Accounts[accountID]; account != nil {
			rows[accountID].ExternalOut = account.ExternalOut
		}
	}
	for _, commitment := range ledger.Commitments {
		if commitment == nil {
			continue
		}
		row := rows[commitment.Sponsor]
		if row == nil {
			row = &AccountExposure{Account: commitment.Sponsor}
			rows[commitment.Sponsor] = row
		}
		row.AsSponsor += CommitmentOutstanding(ledger, commitment)
		row.ReleasedToAccount += commitment.ReleasedToSponsor
	}
	for _, sub := range ledger.Subs {
		if sub == nil {
			continue
		}
		if !IsOpenSubState(sub.State) {
			continue
		}
		row := rows[sub.Beneficiary]
		if row == nil {
			row = &AccountExposure{Account: sub.Beneficiary}
			rows[sub.Beneficiary] = row
		}
		row.AsBeneficiary += RemainingAmount(sub.Amount, sub.Claimed, sub.Penalty)
	}
	keys := make([]string, 0, len(rows))
	for key := range rows {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := []AccountExposure{}
	for _, key := range keys {
		out = append(out, *rows[key])
	}
	return out
}

func HasOpenChild(ledger *Ledger, commitmentID string) bool {
	commitment := ledger.Commitments[commitmentID]
	if commitment == nil {
		return false
	}
	for _, subID := range commitment.SubIDs {
		sub := ledger.Subs[subID]
		if sub != nil && IsOpenSubState(sub.State) {
			return true
		}
	}
	return false
}

func HasClaimableChild(ledger *Ledger, commitmentID string) bool {
	commitment := ledger.Commitments[commitmentID]
	if commitment == nil {
		return false
	}
	for _, subID := range commitment.SubIDs {
		sub := ledger.Subs[subID]
		if sub != nil && sub.State == StateClaimable {
			return true
		}
	}
	return false
}

func HasPendingChild(ledger *Ledger, commitmentID string) bool {
	commitment := ledger.Commitments[commitmentID]
	if commitment == nil {
		return false
	}
	for _, subID := range commitment.SubIDs {
		sub := ledger.Subs[subID]
		if sub != nil && (sub.State == StatePending || sub.State == StateDelivered) {
			return true
		}
	}
	return false
}

func ParentLockState(ledger *Ledger, commitmentID string) string {
	commitment := ledger.Commitments[commitmentID]
	if commitment == nil {
		return ""
	}
	lock := ledger.Locks[commitment.ParentLockID]
	if lock == nil {
		return ""
	}
	return lock.State
}

func SubLockState(ledger *Ledger, subID string) string {
	sub := ledger.Subs[subID]
	if sub == nil {
		return ""
	}
	lock := ledger.Locks[sub.SubLockID]
	if lock == nil {
		return ""
	}
	return lock.State
}

func VaultAssetForCommitment(ledger *Ledger, commitmentID string) string {
	commitment := ledger.Commitments[commitmentID]
	if commitment == nil {
		return ""
	}
	vault := ledger.Vaults[commitment.VaultID]
	if vault == nil {
		return ""
	}
	return vault.Asset
}
