package config

import (
	"reflect"
	"testing"
)

func TestDefaultCopilotAllowTools(t *testing.T) {
	t.Parallel()

	want := []string{}
	if !reflect.DeepEqual(DefaultCopilotAllowTools, want) {
		t.Fatalf("expected default copilot allow tools %v, got %v", want, DefaultCopilotAllowTools)
	}
}
