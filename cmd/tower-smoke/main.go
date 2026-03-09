// tower-smoke starts a Tower daemon with hook logging so you can validate
// the hook integration against a real Claude Code session.
//
// Usage:
//
//	Terminal 1:  go run cmd/tower-smoke/main.go [project-dir]
//	Terminal 2:  cd [project-dir] && claude
//
// Every hook event Claude sends is logged to stdout and saved to
// tower-smoke-capture.jsonl in the working directory. Ctrl+C to stop.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"tower/internal/adapters/claude"
	"tower/internal/daemon"
)

func main() {
	projectDir, _ := os.Getwd()
	if len(os.Args) > 1 {
		projectDir = os.Args[1]
	}
	abs, err := filepath.Abs(projectDir)
	if err != nil {
		log.Fatalf("resolve path: %v", err)
	}
	projectDir = abs

	// Generate a token.
	token := fmt.Sprintf("smoke-%d", time.Now().UnixNano())
	sessionID := fmt.Sprintf("SMOKE-%d", time.Now().Unix())

	// Open capture log.
	captureFile, err := os.Create("tower-smoke-capture.jsonl")
	if err != nil {
		log.Fatalf("create capture file: %v", err)
	}
	defer captureFile.Close()

	// Create the real daemon server (for risk classification and response logic).
	srv := daemon.NewServer(token)
	srv.RegisterSession(sessionID)

	// Wrap with logging middleware.
	handler := &loggingHandler{
		inner:      srv,
		token:      token,
		sessionID:  sessionID,
		captureLog: captureFile,
	}

	// Listen on ephemeral port.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	// Write hook config.
	cfg := claude.GenerateHookConfig(sessionID, port)
	hookPath, err := claude.WriteHookConfig(projectDir, cfg)
	if err != nil {
		log.Fatalf("write hook config: %v", err)
	}

	fmt.Println("=== Tower Smoke Test ===")
	fmt.Println()
	fmt.Printf("  daemon:      http://localhost:%d\n", port)
	fmt.Printf("  token:       %s\n", token)
	fmt.Printf("  session:     %s\n", sessionID)
	fmt.Printf("  hook config: %s\n", hookPath)
	fmt.Printf("  capture log: tower-smoke-capture.jsonl\n")
	fmt.Printf("  project:     %s\n", projectDir)
	fmt.Println()
	fmt.Println("In another terminal, run:")
	fmt.Println()
	fmt.Printf("  export TOWER_HOOK_TOKEN=\"%s\"\n", token)
	fmt.Printf("  cd \"%s\" && claude\n", projectDir)
	fmt.Println()
	fmt.Println("Hook events will appear below. Ctrl+C to stop.")
	fmt.Println(strings.Repeat("─", 70))

	// Start HTTP server.
	httpSrv := &http.Server{Handler: handler}
	go httpSrv.Serve(listener)

	// Wait for Ctrl+C.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	<-ctx.Done()

	fmt.Println()
	fmt.Println(strings.Repeat("─", 70))

	httpSrv.Shutdown(context.Background())

	// Clean up hook config.
	os.Remove(hookPath)
	claudeDir := filepath.Dir(hookPath)
	os.Remove(claudeDir) // only succeeds if empty

	// Print summary.
	fmt.Println("Stopped. Cleaned up hook config.")
	fmt.Printf("Capture log: tower-smoke-capture.jsonl (%d events)\n", handler.count())

	events := srv.ReceivedEvents(sessionID)
	counts := make(map[string]int)
	for _, e := range events {
		counts[e.HookEventName]++
	}
	fmt.Println()
	fmt.Println("Event summary:")
	for name, c := range counts {
		fmt.Printf("  %-25s %d\n", name, c)
	}
}

type loggingHandler struct {
	inner      http.Handler
	token      string
	sessionID  string
	captureLog *os.File
	mu         sync.Mutex
	n          int
}

func (h *loggingHandler) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.n
}

func (h *loggingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Skip health checks from noise.
	if r.URL.Path == "/healthz" {
		h.inner.ServeHTTP(w, r)
		return
	}

	// Read and buffer the body so we can log it AND pass it to the real handler.
	body, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(strings.NewReader(string(body)))

	// Parse for display.
	var parsed map[string]any
	json.Unmarshal(body, &parsed)

	eventName, _ := parsed["hook_event_name"].(string)
	toolName, _ := parsed["tool_name"].(string)

	// Classify risk.
	var toolInput map[string]any
	if ti, ok := parsed["tool_input"].(map[string]any); ok {
		toolInput = ti
	}
	risk := daemon.ClassifyRisk(toolName, toolInput)

	// Log to stdout.
	ts := time.Now().Format("15:04:05.000")
	pathParts := strings.Split(r.URL.Path, "/")
	eventSlug := ""
	if len(pathParts) >= 4 {
		eventSlug = pathParts[3]
	}

	h.mu.Lock()
	h.n++
	n := h.n

	if toolName != "" {
		fmt.Printf("#%-4d [%s] %-22s tool=%-12s risk=%s\n", n, ts, eventName, toolName, risk)
		if cmd, ok := toolInput["command"].(string); ok {
			if len(cmd) > 90 {
				cmd = cmd[:87] + "..."
			}
			fmt.Printf("       cmd: %s\n", cmd)
		}
		if fp, ok := toolInput["file_path"].(string); ok {
			fmt.Printf("       path: %s\n", fp)
		}
		if pattern, ok := toolInput["pattern"].(string); ok {
			fmt.Printf("       pattern: %s\n", pattern)
		}
	} else {
		fmt.Printf("#%-4d [%s] %-22s  (%s)\n", n, ts, eventName, eventSlug)
	}

	// Write to capture log.
	capture := map[string]any{
		"n":          n,
		"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		"method":     r.Method,
		"path":       r.URL.Path,
		"event_slug": eventSlug,
		"body":       parsed,
		"risk_class": string(risk),
	}
	line, _ := json.Marshal(capture)
	fmt.Fprintf(h.captureLog, "%s\n", line)

	h.mu.Unlock()

	// Wrap response writer to log the response.
	rec := &responseRecorder{ResponseWriter: w}
	h.inner.ServeHTTP(rec, r)

	// Log response for sync events.
	if rec.body.Len() > 0 {
		var respParsed map[string]any
		json.Unmarshal(rec.body.Bytes(), &respParsed)
		if output, ok := respParsed["hookSpecificOutput"].(map[string]any); ok {
			if decision, ok := output["permissionDecision"].(string); ok {
				fmt.Printf("       >>> auto-approve: %s\n", decision)
			}
			if dec, ok := output["decision"].(map[string]any); ok {
				fmt.Printf("       >>> decision: %v\n", dec["behavior"])
			}
		}
	}
}

type responseRecorder struct {
	http.ResponseWriter
	body   bytes.Buffer
	status int
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
