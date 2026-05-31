package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var skillSetNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

var managedSkillNames = []string{"design", "implement", "implement_fix", "review", "review_fix"}

type SkillFile struct {
	Definition     Definition `json:"definition"`
	InputTemplate  string     `json:"inputTemplate"`
	PromptTemplate string     `json:"promptTemplate"`
}

type SkillSet struct {
	Name    string               `json:"name"`
	Mutable bool                 `json:"mutable"`
	Skills  map[string]SkillFile `json:"skills"`
}

type SkillSetSummary struct {
	Name    string `json:"name"`
	Mutable bool   `json:"mutable"`
}

func ListSkillSets(root string) ([]SkillSetSummary, error) {
	skillRoot := filepath.Join(root, "skills")
	entries, err := os.ReadDir(skillRoot)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	names := map[string]bool{
		"default": false,
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if name == "" {
			continue
		}
		if slices.Contains(managedSkillNames, name) {
			continue
		}
		names[name] = name != "default"
	}

	out := make([]SkillSetSummary, 0, len(names))
	for name, mutable := range names {
		out = append(out, SkillSetSummary{Name: name, Mutable: mutable})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == "default" {
			return true
		}
		if out[j].Name == "default" {
			return false
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func LoadSkillSet(root string, name string) (SkillSet, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return SkillSet{}, fmt.Errorf("skill set name is required")
	}

	set := SkillSet{
		Name:    trimmedName,
		Mutable: trimmedName != "default",
		Skills:  make(map[string]SkillFile, len(managedSkillNames)),
	}
	for _, skillName := range managedSkillNames {
		file, err := loadSkillFile(root, trimmedName, skillName)
		if err != nil {
			return SkillSet{}, err
		}
		set.Skills[skillName] = file
	}
	return set, nil
}

func CreateSkillSet(root string, name string, source string) (SkillSet, error) {
	trimmedName := strings.TrimSpace(name)
	if err := validateMutableSkillSetName(trimmedName); err != nil {
		return SkillSet{}, err
	}

	existing, err := ListSkillSets(root)
	if err != nil {
		return SkillSet{}, err
	}
	for _, candidate := range existing {
		if candidate.Name == trimmedName {
			return SkillSet{}, fmt.Errorf("skill set %q already exists", trimmedName)
		}
	}

	sourceName := strings.TrimSpace(source)
	if sourceName == "" {
		sourceName = "default"
	}
	base, err := LoadSkillSet(root, sourceName)
	if err != nil {
		return SkillSet{}, err
	}
	base.Name = trimmedName
	base.Mutable = true

	if err := SaveSkillSet(root, base); err != nil {
		return SkillSet{}, err
	}
	return LoadSkillSet(root, trimmedName)
}

func SaveSkillSet(root string, set SkillSet) error {
	name := strings.TrimSpace(set.Name)
	if err := validateMutableSkillSetName(name); err != nil {
		return err
	}
	if len(set.Skills) == 0 {
		return fmt.Errorf("skills are required")
	}

	for _, skillName := range managedSkillNames {
		file, ok := set.Skills[skillName]
		if !ok {
			return fmt.Errorf("skill %q is required", skillName)
		}
		if err := writeSkillFile(root, name, skillName, file); err != nil {
			return err
		}
	}
	return nil
}

func DeleteSkillSet(root string, name string) error {
	trimmedName := strings.TrimSpace(name)
	if err := validateMutableSkillSetName(trimmedName); err != nil {
		return err
	}
	target := filepath.Join(root, "skills", trimmedName)
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("skill set %q is not a directory", trimmedName)
	}
	return os.RemoveAll(target)
}

func loadSkillFile(root string, setName string, skillName string) (SkillFile, error) {
	definition, skillDir, err := loadDefinitionFromSkillSet(root, setName, skillName)
	if err != nil && skillName == "implement_fix" {
		definition, skillDir, err = loadDefinitionFromSkillSet(root, setName, "fix")
	}
	if err != nil {
		return SkillFile{}, err
	}

	inputTemplatePath := filepath.Join(skillDir, "input.md.tmpl")
	inputTemplate, err := os.ReadFile(inputTemplatePath)
	inputTemplateExists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return SkillFile{}, err
	}
	rawPrompt, err := os.ReadFile(filepath.Join(skillDir, "prompt.md.tmpl"))
	if err != nil {
		return SkillFile{}, err
	}
	definition.PromptFile = ""
	definition.PromptTemplates = normalizePromptTemplates(definition.PromptTemplates, inputTemplateExists)
	return SkillFile{
		Definition:     definition,
		InputTemplate:  string(inputTemplate),
		PromptTemplate: string(rawPrompt),
	}, nil
}

func writeSkillFile(root string, setName string, skillName string, file SkillFile) error {
	definition := file.Definition
	definition.Name = skillName
	definition.PromptFile = ""
	definition.PromptTemplates = normalizePromptTemplates(definition.PromptTemplates, strings.TrimSpace(file.InputTemplate) != "")

	skillDir := filepath.Join(root, "skills", setName, skillName)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return err
	}

	rawDefinition, err := yaml.Marshal(definition)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(skillDir, "skill.yaml"), rawDefinition, 0o644); err != nil {
		return err
	}
	if containsString(definition.PromptTemplates, "input.md.tmpl") {
		if err := os.WriteFile(filepath.Join(skillDir, "input.md.tmpl"), []byte(file.InputTemplate), 0o644); err != nil {
			return err
		}
	} else if err := os.Remove(filepath.Join(skillDir, "input.md.tmpl")); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.WriteFile(filepath.Join(skillDir, "prompt.md.tmpl"), []byte(file.PromptTemplate), 0o644); err != nil {
		return err
	}
	if skillName == "implement_fix" {
		if err := os.RemoveAll(filepath.Join(root, "skills", setName, "fix")); err != nil {
			return err
		}
	}
	return nil
}

func normalizePromptTemplates(values []string, hasInputTemplate bool) []string {
	if len(values) == 0 {
		if hasInputTemplate {
			return []string{"input.md.tmpl", "prompt.md.tmpl"}
		}
		return []string{"prompt.md.tmpl"}
	}

	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	hasInput := false
	for _, value := range values {
		name := strings.TrimSpace(value)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		if name == "input.md.tmpl" {
			hasInput = true
			continue
		}
		out = append(out, name)
	}
	if hasInputTemplate || hasInput {
		out = append([]string{"input.md.tmpl"}, out...)
	}
	out = append(out, "prompt.md.tmpl")
	return out
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func validateMutableSkillSetName(name string) error {
	if name == "" {
		return fmt.Errorf("skill set name is required")
	}
	if name == "default" {
		return fmt.Errorf("default skill set is read-only")
	}
	if !skillSetNamePattern.MatchString(name) {
		return fmt.Errorf("skill set name must match %s", skillSetNamePattern.String())
	}
	return nil
}
