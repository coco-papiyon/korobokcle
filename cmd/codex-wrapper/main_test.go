package main

import (
	"encoding/json"
	"testing"
)

func TestDefaultChildArgsJSONUsesExec(t *testing.T) {
	var args []string
	if err := json.Unmarshal([]byte(defaultChildArgsJSON), &args); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(args) != 1 || args[0] != "exec" {
		t.Fatalf("expected default child args to be [\"exec\"], got %#v", args)
	}
}
