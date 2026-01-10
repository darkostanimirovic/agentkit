package agentkit

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// CollaborationSession represents a real-time discussion between multiple agents.
// Unlike handoffs, collaborations are not hierarchical - all agents are peers
// who contribute to a shared conversation. Think of this as a breakout room
// where everyone hashes out ideas together.
type CollaborationSession struct {
	facilitator *Agent   // The agent who runs the conversation flow
	peers       []*Agent // Other agents participating as equals
	options     collaborationOptions
	mu          sync.RWMutex
}

type collaborationOptions struct {
	maxRounds      int           // Maximum number of discussion rounds
	roundTimeout   time.Duration // Timeout for each round
	captureHistory bool          // Whether to capture full conversation history
}

// CollaborationOption configures a collaboration session.
type CollaborationOption func(*collaborationOptions)

// WithMaxRounds sets the maximum number of discussion rounds.
// Each round gives every agent a chance to contribute.
func WithMaxRounds(max int) CollaborationOption {
	return func(o *collaborationOptions) {
		o.maxRounds = max
	}
}

// WithRoundTimeout sets a timeout for each discussion round.
func WithRoundTimeout(timeout time.Duration) CollaborationOption {
	return func(o *collaborationOptions) {
		o.roundTimeout = timeout
	}
}

// WithCaptureHistory enables capturing the full conversation history.
func WithCaptureHistory(capture bool) CollaborationOption {
	return func(o *collaborationOptions) {
		o.captureHistory = capture
	}
}

// CollaborationResult contains the outcome of a collaborative discussion.
type CollaborationResult struct {
	FinalResponse string                       // The synthesized final answer
	Rounds        []CollaborationRound         // History of the discussion
	Summary       string                       // Summary of the collaboration
	Participants  []string                     // Names/IDs of participating agents
	Metadata      map[string]any               // Additional metadata
}

// CollaborationRound represents one round of discussion.
type CollaborationRound struct {
	Number        int                           // Round number (1-indexed)
	Contributions []CollaborationContribution   // Each agent's contribution
	Synthesis     string                        // How the facilitator synthesized this round
}

// CollaborationContribution represents one agent's input in a round.
type CollaborationContribution struct {
	Agent   string    // Agent identifier
	Content string    // What the agent said
	Time    time.Time // When they contributed
}

var (
	ErrCollaborationNoFacilitator = errors.New("agentkit: collaboration requires a facilitator agent")
	ErrCollaborationNoPeers       = errors.New("agentkit: collaboration requires at least one peer agent")
	ErrCollaborationTopicEmpty    = errors.New("agentkit: collaboration topic cannot be empty")
	ErrCollaborationFailed        = errors.New("agentkit: collaboration failed")
)

// NewCollaborationSession creates a new collaboration session.
// The facilitator runs the conversation, and peers contribute as equals.
//
// Example:
//
//	session := agentkit.NewCollaborationSession(
//	    facilitatorAgent,
//	    engineerAgent, designerAgent, productAgent,
//	)
//	
//	result, err := session.Discuss(ctx, "How should we design the authentication API?")
func NewCollaborationSession(facilitator *Agent, peers ...*Agent) *CollaborationSession {
	return &CollaborationSession{
		facilitator: facilitator,
		peers:       peers,
		options: collaborationOptions{
			maxRounds:      3,                // Default: 3 rounds of discussion
			roundTimeout:   2 * time.Minute,  // Default: 2 minutes per round
			captureHistory: true,
		},
	}
}

// Configure applies options to the collaboration session.
func (cs *CollaborationSession) Configure(opts ...CollaborationOption) *CollaborationSession {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	
	for _, opt := range opts {
		opt(&cs.options)
	}
	return cs
}

// Discuss starts a collaborative discussion on a topic.
// All agents participate as peers, sharing ideas and building on each other's contributions.
//
// Example:
//
//	result, err := session.Discuss(ctx, 
//	    "What's the best approach for handling user sessions?",
//	    WithMaxRounds(5),
//	)
func (cs *CollaborationSession) Discuss(ctx context.Context, topic string, opts ...CollaborationOption) (*CollaborationResult, error) {
	if cs.facilitator == nil {
		return nil, ErrCollaborationNoFacilitator
	}
	if len(cs.peers) == 0 {
		return nil, ErrCollaborationNoPeers
	}
	if topic == "" {
		return nil, ErrCollaborationTopicEmpty
	}

	// Apply any runtime options
	options := cs.options
	for _, opt := range opts {
		opt(&options)
	}

	// Get tracer for this collaboration
	tracer := GetTracer(ctx)
	if tracer == nil {
		tracer = cs.facilitator.tracer
	}

	// Create a span for the entire collaboration
	var spanCtx context.Context
	var endSpan func()
	if tracer != nil && !isNoOpTracer(tracer) {
		spanCtx, endSpan = tracer.StartSpan(ctx, "collaboration")
		defer endSpan()

		tracer.SetSpanAttributes(spanCtx, map[string]any{
			"topic":           topic,
			"participant_count": len(cs.peers) + 1, // +1 for facilitator
			"max_rounds":      options.maxRounds,
			"round_timeout":   options.roundTimeout.String(),
			"capture_history": options.captureHistory,
		})
	} else {
		spanCtx = ctx
	}

	// Execute the collaboration
	result, err := cs.executeCollaboration(spanCtx, topic, options, tracer)
	if err != nil {
		if tracer != nil && spanCtx != nil {
			tracer.SetSpanAttributes(spanCtx, map[string]any{
				"error": err.Error(),
			})
		}
		return nil, fmt.Errorf("%w: %v", ErrCollaborationFailed, err)
	}

	// Record success metrics
	if tracer != nil && spanCtx != nil {
		tracer.SetSpanAttributes(spanCtx, map[string]any{
			"rounds_completed": len(result.Rounds),
			"response_length":  len(result.FinalResponse),
		})
	}

	return result, nil
}

// executeCollaboration orchestrates the collaborative discussion.
func (cs *CollaborationSession) executeCollaboration(
	ctx context.Context,
	topic string,
	opts collaborationOptions,
	tracer Tracer,
) (*CollaborationResult, error) {
	result := &CollaborationResult{
		Rounds:       make([]CollaborationRound, 0, opts.maxRounds),
		Participants: cs.getParticipantNames(),
		Metadata:     make(map[string]any),
	}

	// Shared conversation context that grows with each round
	conversationHistory := []string{fmt.Sprintf("Topic: %s", topic)}

	// Run discussion rounds
	for roundNum := 1; roundNum <= opts.maxRounds; roundNum++ {
		// Create a span for this round
		var roundCtx context.Context
		var endRoundSpan func()
		if tracer != nil && !isNoOpTracer(tracer) {
			roundCtx, endRoundSpan = tracer.StartSpan(ctx, fmt.Sprintf("collaboration_round_%d", roundNum))
			defer endRoundSpan()
			
			tracer.SetSpanAttributes(roundCtx, map[string]any{
				"round_number": roundNum,
			})
		} else {
			roundCtx = ctx
		}

		// Set timeout for this round
		var cancel context.CancelFunc
		if opts.roundTimeout > 0 {
			roundCtx, cancel = context.WithTimeout(roundCtx, opts.roundTimeout)
			defer cancel()
		}

		// Execute the round
		round, shouldContinue, err := cs.executeRound(roundCtx, roundNum, conversationHistory, tracer)
		if err != nil {
			// Don't fail the entire collaboration if one round fails
			// Just record the error and stop
			if tracer != nil && roundCtx != nil {
				tracer.SetSpanAttributes(roundCtx, map[string]any{
					"error": err.Error(),
				})
			}
			break
		}

		result.Rounds = append(result.Rounds, round)

		// Update conversation history for next round
		if opts.captureHistory {
			for _, contrib := range round.Contributions {
				conversationHistory = append(conversationHistory, 
					fmt.Sprintf("%s: %s", contrib.Agent, contrib.Content))
			}
			if round.Synthesis != "" {
				conversationHistory = append(conversationHistory, 
					fmt.Sprintf("Synthesis: %s", round.Synthesis))
			}
		}

		// If facilitator signals completion, stop
		if !shouldContinue {
			break
		}
	}

	// Have facilitator create final synthesis
	finalResponse, err := cs.generateFinalSynthesis(ctx, topic, result.Rounds, tracer)
	if err != nil {
		return nil, err
	}

	result.FinalResponse = finalResponse
	result.Summary = cs.generateSummary(result)

	return result, nil
}

// executeRound runs one round of discussion where each peer contributes.
func (cs *CollaborationSession) executeRound(
	ctx context.Context,
	roundNum int,
	history []string,
	tracer Tracer,
) (CollaborationRound, bool, error) {
	round := CollaborationRound{
		Number:        roundNum,
		Contributions: make([]CollaborationContribution, 0, len(cs.peers)),
	}

	// Each peer contributes
	for i, peer := range cs.peers {
		// Create context for this peer's contribution
		peerPrompt := cs.buildPeerPrompt(roundNum, history)
		
		// Create span for peer contribution
		var peerCtx context.Context
		var endPeerSpan func()
		if tracer != nil && !isNoOpTracer(tracer) {
			peerCtx, endPeerSpan = tracer.StartSpan(ctx, fmt.Sprintf("peer_%d_contribution", i+1))
			defer endPeerSpan()
		} else {
			peerCtx = ctx
		}

		// Get peer's contribution
		peerWithTracer := *peer
		if tracer != nil && !isNoOpTracer(tracer) {
			peerWithTracer.tracer = tracer
		}

		events := peerWithTracer.Run(peerCtx, peerPrompt)
		
		var response string
		for event := range events {
			if event.Type == EventTypeFinalOutput {
				if resp, ok := event.Data["response"].(string); ok {
					response = resp
				}
			}
		}
		
		if response == "" {
			// No valid response, skip this peer
			continue
		}

		contribution := CollaborationContribution{
			Agent:   cs.getPeerName(i),
			Content: response,
			Time:    time.Now(),
		}
		round.Contributions = append(round.Contributions, contribution)

		// Add to history for next peer in this round
		history = append(history, fmt.Sprintf("%s: %s", contribution.Agent, contribution.Content))
	}

	// Facilitator synthesizes this round
	synthesis, shouldContinue, err := cs.facilitatorSynthesis(ctx, roundNum, round.Contributions, history, tracer)
	if err != nil {
		return round, false, err
	}

	round.Synthesis = synthesis
	return round, shouldContinue, nil
}

// buildPeerPrompt creates the prompt for a peer agent's contribution.
func (cs *CollaborationSession) buildPeerPrompt(roundNum int, history []string) string {
	prompt := fmt.Sprintf("You are participating in a collaborative discussion (Round %d).\n\n", roundNum)
	
	if len(history) > 0 {
		prompt += "Discussion so far:\n"
		for _, line := range history {
			prompt += fmt.Sprintf("- %s\n", line)
		}
		prompt += "\n"
	}
	
	prompt += "Please share your perspective, ideas, or questions. Be concise and constructive."
	return prompt
}

// facilitatorSynthesis has the facilitator synthesize the round and decide if discussion should continue.
func (cs *CollaborationSession) facilitatorSynthesis(
	ctx context.Context,
	roundNum int,
	contributions []CollaborationContribution,
	history []string,
	tracer Tracer,
) (string, bool, error) {
	// Build synthesis prompt
	prompt := fmt.Sprintf("You are facilitating a collaborative discussion (Round %d).\n\n", roundNum)
	prompt += "Contributions this round:\n"
	for _, contrib := range contributions {
		prompt += fmt.Sprintf("- %s: %s\n", contrib.Agent, contrib.Content)
	}
	prompt += "\nSynthesize the key insights and decide if we need another round. "
	prompt += "If the discussion has converged or the topic is well-explored, say 'CONCLUDE' at the start of your response."

	// Create span for synthesis
	var synthCtx context.Context
	var endSynthSpan func()
	if tracer != nil && !isNoOpTracer(tracer) {
		synthCtx, endSynthSpan = tracer.StartSpan(ctx, "facilitator_synthesis")
		defer endSynthSpan()
	} else {
		synthCtx = ctx
	}

	facilitatorWithTracer := *cs.facilitator
	if tracer != nil && !isNoOpTracer(tracer) {
		facilitatorWithTracer.tracer = tracer
	}

	events := facilitatorWithTracer.Run(synthCtx, prompt)
	
	var synthesis string
	for event := range events {
		if event.Type == EventTypeFinalOutput {
			if resp, ok := event.Data["response"].(string); ok {
				synthesis = resp
			}
		}
	}
	
	if synthesis == "" {
		return "", false, fmt.Errorf("facilitator failed to synthesize round %d", roundNum)
	}

	// Check if facilitator wants to conclude
	shouldContinue := true
	if len(synthesis) >= 8 && synthesis[:8] == "CONCLUDE" {
		shouldContinue = false
		// Strip the CONCLUDE marker from synthesis
		if len(synthesis) > 8 {
			synthesis = synthesis[9:]
		}
	}

	return synthesis, shouldContinue, nil
}

// generateFinalSynthesis creates the final response from all rounds.
func (cs *CollaborationSession) generateFinalSynthesis(
	ctx context.Context,
	topic string,
	rounds []CollaborationRound,
	tracer Tracer,
) (string, error) {
	prompt := fmt.Sprintf("Based on the following collaborative discussion about '%s', provide a final synthesized answer.\n\n", topic)
	
	for _, round := range rounds {
		prompt += fmt.Sprintf("Round %d:\n", round.Number)
		for _, contrib := range round.Contributions {
			prompt += fmt.Sprintf("- %s: %s\n", contrib.Agent, contrib.Content)
		}
		if round.Synthesis != "" {
			prompt += fmt.Sprintf("Synthesis: %s\n", round.Synthesis)
		}
		prompt += "\n"
	}
	
	prompt += "Provide a clear, comprehensive final answer that incorporates the best insights from all participants."

	// Create span for final synthesis
	var finalCtx context.Context
	var endFinalSpan func()
	if tracer != nil && !isNoOpTracer(tracer) {
		finalCtx, endFinalSpan = tracer.StartSpan(ctx, "final_synthesis")
		defer endFinalSpan()
	} else {
		finalCtx = ctx
	}

	facilitatorWithTracer := *cs.facilitator
	if tracer != nil && !isNoOpTracer(tracer) {
		facilitatorWithTracer.tracer = tracer
	}

	events := facilitatorWithTracer.Run(finalCtx, prompt)
	
	var finalResponse string
	for event := range events {
		if event.Type == EventTypeFinalOutput {
			if resp, ok := event.Data["response"].(string); ok {
				finalResponse = resp
			}
		}
	}
	
	if finalResponse == "" {
		return "", fmt.Errorf("facilitator failed to generate final synthesis")
	}
	
	return finalResponse, nil
}

// generateSummary creates a summary of the collaboration.
func (cs *CollaborationSession) generateSummary(result *CollaborationResult) string {
	totalContributions := 0
	for _, round := range result.Rounds {
		totalContributions += len(round.Contributions)
	}
	
	return fmt.Sprintf("Collaboration completed in %d round(s) with %d total contribution(s) from %d participant(s)",
		len(result.Rounds), totalContributions, len(result.Participants))
}

// getParticipantNames returns names of all participants.
func (cs *CollaborationSession) getParticipantNames() []string {
	names := make([]string, 0, len(cs.peers)+1)
	names = append(names, cs.facilitator.getAgentName())
	for i := range cs.peers {
		names = append(names, cs.getPeerName(i))
	}
	return names
}

// getPeerName returns a name for a peer agent.
func (cs *CollaborationSession) getPeerName(index int) string {
	if index >= len(cs.peers) {
		return fmt.Sprintf("peer_%d", index)
	}
	return cs.peers[index].getAgentName()
}

// AsTool converts a collaboration session into a Tool that can be registered with another agent.
// This enables collaborations to be triggered by the LLM through tool calling, with the topic
// provided dynamically at runtime.
//
// Example:
//
//	session := agentkit.NewCollaborationSession(facilitator, engineer, designer, product)
//	
//	coordinatorAgent.AddTool(session.AsTool(
//	    "design_collaboration",
//	    "Form a collaborative discussion with engineering, design, and product teams",
//	))
//
// The LLM can then decide when to collaborate and provide the topic:
//
//	coordinatorAgent.Run(ctx, "We need to decide on the authentication approach...")
//	// LLM calls: design_collaboration(topic: "How should we design the authentication API?")
func (cs *CollaborationSession) AsTool(name, description string, opts ...CollaborationOption) Tool {
	return NewTool(name).
		WithDescription(description).
		WithParameter("topic", String().Required().WithDescription("The topic or question for the collaborative discussion")).
		WithHandler(func(ctx context.Context, args map[string]any) (any, error) {
			topic, ok := args["topic"].(string)
			if !ok || topic == "" {
				return nil, ErrCollaborationTopicEmpty
			}

			// Execute the collaboration with provided options
			result, err := cs.Discuss(ctx, topic, opts...)
			if err != nil {
				return nil, err
			}

			// Return structured result
			return map[string]any{
				"final_response": result.FinalResponse,
				"summary":        result.Summary,
				"rounds":         len(result.Rounds),
				"participants":   result.Participants,
			}, nil
		}).
		Build()
}
