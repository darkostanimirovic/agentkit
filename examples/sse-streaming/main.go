package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/darkostanimirovic/agentkit"
)

func main() {
	// Get API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	// Create a simple agent
	agent, err := agentkit.NewAgent(agentkit.Config{
		APIKey:          apiKey,
		Model:           "gpt-4o-mini",
		MaxIterations:   5,
		StreamResponses: true,
		SystemPrompt: func(ctx context.Context) string {
			return "You are a helpful assistant that provides clear, concise answers."
		},
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Register some example tools
	agent.RegisterTool(agentkit.NewTool(
		"get_weather",
		"Get current weather for a location",
		func(ctx context.Context, location string) (string, error) {
			return fmt.Sprintf("The weather in %s is sunny, 22°C", location), nil
		},
	))

	agent.RegisterTool(agentkit.NewTool(
		"search_web",
		"Search the web for information",
		func(ctx context.Context, query string) (string, error) {
			return fmt.Sprintf("Search results for '%s': [Mock results]", query), nil
		},
	))

	// Create HTTP server with SSE endpoint
	http.HandleFunc("/api/agent/stream", func(w http.ResponseWriter, r *http.Request) {
		streamAgentEvents(w, r, agent)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, htmlPage)
	})

	port := ":8080"
	fmt.Printf("🚀 SSE Streaming Example running on http://localhost%s\n", port)
	fmt.Println("📖 Open http://localhost:8080 in your browser to see it in action")
	log.Fatal(http.ListenAndServe(port, nil))
}

func streamAgentEvents(w http.ResponseWriter, r *http.Request, agent *agentkit.Agent) {
	// Get user message from query parameter
	userMessage := r.URL.Query().Get("message")
	if userMessage == "" {
		userMessage = "What's the weather like in San Francisco?"
	}

	// Setup SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Run agent and stream events
	ctx := r.Context()
	eventChan := agent.Run(ctx, userMessage)

	for event := range eventChan {
		// Convert event to JSON
		eventJSON, err := json.Marshal(event)
		if err != nil {
			continue
		}

		// Write SSE event
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, eventJSON)
		flusher.Flush()
	}
}

const htmlPage = `<!DOCTYPE html>
<html>
<head>
    <title>AgentKit SSE Streaming</title>
    <style>
        body { font-family: system-ui; max-width: 900px; margin: 50px auto; padding: 20px; }
        .event-log { background: #000; color: #0f0; padding: 15px; font-family: monospace; 
                     height: 500px; overflow-y: auto; }
        .event { margin: 5px 0; padding: 5px; border-left: 3px solid #0f0; padding-left: 10px; }
        .event-type { color: #ff0; font-weight: bold; margin-right: 10px; }
    </style>
</head>
<body>
    <h1>🤖 AgentKit SSE Streaming</h1>
    <input type="text" id="msg" value="What's the weather?" style="width: 80%; padding: 10px;">
    <button onclick="start()" style="padding: 10px 20px;">Send</button>
    <div class="event-log" id="log"></div>

    <script>
        let es;
        function start() {
            if (es) es.close();
            document.getElementById('log').innerHTML = '';
            const msg = document.getElementById('msg').value;
            es = new EventSource('/api/agent/stream?message=' + encodeURIComponent(msg));
            
            const types = ['agent.start', 'agent.complete', 'thinking_chunk', 'action_detected', 
                          'action_result', 'handoff.start', 'handoff.complete', 
                          'collaboration.agent.contribution', 'final_output', 'error'];
            
            types.forEach(type => {
                es.addEventListener(type, e => {
                    const ev = JSON.parse(e.data);
                    const div = document.createElement('div');
                    div.className = 'event';
                    div.innerHTML = '<span class="event-type">' + ev.type + '</span>' + 
                                    JSON.stringify(ev.data);
                    document.getElementById('log').appendChild(div);
                    document.getElementById('log').scrollTop = 999999;
                });
            });
            
            es.onerror = () => { es.close(); };
        }
        window.onload = start;
    </script>
</body>
</html>`
