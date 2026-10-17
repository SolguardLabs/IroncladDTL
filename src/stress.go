package main

import (
	"errors"
	"sort"
)

const (
	HorizonImmediate = "immediate"
	HorizonNear      = "near"
	HorizonTerm      = "term"
	HorizonLong      = "long"
)

// StressParameters defines a deterministic solvency corridor. All rates use
// basis points and all amounts use the same integer units as the ledger.
type StressParameters struct {
	ReserveHaircutBps      int64 `json:"reserve_haircut_bps"`
	FeeRealizationBps      int64 `json:"fee_realization_bps"`
	CounterpartyDefaultBps int64 `json:"counterparty_default_bps"`
	RecoveryBps            int64 `json:"recovery_bps"`
	LiquidityBufferBps     int64 `json:"liquidity_buffer_bps"`
	ConcentrationLimitBps  int64 `json:"concentration_limit_bps"`
	NearEpochs             int64 `json:"near_epochs"`
	TermEpochs             int64 `json:"term_epochs"`
}

type StressClaim struct {
	ID           string `json:"id"`
	Beneficiary  string `json:"beneficiary"`
	Amount       int64  `json:"amount"`
	DueEpoch     int64  `json:"due_epoch"`
	State        string `json:"state"`
	DeliveredBps int64  `json:"delivered_bps"`
}

type StressInput struct {
	Epoch   int64         `json:"epoch"`
	Reserve int64         `json:"reserve"`
	Fees    int64         `json:"fees"`
	Claims  []StressClaim `json:"claims"`
}

type HorizonPosition struct {
	Horizon     string `json:"horizon"`
	Claims      int64  `json:"claims"`
	Expected    int64  `json:"expected"`
	Buffered    int64  `json:"buffered"`
	ClaimCount  int64  `json:"claim_count"`
	CoverageBps int64  `json:"coverage_bps"`
	FundingGap  int64  `json:"funding_gap"`
}

type ConcentrationPosition struct {
	Beneficiary string `json:"beneficiary"`
	Amount      int64  `json:"amount"`
	ShareBps    int64  `json:"share_bps"`
}

type StressResult struct {
	ReserveAfterHaircut   int64                   `json:"reserve_after_haircut"`
	RealizableFees        int64                   `json:"realizable_fees"`
	AvailableLiquidity    int64                   `json:"available_liquidity"`
	GrossClaims           int64                   `json:"gross_claims"`
	ExpectedClaims        int64                   `json:"expected_claims"`
	BufferedClaims        int64                   `json:"buffered_claims"`
	CoverageBps           int64                   `json:"coverage_bps"`
	FundingGap            int64                   `json:"funding_gap"`
	WeightedMaturityEpoch int64                   `json:"weighted_maturity_epoch"`
	MaxConcentrationBps   int64                   `json:"max_concentration_bps"`
	ConcentrationPass     bool                    `json:"concentration_pass"`
	SolvencyPass          bool                    `json:"solvency_pass"`
	Horizons              []HorizonPosition       `json:"horizons"`
	Concentrations        []ConcentrationPosition `json:"concentrations"`
}

func DefaultStressParameters() StressParameters {
	return StressParameters{
		ReserveHaircutBps:      500,
		FeeRealizationBps:      8_000,
		CounterpartyDefaultBps: 1_500,
		RecoveryBps:            4_000,
		LiquidityBufferBps:     1_250,
		ConcentrationLimitBps:  3_500,
		NearEpochs:             2,
		TermEpochs:             8,
	}
}

func (p StressParameters) Validate() error {
	rates := []struct {
		name  string
		value int64
	}{
		{"reserve_haircut_bps", p.ReserveHaircutBps},
		{"fee_realization_bps", p.FeeRealizationBps},
		{"counterparty_default_bps", p.CounterpartyDefaultBps},
		{"recovery_bps", p.RecoveryBps},
		{"liquidity_buffer_bps", p.LiquidityBufferBps},
		{"concentration_limit_bps", p.ConcentrationLimitBps},
	}
	for _, rate := range rates {
		if rate.value < 0 || rate.value > BasisPoints {
			return errors.New(rate.name + " must be between 0 and 10000")
		}
	}
	if p.NearEpochs < 0 {
		return errors.New("near_epochs must be non-negative")
	}
	if p.TermEpochs < p.NearEpochs {
		return errors.New("term_epochs must be greater than or equal to near_epochs")
	}
	return nil
}

func EvaluateStress(input StressInput, parameters StressParameters) (StressResult, error) {
	if err := parameters.Validate(); err != nil {
		return StressResult{}, err
	}
	if input.Epoch < 0 || input.Reserve < 0 || input.Fees < 0 {
		return StressResult{}, errors.New("epoch, reserve and fees must be non-negative")
	}

	reserveHaircut, err := MulBps(input.Reserve, parameters.ReserveHaircutBps)
	if err != nil {
		return StressResult{}, err
	}
	reserveAfterHaircut, err := SubAmount(input.Reserve, reserveHaircut)
	if err != nil {
		return StressResult{}, err
	}
	realizableFees, err := MulBps(input.Fees, parameters.FeeRealizationBps)
	if err != nil {
		return StressResult{}, err
	}
	available, err := AddAmount(reserveAfterHaircut, realizableFees)
	if err != nil {
		return StressResult{}, err
	}

	positions := map[string]*HorizonPosition{
		HorizonImmediate: {Horizon: HorizonImmediate},
		HorizonNear:      {Horizon: HorizonNear},
		HorizonTerm:      {Horizon: HorizonTerm},
		HorizonLong:      {Horizon: HorizonLong},
	}
	beneficiaryAmounts := map[string]int64{}
	result := StressResult{
		ReserveAfterHaircut: reserveAfterHaircut,
		RealizableFees:      realizableFees,
		AvailableLiquidity:  available,
	}
	var weightedMaturity int64

	seen := map[string]struct{}{}
	for _, claim := range input.Claims {
		if err := validateStressClaim(claim); err != nil {
			return StressResult{}, err
		}
		if _, exists := seen[claim.ID]; exists {
			return StressResult{}, errors.New("duplicate claim id: " + claim.ID)
		}
		seen[claim.ID] = struct{}{}
		if claim.Amount == 0 || stressClaimTerminal(claim.State) {
			continue
		}

		expected, err := expectedStressClaim(claim, parameters)
		if err != nil {
			return StressResult{}, err
		}
		buffer, err := MulBpsCeil(expected, parameters.LiquidityBufferBps)
		if err != nil {
			return StressResult{}, err
		}
		buffered, err := AddAmount(expected, buffer)
		if err != nil {
			return StressResult{}, err
		}
		horizon := horizonForEpoch(input.Epoch, claim.DueEpoch, parameters)
		position := positions[horizon]
		position.Claims, err = AddAmount(position.Claims, claim.Amount)
		if err != nil {
			return StressResult{}, err
		}
		position.Expected, err = AddAmount(position.Expected, expected)
		if err != nil {
			return StressResult{}, err
		}
		position.Buffered, err = AddAmount(position.Buffered, buffered)
		if err != nil {
			return StressResult{}, err
		}
		position.ClaimCount++
		result.GrossClaims, err = AddAmount(result.GrossClaims, claim.Amount)
		if err != nil {
			return StressResult{}, err
		}
		result.ExpectedClaims, err = AddAmount(result.ExpectedClaims, expected)
		if err != nil {
			return StressResult{}, err
		}
		result.BufferedClaims, err = AddAmount(result.BufferedClaims, buffered)
		if err != nil {
			return StressResult{}, err
		}
		beneficiaryAmounts[claim.Beneficiary], err = AddAmount(beneficiaryAmounts[claim.Beneficiary], claim.Amount)
		if err != nil {
			return StressResult{}, err
		}
		maturity := claim.DueEpoch - input.Epoch
		if maturity < 0 {
			maturity = 0
		}
		weighted, err := multiplyChecked(claim.Amount, maturity)
		if err != nil {
			return StressResult{}, err
		}
		weightedMaturity, err = AddAmount(weightedMaturity, weighted)
		if err != nil {
			return StressResult{}, err
		}
	}

	if result.BufferedClaims > result.AvailableLiquidity {
		result.FundingGap = result.BufferedClaims - result.AvailableLiquidity
	}
	result.CoverageBps = RatioBps(result.AvailableLiquidity, result.BufferedClaims)
	result.SolvencyPass = result.FundingGap == 0
	if result.GrossClaims > 0 {
		result.WeightedMaturityEpoch = weightedMaturity / result.GrossClaims
	}
	result.Concentrations = buildConcentrations(beneficiaryAmounts, result.GrossClaims)
	if len(result.Concentrations) > 0 {
		result.MaxConcentrationBps = result.Concentrations[0].ShareBps
	}
	result.ConcentrationPass = result.MaxConcentrationBps <= parameters.ConcentrationLimitBps

	orderedHorizons := []string{HorizonImmediate, HorizonNear, HorizonTerm, HorizonLong}
	var cumulative int64
	for _, name := range orderedHorizons {
		position := positions[name]
		cumulative, err = AddAmount(cumulative, position.Buffered)
		if err != nil {
			return StressResult{}, err
		}
		position.CoverageBps = RatioBps(result.AvailableLiquidity, cumulative)
		if cumulative > result.AvailableLiquidity {
			position.FundingGap = cumulative - result.AvailableLiquidity
		}
		result.Horizons = append(result.Horizons, *position)
	}
	return result, nil
}

func StressInputFromLedger(ledger *Ledger, vaultID string) (StressInput, error) {
	vault, err := ledger.Vault(vaultID)
	if err != nil {
		return StressInput{}, err
	}
	input := StressInput{Epoch: ledger.Epoch, Reserve: vault.Reserve, Fees: vault.Fees, Claims: []StressClaim{}}
	for _, subID := range ledger.SortedSubIDs() {
		sub := ledger.Subs[subID]
		if sub == nil || sub.VaultID != vaultID {
			continue
		}
		input.Claims = append(input.Claims, StressClaim{
			ID: sub.ID, Beneficiary: sub.Beneficiary,
			Amount:   RemainingAmount(sub.Amount, sub.Claimed, sub.Penalty),
			DueEpoch: sub.DueEpoch, State: sub.State,
			DeliveredBps: DeliveryRatio(sub.Delivered, sub.Amount),
		})
	}
	return input, nil
}

func validateStressClaim(claim StressClaim) error {
	if claim.ID == "" || claim.Beneficiary == "" {
		return errors.New("claim id and beneficiary are required")
	}
	if claim.Amount < 0 || claim.DueEpoch < 0 {
		return errors.New("claim amount and due epoch must be non-negative")
	}
	if claim.DeliveredBps < 0 || claim.DeliveredBps > BasisPoints {
		return errors.New("delivered_bps must be between 0 and 10000")
	}
	return nil
}

func expectedStressClaim(claim StressClaim, parameters StressParameters) (int64, error) {
	if claim.State == StateClaimable || claim.State == StateDelivered {
		return claim.Amount, nil
	}
	performanceBps := BasisPoints - parameters.CounterpartyDefaultBps
	recoveredDefaultBps, err := MulBps(parameters.CounterpartyDefaultBps, parameters.RecoveryBps)
	if err != nil {
		return 0, err
	}
	baseProbabilityBps, err := AddAmount(performanceBps, recoveredDefaultBps)
	if err != nil {
		return 0, err
	}
	deliveryFloor := claim.DeliveredBps
	if deliveryFloor > baseProbabilityBps {
		baseProbabilityBps = deliveryFloor
	}
	return MulBpsCeil(claim.Amount, baseProbabilityBps)
}

func horizonForEpoch(current int64, due int64, parameters StressParameters) string {
	distance := due - current
	if distance <= 0 {
		return HorizonImmediate
	}
	if distance <= parameters.NearEpochs {
		return HorizonNear
	}
	if distance <= parameters.TermEpochs {
		return HorizonTerm
	}
	return HorizonLong
}

func stressClaimTerminal(state string) bool {
	return state == StateClaimed || state == StateClosed || state == StateCancelled || state == StateDefaulted
}

func buildConcentrations(values map[string]int64, total int64) []ConcentrationPosition {
	out := make([]ConcentrationPosition, 0, len(values))
	for beneficiary, amount := range values {
		out = append(out, ConcentrationPosition{
			Beneficiary: beneficiary,
			Amount:      amount,
			ShareBps:    RatioBps(amount, total),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Amount == out[j].Amount {
			return out[i].Beneficiary < out[j].Beneficiary
		}
		return out[i].Amount > out[j].Amount
	})
	return out
}

func multiplyChecked(left int64, right int64) (int64, error) {
	if left == 0 || right == 0 {
		return 0, nil
	}
	if left < 0 || right < 0 || left > (1<<63-1)/right {
		return 0, errors.New("multiplication overflow")
	}
	return left * right, nil
}
