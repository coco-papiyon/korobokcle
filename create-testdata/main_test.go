package main

import (
	"strings"
	"testing"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func TestBuildJobsIncludesAcceptanceFixtures(t *testing.T) {
	fixtures := make(map[domain.JobState]artifactJob)
	for _, entry := range buildJobs() {
		if entry.job.Kind == domain.JobKindPRAcceptance {
			fixtures[entry.job.State] = entry
		}
	}

	tests := []struct {
		state         domain.JobState
		writeArtifact bool
	}{
		{domain.StateAcceptanceTesting, false},
		{domain.StateAcceptanceTestReady, true},
		{domain.StateAcceptanceTestApproved, true},
	}
	for _, tt := range tests {
		entry, ok := fixtures[tt.state]
		if !ok {
			t.Fatalf("acceptance fixture for state %s not found", tt.state)
		}
		if entry.writeArtifact != tt.writeArtifact {
			t.Fatalf("writeArtifact for %s = %t, want %t", tt.state, entry.writeArtifact, tt.writeArtifact)
		}
		if entry.artifactSubDir != "acceptance_test" || entry.logSubDir != "acceptance_test" {
			t.Fatalf("fixture directories for %s = (%q, %q)", tt.state, entry.artifactSubDir, entry.logSubDir)
		}
		if !strings.Contains(entry.job.IssueContext, "## 受入基準") {
			t.Fatalf("acceptance criteria missing from fixture %s", tt.state)
		}
		if !strings.Contains(entry.job.Branch, "issue_#") {
			t.Fatalf("issue branch missing from fixture %s: %q", tt.state, entry.job.Branch)
		}
	}
}
