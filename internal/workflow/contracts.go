// Package workflow contains contracts shared by outbox dispatch and Temporal
// worker implementations. Workflow and activity behavior lives in worker code.
package workflow

import (
	"context"
	"encoding/json"
	"time"

	"github.com/amh1k/mythborn/internal/domain"
)

type Dispatcher interface {
	Dispatch(context.Context, domain.WorkflowCommand) error
}

// WorkflowNames centralizes the stable Temporal names used by dispatchers.
const (
	CoordinatorWorkflowName     = "mythborn.world.coordinator"
	AgentWorkflowName           = "mythborn.world.agent"
	AccountDeletionWorkflowName = "mythborn.account.deletion"
	CoordinatorSignalName       = "mythborn.coordinator.command"
	AccountDeletionSignalName   = "mythborn.account.deletion"
	AgentRequestSignalName      = "mythborn.agent.request"
	AgentResultSignalName       = "mythborn.agent.result"
	WorkflowTaskQueue           = "mythborn-workers"
	MessengerReplyTTL           = 24 * time.Hour
)

// WorkflowID is stable across at-least-once outbox delivery. Each game has one
// coordinator and one durable workflow per founding agent.
func WorkflowID(gameID domain.ID) string {
	return "mythborn/game/" + string(gameID)
}

func AgentWorkflowID(gameID, agentID domain.ID) string {
	return WorkflowID(gameID) + "/agent/" + string(agentID)
}

func AccountDeletionWorkflowID(accountID domain.ID) string {
	return "mythborn/account/" + string(accountID) + "/deletion"
}

type CommandType string

const (
	CommandStartGame         CommandType = "game.start"
	CommandProcessRound      CommandType = "round.process"
	CommandDescriptionChoice CommandType = "description.decision"
	CommandRetryRound        CommandType = "round.retry"
	CommandAbandonRound      CommandType = "round.abandon"
	CommandEndGame           CommandType = "game.end"
	CommandMessengerExchange CommandType = "messenger.exchange"
	CommandRunCouncil        CommandType = "council.run"
	CommandDeleteGame        CommandType = "game.delete"
	CommandDeleteAccount     CommandType = "account.delete"
)

// CommandPayload is the bounded union stored in workflow_outbox.payload. It
// carries only IDs and small player input; never photos or whole game ledgers.
type CommandPayload struct {
	AgentID                domain.ID `json:"agent_id,omitempty"`
	Message                string    `json:"message,omitempty"`
	ExpectedSummaryVersion int64     `json:"expected_summary_version,omitempty"`
	DescriptionDecision    string    `json:"description_decision,omitempty"` // accept, correct, continue_uncertain, try_another_photo
	Correction             string    `json:"correction,omitempty"`
}

// Command is the decoded form of a durable outbox row.
type Command struct {
	EventID        domain.ID      `json:"event_id"`
	AccountID      domain.ID      `json:"account_id"`
	GameID         domain.ID      `json:"game_id"`
	RoundID        domain.ID      `json:"round_id,omitempty"`
	Type           CommandType    `json:"command_type"`
	IdempotencyKey string         `json:"idempotency_key"`
	Payload        CommandPayload `json:"payload,omitempty"`
}

func DecodeCommand(row domain.WorkflowCommand) (Command, error) {
	var payload CommandPayload
	if len(row.Payload) != 0 {
		if err := json.Unmarshal(row.Payload, &payload); err != nil {
			return Command{}, err
		}
	}
	command := Command{
		EventID: row.ID, AccountID: row.AccountID, Type: CommandType(row.Type),
		IdempotencyKey: row.IdempotencyKey, Payload: payload,
	}
	if row.GameID != nil {
		command.GameID = *row.GameID
	}
	if row.RoundID != nil {
		command.RoundID = *row.RoundID
	}
	return command, nil
}

type ObservationContext struct {
	ID                domain.ID `json:"id"`
	GameID            domain.ID `json:"game_id"`
	PhotoObjectPath   string    `json:"photo_object_path"`
	MIMEType          string    `json:"mime_type"`
	VisualDescription string    `json:"visual_description,omitempty"`
	PlayerCorrection  string    `json:"player_correction,omitempty"`
	PlayerStatement   string    `json:"player_statement,omitempty"`
	DescriptionStatus string    `json:"description_status"`
}

type AgentContext struct {
	Agent   domain.Agent    `json:"agent"`
	Beliefs []domain.Belief `json:"beliefs"`
}

type TraditionContext struct {
	Tradition    domain.Tradition `json:"tradition"`
	SupporterIDs []domain.ID      `json:"supporter_ids"`
}

type Memory struct {
	Content    string    `json:"content"`
	SourceType string    `json:"source_type"`
	SourceID   domain.ID `json:"source_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// RoundContext is an activity-loaded snapshot. It is passed to model
// activities only as needed and is not written back as debate history.
type RoundContext struct {
	Game             domain.Game         `json:"game"`
	Round            domain.Round        `json:"round"`
	Observation      *ObservationContext `json:"observation,omitempty"`
	Agents           []AgentContext      `json:"agents"`
	Traditions       []TraditionContext  `json:"traditions"`
	HistorianAgentID domain.ID           `json:"historian_agent_id"`
}

type AgentReaction struct {
	EventID        domain.ID       `json:"event_id"`
	AgentID        domain.ID       `json:"agent_id"`
	Interpretation string          `json:"interpretation"`
	Reasoning      string          `json:"reasoning"`
	TraditionIdeas []TraditionIdea `json:"tradition_ideas,omitempty"`
}

type TraditionIdea struct {
	TraditionID  *domain.ID           `json:"tradition_id,omitempty"`
	CandidateKey string               `json:"candidate_key,omitempty"`
	Type         domain.TraditionType `json:"type"`
	Title        string               `json:"title"`
	Description  string               `json:"description"`
}

type BeliefProposal struct {
	AgentID     domain.ID               `json:"agent_id"`
	BeliefID    *domain.ID              `json:"belief_id,omitempty"`
	NewBeliefID domain.ID               `json:"new_belief_id,omitempty"`
	ChangeType  domain.BeliefChangeType `json:"change_type"`
	NewClaim    string                  `json:"new_claim"`
	NewState    domain.BeliefState      `json:"new_state"`
	Reason      string                  `json:"reason"`
}

// SearchDocument is a bounded derived-index upsert committed atomically with
// its source lore. Exactly one source ID must be set. AgentID scopes private
// belief and conversation memory; shared observation/chronicle docs use nil.
type SearchDocument struct {
	GameID                domain.ID  `json:"game_id"`
	AgentID               *domain.ID `json:"agent_id,omitempty"`
	ObservationID         *domain.ID `json:"observation_id,omitempty"`
	ChronicleID           *domain.ID `json:"chronicle_id,omitempty"`
	BeliefID              *domain.ID `json:"belief_id,omitempty"`
	ConversationSummaryID *domain.ID `json:"conversation_summary_id,omitempty"`
	Content               string     `json:"content"`
	Embedding             []float32  `json:"embedding,omitempty"`
	SourceVersion         int64      `json:"source_version"`
}

type TraditionSupport struct {
	AgentID      domain.ID  `json:"agent_id"`
	TraditionID  *domain.ID `json:"tradition_id,omitempty"`
	CandidateKey string     `json:"candidate_key,omitempty"`
	Support      bool       `json:"support"`
	Reason       string     `json:"reason,omitempty"`
}

type AgentRebuttal struct {
	EventID           domain.ID          `json:"event_id"`
	AgentID           domain.ID          `json:"agent_id"`
	Response          string             `json:"response"`
	BeliefProposals   []BeliefProposal   `json:"belief_proposals,omitempty"`
	TraditionSupport  []TraditionSupport `json:"tradition_support,omitempty"`
	NewTraditionIdeas []TraditionIdea    `json:"new_tradition_ideas,omitempty"`
}

type TraditionChange struct {
	TraditionID  domain.ID             `json:"tradition_id"`
	Type         domain.TraditionType  `json:"type"`
	Title        string                `json:"title"`
	Description  string                `json:"description"`
	State        domain.TraditionState `json:"state"`
	SupporterIDs []domain.ID           `json:"supporter_ids"`
	Reason       string                `json:"reason"`
}

type HistorianRecord struct {
	Title      string                  `json:"title"`
	Outcome    domain.ChronicleOutcome `json:"outcome"`
	Verdict    string                  `json:"verdict"`
	Body       string                  `json:"body"`
	Suggestion *string                 `json:"suggestion,omitempty"`
	Metadata   map[string]any          `json:"metadata,omitempty"`
}

// RoundCommit is the complete atomic finalization request. Repository code
// owns the transaction, version reads, revision writes, and processed marker.
type RoundCommit struct {
	EventID            domain.ID         `json:"event_id"`
	ProcessedCommandID domain.ID         `json:"processed_command_id"`
	Chronicle          domain.Chronicle  `json:"chronicle"`
	BeliefProposals    []BeliefProposal  `json:"belief_proposals,omitempty"`
	TraditionChanges   []TraditionChange `json:"tradition_changes,omitempty"`
	SearchDocuments    []SearchDocument  `json:"search_documents,omitempty"`
}

type MessengerInput struct {
	AgentID                domain.ID `json:"agent_id"`
	RoundID                domain.ID `json:"round_id"`
	Message                string    `json:"message"`
	ExpectedSummaryVersion int64     `json:"expected_summary_version"`
}

type MessengerContext struct {
	Game    domain.Game                 `json:"game"`
	Round   domain.Round                `json:"round"`
	Agent   domain.Agent                `json:"agent"`
	Beliefs []domain.Belief             `json:"beliefs"`
	Summary *domain.ConversationSummary `json:"summary,omitempty"`
}

type MessengerCommit struct {
	EventID            domain.ID        `json:"event_id"`
	ProcessedCommandID domain.ID        `json:"processed_command_id"`
	SummaryID          domain.ID        `json:"summary_id"`
	GameID             domain.ID        `json:"game_id"`
	RoundID            domain.ID        `json:"round_id"`
	AgentID            domain.ID        `json:"agent_id"`
	ExpectedVersion    int64            `json:"expected_version"`
	NewSummary         string           `json:"new_summary"`
	BeliefProposals    []BeliefProposal `json:"belief_proposals,omitempty"`
	ReplyPayload       json.RawMessage  `json:"reply_payload"`
	ResultExpiresAt    time.Time        `json:"result_expires_at"`
	SearchDocuments    []SearchDocument `json:"search_documents,omitempty"`
}

type AgentRequest struct {
	RequestID   domain.ID           `json:"request_id"`
	GameID      domain.ID           `json:"game_id"`
	Agent       AgentContext        `json:"agent"`
	RoundID     domain.ID           `json:"round_id"`
	RoundKind   domain.RoundKind    `json:"round_kind"`
	Observation *ObservationContext `json:"observation,omitempty"`
	Traditions  []TraditionContext  `json:"traditions"`
	Phase       string              `json:"phase"` // reaction or rebuttal
	Reactions   []AgentReaction     `json:"reactions,omitempty"`
	Candidates  []TraditionIdea     `json:"candidates,omitempty"`
}

type AgentResult struct {
	RequestID domain.ID      `json:"request_id"`
	Reaction  *AgentReaction `json:"reaction,omitempty"`
	Rebuttal  *AgentRebuttal `json:"rebuttal,omitempty"`
	ErrorCode string         `json:"error_code,omitempty"`
}
