package skill

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"gopkg.in/yaml.v3"
)

func LoadDefinition(root string, skillName string) (Definition, error) {
	skillDir := filepath.Join(root, "skills", skillName)
	raw, err := os.ReadFile(filepath.Join(skillDir, "skill.yaml"))
	if err != nil {
		return Definition{}, err
	}

	var definition Definition
	if err := yaml.Unmarshal(raw, &definition); err != nil {
		return Definition{}, err
	}
	definition.PromptFile = filepath.Join(skillDir, "prompt.md.tmpl")
	if definition.Name == "" {
		return Definition{}, fmt.Errorf("skill %q is missing name", skillName)
	}
	if definition.Artifacts.OutputFile == "" {
		definition.Artifacts.OutputFile = skillName + ".md"
	}
	return definition, nil
}

func RenderPrompt(path string, data any) (string, error) {
	tmpl, err := template.ParseFiles(path)
	if err != nil {
		return "", err
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return "", err
	}
	return out.String(), nil
}
