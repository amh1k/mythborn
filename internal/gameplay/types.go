// Package gameplay defines bounded, committed continuity state. It contains no model or database I/O.
package gameplay

import (
	"time"

	"github.com/amh1k/mythborn/internal/domain"
)

type Features struct {
	Mysteries     bool `json:"mysteries"`
	Relationships bool `json:"relationships"`
	Culture       bool `json:"culture"`
}

func Enabled() Features { return Features{true, true, true} }

type Explanation struct {
	Key      string      `json:"key"`
	Claim    string      `json:"claim"`
	AgentIDs []domain.ID `json:"agent_ids"`
}
type Mystery struct {
	ID              domain.ID     `json:"id"`
	GameID          domain.ID     `json:"game_id"`
	Title           string        `json:"title"`
	Question        string        `json:"question"`
	State           string        `json:"state"`
	Explanations    []Explanation `json:"explanations"`
	EvidenceSummary string        `json:"evidence_summary"`
	NextObservation string        `json:"next_observation"`
	OriginRoundID   domain.ID     `json:"origin_round_id"`
	LatestRoundID   domain.ID     `json:"latest_round_id"`
	Version         int64         `json:"version"`
	UpdatedAt       time.Time     `json:"updated_at"`
}
type MysteryProposal struct {
	MysteryID       domain.ID     `json:"mystery_id,omitempty"`
	ExpectedVersion int64         `json:"expected_version"`
	Title           string        `json:"title"`
	Question        string        `json:"question"`
	State           string        `json:"state"`
	Explanations    []Explanation `json:"explanations"`
	EvidenceSummary string        `json:"evidence_summary"`
	NextObservation string        `json:"next_observation"`
	Reason          string        `json:"reason"`
}
type Relationship struct {
	ID            domain.ID  `json:"id"`
	FromAgentID   domain.ID  `json:"from_agent_id"`
	ToAgentID     domain.ID  `json:"to_agent_id"`
	Trust         string     `json:"trust"`
	Rapport       string     `json:"rapport"`
	Summary       string     `json:"summary"`
	LatestRoundID *domain.ID `json:"latest_round_id,omitempty"`
	Version       int64      `json:"version"`
}
type RelationshipProposal struct {
	FromAgentID     domain.ID `json:"from_agent_id"`
	ToAgentID       domain.ID `json:"to_agent_id"`
	ExpectedVersion int64     `json:"expected_version"`
	Trust           string    `json:"trust"`
	Rapport         string    `json:"rapport"`
	Summary         string    `json:"summary"`
	Reason          string    `json:"reason"`
}
type Influence struct {
	ID               domain.ID `json:"id,omitempty"`
	TraditionID      domain.ID `json:"tradition_id"`
	AgentID          domain.ID `json:"agent_id"`
	Category         string    `json:"category"`
	Explanation      string    `json:"explanation"`
	TraditionVersion int64     `json:"tradition_version"`
	TraditionState   string    `json:"tradition_state"`
	RoundID          domain.ID `json:"round_id,omitempty"`
	Title            string    `json:"title,omitempty"`
}
type Revision struct {
	ID            domain.ID `json:"id"`
	SourceRoundID domain.ID `json:"source_round_id"`
	Version       int64     `json:"version"`
	Kind          string    `json:"kind"`
	Before        any       `json:"before,omitempty"`
	After         any       `json:"after"`
	Reason        string    `json:"reason"`
	CreatedAt     time.Time `json:"created_at"`
}
type AgentIdentity struct {
	ID   domain.ID        `json:"id"`
	Name string           `json:"name"`
	Type domain.AgentType `json:"type"`
}

type Context struct {
	Agents           []AgentIdentity `json:"agents"`
	ContractVersion  int             `json:"contract_version"`
	Features         Features        `json:"features"`
	Mysteries        []Mystery       `json:"mysteries"`
	Relationships    []Relationship  `json:"relationships"`
	TraditionIDs     []domain.ID     `json:"tradition_ids"`
	OpenMysteryCount int             `json:"open_mystery_count"`
}
type Changes struct {
	Mysteries     []MysteryProposal      `json:"mysteries,omitempty"`
	Relationships []RelationshipProposal `json:"relationships,omitempty"`
	Influences    []Influence            `json:"influences,omitempty"`
}

type TraditionRevision struct {
	ID                   domain.ID   `json:"id"`
	SourceRoundID        domain.ID   `json:"source_round_id"`
	Version              int64       `json:"version"`
	PreviousTitle        *string     `json:"previous_title,omitempty"`
	PreviousDescription  *string     `json:"previous_description,omitempty"`
	PreviousState        *string     `json:"previous_state,omitempty"`
	NewTitle             string      `json:"new_title"`
	NewDescription       string      `json:"new_description"`
	NewState             string      `json:"new_state"`
	PreviousSupporterIDs []domain.ID `json:"previous_supporter_ids"`
	NewSupporterIDs      []domain.ID `json:"new_supporter_ids"`
	Reason               string      `json:"reason"`
	CreatedAt            time.Time   `json:"created_at"`
}
