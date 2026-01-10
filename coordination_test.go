package agentkit

import (
	"context"
	"testing"
)

func TestHandoff_Basic(t *testing.T) {
	// Create a mock delegated agent
	delegateAgent := &Agent{
		model:         "test-model",
		maxIterations: 3,
		eventBuffer:   10,
		tracer:        &NoOpTracer{},
	}

	// Create a delegating agent
	delegatingAgent := &Agent{
		model:         "test-model",
		maxIterations: 3,
		eventBuffer:   10,
		tracer:        &NoOpTracer{},
	}

	ctx := context.Background()
	
	// Test that Handoff validates inputs
	t.Run("NilAgent", func(t *testing.T) {
		_, err := delegatingAgent.Handoff(ctx, nil, "task")
		if err != ErrHandoffAgentNil {
			t.Errorf("Expected ErrHandoffAgentNil, got %v", err)
		}
	})

	t.Run("EmptyTask", func(t *testing.T) {
		_, err := delegatingAgent.Handoff(ctx, delegateAgent, "")
		if err != ErrHandoffTaskEmpty {
			t.Errorf("Expected ErrHandoffTaskEmpty, got %v", err)
		}
	})
}

func TestHandoff_Options(t *testing.T) {
	t.Run("WithFullContext", func(t *testing.T) {
		opts := handoffOptions{}
		WithFullContext(true)(&opts)
		
		if !opts.fullContext {
			t.Error("Expected fullContext to be true")
		}
	})

	t.Run("WithMaxTurns", func(t *testing.T) {
		opts := handoffOptions{}
		WithMaxTurns(5)(&opts)
		
		if opts.maxTurns != 5 {
			t.Errorf("Expected maxTurns to be 5, got %d", opts.maxTurns)
		}
	})

	t.Run("WithContext", func(t *testing.T) {
		opts := handoffOptions{}
		ctx := HandoffContext{
			Background: "test background",
			Metadata:   map[string]any{"key": "value"},
		}
		WithContext(ctx)(&opts)
		
		if opts.context.Background != "test background" {
			t.Error("Expected background to be set")
		}
		if opts.context.Metadata["key"] != "value" {
			t.Error("Expected metadata to be set")
		}
	})
}

func TestHandoff_AsHandoffTool(t *testing.T) {
	agent := &Agent{
		model:         "test-model",
		maxIterations: 3,
		eventBuffer:   10,
		tracer:        &NoOpTracer{},
	}

	tool := agent.AsHandoffTool("test_tool", "Test tool description")
	
	// Tool is created - we can't directly test private fields,
	// but we can verify it's not nil and has the expected structure
	if tool.name != "test_tool" {
		t.Errorf("Expected tool name 'test_tool', got '%s'", tool.name)
	}
	
	if tool.description != "Test tool description" {
		t.Errorf("Expected tool description 'Test tool description', got '%s'", tool.description)
	}
}

func TestHandoff_GenerateSummary(t *testing.T) {
	t.Run("EmptyTrace", func(t *testing.T) {
		summary := generateHandoffSummary(nil)
		if summary != "" {
			t.Errorf("Expected empty summary, got '%s'", summary)
		}
	})

	t.Run("WithToolCalls", func(t *testing.T) {
		trace := []HandoffTraceItem{
			{Type: "thought", Content: "thinking"},
			{Type: "tool_call", Content: "calling tool"},
			{Type: "tool_result", Content: "result"},
			{Type: "response", Content: "final response"},
		}
		
		summary := generateHandoffSummary(trace)
		if summary == "" {
			t.Error("Expected non-empty summary")
		}
		
		// Should mention tool calls
		// Implementation may vary, so just check it's not empty
	})

	t.Run("WithoutToolCalls", func(t *testing.T) {
		trace := []HandoffTraceItem{
			{Type: "thought", Content: "thinking"},
			{Type: "response", Content: "final response"},
		}
		
		summary := generateHandoffSummary(trace)
		if summary == "" {
			t.Error("Expected non-empty summary")
		}
	})
}

func TestHandoffConfiguration_AsTool(t *testing.T) {
	coordinatorAgent := &Agent{
		model:         "test-model",
		maxIterations: 3,
		eventBuffer:   10,
		tracer:        &NoOpTracer{},
	}
	
	delegateAgent := &Agent{
		model:         "test-model",
		maxIterations: 3,
		eventBuffer:   10,
		tracer:        &NoOpTracer{},
	}

	config := NewHandoffConfiguration(coordinatorAgent, delegateAgent, WithFullContext(true))
	tool := config.AsTool("research", "Delegate research tasks")
	
	if tool.name != "research" {
		t.Errorf("Expected tool name 'research', got '%s'", tool.name)
	}
	
	if tool.description != "Delegate research tasks" {
		t.Errorf("Expected tool description 'Delegate research tasks', got '%s'", tool.description)
	}
}

func TestHandoffConfiguration_AsTool_Execute(t *testing.T) {
	coordinatorAgent := &Agent{
		model:         "test-model",
		maxIterations: 3,
		eventBuffer:   10,
		tracer:        &NoOpTracer{},
	}
	
	delegateAgent := &Agent{
		model:         "test-model",
		maxIterations: 3,
		eventBuffer:   10,
		tracer:        &NoOpTracer{},
	}

	config := NewHandoffConfiguration(coordinatorAgent, delegateAgent)
	tool := config.AsTool("research", "Delegate research tasks")
	
	ctx := context.Background()
	
	t.Run("MissingTask", func(t *testing.T) {
		_, err := tool.handler(ctx, map[string]any{})
		if err != ErrHandoffTaskEmpty {
			t.Errorf("Expected ErrHandoffTaskEmpty, got %v", err)
		}
	})
	
	t.Run("EmptyTask", func(t *testing.T) {
		_, err := tool.handler(ctx, map[string]any{"task": ""})
		if err != ErrHandoffTaskEmpty {
			t.Errorf("Expected ErrHandoffTaskEmpty, got %v", err)
		}
	})
}

func TestCollaboration_Validation(t *testing.T) {
	facilitator := &Agent{
		model:         "test-model",
		maxIterations: 3,
		eventBuffer:   10,
		tracer:        &NoOpTracer{},
	}

	peer1 := &Agent{
		model:         "test-model",
		maxIterations: 3,
		eventBuffer:   10,
		tracer:        &NoOpTracer{},
	}

	ctx := context.Background()

	t.Run("NoFacilitator", func(t *testing.T) {
		session := &CollaborationSession{
			facilitator: nil,
			peers:       []*Agent{peer1},
		}
		
		_, err := session.Discuss(ctx, "topic")
		if err != ErrCollaborationNoFacilitator {
			t.Errorf("Expected ErrCollaborationNoFacilitator, got %v", err)
		}
	})

	t.Run("NoPeers", func(t *testing.T) {
		session := &CollaborationSession{
			facilitator: facilitator,
			peers:       []*Agent{},
		}
		
		_, err := session.Discuss(ctx, "topic")
		if err != ErrCollaborationNoPeers {
			t.Errorf("Expected ErrCollaborationNoPeers, got %v", err)
		}
	})

	t.Run("EmptyTopic", func(t *testing.T) {
		session := NewCollaborationSession(facilitator, peer1)
		
		_, err := session.Discuss(ctx, "")
		if err != ErrCollaborationTopicEmpty {
			t.Errorf("Expected ErrCollaborationTopicEmpty, got %v", err)
		}
	})
}

func TestCollaboration_Options(t *testing.T) {
	t.Run("WithMaxRounds", func(t *testing.T) {
		opts := collaborationOptions{}
		WithMaxRounds(5)(&opts)
		
		if opts.maxRounds != 5 {
			t.Errorf("Expected maxRounds to be 5, got %d", opts.maxRounds)
		}
	})

	t.Run("WithCaptureHistory", func(t *testing.T) {
		opts := collaborationOptions{}
		WithCaptureHistory(true)(&opts)
		
		if !opts.captureHistory {
			t.Error("Expected captureHistory to be true")
		}
	})
}

func TestCollaboration_NewSession(t *testing.T) {
	facilitator := &Agent{
		model:         "test-model",
		maxIterations: 3,
		eventBuffer:   10,
		tracer:        &NoOpTracer{},
	}

	peer1 := &Agent{
		model:         "test-model-1",
		maxIterations: 3,
		eventBuffer:   10,
		tracer:        &NoOpTracer{},
	}

	peer2 := &Agent{
		model:         "test-model-2",
		maxIterations: 3,
		eventBuffer:   10,
		tracer:        &NoOpTracer{},
	}

	session := NewCollaborationSession(facilitator, peer1, peer2)
	
	if session.facilitator != facilitator {
		t.Error("Expected facilitator to be set")
	}
	
	if len(session.peers) != 2 {
		t.Errorf("Expected 2 peers, got %d", len(session.peers))
	}
	
	// Check defaults
	if session.options.maxRounds != 3 {
		t.Errorf("Expected default maxRounds to be 3, got %d", session.options.maxRounds)
	}
	
	if !session.options.captureHistory {
		t.Error("Expected captureHistory to be true by default")
	}
}

func TestCollaboration_Configure(t *testing.T) {
	facilitator := &Agent{
		model:         "test-model",
		maxIterations: 3,
		eventBuffer:   10,
		tracer:        &NoOpTracer{},
	}

	peer1 := &Agent{
		model:         "test-model-1",
		maxIterations: 3,
		eventBuffer:   10,
		tracer:        &NoOpTracer{},
	}

	session := NewCollaborationSession(facilitator, peer1).Configure(
		WithMaxRounds(10),
		WithCaptureHistory(false),
	)
	
	if session.options.maxRounds != 10 {
		t.Errorf("Expected maxRounds to be 10, got %d", session.options.maxRounds)
	}
	
	if session.options.captureHistory {
		t.Error("Expected captureHistory to be false")
	}
}

func TestCollaboration_GenerateSummary(t *testing.T) {
	session := &CollaborationSession{}
	
	result := &CollaborationResult{
		Rounds: []CollaborationRound{
			{
				Number: 1,
				Contributions: []CollaborationContribution{
					{Agent: "agent1", Content: "content1"},
					{Agent: "agent2", Content: "content2"},
				},
			},
			{
				Number: 2,
				Contributions: []CollaborationContribution{
					{Agent: "agent1", Content: "content3"},
				},
			},
		},
		Participants: []string{"agent1", "agent2", "facilitator"},
	}
	
	summary := session.generateSummary(result)
	
	if summary == "" {
		t.Error("Expected non-empty summary")
	}
	
	// Should mention rounds, contributions, and participants
	// Implementation may vary, but should contain key numbers
}

func TestHandoff_ExecuteHandoff(t *testing.T) {
	// Create a mock LLM provider that returns predictable results
	mockLLM := NewMockLLM().WithFinalResponse("Test handoff response")

	agent, err := New(Config{
		Model:         "test-model",
		MaxIterations: 3,
		LLMProvider:   mockLLM,
		Temperature:   0.5,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	ctx := context.Background()
	
	opts := handoffOptions{
		fullContext: true,
		maxTurns:    3,
	}
	
	response, summary, trace, err := executeHandoff(ctx, agent, "test task", opts)
	if err != nil {
		t.Fatalf("executeHandoff failed: %v", err)
	}
	
	if response == "" {
		t.Error("Expected non-empty response")
	}
	
	if summary == "" {
		t.Error("Expected non-empty summary")
	}
	
	if len(trace) == 0 {
		t.Error("Expected trace items when fullContext is true")
	}
}

func TestHandoff_ExecuteHandoff_WithoutTrace(t *testing.T) {
	mockLLM := NewMockLLM().WithFinalResponse("Response without trace")

	agent, err := New(Config{
		Model:         "test-model",
		MaxIterations: 2,
		LLMProvider:   mockLLM,
		Temperature:   0.5,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	ctx := context.Background()
	opts := handoffOptions{
		fullContext: false,
		maxTurns:    2,
	}
	
	response, _, trace, err := executeHandoff(ctx, agent, "task", opts)
	if err != nil {
		t.Fatalf("executeHandoff failed: %v", err)
	}
	
	if response == "" {
		t.Error("Expected response")
	}
	
	// Trace should be empty when fullContext is false
	if len(trace) > 0 {
		t.Error("Expected empty trace when fullContext is false")
	}
}

func TestHandoff_GetAgentName(t *testing.T) {
	agent := &Agent{
		model: "test-model-123",
	}
	
	name := agent.getAgentName()
	if name != "test-model-123" {
		t.Errorf("Expected 'test-model-123', got '%s'", name)
	}
}

func TestCollaboration_BuildPeerPrompt(t *testing.T) {
	session := &CollaborationSession{}
	
	t.Run("WithHistory", func(t *testing.T) {
		history := []string{
			"Topic: Test topic",
			"Agent1: First contribution",
			"Agent2: Second contribution",
		}
		
		prompt := session.buildPeerPrompt(2, history)
		
		if prompt == "" {
			t.Error("Expected non-empty prompt")
		}
		
		// Should contain round number
		// Should contain history
		// Implementation details may vary
	})
	
	t.Run("WithoutHistory", func(t *testing.T) {
		prompt := session.buildPeerPrompt(1, []string{})
		
		if prompt == "" {
			t.Error("Expected non-empty prompt")
		}
	})
}

func TestCollaboration_GetParticipantNames(t *testing.T) {
	facilitator := &Agent{model: "facilitator-model"}
	peer1 := &Agent{model: "peer1-model"}
	peer2 := &Agent{model: "peer2-model"}
	
	session := NewCollaborationSession(facilitator, peer1, peer2)
	names := session.getParticipantNames()
	
	if len(names) != 3 {
		t.Errorf("Expected 3 names, got %d", len(names))
	}
}

func TestCollaboration_GetPeerName(t *testing.T) {
	peer1 := &Agent{model: "peer1-model"}
	peer2 := &Agent{model: "peer2-model"}
	
	session := NewCollaborationSession(&Agent{model: "facilitator"}, peer1, peer2)
	
	name1 := session.getPeerName(0)
	if name1 == "" {
		t.Error("Expected non-empty peer name")
	}
	
	name2 := session.getPeerName(1)
	if name2 == "" {
		t.Error("Expected non-empty peer name")
	}
	
	// Test out of bounds
	nameOOB := session.getPeerName(10)
	if nameOOB == "" {
		t.Error("Expected non-empty name even for out of bounds")
	}
}

func TestHandoff_WithBackground(t *testing.T) {
	mockLLM := NewMockLLM().WithFinalResponse("Response with background")

	delegatingAgent, err := New(Config{
		Model:         "test-model",
		MaxIterations: 2,
		LLMProvider:   mockLLM,
		Temperature:   0.5,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	delegateAgent, err := New(Config{
		Model:         "test-model",
		MaxIterations: 2,
		LLMProvider:   NewMockLLM().WithFinalResponse("Delegate response"),
		Temperature:   0.5,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	ctx := context.Background()
	
	result, err := delegatingAgent.Handoff(
		ctx,
		delegateAgent,
		"Test task",
		WithContext(HandoffContext{
			Background: "This is background context",
			Metadata:   map[string]any{"key": "value"},
		}),
	)
	
	if err != nil {
		t.Fatalf("Handoff with context failed: %v", err)
	}
	
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
}

func TestHandoff_AsHandoffTool_Execute(t *testing.T) {
	agent, err := New(Config{
		Model:         "test-model",
		MaxIterations: 2,
		LLMProvider:   NewMockLLM().WithFinalResponse("Tool execution response").WithFinalResponse("Tool execution response 2").WithFinalResponse("Tool execution response 3"),
		Temperature:   0.5,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	tool := agent.AsHandoffTool("test_tool", "Test description")
	
	ctx := context.Background()
	
	// Test with valid args
	args := map[string]any{
		"task": "test task",
	}
	
	result, err := tool.handler(ctx, args)
	if err != nil {
		t.Fatalf("Tool handler failed: %v", err)
	}
	
	if result == nil {
		t.Error("Expected non-nil result")
	}
	
	// Test with missing task
	emptyArgs := map[string]any{}
	_, err = tool.handler(ctx, emptyArgs)
	if err != ErrHandoffTaskEmpty {
		t.Errorf("Expected ErrHandoffTaskEmpty, got %v", err)
	}
	
	// Test with background
	argsWithBg := map[string]any{
		"task":       "test task",
		"background": "background info",
	}
	
	result, err = tool.handler(ctx, argsWithBg)
	if err != nil {
		t.Fatalf("Tool handler with background failed: %v", err)
	}
	
	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestCollaboration_AsTool(t *testing.T) {
	facilitator, _ := New(Config{
		Model:         "test-model",
		MaxIterations: 2,
		LLMProvider:   NewMockLLM().WithFinalResponse("Facilitator response"),
		Temperature:   0.5,
	})

	peer1, _ := New(Config{
		Model:         "test-model",
		MaxIterations: 2,
		LLMProvider:   NewMockLLM().WithFinalResponse("Peer 1 response"),
		Temperature:   0.5,
	})

	session := NewCollaborationSession(facilitator, peer1)
	
	tool := session.AsTool(
		"team_collaboration",
		"Form a collaborative discussion with the team",
	)
	
	if tool.name != "team_collaboration" {
		t.Errorf("Expected tool name 'team_collaboration', got '%s'", tool.name)
	}
	
	if tool.description != "Form a collaborative discussion with the team" {
		t.Errorf("Expected correct description, got '%s'", tool.description)
	}
}

func TestCollaboration_AsTool_Execute(t *testing.T) {
	// Create agents with multiple mock responses
	facilitator, _ := New(Config{
		Model:         "test-model",
		MaxIterations: 2,
		LLMProvider:   NewMockLLM().WithFinalResponse("Facilitator synthesis").WithFinalResponse("Final synthesis"),
		Temperature:   0.5,
	})

	peer1, _ := New(Config{
		Model:         "test-model",
		MaxIterations: 2,
		LLMProvider:   NewMockLLM().WithFinalResponse("Peer 1 contribution"),
		Temperature:   0.5,
	})

	session := NewCollaborationSession(facilitator, peer1).Configure(
		WithMaxRounds(1), // Keep it simple for testing
	)
	
	tool := session.AsTool(
		"team_collaboration",
		"Form a collaborative discussion",
	)
	
	ctx := context.Background()
	
	// Test with valid topic
	args := map[string]any{
		"topic": "Should we use microservices?",
	}
	
	result, err := tool.handler(ctx, args)
	if err != nil {
		t.Fatalf("Tool handler failed: %v", err)
	}
	
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	
	// Check result structure
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatal("Expected result to be a map")
	}
	
	if _, ok := resultMap["final_response"]; !ok {
		t.Error("Expected final_response in result")
	}
	
	if _, ok := resultMap["summary"]; !ok {
		t.Error("Expected summary in result")
	}
	
	if _, ok := resultMap["rounds"]; !ok {
		t.Error("Expected rounds in result")
	}
	
	if _, ok := resultMap["participants"]; !ok {
		t.Error("Expected participants in result")
	}
	
	// Test with missing topic
	emptyArgs := map[string]any{}
	_, err = tool.handler(ctx, emptyArgs)
	if err != ErrCollaborationTopicEmpty {
		t.Errorf("Expected ErrCollaborationTopicEmpty, got %v", err)
	}
}
