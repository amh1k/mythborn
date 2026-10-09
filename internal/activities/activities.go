// Package activities contains all worker operations that touch SQL, Storage,
// retrieval, or model providers. Workflow code stays deterministic.
package activities

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/models"
	"github.com/amh1k/mythborn/internal/retrieval"
	"github.com/amh1k/mythborn/internal/storage"
	"github.com/amh1k/mythborn/internal/workflow"
)

const maxPhotoBytes = 10 << 20

type Store interface {
	LoadRoundContext(context.Context, domain.ID, domain.ID) (workflow.RoundContext, error)
	SetRoundStage(context.Context, domain.ID, domain.ID, domain.RoundStatus, domain.RoundStage) error
	ActivateGame(context.Context, domain.ID) error
	CommitRound(context.Context, workflow.RoundCommit) error
	LoadMessengerContext(context.Context, domain.ID, domain.ID, domain.ID) (workflow.MessengerContext, error)
	CommitMessenger(context.Context, workflow.MessengerCommit) error
	SavePhotoDescription(context.Context, domain.ID, domain.ID, domain.ID, string, string) error
	ApplyDescriptionDecision(context.Context, domain.ID, domain.ID, domain.ID, domain.ID, string, string) (bool, error)
	FindCouncilCandidates(context.Context, time.Time, time.Duration) ([]domain.ID, error)
	EnqueueCouncilIfEligible(context.Context, domain.ID, time.Time) (domain.WorkflowCommand, bool, error)
	LoadGamePhotoPaths(context.Context, domain.ID) ([]string, error)
	CompleteGameDeletion(context.Context, domain.ID, domain.ID) error
	EnsureClosingRound(context.Context, domain.ID, domain.ID) (domain.ID, error)
	AccountDeletionReady(context.Context, domain.ID, domain.ID) (bool, error)
	CompleteAccountDeletion(context.Context, domain.ID, domain.ID) error
}

type JSONModel interface {
	GenerateJSON(context.Context, string, []byte, string, any) error
}

type AuthAdmin interface {
	DeleteUser(context.Context, domain.ID) error
}

type Activities struct {
	Store     Store
	Photos    storage.PhotoStorage
	Models    JSONModel
	Retriever retrieval.Retriever
	Auth      AuthAdmin
	Now       func() time.Time
}

type DescriptionInput struct {
	GameID  domain.ID `json:"game_id"`
	RoundID domain.ID `json:"round_id"`
}

type DescriptionResult struct {
	Description string `json:"description"`
	Usable      bool   `json:"usable"`
	Reason      string `json:"reason,omitempty"`
}

func (a *Activities) DescribePhoto(ctx context.Context, input DescriptionInput) (DescriptionResult, error) {
	round, err := a.Store.LoadRoundContext(ctx, input.GameID, input.RoundID)
	if err != nil {
		return DescriptionResult{}, err
	}
	if round.Observation == nil {
		return DescriptionResult{}, fmt.Errorf("discovery has no observation")
	}
	reader, err := a.Photos.Open(ctx, round.Observation.PhotoObjectPath)
	if err != nil {
		return DescriptionResult{}, err
	}
	defer reader.Close()
	image, err := io.ReadAll(io.LimitReader(reader, maxPhotoBytes+1))
	if err != nil {
		return DescriptionResult{}, err
	}
	if len(image) == 0 || len(image) > maxPhotoBytes {
		return DescriptionResult{}, fmt.Errorf("stored photo is empty or exceeds the processing limit")
	}
	prompt := `Describe only directly visible details in this outdoor photo. Do not infer location, identity, cause, mythology, emotion, or hidden context. Return JSON with description (one or two factual sentences), usable (true only if the image contains enough visible detail to discuss), and reason (brief, only when unusable).`
	var result DescriptionResult
	if err = a.Models.GenerateJSON(ctx, prompt, image, round.Observation.MIMEType, &result); err != nil {
		return result, err
	}
	result.Description = boundedText(result.Description, 3000)
	result.Reason = boundedText(result.Reason, 500)
	if result.Description == "" {
		return result, fmt.Errorf("model returned no photo description")
	}
	status := "accepted"
	if !result.Usable {
		status = "uncertain"
	} else if round.Game.ReviewPhotoDescription {
		status = "awaiting_review"
	}
	if err = a.Store.SavePhotoDescription(ctx, input.GameID, input.RoundID, round.Observation.ID, result.Description, status); err != nil {
		return result, err
	}
	return result, nil
}

type DescriptionDecisionInput struct {
	Command workflow.Command `json:"command"`
}

func (a *Activities) ApplyDescriptionDecision(ctx context.Context, input DescriptionDecisionInput) (bool, error) {
	cmd := input.Command
	if cmd.RoundID == "" || cmd.GameID == "" {
		return false, fmt.Errorf("description command is missing game or round")
	}
	round, err := a.Store.LoadRoundContext(ctx, cmd.GameID, cmd.RoundID)
	if err != nil {
		return false, err
	}
	if round.Observation == nil {
		return false, fmt.Errorf("discovery has no observation")
	}
	return a.Store.ApplyDescriptionDecision(ctx, cmd.EventID, cmd.GameID, cmd.RoundID, round.Observation.ID, string(cmd.Payload.DescriptionDecision), cmd.Payload.Correction)
}

func (a *Activities) SetStage(ctx context.Context, gameID, roundID domain.ID, status domain.RoundStatus, stage domain.RoundStage) error {
	return a.Store.SetRoundStage(ctx, gameID, roundID, status, stage)
}

func (a *Activities) ActivateGame(ctx context.Context, gameID domain.ID) error {
	return a.Store.ActivateGame(ctx, gameID)
}

func (a *Activities) LoadRound(ctx context.Context, gameID, roundID domain.ID) (workflow.RoundContext, error) {
	return a.Store.LoadRoundContext(ctx, gameID, roundID)
}

type AgentPhaseInput struct {
	Request workflow.AgentRequest `json:"request"`
}

func (a *Activities) GenerateAgentPhase(ctx context.Context, input AgentPhaseInput) (workflow.AgentResult, error) {
	req := input.Request
	if req.Agent.Agent.ID == "" || req.Agent.Agent.GameID != req.GameID {
		return workflow.AgentResult{}, fmt.Errorf("agent is not part of the round")
	}
	agent := &req.Agent
	query := string(req.RoundKind)
	if req.Observation != nil {
		query += " " + req.Observation.VisualDescription + " " + req.Observation.PlayerStatement + " " + req.Observation.PlayerCorrection
	}
	memories, err := a.Retriever.Relevant(ctx, req.GameID, req.Agent.Agent.ID, query)
	if err != nil {
		return workflow.AgentResult{}, err
	}
	base := map[string]any{
		"agent_type": agent.Agent.Type, "agent_name": agent.Agent.DisplayName,
		"personality": agent.Agent.PersonalityPrompt, "beliefs": agent.Beliefs,
		"memories": memories, "round_kind": req.RoundKind,
		"description": "", "player_statement": "", "player_correction": "",
		"current_traditions": req.Traditions,
	}
	if req.Observation != nil {
		base["description"] = req.Observation.VisualDescription
		base["player_statement"] = req.Observation.PlayerStatement
		base["player_correction"] = req.Observation.PlayerCorrection
	}
	result := workflow.AgentResult{RequestID: req.RequestID}
	if req.Phase == "reaction" {
		prompt := fmt.Sprintf("You are a persistent character in a small civilization. Stay in character but distinguish visible evidence from claims and memory. Make one interpretation and, if meaningful, raise up to two candidate myths, rituals, or taboos. Existing traditions should be raised for reconsideration only when this event is relevant; if doing so, include the exact tradition_id from current_traditions. Return JSON: {\"interpretation\":\"...\",\"reasoning\":\"...\",\"tradition_ideas\":[{\"tradition_id\":null,\"type\":\"myth|ritual|taboo\",\"title\":\"...\",\"description\":\"...\"}]}. For an existing tradition, set its actual tradition_id and keep its type/title/description. Input: %s", encodePrompt(base))
		var output workflow.AgentReaction
		if err = a.Models.GenerateJSON(ctx, prompt, nil, "", &output); err != nil {
			return result, err
		}
		output.EventID, output.AgentID = req.RequestID, req.Agent.Agent.ID
		output.Interpretation = boundedText(output.Interpretation, 1200)
		output.Reasoning = boundedText(output.Reasoning, 1200)
		if err = validateReaction(output, req.Traditions); err != nil {
			return result, err
		}
		result.Reaction = &output
		return result, nil
	}
	if req.Phase != "rebuttal" {
		return result, fmt.Errorf("unknown agent phase %q", req.Phase)
	}
	base["reactions"] = req.Reactions
	base["candidates"] = req.Candidates
	prompt := fmt.Sprintf("You are rebutting once after hearing the same evidence and four initial reactions. Return JSON with response; optional belief_proposals [{belief_id, change_type, new_claim, new_state, reason}] (omit belief_id only for a new belief with change_type formed); tradition_support with exactly one vote for every candidate [{candidate_key, tradition_id, support, reason}] (copy candidate_key for new ideas, tradition_id for existing); and optional new_tradition_ideas [{type,title,description}] only for the chronicle to mention later, do not vote these. Set support to true or false. Keep disagreements respectful and explain each vote. Input: %s", encodePrompt(base))
	var output workflow.AgentRebuttal
	if err = a.Models.GenerateJSON(ctx, prompt, nil, "", &output); err != nil {
		return result, err
	}
	output.EventID, output.AgentID = req.RequestID, req.Agent.Agent.ID
	output.Response = boundedText(output.Response, 1200)
	if err = validateRebuttal(output, req.Candidates, agent); err != nil {
		return result, err
	}
	result.Rebuttal = &output
	return result, nil
}

type HistorianInput struct {
	Context       workflow.RoundContext      `json:"context"`
	Reactions     []workflow.AgentReaction   `json:"reactions,omitempty"`
	Rebuttals     []workflow.AgentRebuttal   `json:"rebuttals,omitempty"`
	BeliefChanges []workflow.BeliefProposal  `json:"belief_changes,omitempty"`
	Traditions    []workflow.TraditionChange `json:"traditions,omitempty"`
}

func (a *Activities) WriteHistorianRecord(ctx context.Context, input HistorianInput) (workflow.HistorianRecord, error) {
	query := evidenceText(input.Context)
	memories, err := a.Retriever.Relevant(ctx, input.Context.Game.ID, input.Context.HistorianAgentID, query)
	if err != nil {
		return workflow.HistorianRecord{}, err
	}
	historian, ok := findAgent(input.Context, input.Context.HistorianAgentID)
	if !ok {
		return workflow.HistorianRecord{}, fmt.Errorf("historian agent is missing")
	}
	promptData := map[string]any{"historian": historian.Agent, "memories": memories, "context": input.Context, "reactions": input.Reactions, "rebuttals": input.Rebuttals, "belief_changes": input.BeliefChanges, "tradition_changes": input.Traditions}
	var output workflow.HistorianRecord
	prompt := fmt.Sprintf("Write the final chronicle for this %s. Decide outcome as consensus, majority, or unresolved from the agents' expressed interpretations; do not force agreement. Explain the leading interpretation, evidence, important dissent, and actual society changes. For closing records, outcome must be null and suggestion null. Only discovery records may have a short optional outdoor suggestion. Return JSON: {\"title\":\"...\",\"outcome\":\"consensus|majority|unresolved|null\",\"verdict\":\"...\",\"body\":\"...\",\"suggestion\":null}. Input: %s", input.Context.Round.Kind, encodePrompt(promptData))
	if err = a.Models.GenerateJSON(ctx, prompt, nil, "", &output); err != nil {
		return output, err
	}
	output.Title = boundedText(output.Title, 180)
	output.Verdict = boundedText(output.Verdict, 1200)
	output.Body = boundedText(output.Body, 6000)
	if output.Suggestion != nil {
		value := boundedText(*output.Suggestion, 500)
		output.Suggestion = &value
	}
	if output.Title == "" || output.Verdict == "" || output.Body == "" {
		return output, fmt.Errorf("historian output is incomplete")
	}
	if input.Context.Round.Kind != domain.RoundKindClosing && output.Outcome != domain.OutcomeConsensus && output.Outcome != domain.OutcomeMajority && output.Outcome != domain.OutcomeUnresolved {
		return output, fmt.Errorf("historian returned invalid outcome")
	}
	if input.Context.Round.Kind != domain.RoundKindDiscovery {
		output.Suggestion = nil
	}
	if input.Context.Round.Kind == domain.RoundKindClosing {
		output.Outcome = ""
	}
	output.Metadata = map[string]any{"model": models.DefaultTextModel, "prompt_version": "v1"}
	return output, nil
}

type RoundCommitInput struct {
	Commit workflow.RoundCommit `json:"commit"`
}

func (a *Activities) CommitRound(ctx context.Context, input RoundCommitInput) error {
	commit := input.Commit
	round, err := a.Store.LoadRoundContext(ctx, commit.Chronicle.GameID, commit.Chronicle.RoundID)
	if err != nil {
		return err
	}
	for i := range commit.BeliefProposals {
		proposal := &commit.BeliefProposals[i]
		id := proposal.BeliefID
		version := int64(1)
		if id == nil {
			newID := stableID("belief", commit.Chronicle.GameID, commit.Chronicle.RoundID, string(proposal.AgentID)+"/"+proposal.NewClaim)
			proposal.NewBeliefID = newID
			id = &newID
		} else if belief, ok := findBelief(round, proposal.AgentID, *id); ok {
			version = belief.Version + 1
		}
		doc := workflow.SearchDocument{GameID: commit.Chronicle.GameID, AgentID: idCopy(proposal.AgentID), BeliefID: idCopy(*id), Content: boundedText(proposal.NewClaim, 12000), SourceVersion: version}
		commit.SearchDocuments = append(commit.SearchDocuments, a.searchDocument(ctx, doc))
	}
	if round.Observation != nil && round.Observation.DescriptionStatus == "accepted" && strings.TrimSpace(round.Observation.VisualDescription) != "" {
		doc := workflow.SearchDocument{GameID: commit.Chronicle.GameID, ObservationID: idCopy(round.Observation.ID), Content: boundedText(round.Observation.VisualDescription+" "+round.Observation.PlayerStatement+" "+round.Observation.PlayerCorrection, 12000), SourceVersion: 1}
		commit.SearchDocuments = append(commit.SearchDocuments, a.searchDocument(ctx, doc))
	}
	chronicleText := strings.TrimSpace(commit.Chronicle.Title + "\n" + commit.Chronicle.Verdict + "\n" + commit.Chronicle.Body)
	if chronicleText != "" {
		doc := workflow.SearchDocument{GameID: commit.Chronicle.GameID, ChronicleID: idCopy(commit.Chronicle.ID), Content: boundedText(chronicleText, 12000), SourceVersion: 1}
		commit.SearchDocuments = append(commit.SearchDocuments, a.searchDocument(ctx, doc))
	}
	return a.Store.CommitRound(ctx, commit)
}

func (a *Activities) EnqueueCouncils(ctx context.Context, gameID domain.ID, now time.Time) error {
	if a.Now != nil {
		now = a.Now()
	}
	_, _, err := a.Store.EnqueueCouncilIfEligible(ctx, gameID, now)
	return err
}

type DeleteGameInput struct {
	GameID    domain.ID `json:"game_id"`
	CommandID domain.ID `json:"command_id"`
}

func (a *Activities) DeleteGame(ctx context.Context, input DeleteGameInput) error {
	paths, err := a.Store.LoadGamePhotoPaths(ctx, input.GameID)
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err = a.Photos.Delete(ctx, path); err != nil {
			return err
		}
	}
	return a.Store.CompleteGameDeletion(ctx, input.GameID, input.CommandID)
}

type EnsureClosingRoundInput struct {
	GameID    domain.ID `json:"game_id"`
	CommandID domain.ID `json:"command_id"`
}

func (a *Activities) EnsureClosingRound(ctx context.Context, input EnsureClosingRoundInput) (domain.ID, error) {
	return a.Store.EnsureClosingRound(ctx, input.GameID, input.CommandID)
}

type DeleteAccountInput struct {
	AccountID domain.ID `json:"account_id"`
	CommandID domain.ID `json:"command_id"`
}

func (a *Activities) DeleteAccount(ctx context.Context, input DeleteAccountInput) (bool, error) {
	ready, err := a.Store.AccountDeletionReady(ctx, input.AccountID, input.CommandID)
	if err != nil || !ready {
		return false, err
	}
	if a.Auth == nil {
		return false, fmt.Errorf("account deletion requires Supabase Auth admin")
	}
	if err = a.Auth.DeleteUser(ctx, input.AccountID); err != nil {
		return false, err
	}
	if err = a.Store.CompleteAccountDeletion(ctx, input.AccountID, input.CommandID); err != nil {
		return false, err
	}
	return true, nil
}

type MessengerInput struct {
	Command workflow.Command `json:"command"`
}

func (a *Activities) RunMessengerExchange(ctx context.Context, input MessengerInput) error {
	cmd := input.Command
	if cmd.RoundID == "" || cmd.GameID == "" || cmd.Payload.AgentID == "" || strings.TrimSpace(cmd.Payload.Message) == "" {
		return fmt.Errorf("messenger command is incomplete")
	}
	ctxData, err := a.Store.LoadMessengerContext(ctx, cmd.GameID, cmd.RoundID, cmd.Payload.AgentID)
	if err != nil {
		return err
	}
	query := strings.TrimSpace(cmd.Payload.Message)
	memories, err := a.Retriever.Relevant(ctx, cmd.GameID, cmd.Payload.AgentID, query)
	if err != nil {
		return err
	}
	version := int64(0)
	var summaryID domain.ID
	var previous string
	if ctxData.Summary != nil {
		version, summaryID, previous = ctxData.Summary.Version, ctxData.Summary.ID, ctxData.Summary.Summary
	}
	if version != cmd.Payload.ExpectedSummaryVersion {
		return fmt.Errorf("conversation summary version conflict")
	}
	var reply struct {
		Reply           string                    `json:"reply"`
		NewSummary      string                    `json:"new_summary"`
		BeliefProposals []workflow.BeliefProposal `json:"belief_proposals,omitempty"`
	}
	prompt := fmt.Sprintf("You are %s, speaking as a persistent character. Answer the messenger's short message. Update the rolling summary with only durable relevant context, and propose belief changes only if the exchange changed your view. Return JSON with reply, new_summary, and optional belief_proposals. Prior summary: %s. Current beliefs: %s. Relevant history: %s. Messenger message: %s", ctxData.Agent.DisplayName, previous, encodePrompt(ctxData.Beliefs), encodePrompt(memories), boundedText(cmd.Payload.Message, 4000))
	if err = a.Models.GenerateJSON(ctx, prompt, nil, "", &reply); err != nil {
		return err
	}
	reply.Reply, reply.NewSummary = boundedText(reply.Reply, 2000), boundedText(reply.NewSummary, 12000)
	if reply.Reply == "" || reply.NewSummary == "" {
		return fmt.Errorf("messenger model output is incomplete")
	}
	for _, proposal := range reply.BeliefProposals {
		if proposal.AgentID != cmd.Payload.AgentID {
			return fmt.Errorf("messenger belief proposal agent mismatch")
		}
	}
	replyPayload, err := json.Marshal(map[string]string{"reply": reply.Reply})
	if err != nil {
		return err
	}
	now := time.Now()
	if a.Now != nil {
		now = a.Now()
	}
	if summaryID == "" {
		summaryID = stableID("conversation-summary", cmd.GameID, cmd.RoundID, string(cmd.Payload.AgentID))
	}
	commit := workflow.MessengerCommit{EventID: cmd.EventID, ProcessedCommandID: cmd.EventID, SummaryID: summaryID, GameID: cmd.GameID, RoundID: cmd.RoundID, AgentID: cmd.Payload.AgentID, ExpectedVersion: version, NewSummary: reply.NewSummary, BeliefProposals: reply.BeliefProposals, ReplyPayload: replyPayload, ResultExpiresAt: now.Add(workflow.MessengerReplyTTL)}
	summaryDoc := workflow.SearchDocument{GameID: cmd.GameID, AgentID: idCopy(cmd.Payload.AgentID), ConversationSummaryID: idCopy(summaryID), Content: boundedText(reply.NewSummary, 12000), SourceVersion: version + 1}
	commit.SearchDocuments = append(commit.SearchDocuments, a.searchDocument(ctx, summaryDoc))
	for i := range commit.BeliefProposals {
		proposal := &commit.BeliefProposals[i]
		id := proposal.BeliefID
		beliefVersion := int64(1)
		if id == nil {
			newID := stableID("belief", cmd.GameID, cmd.RoundID, string(cmd.EventID)+"/"+string(proposal.AgentID)+"/"+proposal.NewClaim)
			proposal.NewBeliefID = newID
			id = &newID
		} else {
			for _, belief := range ctxData.Beliefs {
				if belief.ID == *id {
					beliefVersion = belief.Version + 1
					break
				}
			}
		}
		doc := workflow.SearchDocument{GameID: cmd.GameID, AgentID: idCopy(proposal.AgentID), BeliefID: idCopy(*id), Content: boundedText(proposal.NewClaim, 12000), SourceVersion: beliefVersion}
		commit.SearchDocuments = append(commit.SearchDocuments, a.searchDocument(ctx, doc))
	}
	return a.Store.CommitMessenger(ctx, commit)
}

type embeddingProvider interface {
	Embed(context.Context, string, bool) ([]float32, error)
}

func (a *Activities) searchDocument(ctx context.Context, document workflow.SearchDocument) workflow.SearchDocument {
	if embedder, ok := a.Models.(embeddingProvider); ok {
		if vector, err := embedder.Embed(ctx, document.Content, false); err == nil && len(vector) == models.EmbeddingDimensions {
			document.Embedding = vector
		}
	}
	return document
}

func stableID(namespace string, gameID, parentID domain.ID, key string) domain.ID {
	sum := sha256.Sum256([]byte("mythborn/" + namespace + "/" + string(gameID) + "/" + string(parentID) + "/" + key))
	b := sum[:16]
	b[6] = b[6]&0x0f | 0x50
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b)
	return domain.ID(h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:])
}

func idCopy(id domain.ID) *domain.ID { return &id }

func findBelief(round workflow.RoundContext, agentID, beliefID domain.ID) (domain.Belief, bool) {
	for _, agent := range round.Agents {
		if agent.Agent.ID != agentID {
			continue
		}
		for _, belief := range agent.Beliefs {
			if belief.ID == beliefID {
				return belief, true
			}
		}
	}
	return domain.Belief{}, false
}

func boundedText(text string, limit int) string {
	text = strings.TrimSpace(text)
	if len(text) > limit {
		return strings.TrimSpace(text[:limit])
	}
	return text
}

func encodePrompt(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	if len(data) > 50000 {
		data = data[:50000]
	}
	return string(data)
}

func evidenceText(ctx workflow.RoundContext) string {
	if ctx.Observation == nil {
		return string(ctx.Round.Kind) + " council; review existing evidence and memories only"
	}
	return ctx.Observation.VisualDescription + " " + ctx.Observation.PlayerStatement + " " + ctx.Observation.PlayerCorrection
}

func findAgent(ctx workflow.RoundContext, agentID domain.ID) (workflow.AgentContext, bool) {
	for _, agent := range ctx.Agents {
		if agent.Agent.ID == agentID {
			return agent, true
		}
	}
	return workflow.AgentContext{}, false
}

func validateReaction(r workflow.AgentReaction, current []workflow.TraditionContext) error {
	if strings.TrimSpace(r.Interpretation) == "" || strings.TrimSpace(r.Reasoning) == "" || len(r.TraditionIdeas) > 2 {
		return fmt.Errorf("agent reaction is incomplete or oversized")
	}
	currentByID := make(map[domain.ID]workflow.TraditionContext, len(current))
	for _, item := range current {
		currentByID[item.Tradition.ID] = item
	}
	for _, idea := range r.TraditionIdeas {
		if idea.TraditionID != nil {
			if _, ok := currentByID[*idea.TraditionID]; !ok {
				return fmt.Errorf("reaction references an unknown tradition")
			}
		} else if !validTraditionType(idea.Type) || idea.Title == "" || idea.Description == "" {
			return fmt.Errorf("reaction must introduce a valid new tradition candidate")
		}
	}
	for i := range r.TraditionIdeas {
		r.TraditionIdeas[i].Title = boundedText(r.TraditionIdeas[i].Title, 120)
		r.TraditionIdeas[i].Description = boundedText(r.TraditionIdeas[i].Description, 500)
		if r.TraditionIdeas[i].TraditionID == nil && (r.TraditionIdeas[i].Title == "" || r.TraditionIdeas[i].Description == "") {
			return fmt.Errorf("new tradition candidate is empty after normalization")
		}
	}
	return nil
}

func validateRebuttal(r workflow.AgentRebuttal, candidates []workflow.TraditionIdea, agent *workflow.AgentContext) error {
	if strings.TrimSpace(r.Response) == "" || len(r.BeliefProposals) > 8 || len(r.TraditionSupport) != len(candidates) || len(r.NewTraditionIdeas) > 2 {
		return fmt.Errorf("agent rebuttal is incomplete or oversized")
	}
	expected := map[string]bool{}
	for _, candidate := range candidates {
		key := candidate.CandidateKey
		if candidate.TraditionID != nil {
			key = string(*candidate.TraditionID)
		}
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(string(candidate.Type) + ":" + candidate.Title))
		}
		expected[key] = true
	}
	seen := map[string]bool{}
	for i := range r.TraditionSupport {
		vote := &r.TraditionSupport[i]
		if vote.AgentID == "" {
			vote.AgentID = agent.Agent.ID
		}
		if vote.AgentID != agent.Agent.ID {
			return fmt.Errorf("tradition vote agent mismatch")
		}
		key := vote.CandidateKey
		if vote.TraditionID != nil {
			key = string(*vote.TraditionID)
		}
		if key == "" || !expected[key] || seen[key] {
			return fmt.Errorf("agent rebuttal has duplicate or unkeyed tradition votes")
		}
		seen[key] = true
	}
	if len(seen) != len(expected) {
		return fmt.Errorf("agent rebuttal omitted a tradition candidate")
	}
	for _, idea := range r.NewTraditionIdeas {
		if !validTraditionType(idea.Type) || idea.Title == "" || idea.Description == "" {
			return fmt.Errorf("invalid deferred tradition idea")
		}
	}
	for i := range r.NewTraditionIdeas {
		r.NewTraditionIdeas[i].Title = boundedText(r.NewTraditionIdeas[i].Title, 120)
		r.NewTraditionIdeas[i].Description = boundedText(r.NewTraditionIdeas[i].Description, 500)
		if r.NewTraditionIdeas[i].Title == "" || r.NewTraditionIdeas[i].Description == "" {
			return fmt.Errorf("deferred tradition idea is empty after normalization")
		}
	}
	seenBeliefs := map[string]bool{}
	for i := range r.BeliefProposals {
		proposal := &r.BeliefProposals[i]
		if proposal.AgentID == "" {
			proposal.AgentID = agent.Agent.ID
		}
		if proposal.AgentID != agent.Agent.ID || strings.TrimSpace(proposal.NewClaim) == "" || strings.TrimSpace(proposal.Reason) == "" {
			return fmt.Errorf("invalid belief proposal")
		}
		if proposal.BeliefID == nil && proposal.ChangeType != domain.BeliefChangeFormed {
			return fmt.Errorf("new beliefs must be marked formed")
		}
		if !validBeliefState(proposal.NewState) || !validBeliefChange(proposal.ChangeType) {
			return fmt.Errorf("invalid belief proposal enum")
		}
		key := "new:" + strings.ToLower(strings.TrimSpace(proposal.NewClaim))
		if proposal.BeliefID != nil {
			key = string(*proposal.BeliefID)
		}
		if seenBeliefs[key] {
			return fmt.Errorf("duplicate belief proposal")
		}
		seenBeliefs[key] = true
		proposal.NewClaim = boundedText(proposal.NewClaim, 1000)
		proposal.Reason = boundedText(proposal.Reason, 500)
	}
	for i := range r.TraditionSupport {
		r.TraditionSupport[i].Reason = boundedText(r.TraditionSupport[i].Reason, 400)
	}
	return nil
}

func validTraditionType(t domain.TraditionType) bool {
	return t == domain.TraditionTypeMyth || t == domain.TraditionTypeRitual || t == domain.TraditionTypeTaboo
}

func validBeliefState(s domain.BeliefState) bool {
	return s == domain.BeliefStateForming || s == domain.BeliefStateHeld || s == domain.BeliefStateQuestioned || s == domain.BeliefStateAbandoned
}

func validBeliefChange(c domain.BeliefChangeType) bool {
	return c == domain.BeliefChangeFormed || c == domain.BeliefChangeStrengthened || c == domain.BeliefChangeWeakened || c == domain.BeliefChangeRevised || c == domain.BeliefChangeAbandoned
}
