package notification

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
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
	return nil
}

func buildWindowsToastScript(event Notification) string {
	return fmt.Sprintf(
		"$ErrorActionPreference = 'Stop'\n"+
			"[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null\n"+
			"[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] > $null\n"+
			"$xml = New-Object Windows.Data.Xml.Dom.XmlDocument\n"+
			"$xml.LoadXml(\"<toast><visual><binding template='ToastGeneric'><text>%s</text><text>%s</text></binding></visual></toast>\")\n"+
			"$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)\n"+
			"[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('Korobokcle').Show($toast)\n",
		xmlEscape(event.Title),
		xmlEscape(event.Message),
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

func xmlEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}
