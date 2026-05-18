package config

import (
	"reflect"
	"testing"
	"time"
)

func TestDefaultCopilotAllowTools(t *testing.T) {
	t.Parallel()

	want := []string{}
	if !reflect.DeepEqual(DefaultCopilotAllowTools, want) {
		t.Fatalf("expected default copilot allow tools %v, got %v", want, DefaultCopilotAllowTools)
	}
}

func TestDefaultScreenRefreshInterval(t *testing.T) {
	t.Parallel()

	if DefaultScreenRefreshInterval != 5*time.Second {
		t.Fatalf("expected default screen refresh interval 5s, got %s", DefaultScreenRefreshInterval)
	}
}
