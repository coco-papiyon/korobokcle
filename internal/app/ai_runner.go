package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/agentworker"
	"github.com/coco-papiyon/korobokcle/internal/domain"
)

type AIRequest struct {
	Provider    domain.AIProvider
	Model       string
	System      string
	Prompt      string
	WorkingDir  string
	ExpectPatch bool
}

type AIResponse struct {
	ArtifactMarkdown string
	GitDiff          string
	RawOutput        string
}

type AIResponseParseError struct {
	Message   string
	RawOutput string
}

func (e *AIResponseParseError) Error() string {
	return e.Message
}

type AIRunner interface {
	Run(context.Context, AIRequest) (AIResponse, error)
}

type CLIAIRunner struct {
	logger   workflowLogger
	mu       sync.Mutex
	worker   agentworker.RequestWorker
	provider domain.AIProvider
	dir      string
}

func NewHTTPAIRunner(_ any, logger workflowLogger) *CLIAIRunner {
	return &CLIAIRunner{logger: logger}
}

func (r *CLIAIRunner) Start(ctx context.Context, provider domain.AIProvider, dir string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.worker != nil {
		return nil
	}
	if provider == domain.AIProviderGitHubCopilot {
		r.worker = agentworker.NewCopilot(agentworker.CopilotConfig{Dir: dir, StopTimeout: 5 * time.Second})
	} else {
		r.worker = agentworker.NewCodex(agentworker.CodexConfig{Dir: dir, Ephemeral: true, StopTimeout: 5 * time.Second})
	}
	r.provider = provider
	r.dir = dir
	if err := r.worker.Start(ctx); err != nil {
		r.worker = nil
		return fmt.Errorf("start %s worker: %w", provider, err)
	}
	if r.logger != nil {
		status := r.worker.GetStatus()
		r.logger.Debugf("AI worker started provider=%s pid=%d dir=%s", provider, status.PID, dir)
	}
	return nil
}

func (r *CLIAIRunner) Stop(ctx context.Context) error {
	r.mu.Lock()
	w := r.worker
	provider := r.provider
	r.worker = nil
	r.mu.Unlock()
	if w == nil {
		return nil
	}
	status := w.GetStatus()
	err := w.Stop(ctx)
	if r.logger != nil {
		r.logger.Debugf("AI worker stopped provider=%s pid=%d error=%v", provider, status.PID, err)
	}
	return err
}

func (r *CLIAIRunner) Run(ctx context.Context, req AIRequest) (AIResponse, error) {
	r.mu.Lock()
	w := r.worker
	provider := r.provider
	dir := r.dir
	r.mu.Unlock()
	if w == nil {
		return AIResponse{}, fmt.Errorf("AI worker is not started")
	}
	if provider != req.Provider {
		if err := r.Stop(ctx); err != nil {
			return AIResponse{}, fmt.Errorf("stop %s worker: %w", provider, err)
		}
		if err := r.Start(ctx, req.Provider, dir); err != nil {
			return AIResponse{}, err
		}
		r.mu.Lock()
		w = r.worker
		r.mu.Unlock()
	}
	prompt := strings.TrimSpace(req.System + "\n\n" + req.Prompt)
	out, err := w.SendPromptAt(ctx, prompt, req.WorkingDir, req.Model)
	if err != nil {
		return AIResponse{}, fmt.Errorf("%s prompt: %w", req.Provider, err)
	}
	if strings.TrimSpace(out) == "" {
		return AIResponse{}, &AIResponseParseError{Message: "AI response is empty", RawOutput: out}
	}
	return AIResponse{ArtifactMarkdown: strings.TrimSpace(out), RawOutput: out}, nil
}

func parseGitHubModelsResponseText(raw []byte) (string, error) {
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("decode GitHub Models response: %w", err)
	}
	if len(resp.Choices) == 0 || strings.TrimSpace(resp.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("GitHub Models response text is empty")
	}
	return resp.Choices[0].Message.Content, nil
}

func parseAIResponse(raw string, expectPatch bool) (AIResponse, error) {
	raw = strings.TrimSpace(stripLeadingNoise(raw))
	if !expectPatch {
		return AIResponse{ArtifactMarkdown: raw, RawOutput: raw}, nil
	}

	cleaned := strings.TrimSpace(stripCodeFence(raw, "json"))
	if extracted, ok := extractFirstJSONObject(cleaned); ok {
		cleaned = extracted
	}
	var parsed struct {
		ArtifactMarkdown string `json:"artifact_markdown"`
		GitDiff          string `json:"git_diff"`
	}
	if err := json.Unmarshal([]byte(cleaned), &parsed); err != nil {
		return AIResponse{}, fmt.Errorf("decode AI JSON response: %w", err)
	}
	if strings.TrimSpace(parsed.ArtifactMarkdown) == "" {
		return AIResponse{}, fmt.Errorf("AI response artifact_markdown is empty")
	}
	return AIResponse{
		ArtifactMarkdown: strings.TrimSpace(parsed.ArtifactMarkdown),
		GitDiff:          strings.TrimSpace(stripCodeFence(parsed.GitDiff, "diff")),
		RawOutput:        raw,
	}, nil
}

func stripCodeFence(raw string, language string) string {
	trimmed := strings.TrimSpace(raw)
	start := "```"
	if language != "" {
		start += language
	}
	if strings.HasPrefix(trimmed, start) {
		trimmed = strings.TrimPrefix(trimmed, start)
		trimmed = strings.TrimSpace(trimmed)
	}
	if strings.HasSuffix(trimmed, "```") {
		trimmed = strings.TrimSuffix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
	}
	return trimmed
}

func stripLeadingNoise(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "#") {
		return trimmed
	}
	if idx := strings.Index(trimmed, "\n{"); idx >= 0 {
		return strings.TrimSpace(trimmed[idx+1:])
	}
	if idx := strings.Index(trimmed, "\n#"); idx >= 0 {
		return strings.TrimSpace(trimmed[idx+1:])
	}
	var buf bytes.Buffer
	for _, line := range strings.Split(trimmed, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	return strings.TrimSpace(buf.String())
}

func extractFirstJSONObject(raw string) (string, bool) {
	start := strings.Index(raw, "{")
	if start < 0 {
		return "", false
	}
	depth := 0
	inString := false
	escaped := false
	for idx := start; idx < len(raw); idx++ {
		ch := raw[idx]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return strings.TrimSpace(raw[start : idx+1]), true
			}
		}
	}
	return "", false
}
