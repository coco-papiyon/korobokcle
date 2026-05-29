//go:build linux

package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

func startSessionProcess(ctx context.Context, cfg SessionConfig) (sessionProcess, error) {
	if cfg.UsePTY {
		return startPTYSessionProcess(ctx, cfg)
	}
	return startPipeSessionProcess(ctx, cfg)
}

func startPTYSessionProcess(ctx context.Context, cfg SessionConfig) (sessionProcess, error) {
	commandPath, err := exec.LookPath(strings.TrimSpace(cfg.Command))
	if err != nil {
		return sessionProcess{}, err
	}

	master, slave, err := openPTY()
	if err != nil {
		return sessionProcess{}, fmt.Errorf("open pty: %w", err)
	}

	cmd := exec.CommandContext(ctx, commandPath, cfg.Args...)
	cmd.Dir = cfg.WorkDir
	if len(cfg.Env) > 0 {
		cmd.Env = append([]string(nil), cfg.Env...)
	}
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		Ctty:    0,
	}

	if err := cmd.Start(); err != nil {
		_ = master.Close()
		_ = slave.Close()
		return sessionProcess{}, err
	}
	if err := slave.Close(); err != nil {
		_ = master.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return sessionProcess{}, err
	}

	go func() {
		<-ctx.Done()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	return sessionProcess{
		stdin:  master,
		stdout: master,
		// PTY merges stdout/stderr into a single stream.
		stderr: nil,
		wait:   cmd.Wait,
		kill: func() error {
			if cmd.Process == nil {
				return nil
			}
			return cmd.Process.Kill()
		},
		cleanup: master.Close,
	}, nil
}

func openPTY() (*os.File, *os.File, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}

	unlock := 0
	if err := ioctlSetInt(master.Fd(), syscall.TIOCSPTLCK, unlock); err != nil {
		_ = master.Close()
		return nil, nil, err
	}

	number, err := ioctlGetInt(master.Fd(), syscall.TIOCGPTN)
	if err != nil {
		_ = master.Close()
		return nil, nil, err
	}

	slavePath := "/dev/pts/" + strconv.Itoa(number)
	slave, err := os.OpenFile(slavePath, os.O_RDWR, 0)
	if err != nil {
		_ = master.Close()
		return nil, nil, err
	}

	if err := ioctlSetWinsize(master.Fd(), syscall.TIOCSWINSZ, &winsize{row: 40, col: 120}); err != nil {
		_ = slave.Close()
		_ = master.Close()
		return nil, nil, err
	}

	return master, slave, nil
}

type winsize struct {
	row    uint16
	col    uint16
	xpixel uint16
	ypixel uint16
}

func ioctlSetInt(fd uintptr, req uint, value int) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(req), uintptr(unsafe.Pointer(&value)))
	if errno != 0 {
		return errno
	}
	return nil
}

func ioctlGetInt(fd uintptr, req uint) (int, error) {
	var value int
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(req), uintptr(unsafe.Pointer(&value)))
	if errno != 0 {
		return 0, errno
	}
	return value, nil
}

func ioctlSetWinsize(fd uintptr, req uint, value *winsize) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(req), uintptr(unsafe.Pointer(value)))
	if errno != 0 {
		return errno
	}
	return nil
}
