package app

import (
	"bytes"
	"fmt"
	"os"
	"text/template"
)

type skillGenerationPromptData struct {
	ProviderDisplayName string
	StageDir            string
	ProjectContext      string
	TestCommand         string
	MaxFixLoops         int
	Missing             []skillPromptDefinition
	IsCodex             bool
}

type skillPromptDefinition struct {
	Name        string
	Purpose     string
	DisplayName string
}

func renderSkillGenerationPrompt(path string, data skillGenerationPromptData) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read skill generation prompt %q: %w", path, err)
	}
	tmpl, err := template.New("skill_generation_prompt").Parse(string(raw))
	if err != nil {
		return "", fmt.Errorf("parse skill generation prompt %q: %w", path, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute skill generation prompt %q: %w", path, err)
	}
	return buf.String(), nil
}
