package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
)

type WrapperRequest struct {
	RequestID string `json:"requestId"`
	Prompt    string `json:"prompt"`
}

var wrapperRequestSeq uint64

func RunWrapper(ctx context.Context, input io.Reader, output io.Writer, sessionCfg SessionConfig) error {
	scanner := bufio.NewScanner(input)
	encoder := json.NewEncoder(output)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var request WrapperRequest
		if err := json.Unmarshal([]byte(line), &request); err != nil {
			if err := encoder.Encode(JSONLEvent{Type: "error", Error: fmt.Sprintf("decode request: %v", err)}); err != nil {
				return err
			}
			if err := encoder.Encode(JSONLEvent{Type: "end"}); err != nil {
				return err
			}
			continue
		}

		request = normalizeWrapperRequest(request)

		if strings.TrimSpace(request.Prompt) == "" {
			if err := encoder.Encode(JSONLEvent{Type: "error", RequestID: request.RequestID, Error: "prompt is required"}); err != nil {
				return err
			}
			if err := encoder.Encode(JSONLEvent{Type: "end", RequestID: request.RequestID}); err != nil {
				return err
			}
			continue
		}

		response, err := runWrapperRequest(ctx, sessionCfg, request.Prompt)
		if err != nil {
			if err := encoder.Encode(JSONLEvent{Type: "error", RequestID: request.RequestID, Error: err.Error()}); err != nil {
				return err
			}
			if err := encoder.Encode(JSONLEvent{Type: "end", RequestID: request.RequestID}); err != nil {
				return err
			}
			continue
		}

		if strings.TrimSpace(response.Stdout) != "" {
			if err := encoder.Encode(JSONLEvent{
				Type:      "result",
				RequestID: request.RequestID,
				Stream:    "stdout",
				Data:      response.Stdout,
			}); err != nil {
				return err
			}
		}
		if strings.TrimSpace(response.Stderr) != "" {
			if err := encoder.Encode(JSONLEvent{
				Type:      "message",
				RequestID: request.RequestID,
				Stream:    "stderr",
				Data:      response.Stderr,
			}); err != nil {
				return err
			}
		}
		if err := encoder.Encode(JSONLEvent{Type: "end", RequestID: request.RequestID}); err != nil {
			return err
		}
	}

	return scanner.Err()
}

func runWrapperRequest(ctx context.Context, sessionCfg SessionConfig, prompt string) (Response, error) {
	if usesExecCommand(sessionCfg.Args) {
		return runExecWrapperRequest(ctx, sessionCfg, prompt)
	}

	session, err := StartSession(ctx, sessionCfg)
	if err != nil {
		return Response{}, err
	}
	defer session.Close()

	return session.Send(ctx, buildChildPrompt(prompt, sessionCfg.EndMarker))
}

func runExecWrapperRequest(ctx context.Context, sessionCfg SessionConfig, prompt string) (Response, error) {
	reqCfg := sessionCfg
	reqCfg.Args = append(append([]string(nil), sessionCfg.Args...), prompt)
	reqCfg.EndMarker = ""

	session, err := StartSession(ctx, reqCfg)
	if err != nil {
		return Response{}, err
	}
	defer session.Close()

	if !reqCfg.UsePTY {
		if err := session.closeInput(); err != nil {
			return Response{}, err
		}
	}

	return session.collectUntilDone(ctx)
}

func normalizeWrapperRequest(request WrapperRequest) WrapperRequest {
	request.RequestID = strings.TrimSpace(request.RequestID)
	request.Prompt = strings.TrimSpace(request.Prompt)
	if request.RequestID == "" {
		request.RequestID = fmt.Sprintf("req-%d", atomic.AddUint64(&wrapperRequestSeq, 1))
	}
	return request
}

func usesExecCommand(args []string) bool {
	return len(args) > 0 && strings.EqualFold(strings.TrimSpace(args[0]), "exec")
}

func buildChildPrompt(prompt, endMarker string) string {
	if strings.TrimSpace(endMarker) == "" {
		return prompt
	}

	return strings.TrimRight(prompt, "\r\n") + "\n\nIMPORTANT:\nAfter completing your response, output the following marker on its own line exactly once:\n" + endMarker
}
