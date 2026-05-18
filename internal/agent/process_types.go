package agent

import "io"

type sessionProcess struct {
	stdin   io.WriteCloser
	stdout  io.Reader
	stderr  io.Reader
	wait    func() error
	kill    func() error
	cleanup func() error
}
