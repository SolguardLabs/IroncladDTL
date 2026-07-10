package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func LoadScenario(path string) (*Scenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var scenario Scenario
	if err := json.Unmarshal(data, &scenario); err != nil {
		return nil, err
	}
	if scenario.Name == "" {
		base := filepath.Base(path)
		scenario.Name = strings.TrimSuffix(base, filepath.Ext(base))
	}
	return &scenario, nil
}

func BuildLedger(scenario *Scenario) (*Ledger, error) {
	if scenario == nil {
		return nil, NewValidationError("scenario", "nil scenario")
	}
	ledger := NewLedger(scenario.Name, scenario.Epoch, scenario.Policy)
	for _, account := range scenario.Accounts {
		if err := ledger.AddAccount(account); err != nil {
			return nil, err
		}
	}
	for _, vault := range scenario.Vaults {
		if err := ledger.AddVault(vault); err != nil {
			return nil, err
		}
	}
	return ledger, nil
}

func RunScenario(scenario *Scenario) (*Ledger, error) {
	ledger, err := BuildLedger(scenario)
	if err != nil {
		return nil, err
	}
	for index, operation := range scenario.Operations {
		err := ledger.ApplyOperation(index, operation)
		if err != nil {
			if operation.ExpectError {
				ledger.RecordRejected(index, operation, err)
				continue
			}
			return nil, err
		}
		if operation.ExpectError {
			ledger.RecordRejected(index, operation, NewConflictError("operation", "expected error but operation succeeded"))
		}
	}
	ledger.RefreshDerivedVaultClaims()
	return ledger, nil
}

func ValidateScenario(scenario *Scenario) error {
	if scenario == nil {
		return NewValidationError("scenario", "nil scenario")
	}
	seenAccounts := map[string]bool{}
	for _, account := range scenario.Accounts {
		if err := validateID("account", account.ID); err != nil {
			return err
		}
		if seenAccounts[account.ID] {
			return NewValidationError("account", "duplicate account "+account.ID)
		}
		seenAccounts[account.ID] = true
		for asset, amount := range account.Reserves {
			if err := validateID("asset", asset); err != nil {
				return err
			}
			if _, err := MustAmount(amount, "reserve"); err != nil {
				return err
			}
		}
	}
	seenVaults := map[string]bool{}
	for _, vault := range scenario.Vaults {
		if err := validateID("vault", vault.ID); err != nil {
			return err
		}
		if err := validateID("asset", vault.Asset); err != nil {
			return err
		}
		if seenVaults[vault.ID] {
			return NewValidationError("vault", "duplicate vault "+vault.ID)
		}
		seenVaults[vault.ID] = true
		if _, err := MustAmount(vault.Reserve, "reserve"); err != nil {
			return err
		}
	}
	for index, op := range scenario.Operations {
		if strings.TrimSpace(op.Type) == "" {
			return NewValidationError("operation", "operation type missing at index "+itoa(index))
		}
		switch op.Type {
		case "advance", "open", "deliver", "settle", "claim", "close", "withdraw", "deposit", "penalize", "sweep":
		default:
			return NewValidationError("operation", "unknown operation "+op.Type)
		}
		if op.Amount < 0 {
			return NewValidationError("amount", "negative operation amount at index "+itoa(index))
		}
		for _, beneficiary := range op.Beneficiaries {
			if beneficiary.Amount < 0 {
				return NewValidationError("beneficiary", "negative beneficiary amount")
			}
			if beneficiary.ID == "" && beneficiary.Beneficiary == "" {
				return NewValidationError("beneficiary", "beneficiary id missing")
			}
		}
	}
	return nil
}

func validateID(field string, value string) error {
	if strings.TrimSpace(value) == "" {
		return NewValidationError(field, "id cannot be empty")
	}
	if strings.ContainsAny(value, " \t\r\n") {
		return NewValidationError(field, "id cannot contain whitespace")
	}
	return nil
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var digits [20]byte
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

func ScenarioFromFile(path string) (*Ledger, error) {
	scenario, err := LoadScenario(path)
	if err != nil {
		return nil, err
	}
	if err := ValidateScenario(scenario); err != nil {
		return nil, err
	}
	return RunScenario(scenario)
}
