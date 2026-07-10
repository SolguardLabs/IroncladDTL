package main

import (
	"sort"
	"strings"
)

type ControlDefinition struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Severity    string   `json:"severity"`
	Component   string   `json:"component"`
	Signal      string   `json:"signal"`
	Rationale   string   `json:"rationale"`
	Remediation string   `json:"remediation"`
	Tags        []string `json:"tags"`
}

type PlaybookStep struct {
	ID          string `json:"id"`
	Phase       string `json:"phase"`
	Trigger     string `json:"trigger"`
	Action      string `json:"action"`
	Expected    string `json:"expected"`
	Owner       string `json:"owner"`
	Automatable bool   `json:"automatable"`
}

var BuiltInScenarioNotes = map[string]string{
	"base_lifecycle":     "single-beneficiary lock, delivery, claim, close",
	"composite_closure":  "multi-beneficiary commitment closed after every subclaim",
	"penalty_withdrawal": "expired subcommitment, penalty, rejected over-withdrawal",
	"composite_exploit":  "threshold-based parent release with pending sublocks",
}

var ControlCatalog = []ControlDefinition{
	{
		ID:          "ICD-COMP-001",
		Title:       "Parent unlock requires terminal children",
		Severity:    "critical",
		Component:   "composite-locks",
		Signal:      "parent lock state is released and at least one child sublock remains pending or claimable",
		Rationale:   "A parent lock represents aggregate reserve backing and must not become sponsor liquidity while child claims still exist.",
		Remediation: "Gate parent release on all subcommitments being claimed, defaulted, cancelled, or explicitly waived.",
		Tags:        []string{"ctf", "composite", "unlock", "economic-claim"},
	},
	{
		ID:          "ICD-COMP-002",
		Title:       "Aggregate delivery is not aggregate settlement",
		Severity:    "high",
		Component:   "delivery-accounting",
		Signal:      "delivered_bps reaches threshold before child-level claims have been paid",
		Rationale:   "Delivery progress and settlement finality are separate states with different owners of value.",
		Remediation: "Track delivered, claimable, claimed, penalized, and residual amounts independently.",
		Tags:        []string{"delivery", "settlement", "state-machine"},
	},
	{
		ID:          "ICD-COMP-003",
		Title:       "Claimable value remains vault-backed",
		Severity:    "critical",
		Component:   "claims",
		Signal:      "claimable child amount exceeds available reserve after sponsor release",
		Rationale:   "A claimable beneficiary has already performed and should have seniority over sponsor withdrawals.",
		Remediation: "Reserve claimable child amounts before any sponsor residual transfer.",
		Tags:        []string{"beneficiary", "claim", "reserve"},
	},
	{
		ID:          "ICD-COMP-004",
		Title:       "Pending child value remains segregated",
		Severity:    "critical",
		Component:   "claims",
		Signal:      "pending child amount is counted as open economic claim while parent release is recorded",
		Rationale:   "Pending value may still belong to a third party if the delivery arrives within its due window.",
		Remediation: "Keep parent collateral locked until the pending child reaches a terminal outcome.",
		Tags:        []string{"pending", "escrow", "third-party"},
	},
	{
		ID:          "ICD-COMP-005",
		Title:       "Composite threshold cannot release full notional",
		Severity:    "high",
		Component:   "thresholds",
		Signal:      "release amount equals parent amount when delivered amount is lower than parent amount",
		Rationale:   "A threshold can drive review or partial unlock, but not full notional release without child settlement.",
		Remediation: "Limit threshold-triggered release to proven residual after beneficiary obligations.",
		Tags:        []string{"threshold", "notional", "partial"},
	},
	{
		ID:          "ICD-LOCK-001",
		Title:       "Technical lock and economic claim reconciliation",
		Severity:    "high",
		Component:   "reconciliation",
		Signal:      "technical_locked is lower than economic_claims while commitments are open",
		Rationale:   "The lock table may appear clean even when economic obligations still exceed reserved funds.",
		Remediation: "Reconcile against child claims, not only against parent lock states.",
		Tags:        []string{"reconciliation", "lock-table", "claims"},
	},
	{
		ID:          "ICD-LOCK-002",
		Title:       "Released parent with claimable child is terminal mismatch",
		Severity:    "critical",
		Component:   "state-machine",
		Signal:      "parent released and child state claimable",
		Rationale:   "A claimable child is senior to sponsor residual value.",
		Remediation: "Block parent release or auto-pay the child before sponsor crediting.",
		Tags:        []string{"claimable", "state", "seniority"},
	},
	{
		ID:          "ICD-LOCK-003",
		Title:       "Released parent with delivered child is timing mismatch",
		Severity:    "high",
		Component:   "state-machine",
		Signal:      "parent released and child state delivered",
		Rationale:   "Delivered children may become claimable during the same settlement cycle.",
		Remediation: "Settle children before evaluating parent residuals.",
		Tags:        []string{"delivered", "cycle-ordering"},
	},
	{
		ID:          "ICD-LOCK-004",
		Title:       "Released parent with pending child is escrow violation",
		Severity:    "critical",
		Component:   "state-machine",
		Signal:      "parent released and child state pending",
		Rationale:   "The pending child still has a contractual delivery window.",
		Remediation: "Leave collateral locked or create a child-specific reserve.",
		Tags:        []string{"pending", "escrow", "window"},
	},
	{
		ID:          "ICD-LOCK-005",
		Title:       "Parent release should be idempotent",
		Severity:    "medium",
		Component:   "idempotency",
		Signal:      "multiple parent release events for the same commitment",
		Rationale:   "Repeated release attempts can hide double-credit bugs or accounting churn.",
		Remediation: "Use monotonic release fields and reject release when parent state is not locked.",
		Tags:        []string{"idempotency", "events"},
	},
	{
		ID:          "ICD-WD-001",
		Title:       "Sponsor withdrawal must exclude locked reserve",
		Severity:    "critical",
		Component:   "withdrawals",
		Signal:      "sponsor external_out increases while related child claims remain open",
		Rationale:   "The exploit path becomes economically damaging only when released funds leave the system.",
		Remediation: "Maintain a reserved balance ledger per sponsor and vault.",
		Tags:        []string{"withdrawal", "sponsor", "reserve"},
	},
	{
		ID:          "ICD-WD-002",
		Title:       "Rejected withdrawals are first-class audit events",
		Severity:    "medium",
		Component:   "withdrawals",
		Signal:      "withdraw operation fails without rejected operation record",
		Rationale:   "CTF tests need deterministic failure evidence for over-withdrawals.",
		Remediation: "Record code, field, operation id, and requested amount on expected rejects.",
		Tags:        []string{"withdrawal", "audit", "tests"},
	},
	{
		ID:          "ICD-WD-003",
		Title:       "Sponsor release should not be fungible until claim window closes",
		Severity:    "high",
		Component:   "withdrawals",
		Signal:      "sponsor balance receives release before due epoch of at least one child",
		Rationale:   "A sponsor can externalize funds before late but valid delivery arrives.",
		Remediation: "Credit sponsor residuals to a pending bucket until every child is terminal.",
		Tags:        []string{"fungibility", "release", "due-epoch"},
	},
	{
		ID:          "ICD-PEN-001",
		Title:       "Penalty must consume only defaulted child value",
		Severity:    "high",
		Component:   "penalties",
		Signal:      "penalty amount is applied to unrelated child or parent notional",
		Rationale:   "A child default should not reduce another beneficiary claim.",
		Remediation: "Apply penalties at subcommitment granularity and roll up totals after settlement.",
		Tags:        []string{"penalty", "child", "rollup"},
	},
	{
		ID:          "ICD-PEN-002",
		Title:       "Grace period participates in overdue checks",
		Severity:    "medium",
		Component:   "penalties",
		Signal:      "penalty fires at due_epoch when grace_epochs is configured",
		Rationale:   "Incorrect timing can make otherwise valid deliveries default.",
		Remediation: "Use due_epoch + grace_epochs for every overdue decision.",
		Tags:        []string{"penalty", "time", "grace"},
	},
	{
		ID:          "ICD-PEN-003",
		Title:       "Penalty and release ordering is explicit",
		Severity:    "high",
		Component:   "settlement-order",
		Signal:      "release event precedes penalty event in the same settlement for related commitment",
		Rationale:   "Ordering errors create stale surplus and stale reserve views.",
		Remediation: "Process child penalties and claims before parent residual calculations.",
		Tags:        []string{"order", "penalty", "release"},
	},
	{
		ID:          "ICD-CLM-001",
		Title:       "Claim shortfall is insolvency, not silent partial payment",
		Severity:    "critical",
		Component:   "claims",
		Signal:      "claim pays less than remaining amount and no insolvency is recorded",
		Rationale:   "Partial payment without insolvency hides loss from reports.",
		Remediation: "Record claim_shortfall and vault insolvency on every underpaid claim.",
		Tags:        []string{"claim", "shortfall", "insolvency"},
	},
	{
		ID:          "ICD-CLM-002",
		Title:       "Claimed child updates parent totals",
		Severity:    "medium",
		Component:   "claims",
		Signal:      "sub claimed amount differs from parent claimed rollup",
		Rationale:   "Parent close logic depends on accurate rollups.",
		Remediation: "Refresh parent totals after every claim and before every close.",
		Tags:        []string{"claim", "rollup", "close"},
	},
	{
		ID:          "ICD-CLM-003",
		Title:       "Claimable state is not terminal",
		Severity:    "medium",
		Component:   "claims",
		Signal:      "commitment close succeeds with child state claimable",
		Rationale:   "Claimable funds still belong to the beneficiary and must be paid or explicitly waived.",
		Remediation: "Reject close until claimable children are claimed.",
		Tags:        []string{"claimable", "close"},
	},
	{
		ID:          "ICD-CLS-001",
		Title:       "Close requires terminal children",
		Severity:    "high",
		Component:   "close",
		Signal:      "close event exists while child state is pending, delivered, or claimable",
		Rationale:   "Closing an aggregate commitment cannot erase child-level economic rights.",
		Remediation: "Require terminal child states or produce deterministic rejection.",
		Tags:        []string{"close", "terminal", "children"},
	},
	{
		ID:          "ICD-CLS-002",
		Title:       "Residual release is subordinate to child claims",
		Severity:    "high",
		Component:   "close",
		Signal:      "residual release amount plus child claims exceeds parent amount",
		Rationale:   "Residual is the leftover after claims, penalties, and fees.",
		Remediation: "Compute residual from child settlement ledger, not from parent lock state alone.",
		Tags:        []string{"residual", "close", "claims"},
	},
	{
		ID:          "ICD-CLS-003",
		Title:       "Closed parent should have no outstanding exposure",
		Severity:    "medium",
		Component:   "close",
		Signal:      "commitment status closed and outstanding exposure is nonzero",
		Rationale:   "A closed commitment should be economically inert.",
		Remediation: "Recalculate outstanding exposure before marking closed.",
		Tags:        []string{"closed", "exposure"},
	},
	{
		ID:          "ICD-RPT-001",
		Title:       "Reports include technical and economic views",
		Severity:    "medium",
		Component:   "reporting",
		Signal:      "JSON output omits either lock state or economic claim state",
		Rationale:   "The CTF hinges on divergence between technical locks and economic ownership.",
		Remediation: "Emit locks, subs, commitments, risk, audit, planner, and events.",
		Tags:        []string{"reporting", "json", "ctf"},
	},
	{
		ID:          "ICD-RPT-002",
		Title:       "Report ordering is deterministic",
		Severity:    "low",
		Component:   "reporting",
		Signal:      "arrays change order across identical runs",
		Rationale:   "Node tests should not depend on Go map iteration order.",
		Remediation: "Sort accounts, vaults, commitments, subs, locks, and warnings by id.",
		Tags:        []string{"determinism", "tests"},
	},
	{
		ID:          "ICD-RPT-003",
		Title:       "Critical findings drive planner output",
		Severity:    "medium",
		Component:   "planner",
		Signal:      "audit critical finding exists but planner has no mitigation action",
		Rationale:   "Operators need a concrete response path during CTF analysis.",
		Remediation: "Map shortfall and parent-release mismatches into planner actions.",
		Tags:        []string{"planner", "audit", "response"},
	},
	{
		ID:          "ICD-TST-001",
		Title:       "Exploit fixture asserts vulnerable behavior",
		Severity:    "high",
		Component:   "tests",
		Signal:      "composite exploit fixture does not produce economic_shortfall",
		Rationale:   "The lab should demonstrate the intended bug reliably.",
		Remediation: "Keep a fixture that releases the parent at threshold and then withdraws sponsor funds.",
		Tags:        []string{"tests", "exploit", "fixture"},
	},
	{
		ID:          "ICD-TST-002",
		Title:       "Healthy composite fixture avoids false positive",
		Severity:    "medium",
		Component:   "tests",
		Signal:      "all children claimed and audit still flags parent mismatch",
		Rationale:   "The detector should distinguish exploit state from normal full closure.",
		Remediation: "Include a fixture where every child is delivered, claimed, and closed.",
		Tags:        []string{"tests", "healthy", "composite"},
	},
	{
		ID:          "ICD-TST-003",
		Title:       "Penalty fixture proves rejected over-withdrawal",
		Severity:    "medium",
		Component:   "tests",
		Signal:      "over-withdrawal returns success after penalty-only release",
		Rationale:   "Basic withdrawal controls must continue to work outside the exploit path.",
		Remediation: "Use expect_error operations for negative-path tests.",
		Tags:        []string{"tests", "withdrawal", "penalty"},
	},
	{
		ID:          "ICD-OPS-001",
		Title:       "Freeze before recapitalization",
		Severity:    "high",
		Component:   "operations",
		Signal:      "shortfall exists and sponsor withdrawals remain enabled",
		Rationale:   "Further withdrawals can amplify loss before top-up arrives.",
		Remediation: "Freeze sponsor withdrawal lanes for affected vault and commitment.",
		Tags:        []string{"ops", "freeze", "shortfall"},
	},
	{
		ID:          "ICD-OPS-002",
		Title:       "Beneficiary notification on claim shortfall",
		Severity:    "medium",
		Component:   "operations",
		Signal:      "claim_shortfall increases",
		Rationale:   "Beneficiaries need deterministic evidence for underpayment.",
		Remediation: "Emit claim_shortfall event with commitment and sub ids.",
		Tags:        []string{"ops", "claim", "notification"},
	},
	{
		ID:          "ICD-OPS-003",
		Title:       "Vault top-up target equals economic shortfall",
		Severity:    "medium",
		Component:   "operations",
		Signal:      "recapitalization action amount differs from economic_shortfall",
		Rationale:   "Top-up plans should match reported economic deficit.",
		Remediation: "Use risk.TotalShortfall as the recapitalization amount.",
		Tags:        []string{"ops", "top-up", "risk"},
	},
}

var IncidentPlaybook = []PlaybookStep{
	{
		ID:          "PB-001",
		Phase:       "detect",
		Trigger:     "audit check no_released_parent_with_open_subs fails",
		Action:      "identify affected commitment and all child sublocks",
		Expected:    "planner emits freeze_sponsor_withdrawals",
		Owner:       "risk",
		Automatable: true,
	},
	{
		ID:          "PB-002",
		Phase:       "contain",
		Trigger:     "sponsor release mismatch confirmed",
		Action:      "pause sponsor withdrawals for the vault asset",
		Expected:    "no additional external_out is recorded for affected sponsor",
		Owner:       "operations",
		Automatable: false,
	},
	{
		ID:          "PB-003",
		Phase:       "measure",
		Trigger:     "economic_shortfall greater than zero",
		Action:      "compute child-level pending and claimable amounts",
		Expected:    "shortfall equals max(economic claims minus reserve, zero)",
		Owner:       "risk",
		Automatable: true,
	},
	{
		ID:          "PB-004",
		Phase:       "repair",
		Trigger:     "vault reserve below economic claims",
		Action:      "recapitalize vault or claw back sponsor release",
		Expected:    "economic_claims_covered check passes",
		Owner:       "treasury",
		Automatable: false,
	},
	{
		ID:          "PB-005",
		Phase:       "patch",
		Trigger:     "root cause accepted",
		Action:      "gate parent release on terminal child states",
		Expected:    "exploit fixture no longer releases parent before all child claims are terminal",
		Owner:       "engineering",
		Automatable: false,
	},
	{
		ID:          "PB-006",
		Phase:       "regress",
		Trigger:     "patch merged",
		Action:      "run healthy and exploit fixtures as regression suite",
		Expected:    "healthy fixtures pass and exploit fixture changes expected assertions",
		Owner:       "qa",
		Automatable: true,
	},
}

func ControlsBySeverity(severity string) []ControlDefinition {
	needle := strings.ToLower(strings.TrimSpace(severity))
	out := []ControlDefinition{}
	for _, control := range ControlCatalog {
		if strings.ToLower(control.Severity) == needle {
			out = append(out, control)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func ControlsByComponent(component string) []ControlDefinition {
	needle := strings.ToLower(strings.TrimSpace(component))
	out := []ControlDefinition{}
	for _, control := range ControlCatalog {
		if strings.ToLower(control.Component) == needle {
			out = append(out, control)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func ControlsByTag(tag string) []ControlDefinition {
	needle := strings.ToLower(strings.TrimSpace(tag))
	out := []ControlDefinition{}
	for _, control := range ControlCatalog {
		for _, candidate := range control.Tags {
			if strings.ToLower(candidate) == needle {
				out = append(out, control)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func CriticalControlIDs() []string {
	controls := ControlsBySeverity("critical")
	ids := make([]string, 0, len(controls))
	for _, control := range controls {
		ids = append(ids, control.ID)
	}
	return ids
}

func ControlIDsForFinding(checkName string) []string {
	switch checkName {
	case "no_released_parent_with_open_subs":
		return []string{"ICD-COMP-001", "ICD-LOCK-002", "ICD-LOCK-004", "ICD-WD-001"}
	case "economic_claims_covered":
		return []string{"ICD-COMP-003", "ICD-CLM-001", "ICD-OPS-003"}
	case "commitment_totals":
		return []string{"ICD-CLM-002", "ICD-CLS-002"}
	case "no_negative_accounts", "no_negative_vaults":
		return []string{"ICD-RPT-001"}
	default:
		return []string{}
	}
}

func PlaybookForPhase(phase string) []PlaybookStep {
	needle := strings.ToLower(strings.TrimSpace(phase))
	out := []PlaybookStep{}
	for _, step := range IncidentPlaybook {
		if strings.ToLower(step.Phase) == needle {
			out = append(out, step)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func ScenarioNote(name string) string {
	if note, ok := BuiltInScenarioNotes[name]; ok {
		return note
	}
	return ""
}

func CatalogSummary() map[string]int {
	summary := map[string]int{}
	for _, control := range ControlCatalog {
		summary[control.Severity]++
		summary["component:"+control.Component]++
		for _, tag := range control.Tags {
			summary["tag:"+tag]++
		}
	}
	return summary
}
