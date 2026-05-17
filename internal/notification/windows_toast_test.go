package notification

import (
	"strings"
	"testing"
)

func TestBuildWindowsToastScriptIncludesRegistrationAndFallback(t *testing.T) {
	t.Parallel()

	script := buildWindowsToastScript(Notification{
		Title:   "hello",
		Message: "world",
	})

	checks := []string{
		`Add-Type -AssemblyName System.Windows.Forms`,
		`Add-Type -AssemblyName System.Drawing`,
		`$notify = New-Object System.Windows.Forms.NotifyIcon`,
		`$notify.ShowBalloonTip(`,
		`$isAdmin = $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)`,
		`Write-Output "balloon=shown interactive=$isInteractive admin=$isAdmin session=$sessionId user=$($identity.Name)"`,
	}
	for _, check := range checks {
		if !strings.Contains(script, check) {
			t.Fatalf("expected script to contain %q", check)
		}
	}
}
