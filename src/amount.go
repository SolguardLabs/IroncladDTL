package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	BasisPoints       int64 = 10_000
	DefaultPrecision  int64 = 1
	DefaultFeeBps     int64 = 25
	DefaultPenaltyBps int64 = 1_000
	MaxAmount         int64 = math.MaxInt64 / 8
)

type Amount int64

func NewAmount(value int64) Amount {
	if value < 0 {
		return 0
	}
	return Amount(value)
}

func MustAmount(value int64, field string) (Amount, error) {
	if value < 0 {
		return 0, NewValidationError(field, "amount cannot be negative")
	}
	if value > MaxAmount {
		return 0, NewValidationError(field, "amount exceeds internal limit")
	}
	return Amount(value), nil
}

func ParseAmount(text string) (Amount, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0, NewValidationError("amount", "empty amount")
	}
	value, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return 0, NewValidationError("amount", err.Error())
	}
	return MustAmount(value, "amount")
}

func (a Amount) Int64() int64 {
	return int64(a)
}

func (a Amount) String() string {
	return strconv.FormatInt(int64(a), 10)
}

func AddAmount(a int64, b int64) (int64, error) {
	if a < 0 || b < 0 {
		return 0, NewValidationError("amount", "cannot add negative values")
	}
	if a > MaxAmount-b {
		return 0, NewValidationError("amount", "addition overflow")
	}
	return a + b, nil
}

func SubAmount(a int64, b int64) (int64, error) {
	if a < 0 || b < 0 {
		return 0, NewValidationError("amount", "cannot subtract negative values")
	}
	if b > a {
		return 0, NewInsufficientFundsError("amount", b, a)
	}
	return a - b, nil
}

func SafeAdd(values ...int64) (int64, error) {
	var total int64
	for _, value := range values {
		next, err := AddAmount(total, value)
		if err != nil {
			return 0, err
		}
		total = next
	}
	return total, nil
}

func MulBps(amount int64, bps int64) (int64, error) {
	if amount < 0 {
		return 0, NewValidationError("amount", "amount cannot be negative")
	}
	if bps < 0 {
		return 0, NewValidationError("bps", "basis points cannot be negative")
	}
	if amount == 0 || bps == 0 {
		return 0, nil
	}
	if amount > MaxAmount/bps {
		return 0, NewValidationError("amount", "multiplication overflow")
	}
	return (amount * bps) / BasisPoints, nil
}

func MulBpsCeil(amount int64, bps int64) (int64, error) {
	if amount < 0 {
		return 0, NewValidationError("amount", "amount cannot be negative")
	}
	if bps < 0 {
		return 0, NewValidationError("bps", "basis points cannot be negative")
	}
	if amount == 0 || bps == 0 {
		return 0, nil
	}
	if amount > MaxAmount/bps {
		return 0, NewValidationError("amount", "multiplication overflow")
	}
	raw := amount * bps
	return (raw + BasisPoints - 1) / BasisPoints, nil
}

func RatioBps(part int64, total int64) int64 {
	if total <= 0 {
		return 0
	}
	if part <= 0 {
		return 0
	}
	if part >= total {
		return BasisPoints
	}
	return (part * BasisPoints) / total
}

func ClampAmount(value int64, min int64, max int64) int64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func MinAmount(values ...int64) int64 {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, value := range values[1:] {
		if value < min {
			min = value
		}
	}
	return min
}

func MaxAmountOf(values ...int64) int64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, value := range values[1:] {
		if value > max {
			max = value
		}
	}
	return max
}

func FormatBps(bps int64) string {
	sign := ""
	if bps < 0 {
		sign = "-"
		bps = -bps
	}
	return fmt.Sprintf("%s%d.%02d%%", sign, bps/100, bps%100)
}

func IsTerminalState(state string) bool {
	switch state {
	case StateClosed, StateClaimed, StateDefaulted, StateCancelled, StateReleased:
		return true
	default:
		return false
	}
}

func IsOpenSubState(state string) bool {
	switch state {
	case StatePending, StateDelivered, StateClaimable:
		return true
	default:
		return false
	}
}

func AmountMapClone(input map[string]int64) map[string]int64 {
	output := make(map[string]int64, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func AmountMapAdd(input map[string]int64, asset string, amount int64) error {
	if amount < 0 {
		return NewValidationError("amount", "cannot add negative value")
	}
	current := input[asset]
	next, err := AddAmount(current, amount)
	if err != nil {
		return err
	}
	input[asset] = next
	return nil
}

func AmountMapSub(input map[string]int64, asset string, amount int64) error {
	if amount < 0 {
		return NewValidationError("amount", "cannot subtract negative value")
	}
	current := input[asset]
	next, err := SubAmount(current, amount)
	if err != nil {
		return err
	}
	input[asset] = next
	return nil
}

func SumAmountMap(input map[string]int64) int64 {
	var total int64
	for _, value := range input {
		total += value
	}
	return total
}

func NormalizeBps(value int64, fallback int64) int64 {
	if value <= 0 {
		return fallback
	}
	if value > BasisPoints {
		return BasisPoints
	}
	return value
}

func RemainingAmount(amount int64, claimed int64, penalty int64) int64 {
	remaining := amount - claimed - penalty
	if remaining < 0 {
		return 0
	}
	return remaining
}

func DeliverySatisfied(delivered int64, amount int64) bool {
	if amount <= 0 {
		return true
	}
	return delivered >= amount
}

func DeliveryRatio(delivered int64, amount int64) int64 {
	if amount <= 0 {
		return BasisPoints
	}
	return RatioBps(ClampAmount(delivered, 0, amount), amount)
}

func PenaltyForMissing(missing int64, bps int64) (int64, error) {
	if missing <= 0 {
		return 0, nil
	}
	return MulBpsCeil(missing, NormalizeBps(bps, DefaultPenaltyBps))
}

func FeeForRelease(amount int64, feeBps int64) (int64, error) {
	if amount <= 0 {
		return 0, nil
	}
	if feeBps <= 0 {
		return 0, nil
	}
	if feeBps > BasisPoints {
		feeBps = BasisPoints
	}
	return MulBps(amount, feeBps)
}

func NetOfFee(amount int64, feeBps int64) (int64, int64, error) {
	fee, err := FeeForRelease(amount, feeBps)
	if err != nil {
		return 0, 0, err
	}
	if fee > amount {
		fee = amount
	}
	return amount - fee, fee, nil
}
