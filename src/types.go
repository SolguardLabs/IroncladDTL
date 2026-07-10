package main

const (
	StatePending   = "pending"
	StateDelivered = "delivered"
	StateClaimable = "claimable"
	StateClaimed   = "claimed"
	StateLocked    = "locked"
	StateReleased  = "released"
	StateClosed    = "closed"
	StateDefaulted = "defaulted"
	StateCancelled = "cancelled"
	StateRejected  = "rejected"
	StateApplied   = "applied"
)

const (
	ScopeParent = "parent"
	ScopeSub    = "sub"
)

type Scenario struct {
	Name       string          `json:"name"`
	Epoch      int64           `json:"epoch"`
	Policy     PolicyConfig    `json:"policy"`
	Accounts   []AccountConfig `json:"accounts"`
	Vaults     []VaultConfig   `json:"vaults"`
	Operations []Operation     `json:"operations"`
}

type PolicyConfig struct {
	MinLockAmount              int64 `json:"min_lock_amount"`
	MaxCommitmentAmount        int64 `json:"max_commitment_amount"`
	DefaultPenaltyBps          int64 `json:"default_penalty_bps"`
	DefaultReleaseThresholdBps int64 `json:"default_release_threshold_bps"`
	CompositeReleaseFeeBps     int64 `json:"composite_release_fee_bps"`
	GraceEpochs                int64 `json:"grace_epochs"`
	AllowPartialClaims         bool  `json:"allow_partial_claims"`
}

type AccountConfig struct {
	ID       string           `json:"id"`
	Label    string           `json:"label"`
	Reserves map[string]int64 `json:"reserves"`
}

type VaultConfig struct {
	ID        string `json:"id"`
	Asset     string `json:"asset"`
	Reserve   int64  `json:"reserve"`
	FeeAccount string `json:"fee_account"`
}

type BeneficiaryConfig struct {
	ID          string `json:"id"`
	Beneficiary string `json:"beneficiary"`
	Amount      int64  `json:"amount"`
	DueEpoch    int64  `json:"due_epoch"`
	Label       string `json:"label"`
}

type Operation struct {
	Type                string              `json:"type"`
	ID                  string              `json:"id"`
	Account             string              `json:"account"`
	Vault               string              `json:"vault"`
	Commitment          string              `json:"commitment"`
	Sub                 string              `json:"sub"`
	Beneficiary         string              `json:"beneficiary"`
	Amount              int64               `json:"amount"`
	Delta               int64               `json:"delta"`
	DueEpoch            int64               `json:"due_epoch"`
	ReleaseThresholdBps int64               `json:"release_threshold_bps"`
	PenaltyBps          int64               `json:"penalty_bps"`
	Beneficiaries       []BeneficiaryConfig `json:"beneficiaries"`
	ExpectError         bool                `json:"expect_error"`
	Note                string              `json:"note"`
}

type Account struct {
	ID               string           `json:"id"`
	Label            string           `json:"label"`
	Reserves         map[string]int64 `json:"reserves"`
	ExternalIn       int64            `json:"external_in"`
	ExternalOut      int64            `json:"external_out"`
	ClaimsReceived   int64            `json:"claims_received"`
	SponsorReleases  int64            `json:"sponsor_releases"`
	PenaltiesPaid    int64            `json:"penalties_paid"`
	RejectedAttempts int64            `json:"rejected_attempts"`
}

type Vault struct {
	ID             string `json:"id"`
	Asset          string `json:"asset"`
	Reserve        int64  `json:"reserve"`
	Locked         int64  `json:"locked"`
	Fees           int64  `json:"fees"`
	Insolvency     int64  `json:"insolvency"`
	ClaimShortfall int64  `json:"claim_shortfall"`
	FeeAccount     string `json:"fee_account"`
	OpenClaims     int64  `json:"open_claims"`
}

type Commitment struct {
	ID                  string   `json:"id"`
	VaultID             string   `json:"vault"`
	Sponsor             string   `json:"sponsor"`
	Amount              int64    `json:"amount"`
	Delivered           int64    `json:"delivered"`
	Claimed             int64    `json:"claimed"`
	ReleasedToSponsor   int64    `json:"released_to_sponsor"`
	PenaltyAccrued      int64    `json:"penalty_accrued"`
	FeeAccrued          int64    `json:"fee_accrued"`
	DueEpoch            int64    `json:"due_epoch"`
	ClosedEpoch         int64    `json:"closed_epoch"`
	Status              string   `json:"status"`
	Composite           bool     `json:"composite"`
	ReleaseThresholdBps int64    `json:"release_threshold_bps"`
	ParentLockID        string   `json:"parent_lock_id"`
	SubIDs              []string `json:"sub_ids"`
}

type SubCommitment struct {
	ID           string `json:"id"`
	ParentID     string `json:"parent_id"`
	VaultID      string `json:"vault"`
	Sponsor      string `json:"sponsor"`
	Beneficiary  string `json:"beneficiary"`
	Label        string `json:"label"`
	Amount       int64  `json:"amount"`
	Delivered    int64  `json:"delivered"`
	Claimed      int64  `json:"claimed"`
	Penalty      int64  `json:"penalty"`
	DueEpoch     int64  `json:"due_epoch"`
	State        string `json:"state"`
	SubLockID    string `json:"sub_lock_id"`
	LastEventSeq int64  `json:"last_event_seq"`
}

type Lock struct {
	ID            string `json:"id"`
	CommitmentID  string `json:"commitment"`
	SubID         string `json:"sub,omitempty"`
	Scope         string `json:"scope"`
	VaultID       string `json:"vault"`
	Account       string `json:"account"`
	Beneficiary   string `json:"beneficiary,omitempty"`
	Amount        int64  `json:"amount"`
	Released      int64  `json:"released"`
	Claimed       int64  `json:"claimed"`
	Penalty       int64  `json:"penalty"`
	State         string `json:"state"`
	CreatedEpoch  int64  `json:"created_epoch"`
	DueEpoch      int64  `json:"due_epoch"`
	ReleasedEpoch int64  `json:"released_epoch"`
}

type Event struct {
	Seq        int64             `json:"seq"`
	Epoch      int64             `json:"epoch"`
	Type       string            `json:"type"`
	Status     string            `json:"status"`
	ID         string            `json:"id,omitempty"`
	Account    string            `json:"account,omitempty"`
	Vault      string            `json:"vault,omitempty"`
	Commitment string            `json:"commitment,omitempty"`
	Sub        string            `json:"sub,omitempty"`
	Amount     int64             `json:"amount,omitempty"`
	Details    map[string]string `json:"details,omitempty"`
}

type RejectedOperation struct {
	Index   int    `json:"index"`
	Type    string `json:"type"`
	ID      string `json:"id,omitempty"`
	Code    string `json:"code"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

type Ledger struct {
	Name       string
	Epoch      int64
	Policy     PolicyConfig
	Accounts   map[string]*Account
	Vaults     map[string]*Vault
	Commitments map[string]*Commitment
	Subs       map[string]*SubCommitment
	Locks      map[string]*Lock
	Events     []Event
	Rejected   []RejectedOperation
	Warnings   []string
	seq        int64
}

func DefaultPolicy() PolicyConfig {
	return PolicyConfig{
		MinLockAmount:              1,
		MaxCommitmentAmount:        1_000_000_000,
		DefaultPenaltyBps:          DefaultPenaltyBps,
		DefaultReleaseThresholdBps: BasisPoints,
		CompositeReleaseFeeBps:     0,
		GraceEpochs:                0,
		AllowPartialClaims:         true,
	}
}

func MergePolicy(input PolicyConfig) PolicyConfig {
	policy := DefaultPolicy()
	if input.MinLockAmount > 0 {
		policy.MinLockAmount = input.MinLockAmount
	}
	if input.MaxCommitmentAmount > 0 {
		policy.MaxCommitmentAmount = input.MaxCommitmentAmount
	}
	if input.DefaultPenaltyBps > 0 {
		policy.DefaultPenaltyBps = input.DefaultPenaltyBps
	}
	if input.DefaultReleaseThresholdBps > 0 {
		policy.DefaultReleaseThresholdBps = input.DefaultReleaseThresholdBps
	}
	if input.CompositeReleaseFeeBps >= 0 {
		policy.CompositeReleaseFeeBps = input.CompositeReleaseFeeBps
	}
	if input.GraceEpochs > 0 {
		policy.GraceEpochs = input.GraceEpochs
	}
	policy.AllowPartialClaims = input.AllowPartialClaims
	return policy
}

func NewLedger(name string, epoch int64, policy PolicyConfig) *Ledger {
	return &Ledger{
		Name:        name,
		Epoch:       epoch,
		Policy:      MergePolicy(policy),
		Accounts:    make(map[string]*Account),
		Vaults:      make(map[string]*Vault),
		Commitments: make(map[string]*Commitment),
		Subs:        make(map[string]*SubCommitment),
		Locks:       make(map[string]*Lock),
		Events:      []Event{},
		Rejected:    []RejectedOperation{},
		Warnings:    []string{},
	}
}
