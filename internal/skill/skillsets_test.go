package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefinitionFallsBackToDefaultSkillSet(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeSkillFixture(t, filepath.Join(root, "skills", "default", "design"), "default design")

	definition, err := LoadDefinition(root, "design")
	if err != nil {
		t.Fatalf("LoadDefinition() error = %v", err)
	}
	if definition.Name != "design" {
		t.Fatalf("expected design, got %q", definition.Name)
	}
	if definition.PromptFile != filepath.Join(root, "skills", "default", "design", "prompt.md.tmpl") {
		t.Fatalf("unexpected prompt file path: %q", definition.PromptFile)
	}
}

func TestCreateAndDeleteSkillSet(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, skillName := range managedSkillNames {
		writeSkillFixture(t, filepath.Join(root, "skills", "default", skillName), skillName+" prompt")
	}

	created, err := CreateSkillSet(root, "team-a", "default")
	if err != nil {
		t.Fatalf("CreateSkillSet() error = %v", err)
	}
	if !created.Mutable {
		t.Fatal("expected created skill set to be mutable")
	}
	if created.Skills["design"].PromptTemplate != "design prompt" {
		t.Fatalf("expected copied prompt, got %q", created.Skills["design"].PromptTemplate)
	}

	created.Skills["design"] = SkillFile{
		Definition: Definition{
			Name:     "design",
			Provider: "codex",
			Inputs:   []string{"issue"},
			Outputs:  []string{"design_doc"},
			Artifacts: ArtifactBlock{
				OutputFile: "design.md",
			},
		},
		PromptTemplate: "updated prompt",
	}
	if err := SaveSkillSet(root, created); err != nil {
		t.Fatalf("SaveSkillSet() error = %v", err)
	}

	loaded, err := LoadSkillSet(root, "team-a")
	if err != nil {
		t.Fatalf("LoadSkillSet() error = %v", err)
	}
	if loaded.Skills["design"].Definition.Provider != "codex" {
		t.Fatalf("expected provider codex, got %q", loaded.Skills["design"].Definition.Provider)
	}
	if loaded.Skills["design"].PromptTemplate != "updated prompt" {
		t.Fatalf("expected updated prompt, got %q", loaded.Skills["design"].PromptTemplate)
	}

	if err := DeleteSkillSet(root, "team-a"); err != nil {
		t.Fatalf("DeleteSkillSet() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "team-a")); !os.IsNotExist(err) {
		t.Fatalf("expected deleted directory, stat err = %v", err)
	}
}

func writeSkillFixture(t *testing.T, dir string, prompt string) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skill.yaml"), []byte("name: "+filepath.Base(dir)+"\nprovider: mock\ninputs:\n  - issue\noutputs:\n  - doc\nartifacts:\n  output_file: out.md\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(skill.yaml) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompt.md.tmpl"), []byte(prompt), 0o644); err != nil {
		t.Fatalf("WriteFile(prompt.md.tmpl) error = %v", err)
	}
}
