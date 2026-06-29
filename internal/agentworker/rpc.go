package agentworker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *rpcError) Error() string {
	if len(e.Data) == 0 || string(e.Data) == "null" {
		return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("rpc error %d: %s (data: %s)", e.Code, e.Message, e.Data)
}

type rpcProcess struct {
	cmd            *exec.Cmd
	stdin          io.WriteCloser
	nextID         atomic.Uint64
	writeMu        sync.Mutex
	waitMu         sync.Mutex
	waiters        map[string]chan rpcMessage
	notices        chan rpcMessage
	done           chan struct{}
	doneOnce       sync.Once
	waitErrMu      sync.RWMutex
	waitErr        error
	serverResponse func(string) any
	includeJSONRPC bool
	outputMu       sync.RWMutex
	stdoutWriter   io.Writer
	stderrWriter   io.Writer
}

func startRPC(ctx context.Context, command string, args, env []string, dir string) (*rpcProcess, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &rpcProcess{
		cmd:            cmd,
		stdin:          stdin,
		waiters:        make(map[string]chan rpcMessage),
		notices:        make(chan rpcMessage, 1024),
		done:           make(chan struct{}),
		includeJSONRPC: true,
	}
	go p.read(stdout)
	go p.readRaw(stderr, true)
	go func() {
		err := cmd.Wait()
		p.waitErrMu.Lock()
		p.waitErr = err
		p.waitErrMu.Unlock()
		p.doneOnce.Do(func() { close(p.done) })
	}()
	return p, nil
}

func (p *rpcProcess) read(r io.Reader) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		p.writeRaw(scanner.Bytes(), false)
		var msg rpcMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		if len(msg.ID) > 0 && msg.Method == "" {
			p.waitMu.Lock()
			waiter := p.waiters[string(msg.ID)]
			p.waitMu.Unlock()
			if waiter != nil {
				waiter <- msg
			}
			continue
		}
		if len(msg.ID) > 0 && msg.Method != "" {
			response := any(map[string]any{"outcome": map[string]any{"outcome": "cancelled"}})
			if p.serverResponse != nil {
				response = p.serverResponse(msg.Method)
			}
			_ = p.respond(msg.ID, response)
			continue
		}
		select {
		case p.notices <- msg:
		case <-p.done:
			return
		}
	}
	if err := scanner.Err(); err != nil {
		p.waitErrMu.Lock()
		p.waitErr = err
		p.waitErrMu.Unlock()
	}
}

func (p *rpcProcess) readRaw(r io.Reader, stderr bool) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		p.writeRaw(scanner.Bytes(), stderr)
	}
	if err := scanner.Err(); err != nil {
		p.waitErrMu.Lock()
		p.waitErr = err
		p.waitErrMu.Unlock()
	}
}

func (p *rpcProcess) setOutputWriters(stdout, stderr io.Writer) {
	p.outputMu.Lock()
	defer p.outputMu.Unlock()
	p.stdoutWriter = stdout
	p.stderrWriter = stderr
}

func (p *rpcProcess) writeRaw(line []byte, stderr bool) {
	p.outputMu.RLock()
	writer := p.stdoutWriter
	if stderr {
		writer = p.stderrWriter
	}
	p.outputMu.RUnlock()
	if writer == nil {
		return
	}
	_, _ = writer.Write(append(append([]byte(nil), line...), '\n'))
}

func (p *rpcProcess) call(ctx context.Context, method string, params, result any) error {
	id := p.nextID.Add(1)
	key := fmt.Sprintf("%d", id)
	waiter := make(chan rpcMessage, 1)
	p.waitMu.Lock()
	p.waiters[key] = waiter
	p.waitMu.Unlock()
	defer func() {
		p.waitMu.Lock()
		delete(p.waiters, key)
		p.waitMu.Unlock()
	}()
	request := map[string]any{"id": id, "method": method, "params": params}
	if p.includeJSONRPC {
		request["jsonrpc"] = "2.0"
	}
	if err := p.write(request); err != nil {
		return err
	}
	select {
	case msg := <-waiter:
		if msg.Error != nil {
			return msg.Error
		}
		if result == nil || len(msg.Result) == 0 {
			return nil
		}
		return json.Unmarshal(msg.Result, result)
	case <-ctx.Done():
		return ctx.Err()
	case <-p.done:
		return p.processError()
	}
}

func (p *rpcProcess) notify(method string, params any) error {
	request := map[string]any{"method": method, "params": params}
	if p.includeJSONRPC {
		request["jsonrpc"] = "2.0"
	}
	return p.write(request)
}

func (p *rpcProcess) respond(id json.RawMessage, result any) error {
	var decoded any
	if err := json.Unmarshal(id, &decoded); err != nil {
		return err
	}
	response := map[string]any{"id": decoded, "result": result}
	if p.includeJSONRPC {
		response["jsonrpc"] = "2.0"
	}
	return p.write(response)
}

func (p *rpcProcess) write(value any) error {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	return json.NewEncoder(p.stdin).Encode(value)
}

func (p *rpcProcess) processError() error {
	p.waitErrMu.RLock()
	defer p.waitErrMu.RUnlock()
	if p.waitErr != nil {
		return p.waitErr
	}
	return ErrStopped
}

func (p *rpcProcess) stop(ctx context.Context, timeout time.Duration) error {
	_ = p.stdin.Close()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		_ = p.cmd.Process.Kill()
		return ctx.Err()
	case <-timer.C:
		if err := p.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		<-p.done
		return nil
	}
}
