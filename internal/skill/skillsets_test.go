package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefinitionFallsBackToDefaultSkillSet(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeSkillFixture(t, filepath.Join(root, "skills", "default", "design"), "default input", "default design")

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
		writeSkillFixture(t, filepath.Join(root, "skills", "default", skillName), skillName+" input", skillName+" prompt")
	}

	created, err := CreateSkillSet(root, "team-a", "default")
	if err != nil {
		t.Fatalf("CreateSkillSet() error = %v", err)
	}
	if !created.Mutable {
		t.Fatal("expected created skill set to be mutable")
	}
	if created.Skills["design"].InputTemplate != "design input" {
		t.Fatalf("expected copied input template, got %q", created.Skills["design"].InputTemplate)
	}
	if created.Skills["design"].PromptTemplate != "design prompt" {
		t.Fatalf("expected copied prompt, got %q", created.Skills["design"].PromptTemplate)
	}

	created.Skills["design"] = SkillFile{
		Definition: Definition{
			Name:            "design",
			Title:           "Updated Title",
			Role:            "Updated Role",
			PromptTemplates: []string{"input.md.tmpl", "prompt.md.tmpl"},
		},
		InputTemplate:  "updated input",
		PromptTemplate: "updated prompt",
	}
	if err := SaveSkillSet(root, created); err != nil {
		t.Fatalf("SaveSkillSet() error = %v", err)
	}

	loaded, err := LoadSkillSet(root, "team-a")
	if err != nil {
		t.Fatalf("LoadSkillSet() error = %v", err)
	}
	if loaded.Skills["design"].Definition.Title != "Updated Title" {
		t.Fatalf("expected updated title, got %q", loaded.Skills["design"].Definition.Title)
	}
	if loaded.Skills["design"].Definition.Role != "Updated Role" {
		t.Fatalf("expected updated role, got %q", loaded.Skills["design"].Definition.Role)
	}
	if loaded.Skills["design"].InputTemplate != "updated input" {
		t.Fatalf("expected updated input template, got %q", loaded.Skills["design"].InputTemplate)
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

func TestLoadSkillSetFallsBackToDefaultForMissingManagedSkill(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, skillName := range managedSkillNames {
		writeSkillFixture(t, filepath.Join(root, "skills", "default", skillName), skillName+" input", skillName+" prompt")
	}

	created, err := CreateSkillSet(root, "team-a", "default")
	if err != nil {
		t.Fatalf("CreateSkillSet() error = %v", err)
	}
	if err := os.RemoveAll(filepath.Join(root, "skills", "team-a", "improvement_implementation")); err != nil {
		t.Fatalf("RemoveAll(improvement_implementation) error = %v", err)
	}

	loaded, err := LoadSkillSet(root, "team-a")
	if err != nil {
		t.Fatalf("LoadSkillSet() error = %v", err)
	}
	if loaded.Skills["improvement_implementation"].PromptTemplate != "improvement_implementation prompt" {
		t.Fatalf("expected fallback prompt from default skill set, got %q", loaded.Skills["improvement_implementation"].PromptTemplate)
	}
	if loaded.Skills["improvement_consideration"].PromptTemplate != "improvement_consideration prompt" {
		t.Fatalf("expected improvement consideration prompt from default skill set, got %q", loaded.Skills["improvement_consideration"].PromptTemplate)
	}
	if created.Skills["improvement_consideration"].PromptTemplate != "improvement_consideration prompt" {
		t.Fatalf("expected created improvement consideration prompt, got %q", created.Skills["improvement_consideration"].PromptTemplate)
	}
}

func writeSkillFixture(t *testing.T, dir string, input string, prompt string) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skill.yaml"), []byte("name: "+filepath.Base(dir)+"\ntitle: title\nrole: role\npromptTemplates:\n  - input.md.tmpl\n  - prompt.md.tmpl\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(skill.yaml) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "input.md.tmpl"), []byte(input), 0o644); err != nil {
		t.Fatalf("WriteFile(input.md.tmpl) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompt.md.tmpl"), []byte(prompt), 0o644); err != nil {
		t.Fatalf("WriteFile(prompt.md.tmpl) error = %v", err)
	}
}
