// Package domain contains the shared, persistence-neutral Mythborn vocabulary.
package domain

import "time"

// ID is a UUID encoded in its canonical string form. Persistence adapters are
// responsible for parsing and validating IDs at input boundaries.
type ID string

type SystemRole string

const (
	SystemRoleUser  SystemRole = "user"
	SystemRoleAdmin SystemRole = "admin"
)

type PlayerRole string

const (
	PlayerRoleObserver  PlayerRole = "observer"
	PlayerRoleGod       PlayerRole = "god"
	PlayerRoleMessenger PlayerRole = "messenger"
)

type GameStatus string

const (
	GameStatusStarting        GameStatus = "starting"
	GameStatusActive          GameStatus = "active"
	GameStatusEndingRequested GameStatus = "ending_requested"
	GameStatusEnding          GameStatus = "ending"
	GameStatusArchived        GameStatus = "archived"
	GameStatusDeleting        GameStatus = "deleting"
)

// Game.PlayerRole is fixed at creation. Authorization for application-wide
// administration belongs to Account.SystemRole and is independent of it.
type Game struct {
	ID                     ID         `json:"id"`
	OwnerAccountID         ID         `json:"owner_account_id"`
	Name                   string     `json:"name"`
	PlayerRole             PlayerRole `json:"player_role"`
	Status                 GameStatus `json:"status"`
	ReviewPhotoDescription bool       `json:"review_photo_description"`
	LastCouncilAt          *time.Time `json:"last_council_at,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
	ArchivedAt             *time.Time `json:"archived_at,omitempty"`
}

type Account struct {
	ID         ID         `json:"id"`
	SystemRole SystemRole `json:"system_role"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type AgentType string

const (
	AgentTypePriest    AgentType = "priest"
	AgentTypeScientist AgentType = "scientist"
	AgentTypeSoldier   AgentType = "soldier"
	AgentTypeHistorian AgentType = "historian"
)

type Agent struct {
	ID                ID        `json:"id"`
	GameID            ID        `json:"game_id"`
	Type              AgentType `json:"agent_type"`
	DisplayName       string    `json:"display_name"`
	PersonalityPrompt string    `json:"personality_prompt"`
	SourceTemplateID  *ID       `json:"source_template_id,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

type RoundKind string

const (
	RoundKindDiscovery RoundKind = "discovery"
	RoundKindCouncil   RoundKind = "council"
	RoundKindClosing   RoundKind = "closing"
)

type RoundStatus string

const (
	RoundStatusQueued         RoundStatus = "queued"
	RoundStatusRunning        RoundStatus = "running"
	RoundStatusNeedsAttention RoundStatus = "needs_attention"
	RoundStatusComplete       RoundStatus = "complete"
	RoundStatusFailed         RoundStatus = "failed"
	RoundStatusAbandoned      RoundStatus = "abandoned"
)

type RoundStage string

const (
	RoundStageQueued                RoundStage = "queued"
	RoundStageDescribing            RoundStage = "describing"
	RoundStageAwaitingReview        RoundStage = "awaiting_review"
	RoundStageAwaitingClarityChoice RoundStage = "awaiting_clarity_choice"
	RoundStageReacting              RoundStage = "reacting"
	RoundStageDebating              RoundStage = "debating"
	RoundStageWriting               RoundStage = "writing"
	RoundStageComplete              RoundStage = "complete"
)

type Round struct {
	ID             ID          `json:"id"`
	GameID         ID          `json:"game_id"`
	SequenceNumber int64       `json:"sequence_number"`
	Kind           RoundKind   `json:"kind"`
	ObservationID  *ID         `json:"observation_id,omitempty"`
	Status         RoundStatus `json:"status"`
	Stage          RoundStage  `json:"stage"`
	ErrorCode      *string     `json:"error_code,omitempty"`
	StartedAt      *time.Time  `json:"started_at,omitempty"`
	CompletedAt    *time.Time  `json:"completed_at,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

type ChronicleOutcome string

const (
	OutcomeConsensus  ChronicleOutcome = "consensus"
	OutcomeMajority   ChronicleOutcome = "majority"
	OutcomeUnresolved ChronicleOutcome = "unresolved"
)

type Chronicle struct {
	ID               ID                `json:"id"`
	GameID           ID                `json:"game_id"`
	RoundID          ID                `json:"round_id"`
	HistorianAgentID ID                `json:"historian_agent_id"`
	Title            string            `json:"title"`
	Outcome          *ChronicleOutcome `json:"outcome,omitempty"` // nil only for closing records.
	Verdict          string            `json:"verdict"`
	Body             string            `json:"body"`
	Suggestion       *string           `json:"suggestion,omitempty"`
	Metadata         map[string]any    `json:"metadata,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
}

type BeliefState string

const (
	BeliefStateForming    BeliefState = "forming"
	BeliefStateHeld       BeliefState = "held"
	BeliefStateQuestioned BeliefState = "questioned"
	BeliefStateAbandoned  BeliefState = "abandoned"
)

type Belief struct {
	ID        ID          `json:"id"`
	GameID    ID          `json:"game_id"`
	AgentID   ID          `json:"agent_id"`
	Claim     string      `json:"claim"`
	State     BeliefState `json:"state"`
	Version   int64       `json:"version"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

type BeliefCause string

const (
	BeliefCauseInitial   BeliefCause = "initial"
	BeliefCauseDiscovery BeliefCause = "discovery"
	BeliefCauseCouncil   BeliefCause = "council"
	BeliefCauseMessenger BeliefCause = "messenger"
)

type BeliefChangeType string

const (
	BeliefChangeFormed       BeliefChangeType = "formed"
	BeliefChangeStrengthened BeliefChangeType = "strengthened"
	BeliefChangeWeakened     BeliefChangeType = "weakened"
	BeliefChangeRevised      BeliefChangeType = "revised"
	BeliefChangeAbandoned    BeliefChangeType = "abandoned"
)

type BeliefRevision struct {
	ID                          ID               `json:"id"`
	GameID                      ID               `json:"game_id"`
	BeliefID                    ID               `json:"belief_id"`
	Version                     int64            `json:"version"`
	EventID                     ID               `json:"event_id"`
	Cause                       BeliefCause      `json:"cause"`
	ChangeType                  BeliefChangeType `json:"change_type"`
	PreviousClaim               *string          `json:"previous_claim,omitempty"`
	PreviousState               *BeliefState     `json:"previous_state,omitempty"`
	NewClaim                    string           `json:"new_claim"`
	NewState                    BeliefState      `json:"new_state"`
	Reason                      string           `json:"reason"`
	SourceRoundID               *ID              `json:"source_round_id,omitempty"`
	SourceConversationSummaryID *ID              `json:"source_conversation_summary_id,omitempty"`
	CreatedAt                   time.Time        `json:"created_at"`
}

type TraditionType string

const (
	TraditionTypeMyth   TraditionType = "myth"
	TraditionTypeRitual TraditionType = "ritual"
	TraditionTypeTaboo  TraditionType = "taboo"
)

type TraditionState string

const (
	TraditionStateAdopted   TraditionState = "adopted"
	TraditionStateContested TraditionState = "contested"
	TraditionStateRetired   TraditionState = "retired"
)

type Tradition struct {
	ID          ID             `json:"id"`
	GameID      ID             `json:"game_id"`
	Type        TraditionType  `json:"type"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	State       TraditionState `json:"state"`
	Version     int64          `json:"version"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type ConversationSummary struct {
	ID          ID        `json:"id"`
	GameID      ID        `json:"game_id"`
	RoundID     ID        `json:"round_id"`
	AgentID     ID        `json:"agent_id"`
	Summary     string    `json:"summary"`
	Version     int64     `json:"version"`
	LastEventID ID        `json:"last_event_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CommandStatus string

const (
	CommandStatusPending    CommandStatus = "pending"
	CommandStatusDispatched CommandStatus = "dispatched"
	CommandStatusProcessed  CommandStatus = "processed"
	CommandStatusFailed     CommandStatus = "failed"
)

// WorkflowCommand is a durable outbox item. Payloads must be bounded and raw
// Messenger input must be cleared after processing, per DATABASE_DESIGN.md.
type WorkflowCommand struct {
	ID              ID            `json:"id"`
	AccountID       ID            `json:"account_id"`
	GameID          *ID           `json:"game_id,omitempty"`
	RoundID         *ID           `json:"round_id,omitempty"`
	Type            string        `json:"command_type"`
	IdempotencyKey  string        `json:"idempotency_key"`
	Payload         []byte        `json:"payload,omitempty"`
	ResultPayload   []byte        `json:"result_payload,omitempty"`
	ResultExpiresAt *time.Time    `json:"result_expires_at,omitempty"`
	Status          CommandStatus `json:"status"`
	AttemptCount    int           `json:"attempt_count"`
	CreatedAt       time.Time     `json:"created_at"`
}
