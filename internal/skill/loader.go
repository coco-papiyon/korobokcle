package skill

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

func LoadDefinition(root string, skillName string) (Definition, error) {
	definition, skillDir, err := loadDefinitionFromSkillName(root, skillName)
	if err != nil {
		return Definition{}, err
	}
	definition.PromptFile = filepath.Join(skillDir, "prompt.md.tmpl")
	if definition.Name == "" {
		return Definition{}, fmt.Errorf("skill %q is missing name", skillName)
	}
	return definition, nil
}

func loadDefinitionFromSkillSet(root string, setName string, skillName string) (Definition, string, error) {
	skillDir := filepath.Join(root, "skills", setName, skillName)
	raw, err := os.ReadFile(filepath.Join(skillDir, "skill.yaml"))
	if err != nil {
		return Definition{}, "", err
	}

	var definition Definition
	if err := yaml.Unmarshal(raw, &definition); err != nil {
		return Definition{}, "", err
	}
	return definition, skillDir, nil
}

func loadDefinitionFromSkillName(root string, skillName string) (Definition, string, error) {
	if skillName == "fix" {
		skillName = "implement_fix"
	}
	for _, candidate := range managedSkillNames {
		if candidate == skillName {
			definition, skillDir, err := loadDefinitionFromSkillSet(root, "default", skillName)
			if err == nil {
				return definition, skillDir, nil
			}
			if skillName == "implement_fix" {
				return loadDefinitionFromSkillSet(root, "default", "fix")
			}
			return Definition{}, "", err
		}
	}
	return loadDefinitionFromSkillSet(root, filepath.Dir(skillName), filepath.Base(skillName))
}

func RenderPrompt(path string, data any) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return RenderPromptText(string(raw), data)
}

func RenderPromptText(raw string, data any) (string, error) {
	tmpl, err := template.New("prompt").Option("missingkey=zero").Parse(raw)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return "", err
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

func RenderSkillPrompt(root string, skillName string, data any) (string, error) {
	definition, skillDir, err := loadDefinitionFromSkillName(root, skillName)
	if err != nil {
		return "", err
	}

	parts := make([]string, 0, 4)
	if title := strings.TrimSpace(definition.Title); title != "" {
		parts = append(parts, "# "+title)
	}
	if role := strings.TrimSpace(definition.Role); role != "" {
		parts = append(parts, role)
	}

	templateNames := definition.PromptTemplates
	if len(templateNames) == 0 {
		templateNames = []string{"prompt.md.tmpl"}
	}
	for _, templateName := range templateNames {
		raw, err := os.ReadFile(filepath.Join(skillDir, templateName))
		if err != nil {
			return "", err
		}
		rendered, err := RenderPromptText(string(raw), data)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(rendered) != "" {
			parts = append(parts, rendered)
		}
	}

	return strings.Join(parts, "\n\n"), nil
}
