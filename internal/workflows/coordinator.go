package workflows

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/amh1k/mythborn/internal/domain"
	"github.com/amh1k/mythborn/internal/workflow"
	"go.temporal.io/sdk/temporal"
	tw "go.temporal.io/sdk/workflow"
)

const (
	activityLoadRound          = "LoadRound"
	activitySetStage           = "SetStage"
	activityActivateGame       = "ActivateGame"
	activityDescribePhoto      = "DescribePhoto"
	activityDescriptionChoice  = "ApplyDescriptionDecision"
	activityGenerateAgent      = "GenerateAgentPhase"
	activityHistorian          = "WriteHistorianRecord"
	activityCommitRound        = "CommitRound"
	activityMessenger          = "RunMessengerExchange"
	activityEnqueueCouncils    = "EnqueueCouncils"
	activityDeleteGame         = "DeleteGame"
	activityEnsureClosingRound = "EnsureClosingRound"
)

type RoundLoader interface {
	LoadRound(tw.Context, domain.ID, domain.ID) (workflow.RoundContext, error)
}

func activityContext(ctx tw.Context) tw.Context {
	return tw.WithActivityOptions(ctx, tw.ActivityOptions{
		StartToCloseTimeout:    3 * time.Minute,
		ScheduleToCloseTimeout: 20 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    2 * time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    45 * time.Second,
			MaximumAttempts:    5,
		},
	})
}

// Coordinator serializes all game mutations and runs a daily council timer.
// Model, storage and SQL I/O are invoked only as activities or child workflows.
func Coordinator(ctx tw.Context, gameID domain.ID, carriedProgress map[domain.ID]*RoundProgress) error {
	commands := tw.GetSignalChannel(ctx, workflow.CoordinatorSignalName)
	logger := tw.GetLogger(ctx)
	ctx = activityContext(ctx)
	timer := tw.NewTimer(ctx, 24*time.Hour)
	commandsSinceContinue := 0
	seenEvents := map[domain.ID]bool{}
	progress := carriedProgress
	if progress == nil {
		progress = map[domain.ID]*RoundProgress{}
	}
	for {
		terminate := false
		selector := tw.NewSelector(ctx)
		selector.AddReceive(commands, func(channel tw.ReceiveChannel, more bool) {
			var command workflow.Command
			channel.Receive(ctx, &command)
			if command.EventID != "" && seenEvents[command.EventID] {
				return
			}
			if command.EventID != "" {
				seenEvents[command.EventID] = true
			}
			if command.GameID != gameID {
				logger.Error("ignored coordinator command for another game", "event_id", command.EventID)
				return
			}
			if err := handleCommand(ctx, command, progress); err != nil {
				logger.Error("game command failed", "event_id", command.EventID, "command_type", command.Type, "error", err)
				if command.RoundID != "" {
					var current workflow.RoundContext
					if err := tw.ExecuteActivity(ctx, activityLoadRound, gameID, command.RoundID).Get(ctx, &current); err == nil {
						stage := current.Round.Stage
						if stage == domain.RoundStageComplete {
							stage = domain.RoundStageWriting
						}
						_ = tw.ExecuteActivity(ctx, activitySetStage, gameID, command.RoundID, domain.RoundStatusNeedsAttention, stage).Get(ctx, nil)
					}
				}
			} else if command.Type == workflow.CommandDeleteGame || command.Type == workflow.CommandEndGame {
				terminate = true
			}
		})
		selector.AddFuture(timer, func(tw.Future) {
			if err := tw.ExecuteActivity(ctx, activityEnqueueCouncils, gameID, tw.Now(ctx)).Get(ctx, nil); err != nil {
				logger.Error("scheduled council scan failed", "error", err)
			}
			timer = tw.NewTimer(ctx, 24*time.Hour)
		})
		selector.Select(ctx)
		commandsSinceContinue++
		if terminate {
			return nil
		}
		if commandsSinceContinue >= 250 {
			return tw.NewContinueAsNewError(ctx, workflow.CoordinatorWorkflowName, gameID, progress)
		}
	}
}

func Agent(ctx tw.Context, request workflow.AgentRequest) (workflow.AgentResult, error) {
	ctx = activityContext(ctx)
	var result workflow.AgentResult
	err := tw.ExecuteActivity(ctx, activityGenerateAgent, struct {
		Request workflow.AgentRequest `json:"request"`
	}{Request: request}).Get(ctx, &result)
	return result, err
}

type RoundProgress struct {
	Fingerprint      string
	Reactions        []workflow.AgentReaction
	Candidates       []workflow.TraditionIdea
	Rebuttals        []workflow.AgentRebuttal
	BeliefProposals  []workflow.BeliefProposal
	TraditionChanges []workflow.TraditionChange
	HistorianRecord  *workflow.HistorianRecord
}

func handleCommand(ctx tw.Context, command workflow.Command, progress map[domain.ID]*RoundProgress) error {
	switch command.Type {
	case workflow.CommandStartGame:
		return tw.ExecuteActivity(ctx, activityActivateGame, command.GameID).Get(ctx, nil)
	case workflow.CommandProcessRound, workflow.CommandRetryRound, workflow.CommandRunCouncil:
		return processRound(ctx, command, progress)
	case workflow.CommandDescriptionChoice:
		var abandon bool
		if err := tw.ExecuteActivity(ctx, activityDescriptionChoice, struct {
			Command workflow.Command `json:"command"`
		}{command}).Get(ctx, &abandon); err != nil {
			return err
		}
		if abandon {
			delete(progress, command.RoundID)
			return nil
		}
		return processRound(ctx, command, progress)
	case workflow.CommandMessengerExchange:
		err := tw.ExecuteActivity(ctx, activityMessenger, struct {
			Command workflow.Command `json:"command"`
		}{command}).Get(ctx, nil)
		if err == nil {
			// Messenger summaries and beliefs feed future retrieval. Discard any
			// uncommitted debate snapshot so a retry sees the new memory.
			roundIDs := make([]domain.ID, 0, len(progress))
			for roundID := range progress {
				roundIDs = append(roundIDs, roundID)
			}
			sort.Slice(roundIDs, func(i, j int) bool { return roundIDs[i] < roundIDs[j] })
			for _, roundID := range roundIDs {
				delete(progress, roundID)
			}
		}
		return err
	case workflow.CommandEndGame:
		return closeGame(ctx, command)
	case workflow.CommandDeleteGame:
		return tw.ExecuteActivity(ctx, activityDeleteGame, struct {
			GameID    domain.ID `json:"game_id"`
			CommandID domain.ID `json:"command_id"`
		}{command.GameID, command.EventID}).Get(ctx, nil)
	default:
		return fmt.Errorf("unsupported workflow command %q", command.Type)
	}
}

func processRound(ctx tw.Context, command workflow.Command, progressByRound map[domain.ID]*RoundProgress) error {
	if command.RoundID == "" {
		return fmt.Errorf("round command is missing round_id")
	}
	var current workflow.RoundContext
	if err := tw.ExecuteActivity(ctx, activityLoadRound, command.GameID, command.RoundID).Get(ctx, &current); err != nil {
		return err
	}
	if current.Round.Status == domain.RoundStatusComplete {
		delete(progressByRound, command.RoundID)
		return nil
	}
	if current.Round.Status == domain.RoundStatusAbandoned {
		delete(progressByRound, command.RoundID)
		return nil
	}
	progress := progressByRound[command.RoundID]
	if progress == nil {
		progress = &RoundProgress{}
		progressByRound[command.RoundID] = progress
	}
	fingerprint := stateFingerprint(current)
	if progress.Fingerprint != "" && progress.Fingerprint != fingerprint {
		*progress = RoundProgress{}
	}
	progress.Fingerprint = fingerprint
	if current.Round.Kind == domain.RoundKindDiscovery {
		if current.Observation == nil {
			return fmt.Errorf("discovery round has no observation")
		}
		if current.Observation.DescriptionStatus == "pending" {
			if err := tw.ExecuteActivity(ctx, activitySetStage, command.GameID, command.RoundID, domain.RoundStatusRunning, domain.RoundStageDescribing).Get(ctx, nil); err != nil {
				return err
			}
			var description activitiesDescriptionResult
			if err := tw.ExecuteActivity(ctx, activityDescribePhoto, descriptionInput(command)).Get(ctx, &description); err != nil {
				return err
			}
			if !description.Usable || current.Game.ReviewPhotoDescription {
				stage := domain.RoundStageAwaitingClarityChoice
				if description.Usable {
					stage = domain.RoundStageAwaitingReview
				}
				return tw.ExecuteActivity(ctx, activitySetStage, command.GameID, command.RoundID, domain.RoundStatusRunning, stage).Get(ctx, nil)
			}
			current.Observation.VisualDescription = description.Description
			current.Observation.DescriptionStatus = "accepted"
			if refreshed, err := loadRound(ctx, command); err == nil {
				current = refreshed
			}
		}
		if current.Observation.DescriptionStatus == "awaiting_review" || current.Observation.DescriptionStatus == "uncertain" {
			return tw.ExecuteActivity(ctx, activitySetStage, command.GameID, command.RoundID, domain.RoundStatusRunning, stageForDescription(current.Observation.DescriptionStatus)).Get(ctx, nil)
		}
	}
	if current.Round.Kind == domain.RoundKindClosing {
		return finalizeClosing(ctx, command, current)
	}
	if current.Round.Stage == domain.RoundStageQueued || current.Round.Stage == domain.RoundStageDescribing || current.Round.Stage == domain.RoundStageAwaitingReview || current.Round.Stage == domain.RoundStageAwaitingClarityChoice {
		if err := tw.ExecuteActivity(ctx, activitySetStage, command.GameID, command.RoundID, domain.RoundStatusRunning, domain.RoundStageReacting).Get(ctx, nil); err != nil {
			return err
		}
		current.Round.Stage = domain.RoundStageReacting
	}
	var err error
	if len(progress.Reactions) == 0 {
		progress.Reactions, _, err = runAgentPhase(ctx, command, current, "reaction", nil, nil)
		if err != nil {
			return err
		}
		if len(progress.Reactions) != 4 {
			return fmt.Errorf("reaction phase did not return four agents")
		}
	}
	if err = tw.ExecuteActivity(ctx, activitySetStage, command.GameID, command.RoundID, domain.RoundStatusRunning, domain.RoundStageDebating).Get(ctx, nil); err != nil {
		return err
	}
	if len(progress.Candidates) == 0 {
		progress.Candidates = buildCandidates(current, progress.Reactions)
	}
	if len(progress.Rebuttals) == 0 {
		_, progress.Rebuttals, err = runAgentPhase(ctx, command, current, "rebuttal", progress.Reactions, progress.Candidates)
		if err != nil {
			return err
		}
		if len(progress.Rebuttals) != 4 {
			return fmt.Errorf("rebuttal phase did not return four agents")
		}
		for _, rebuttal := range progress.Rebuttals {
			progress.BeliefProposals = append(progress.BeliefProposals, rebuttal.BeliefProposals...)
		}
		votes := make([]workflow.TraditionSupport, 0, len(progress.Rebuttals)*len(progress.Candidates))
		for _, rebuttal := range progress.Rebuttals {
			votes = append(votes, rebuttal.TraditionSupport...)
		}
		agentIDs := make([]domain.ID, len(current.Agents))
		for i, agent := range current.Agents {
			agentIDs[i] = agent.Agent.ID
		}
		progress.TraditionChanges, err = ComputeTraditionChanges(command.GameID, command.RoundID, current.Traditions, progress.Candidates, votes, agentIDs)
		if err != nil {
			return err
		}
	}
	if err = tw.ExecuteActivity(ctx, activitySetStage, command.GameID, command.RoundID, domain.RoundStatusRunning, domain.RoundStageWriting).Get(ctx, nil); err != nil {
		return err
	}
	if progress.HistorianRecord == nil {
		var record workflow.HistorianRecord
		if err = tw.ExecuteActivity(ctx, activityHistorian, activitiesHistorianInput{Context: current, Reactions: progress.Reactions, Rebuttals: progress.Rebuttals, BeliefChanges: progress.BeliefProposals, Traditions: progress.TraditionChanges}).Get(ctx, &record); err != nil {
			return err
		}
		progress.HistorianRecord = &record
	}
	record := *progress.HistorianRecord
	outcome := record.Outcome
	chronicle := domain.Chronicle{ID: StableTraditionID(command.GameID, command.RoundID, "chronicle"), GameID: command.GameID, RoundID: command.RoundID, HistorianAgentID: current.HistorianAgentID, Title: record.Title, Outcome: &outcome, Verdict: record.Verdict, Body: record.Body, Suggestion: record.Suggestion, Metadata: record.Metadata}
	commit := workflow.RoundCommit{EventID: command.EventID, ProcessedCommandID: command.EventID, Chronicle: chronicle, BeliefProposals: progress.BeliefProposals, TraditionChanges: progress.TraditionChanges}
	if err = tw.ExecuteActivity(ctx, activityCommitRound, struct {
		Commit workflow.RoundCommit `json:"commit"`
	}{commit}).Get(ctx, nil); err != nil {
		return err
	}
	delete(progressByRound, command.RoundID)
	return nil
}

func stateFingerprint(round workflow.RoundContext) string {
	state := struct {
		Agents     []workflow.AgentContext     `json:"agents"`
		Traditions []workflow.TraditionContext `json:"traditions"`
	}{round.Agents, round.Traditions}
	data, _ := json.Marshal(state)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

type activitiesDescriptionResult struct {
	Description string `json:"description"`
	Usable      bool   `json:"usable"`
	Reason      string `json:"reason,omitempty"`
}
type activitiesHistorianInput struct {
	Context       workflow.RoundContext      `json:"context"`
	Reactions     []workflow.AgentReaction   `json:"reactions,omitempty"`
	Rebuttals     []workflow.AgentRebuttal   `json:"rebuttals,omitempty"`
	BeliefChanges []workflow.BeliefProposal  `json:"belief_changes,omitempty"`
	Traditions    []workflow.TraditionChange `json:"traditions,omitempty"`
}

func descriptionInput(c workflow.Command) struct {
	GameID  domain.ID `json:"game_id"`
	RoundID domain.ID `json:"round_id"`
} {
	return struct {
		GameID  domain.ID `json:"game_id"`
		RoundID domain.ID `json:"round_id"`
	}{c.GameID, c.RoundID}
}
func loadRound(ctx tw.Context, c workflow.Command) (workflow.RoundContext, error) {
	var value workflow.RoundContext
	err := tw.ExecuteActivity(ctx, activityLoadRound, c.GameID, c.RoundID).Get(ctx, &value)
	return value, err
}
func stageForDescription(status string) domain.RoundStage {
	if status == "awaiting_review" {
		return domain.RoundStageAwaitingReview
	}
	return domain.RoundStageAwaitingClarityChoice
}

func runAgentPhase(ctx tw.Context, command workflow.Command, current workflow.RoundContext, phase string, reactions []workflow.AgentReaction, candidates []workflow.TraditionIdea) ([]workflow.AgentReaction, []workflow.AgentRebuttal, error) {
	type completed struct {
		reaction *workflow.AgentReaction
		rebuttal *workflow.AgentRebuttal
	}
	futures := make([]tw.Future, 0, len(current.Agents))
	for _, agent := range current.Agents {
		requestID := StableTraditionID(command.GameID, command.RoundID, string(command.EventID)+"/"+string(agent.Agent.ID)+"/"+phase)
		request := workflow.AgentRequest{RequestID: requestID, GameID: command.GameID, Agent: agent, RoundID: command.RoundID, RoundKind: current.Round.Kind, Observation: current.Observation, Traditions: current.Traditions, Phase: phase, Reactions: reactions, Candidates: candidates}
		childID := workflow.AgentWorkflowID(command.GameID, agent.Agent.ID) + "/round/" + string(command.RoundID) + "/event/" + string(command.EventID) + "/" + phase
		childCtx := tw.WithChildOptions(ctx, tw.ChildWorkflowOptions{WorkflowID: childID})
		futures = append(futures, tw.ExecuteChildWorkflow(childCtx, workflow.AgentWorkflowName, request))
	}
	results := make([]completed, 0, len(futures))
	for _, future := range futures {
		var result workflow.AgentResult
		if err := future.Get(ctx, &result); err != nil {
			return nil, nil, err
		}
		results = append(results, completed{reaction: result.Reaction, rebuttal: result.Rebuttal})
	}
	reactionsOut := make([]workflow.AgentReaction, 0, 4)
	rebuttalsOut := make([]workflow.AgentRebuttal, 0, 4)
	for _, result := range results {
		if result.reaction != nil {
			reactionsOut = append(reactionsOut, *result.reaction)
		}
		if result.rebuttal != nil {
			rebuttalsOut = append(rebuttalsOut, *result.rebuttal)
		}
	}
	sort.Slice(reactionsOut, func(i, j int) bool { return reactionsOut[i].AgentID < reactionsOut[j].AgentID })
	sort.Slice(rebuttalsOut, func(i, j int) bool { return rebuttalsOut[i].AgentID < rebuttalsOut[j].AgentID })
	return reactionsOut, rebuttalsOut, nil
}

func buildCandidates(current workflow.RoundContext, reactions []workflow.AgentReaction) []workflow.TraditionIdea {
	byKey := map[string]workflow.TraditionIdea{}
	traditions := make(map[domain.ID]workflow.TraditionContext, len(current.Traditions))
	for _, tradition := range current.Traditions {
		traditions[tradition.Tradition.ID] = tradition
	}
	for _, reaction := range reactions {
		for _, idea := range reaction.TraditionIdeas {
			if idea.TraditionID != nil {
				tradition, ok := traditions[*idea.TraditionID]
				if !ok {
					continue
				}
				id := tradition.Tradition.ID
				idea = workflow.TraditionIdea{TraditionID: &id, CandidateKey: string(id), Type: tradition.Tradition.Type, Title: tradition.Tradition.Title, Description: tradition.Tradition.Description}
			}
			key := CandidateKey(idea)
			if _, ok := byKey[key]; !ok {
				idea.CandidateKey = key
				byKey[key] = idea
			}
		}
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	items := make([]workflow.TraditionIdea, 0, len(keys))
	for _, key := range keys {
		items = append(items, byKey[key])
	}
	return items
}

func finalizeClosing(ctx tw.Context, command workflow.Command, current workflow.RoundContext) error {
	if err := tw.ExecuteActivity(ctx, activitySetStage, command.GameID, command.RoundID, domain.RoundStatusRunning, domain.RoundStageWriting).Get(ctx, nil); err != nil {
		return err
	}
	var record workflow.HistorianRecord
	if err := tw.ExecuteActivity(ctx, activityHistorian, activitiesHistorianInput{Context: current}).Get(ctx, &record); err != nil {
		return err
	}
	chronicle := domain.Chronicle{ID: StableTraditionID(command.GameID, command.RoundID, "closing-chronicle"), GameID: command.GameID, RoundID: command.RoundID, HistorianAgentID: current.HistorianAgentID, Title: record.Title, Verdict: record.Verdict, Body: record.Body, Metadata: record.Metadata}
	commit := workflow.RoundCommit{EventID: command.EventID, ProcessedCommandID: command.EventID, Chronicle: chronicle}
	return tw.ExecuteActivity(ctx, activityCommitRound, struct {
		Commit workflow.RoundCommit `json:"commit"`
	}{commit}).Get(ctx, nil)
}

func closeGame(ctx tw.Context, command workflow.Command) error {
	var roundID domain.ID
	if err := tw.ExecuteActivity(ctx, activityEnsureClosingRound, struct {
		GameID    domain.ID `json:"game_id"`
		CommandID domain.ID `json:"command_id"`
	}{command.GameID, command.EventID}).Get(ctx, &roundID); err != nil {
		return err
	}
	command.RoundID = roundID
	var current workflow.RoundContext
	if err := tw.ExecuteActivity(ctx, activityLoadRound, command.GameID, command.RoundID).Get(ctx, &current); err != nil {
		return err
	}
	return finalizeClosing(ctx, command, current)
}
