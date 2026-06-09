package app

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type ImprovementSource struct {
	JobID       string `yaml:"jobId,omitempty"`
	IssueNumber int    `yaml:"issueNumber,omitempty"`
	Repository  string `yaml:"repository,omitempty"`
	Event       string `yaml:"event,omitempty"`
	CommentID   string `yaml:"commentId,omitempty"`
	ApprovalID  string `yaml:"approvalId,omitempty"`
}

type ImprovementFrontMatter struct {
	ID        string            `yaml:"id"`
	Title     string            `yaml:"title"`
	Scope     string            `yaml:"scope"`
	Phases    []string          `yaml:"phases"`
	Status    string            `yaml:"status"`
	UpdatedAt time.Time         `yaml:"updatedAt"`
	Source    ImprovementSource `yaml:"source"`
}

type ImprovementDocument struct {
	FrontMatter ImprovementFrontMatter
	Body        string
}

func (d ImprovementDocument) MarshalMarkdown() ([]byte, error) {
	frontMatter, err := yaml.Marshal(d.FrontMatter)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	out.WriteString("---\n")
	out.Write(frontMatter)
	out.WriteString("---\n")
	body := strings.TrimSpace(d.Body)
	if body != "" {
		out.WriteString("\n")
		out.WriteString(body)
		out.WriteString("\n")
	}
	return out.Bytes(), nil
}

func ParseImprovementMarkdown(raw []byte) (ImprovementDocument, error) {
	text := string(raw)
	if !strings.HasPrefix(text, "---\n") {
		return ImprovementDocument{}, fmt.Errorf("improvement markdown must start with front matter delimiter")
	}

	remaining := strings.TrimPrefix(text, "---\n")
	idx := strings.Index(remaining, "\n---\n")
	if idx < 0 {
		return ImprovementDocument{}, fmt.Errorf("improvement markdown must include closing front matter delimiter")
	}

	frontMatterRaw := remaining[:idx]
	bodyRaw := remaining[idx+len("\n---\n"):]

	var frontMatter ImprovementFrontMatter
	if err := yaml.Unmarshal([]byte(frontMatterRaw), &frontMatter); err != nil {
		return ImprovementDocument{}, fmt.Errorf("decode improvement front matter: %w", err)
	}

	return ImprovementDocument{
		FrontMatter: frontMatter,
		Body:        strings.TrimSpace(bodyRaw),
	}, nil
}
