package main

import "testing"

func TestEvaluateStressBuildsLiquidityHorizons(t *testing.T) {
	input := StressInput{
		Epoch: 10, Reserve: 200_000, Fees: 10_000,
		Claims: []StressClaim{
			{ID: "c-1", Beneficiary: "carrier-a", Amount: 40_000, DueEpoch: 10, State: StateClaimable, DeliveredBps: 10_000},
			{ID: "c-2", Beneficiary: "carrier-b", Amount: 60_000, DueEpoch: 12, State: StatePending, DeliveredBps: 2_500},
			{ID: "c-3", Beneficiary: "carrier-c", Amount: 50_000, DueEpoch: 18, State: StatePending, DeliveredBps: 0},
			{ID: "c-4", Beneficiary: "carrier-a", Amount: 10_000, DueEpoch: 30, State: StatePending, DeliveredBps: 0},
		},
	}
	result, err := EvaluateStress(input, DefaultStressParameters())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Horizons) != 4 {
		t.Fatalf("expected 4 horizons, got %d", len(result.Horizons))
	}
	if result.Horizons[0].Claims != 40_000 || result.Horizons[1].Claims != 60_000 {
		t.Fatalf("unexpected horizon claims: %+v", result.Horizons)
	}
	if result.GrossClaims != 160_000 {
		t.Fatalf("unexpected gross claims: %d", result.GrossClaims)
	}
	if result.AvailableLiquidity != 198_000 {
		t.Fatalf("unexpected available liquidity: %d", result.AvailableLiquidity)
	}
	if result.WeightedMaturityEpoch != 4 {
		t.Fatalf("unexpected weighted maturity: %d", result.WeightedMaturityEpoch)
	}
}

func TestEvaluateStressRejectsDuplicateClaims(t *testing.T) {
	input := StressInput{Claims: []StressClaim{
		{ID: "same", Beneficiary: "a", Amount: 1, State: StatePending},
		{ID: "same", Beneficiary: "b", Amount: 1, State: StatePending},
	}}
	if _, err := EvaluateStress(input, DefaultStressParameters()); err == nil {
		t.Fatal("expected duplicate claim error")
	}
}

func TestEvaluateStressReportsFundingGap(t *testing.T) {
	parameters := DefaultStressParameters()
	parameters.ReserveHaircutBps = 1_000
	input := StressInput{
		Reserve: 50_000,
		Claims:  []StressClaim{{ID: "claim", Beneficiary: "carrier", Amount: 100_000, State: StateClaimable}},
	}
	result, err := EvaluateStress(input, parameters)
	if err != nil {
		t.Fatal(err)
	}
	if result.FundingGap != 67_500 {
		t.Fatalf("unexpected gap: %d", result.FundingGap)
	}
	if result.SolvencyPass {
		t.Fatal("expected solvency corridor to fail")
	}
}

func TestStressParametersValidation(t *testing.T) {
	parameters := DefaultStressParameters()
	parameters.TermEpochs = 1
	parameters.NearEpochs = 2
	if err := parameters.Validate(); err == nil {
		t.Fatal("expected invalid horizon configuration")
	}
}

func TestBuildConcentrationsUsesDeterministicTieBreak(t *testing.T) {
	rows := buildConcentrations(map[string]int64{"beta": 50, "alpha": 50}, 100)
	if len(rows) != 2 || rows[0].Beneficiary != "alpha" || rows[0].ShareBps != 5_000 {
		t.Fatalf("unexpected concentrations: %+v", rows)
	}
}

func TestExpectedStressClaimHonorsDeliveryFloor(t *testing.T) {
	parameters := DefaultStressParameters()
	claim := StressClaim{ID: "claim", Beneficiary: "carrier", Amount: 100_000, State: StatePending, DeliveredBps: 9_500}
	amount, err := expectedStressClaim(claim, parameters)
	if err != nil {
		t.Fatal(err)
	}
	if amount != 95_000 {
		t.Fatalf("expected 95000, got %d", amount)
	}
}
