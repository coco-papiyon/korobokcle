//go:build windows

package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

func startSessionProcess(ctx context.Context, cfg SessionConfig) (sessionProcess, error) {
	if cfg.UsePTY {
		return startConPTYSessionProcess(ctx, cfg)
	}
	return startPipeSessionProcess(ctx, cfg)
}

func startConPTYSessionProcess(ctx context.Context, cfg SessionConfig) (sessionProcess, error) {
	commandPath, err := exec.LookPath(strings.TrimSpace(cfg.Command))
	if err != nil {
		return sessionProcess{}, err
	}

	ptyInputRead, inputWriter, err := createInheritablePipe()
	if err != nil {
		return sessionProcess{}, fmt.Errorf("create conpty input pipe: %w", err)
	}
	ptyOutputRead, ptyOutputWrite, err := createInheritablePipe()
	if err != nil {
		windows.CloseHandle(ptyInputRead)
		windows.CloseHandle(inputWriter)
		return sessionProcess{}, fmt.Errorf("create conpty output pipe: %w", err)
	}

	var pseudoConsole windows.Handle
	size := windows.Coord{X: 120, Y: 40}
	if err := windows.CreatePseudoConsole(size, ptyInputRead, ptyOutputWrite, 0, &pseudoConsole); err != nil {
		windows.CloseHandle(ptyInputRead)
		windows.CloseHandle(inputWriter)
		windows.CloseHandle(ptyOutputRead)
		windows.CloseHandle(ptyOutputWrite)
		return sessionProcess{}, fmt.Errorf("create pseudoconsole: %w", err)
	}

	attrList, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		windows.ClosePseudoConsole(pseudoConsole)
		windows.CloseHandle(inputWriter)
		windows.CloseHandle(ptyOutputRead)
		return sessionProcess{}, fmt.Errorf("allocate proc thread attribute list: %w", err)
	}
	if err := attrList.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, unsafe.Pointer(pseudoConsole), unsafe.Sizeof(pseudoConsole)); err != nil {
		attrList.Delete()
		windows.ClosePseudoConsole(pseudoConsole)
		windows.CloseHandle(inputWriter)
		windows.CloseHandle(ptyOutputRead)
		return sessionProcess{}, fmt.Errorf("attach pseudoconsole attribute: %w", err)
	}

	commandLine, err := windows.UTF16PtrFromString(buildWindowsCommandLine(commandPath, cfg.Args))
	if err != nil {
		attrList.Delete()
		windows.ClosePseudoConsole(pseudoConsole)
		windows.CloseHandle(inputWriter)
		windows.CloseHandle(ptyOutputRead)
		return sessionProcess{}, fmt.Errorf("encode command line: %w", err)
	}
	applicationName, err := windows.UTF16PtrFromString(commandPath)
	if err != nil {
		attrList.Delete()
		windows.ClosePseudoConsole(pseudoConsole)
		windows.CloseHandle(inputWriter)
		windows.CloseHandle(ptyOutputRead)
		return sessionProcess{}, fmt.Errorf("encode application name: %w", err)
	}

	var currentDir *uint16
	if strings.TrimSpace(cfg.WorkDir) != "" {
		currentDir, err = windows.UTF16PtrFromString(cfg.WorkDir)
		if err != nil {
			attrList.Delete()
			windows.ClosePseudoConsole(pseudoConsole)
			windows.CloseHandle(inputWriter)
			windows.CloseHandle(ptyOutputRead)
			return sessionProcess{}, fmt.Errorf("encode work dir: %w", err)
		}
	}

	envBlock, err := buildWindowsEnvironmentBlock(cfg.Env)
	if err != nil {
		attrList.Delete()
		windows.ClosePseudoConsole(pseudoConsole)
		windows.CloseHandle(inputWriter)
		windows.CloseHandle(ptyOutputRead)
		return sessionProcess{}, fmt.Errorf("build environment block: %w", err)
	}

	startupInfo := windows.StartupInfoEx{
		StartupInfo: windows.StartupInfo{
			Cb:        uint32(unsafe.Sizeof(windows.StartupInfoEx{})),
			Flags:     windows.STARTF_USESTDHANDLES,
			StdInput:  ptyInputRead,
			StdOutput: ptyOutputWrite,
			StdErr:    ptyOutputWrite,
		},
		ProcThreadAttributeList: attrList.List(),
	}
	processInfo := new(windows.ProcessInformation)
	flags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT | windows.CREATE_UNICODE_ENVIRONMENT)
	if err := windows.CreateProcess(
		applicationName,
		commandLine,
		nil,
		nil,
		false,
		flags,
		envBlock,
		currentDir,
		&startupInfo.StartupInfo,
		processInfo,
	); err != nil {
		attrList.Delete()
		windows.ClosePseudoConsole(pseudoConsole)
		windows.CloseHandle(inputWriter)
		windows.CloseHandle(ptyOutputRead)
		return sessionProcess{}, fmt.Errorf("create process with pseudoconsole: %w", err)
	}

	_ = windows.CloseHandle(processInfo.Thread)

	inputFile := os.NewFile(uintptr(inputWriter), "conpty-stdin")
	outputFile := os.NewFile(uintptr(ptyOutputRead), "conpty-stdout")

	go func() {
		<-ctx.Done()
		_ = windows.TerminateProcess(processInfo.Process, 1)
	}()

	return sessionProcess{
		stdin:  inputFile,
		stdout: outputFile,
		// ConPTY merges the child's stdout/stderr into a single output stream.
		stderr: nil,
		wait: func() error {
			_, err := windows.WaitForSingleObject(processInfo.Process, windows.INFINITE)
			if err != nil {
				return err
			}
			var exitCode uint32
			if err := windows.GetExitCodeProcess(processInfo.Process, &exitCode); err != nil {
				return err
			}
			if exitCode != 0 {
				return fmt.Errorf("process exited with code %d", exitCode)
			}
			return nil
		},
		kill: func() error {
			return windows.TerminateProcess(processInfo.Process, 1)
		},
		cleanup: func() error {
			_ = inputFile.Close()
			_ = outputFile.Close()
			_ = windows.CloseHandle(ptyInputRead)
			_ = windows.CloseHandle(ptyOutputWrite)
			windows.ClosePseudoConsole(pseudoConsole)
			attrList.Delete()
			return windows.CloseHandle(processInfo.Process)
		},
	}, nil
}

func createInheritablePipe() (windows.Handle, windows.Handle, error) {
	security := &windows.SecurityAttributes{
		Length:        uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		InheritHandle: 1,
	}
	var readHandle windows.Handle
	var writeHandle windows.Handle
	if err := windows.CreatePipe(&readHandle, &writeHandle, security, 0); err != nil {
		return 0, 0, err
	}
	return readHandle, writeHandle, nil
}

func buildWindowsEnvironmentBlock(values []string) (*uint16, error) {
	if len(values) == 0 {
		return nil, nil
	}

	block := strings.Join(values, "\x00") + "\x00\x00"
	encoded := utf16.Encode([]rune(block))
	return &encoded[0], nil
}

func buildWindowsCommandLine(command string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, windowsEscapeArg(command))
	for _, arg := range args {
		parts = append(parts, windowsEscapeArg(arg))
	}
	return strings.Join(parts, " ")
}

func windowsEscapeArg(value string) string {
	if value == "" {
		return `""`
	}
	if !strings.ContainsAny(value, " \t\"") {
		return value
	}

	var builder strings.Builder
	builder.WriteByte('"')
	backslashes := 0
	for _, r := range value {
		switch r {
		case '\\':
			backslashes++
		case '"':
			builder.WriteString(strings.Repeat(`\`, backslashes*2+1))
			builder.WriteRune('"')
			backslashes = 0
		default:
			if backslashes > 0 {
				builder.WriteString(strings.Repeat(`\`, backslashes))
				backslashes = 0
			}
			builder.WriteRune(r)
		}
	}
	if backslashes > 0 {
		builder.WriteString(strings.Repeat(`\`, backslashes*2))
	}
	builder.WriteByte('"')
	return builder.String()
}
