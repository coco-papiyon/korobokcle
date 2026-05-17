package notification

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"strings"
	"unicode/utf16"
)

type WindowsToastNotifier struct{}

func NewWindowsToastNotifier() *WindowsToastNotifier {
	return &WindowsToastNotifier{}
}

func (n *WindowsToastNotifier) Notify(ctx context.Context, event Notification) error {
	if runtime.GOOS != "windows" {
		return nil
	}

	script := buildWindowsToastScript(event)
	cmd := exec.CommandContext(ctx, "powershell.exe",
		"-NoLogo",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-EncodedCommand", encodePowerShellCommand(script),
	)
	raw, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("show windows toast: %w: %s", err, strings.TrimSpace(string(raw)))
	}
	if output := strings.TrimSpace(string(raw)); output != "" {
		log.Printf("info windows toast notifier output: %s", output)
	}
	return nil
}

func buildWindowsToastScript(event Notification) string {
	return fmt.Sprintf(
		"$ErrorActionPreference = 'Stop'\n"+
			"Add-Type -AssemblyName System.Windows.Forms\n"+
			"Add-Type -AssemblyName System.Drawing\n"+
			"$identity = [Security.Principal.WindowsIdentity]::GetCurrent()\n"+
			"$principal = [Security.Principal.WindowsPrincipal]::new($identity)\n"+
			"$isAdmin = $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)\n"+
			"$isInteractive = [Environment]::UserInteractive\n"+
			"$sessionId = [System.Diagnostics.Process]::GetCurrentProcess().SessionId\n"+
			"$notify = New-Object System.Windows.Forms.NotifyIcon\n"+
			"$notify.Icon = [System.Drawing.SystemIcons]::Information\n"+
			"$notify.Visible = $true\n"+
			"$notify.ShowBalloonTip(\n"+
			"  5000,\n"+
			"  '%s',\n"+
			"  '%s',\n"+
			"  [System.Windows.Forms.ToolTipIcon]::Info\n"+
			")\n"+
			"Start-Sleep -Seconds 6\n"+
			"$notify.Dispose()\n"+
			"Write-Output \"balloon=shown interactive=$isInteractive admin=$isAdmin session=$sessionId user=$($identity.Name)\"\n",
		psSingleQuoteEscape(event.Title),
		psSingleQuoteEscape(event.Message),
	)
}

func encodePowerShellCommand(command string) string {
	encoded := utf16.Encode([]rune(command))
	buf := bytes.NewBuffer(make([]byte, 0, len(encoded)*2))
	for _, r := range encoded {
		buf.WriteByte(byte(r))
		buf.WriteByte(byte(r >> 8))
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func psSingleQuoteEscape(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}
