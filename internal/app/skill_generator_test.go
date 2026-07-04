package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func TestSkillStatusUsesSimpleCheckOnly(t *testing.T) {
	baseDir := t.TempDir()
	path := filepath.Join(baseDir, ".github", "skills", "custom-issue-planner")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: custom-issue-planner\ndescription: Design a solution from a GitHub issue.\n---\n"
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	generator := NewSkillGeneratorWithFactory(baseDir, t.TempDir(), t.TempDir(), nil, nil, nil)
	statuses, err := generator.SkillStatus(context.Background())
	if err != nil {
		t.Fatalf("SkillStatus() error = %v", err)
	}
	if !statuses[0].Exists {
		t.Fatalf("issue design status = %+v, want simple existing", statuses[0])
	}
	wantPath := filepath.Join(".github", "skills", "custom-issue-planner", "SKILL.md")
	if statuses[0].Path != wantPath {
		t.Fatalf("issue design path = %q, want %q", statuses[0].Path, wantPath)
	}
	if statuses[0].AIExists {
		t.Fatalf("issue design status = %+v, want aiExists=false without generation cache", statuses[0])
	}
	if statuses[0].Generated {
		t.Fatalf("issue design status = %+v, want generated=false", statuses[0])
	}
}

func TestGenerateSkillsSkipsAIExistingSkill(t *testing.T) {
	baseDir := t.TempDir()
	toolDir := t.TempDir()
	workDir := t.TempDir()
	installSkillGenerationPrompt(t, toolDir)
	testCommand := "go test ./..."
	path := filepath.Join(baseDir, ".github", "skills", "custom-issue-planner")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: custom-issue-planner\ndescription: Design a solution from a GitHub issue.\n---\n"
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	generator := NewSkillGeneratorWithFactory(baseDir, toolDir, workDir, staticSkillSettingsStore{
		settings: domain.WatchSettings{AIProvider: domain.AIProviderCodex},
	}, nil, func() SkillAgent {
		return &fakeSkillAgent{
			run: func(ctx context.Context, req AIRequest) (AIResponse, error) {
				if strings.Contains(req.Prompt, "Existing skill file:") {
					return AIResponse{RawOutput: `{"matches":true,"reason":"same workflow","confidence":"high"}`}, nil
				}
				if strings.Contains(req.Prompt, "Agent Skillを生成してください") {
					return AIResponse{}, writeGeneratedSkillFiles(req.WorkingDir, testCommand)
				}
				return AIResponse{}, errors.New("unexpected prompt")
			},
		}
	})

	result, err := generator.GenerateSkills(context.Background(), domain.SkillGenerationRequest{
		TestCommand: "go test ./...",
		MaxFixLoops: 4,
	})
	if err != nil {
		t.Fatalf("GenerateSkills() error = %v", err)
	}
	if !result.Skills[0].AIExists {
		t.Fatalf("skill status = %+v, want aiExists=true after confirmation", result.Skills[0])
	}
	if result.Skills[0].Generated {
		t.Fatalf("skill status = %+v, want generated=false", result.Skills[0])
	}
	if _, err := os.Stat(filepath.Join(baseDir, ".agents", "skills", "design-from-issue", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("design-from-issue should not be created, stat err=%v", err)
	}

	logFiles, err := filepath.Glob(filepath.Join(workDir, "logs", "skill", "*.log"))
	if err != nil {
		t.Fatalf("filepath.Glob() error = %v", err)
	}
	if len(logFiles) == 0 {
		t.Fatalf("expected skill generation log file in %s", filepath.Join(workDir, "logs", "skill"))
	}
	logContent, err := os.ReadFile(logFiles[0])
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(logContent), "request") || !strings.Contains(string(logContent), "complete") {
		t.Fatalf("skill log missing expected entries: %s", string(logContent))
	}
}

func TestGenerateSkillsCanForceAIExistingSkill(t *testing.T) {
	baseDir := t.TempDir()
	toolDir := t.TempDir()
	workDir := t.TempDir()
	installSkillGenerationPrompt(t, toolDir)
	testCommand := "go test ./..."
	path := filepath.Join(baseDir, ".github", "skills", "custom-issue-planner")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: custom-issue-planner\ndescription: Design a solution from a GitHub issue.\n---\n"
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	generator := NewSkillGeneratorWithFactory(baseDir, toolDir, workDir, staticSkillSettingsStore{
		settings: domain.WatchSettings{AIProvider: domain.AIProviderCodex},
	}, nil, func() SkillAgent {
		return &fakeSkillAgent{
			run: func(ctx context.Context, req AIRequest) (AIResponse, error) {
				if strings.Contains(req.Prompt, "Existing skill file:") {
					return AIResponse{RawOutput: `{"matches":true,"reason":"same workflow","confidence":"high"}`}, nil
				}
				if strings.Contains(req.Prompt, "Agent Skillを生成してください") {
					return AIResponse{}, writeGeneratedSkillFiles(req.WorkingDir, testCommand)
				}
				return AIResponse{}, errors.New("unexpected prompt")
			},
		}
	})

	result, err := generator.GenerateSkills(context.Background(), domain.SkillGenerationRequest{
		TestCommand:   "go test ./...",
		MaxFixLoops:   4,
		ForcePurposes: []domain.SkillPurpose{domain.SkillPurposeIssueDesign},
	})
	if err != nil {
		t.Fatalf("GenerateSkills() error = %v", err)
	}
	if !result.Skills[0].Generated {
		t.Fatalf("skill status = %+v, want generated=true after force create", result.Skills[0])
	}
	if !result.Skills[0].AIExists {
		t.Fatalf("skill status = %+v, want aiExists=true after force create", result.Skills[0])
	}
	if _, err := os.Stat(filepath.Join(baseDir, ".agents", "skills", "design-from-issue", "SKILL.md")); err != nil {
		t.Fatalf("design-from-issue should be created, stat err=%v", err)
	}
}

func TestGenerateSkillsCanOverwriteExistingSkill(t *testing.T) {
	baseDir := t.TempDir()
	toolDir := t.TempDir()
	workDir := t.TempDir()
	installSkillGenerationPrompt(t, toolDir)
	testCommand := "go test ./..."
	path := filepath.Join(baseDir, ".agents", "skills", "design-from-issue")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := "---\nname: design-from-issue\ndescription: stale skill\n---\n<!-- korobokcle-purpose: issue_design -->\nold content\n"
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	generator := NewSkillGeneratorWithFactory(baseDir, toolDir, workDir, staticSkillSettingsStore{
		settings: domain.WatchSettings{AIProvider: domain.AIProviderCodex},
	}, nil, func() SkillAgent {
		return &fakeSkillAgent{
			run: func(ctx context.Context, req AIRequest) (AIResponse, error) {
				if strings.Contains(req.Prompt, "Existing skill file:") {
					return AIResponse{RawOutput: `{"matches":true,"reason":"same workflow","confidence":"high"}`}, nil
				}
				if strings.Contains(req.Prompt, "Agent Skillを生成してください") {
					return AIResponse{}, writeGeneratedSkillFiles(req.WorkingDir, testCommand)
				}
				return AIResponse{}, errors.New("unexpected prompt")
			},
		}
	})

	result, err := generator.GenerateSkills(context.Background(), domain.SkillGenerationRequest{
		TestCommand:       "go test ./...",
		MaxFixLoops:       4,
		ForcePurposes:     []domain.SkillPurpose{domain.SkillPurposeIssueDesign},
		OverwriteExisting: true,
	})
	if err != nil {
		t.Fatalf("GenerateSkills() error = %v", err)
	}
	if !result.Skills[0].Generated {
		t.Fatalf("skill status = %+v, want generated=true after overwrite", result.Skills[0])
	}
	content, err := os.ReadFile(filepath.Join(baseDir, ".agents", "skills", "design-from-issue", "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(content), "generated-by: korobokcle") {
		t.Fatalf("overwrite did not replace existing content: %s", string(content))
	}
	if strings.Contains(string(content), "stale skill") {
		t.Fatalf("overwrite kept stale content: %s", string(content))
	}
}

func TestGenerateSkillsSkipsExistingSelectedSkillWithoutOverwrite(t *testing.T) {
	baseDir := t.TempDir()
	toolDir := t.TempDir()
	workDir := t.TempDir()
	installSkillGenerationPrompt(t, toolDir)
	testCommand := "go test ./..."
	path := filepath.Join(baseDir, ".agents", "skills", "design-from-issue")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := "---\nname: design-from-issue\ndescription: existing skill\n---\n<!-- korobokcle-purpose: issue_design -->\nexisting content\n"
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	runCount := 0
	generator := NewSkillGeneratorWithFactory(baseDir, toolDir, workDir, staticSkillSettingsStore{
		settings: domain.WatchSettings{AIProvider: domain.AIProviderCodex},
	}, nil, func() SkillAgent {
		return &fakeSkillAgent{
			run: func(ctx context.Context, req AIRequest) (AIResponse, error) {
				runCount++
				if strings.Contains(req.Prompt, "Existing skill file:") {
					return AIResponse{RawOutput: `{"matches":true,"reason":"same workflow","confidence":"high"}`}, nil
				}
				if strings.Contains(req.Prompt, "Agent Skillを生成してください") {
					return AIResponse{}, writeGeneratedSkillFiles(req.WorkingDir, testCommand)
				}
				return AIResponse{}, errors.New("unexpected prompt")
			},
		}
	})

	result, err := generator.GenerateSkills(context.Background(), domain.SkillGenerationRequest{
		TestCommand:   "go test ./...",
		MaxFixLoops:   4,
		ForcePurposes: []domain.SkillPurpose{domain.SkillPurposeIssueDesign},
	})
	if err != nil {
		t.Fatalf("GenerateSkills() error = %v", err)
	}
	if runCount != 0 {
		t.Fatalf("Run() count = %d, want 0 when existing selected skill is skipped", runCount)
	}
	if result.Message != "既存スキル 1 件をスキップしました。" {
		t.Fatalf("message = %q", result.Message)
	}
	content, err := os.ReadFile(filepath.Join(baseDir, ".agents", "skills", "design-from-issue", "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(content) != existing {
		t.Fatalf("existing skill was modified: %s", string(content))
	}
}

func TestValidateGeneratedImplementationSkill(t *testing.T) {
	dir := t.TempDir()
	content := `---
name: implement-from-design
description: 承認済みの設計に基づく実装結果の出力形式を規定する。
---
<!-- generated-by: korobokcle -->
<!-- korobokcle-purpose: issue_implementation -->
## 必須出力形式
概要、変更内容、テスト結果、残課題の順で出力する。
テスト結果にはコマンド go test ./...、実行結果、最大4回に対する修正回数を記載する。
`
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	definition := issueDrivenSkillDefinitions[1]
	request := domain.SkillGenerationRequest{TestCommand: "go test ./...", MaxFixLoops: 4}
	if err := validateGeneratedSkill(dir, definition, request); err != nil {
		t.Fatalf("validateGeneratedSkill() error = %v", err)
	}
}

func TestValidateGeneratedImplementationSkillWithMultipleCommands(t *testing.T) {
	dir := t.TempDir()
	content := `---
name: implement-from-design
description: 承認済みの設計に基づく実装結果の出力形式を規定する。
---
<!-- generated-by: korobokcle -->
<!-- korobokcle-purpose: issue_implementation -->
## 必須出力形式
概要、変更内容、テスト結果、残課題の順で出力する。
テスト結果には次のコマンドをすべて記載する。
- go test ./...
- go test ./internal/app
必要に応じて npm ci を実行する。
各コマンドの実行結果と、最大4回に対する修正回数を記載する。
`
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	definition := issueDrivenSkillDefinitions[1]
	request := domain.SkillGenerationRequest{TestCommand: "go test ./...\ngo test ./internal/app\n必要に応じて npm ci を実行する。", MaxFixLoops: 4}
	if err := validateGeneratedSkill(dir, definition, request); err != nil {
		t.Fatalf("validateGeneratedSkill() error = %v", err)
	}
}

func TestValidateGeneratedVerificationSkill(t *testing.T) {
	dir := t.TempDir()
	content := `---
name: verifier-from-design
description: 設計に基づく検証結果の出力形式を規定する。
---
<!-- generated-by: korobokcle -->
<!-- korobokcle-purpose: issue_verification -->
## 必須出力形式
判定結果、確認内容、検証結果、残課題の順で出力する。
検証結果には設計で指定されたテストコマンドを記載する。
npm test
テスト結果には npm test の実行結果を記載する。
各コマンドの実行結果と判定結果を記載する。
`
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	definition := issueDrivenSkillDefinitions[2]
	request := domain.SkillGenerationRequest{TestCommand: "npm test", MaxFixLoops: 4}
	if err := validateGeneratedSkill(dir, definition, request); err != nil {
		t.Fatalf("validateGeneratedSkill() error = %v", err)
	}
}

func TestValidateGeneratedConflictResolutionSkill(t *testing.T) {
	dir := t.TempDir()
	content := `---
name: resolve-pr-conflicts
description: PRのコンフリクト解消結果の出力形式を規定する。
---
<!-- generated-by: korobokcle -->
<!-- korobokcle-purpose: pr_conflict_resolution -->
## 必須出力形式
概要、確認した情報、解消方針、変更内容、テスト結果、残課題の順で出力する。
テスト結果には go test ./... の実行結果と修正回数を記載する。
`
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	definition := issueDrivenSkillDefinitions[len(issueDrivenSkillDefinitions)-1]
	request := domain.SkillGenerationRequest{TestCommand: "go test ./...", MaxFixLoops: 4}
	if err := validateGeneratedSkill(dir, definition, request); err != nil {
		t.Fatalf("validateGeneratedSkill() error = %v", err)
	}
}

func TestRenderSkillGenerationPromptReloadsEditedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skill_generation_prompt.tmpl")
	data := skillGenerationPromptData{ProviderDisplayName: "Codex"}
	if err := os.WriteFile(path, []byte("変更前: {{.ProviderDisplayName}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := renderSkillGenerationPrompt(path, data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("変更後: {{.ProviderDisplayName}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := renderSkillGenerationPrompt(path, data)
	if err != nil {
		t.Fatal(err)
	}
	if first != "変更前: Codex" || second != "変更後: Codex" {
		t.Fatalf("rendered prompts = %q, %q", first, second)
	}
}

func writeGeneratedSkillFiles(workDir string, testCommand string) error {
	if testCommand == "" {
		testCommand = "go test ./..."
	}
	files := map[string]string{
		"design-from-issue": `---
name: design-from-issue
description: GitHub Issueから作成する設計の出力形式を規定する。
---
<!-- generated-by: korobokcle -->
<!-- korobokcle-purpose: issue_design -->
## 必須出力形式
概要、要件、設計、変更対象ファイル、テスト計画、リスクの順で出力する。
`,
		"implement-from-design": `---
name: implement-from-design
description: Implement the approved design and verify it with tests.
---
<!-- generated-by: korobokcle -->
<!-- korobokcle-purpose: issue_implementation -->
## 必須出力形式
概要、変更内容、テスト結果、残課題の順で出力する。
テスト結果には次のコマンドを記載する。
` + testCommand + `
必要に応じて npm ci を実行する。
各コマンドの実行結果と、最大4回に対する修正回数を記載する。
`,
		"verifier-from-design": `---
name: verifier-from-design
description: Verify the implementation against the approved design and tests.
---
<!-- generated-by: korobokcle -->
<!-- korobokcle-purpose: issue_verification -->
## 必須出力形式
判定結果、確認内容、検証結果、残課題の順で出力する。
検証結果には設計で指定されたテストコマンドを記載する。
` + testCommand + `
テスト結果には ` + testCommand + ` の実行結果を記載する。
各コマンドの実行結果と、判定結果を記載する。
`,
		"review-pull-request": `---
name: review-pull-request
description: Review a pull request with an emphasis on defects and missing tests.
---
<!-- generated-by: korobokcle -->
<!-- korobokcle-purpose: pr_review -->
## 必須出力形式
指摘事項、確認事項、概要の順で出力する。
`,
		"design-review-fix": `---
name: design-review-fix
description: Rework a design based on review feedback.
---
<!-- generated-by: korobokcle -->
<!-- korobokcle-purpose: review_feedback_design -->
## 必須出力形式
概要、要件、設計、変更対象ファイル、テスト計画、リスクの順で出力する。
`,
		"review-comment-fix": `---
name: review-comment-fix
description: Implement review feedback and verify it with tests.
---
<!-- generated-by: korobokcle -->
<!-- korobokcle-purpose: review_feedback_implementation -->
## 必須出力形式
概要、変更内容、テスト結果、残課題の順で出力する。
テスト結果には次のコマンドを記載する。
` + testCommand + `
各コマンドの実行結果と、最大4回に対する修正回数を記載する。
`,
		"resolve-pr-conflicts": `---
name: resolve-pr-conflicts
description: PRのコンフリクトを解消し、検証結果をまとめる。
---
<!-- generated-by: korobokcle -->
<!-- korobokcle-purpose: pr_conflict_resolution -->
## 必須出力形式
概要、確認した情報、解消方針、変更内容、テスト結果、残課題の順で出力する。
テスト結果には次のコマンドを記載する。
` + testCommand + `
各コマンドの実行結果と、最大4回に対する修正回数を記載する。
`,
	}
	for name, content := range files {
		dir := filepath.Join(workDir, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func installSkillGenerationPrompt(t *testing.T, toolDir string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "prompt", "skill_generation_prompt.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(toolDir, "prompt")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skill_generation_prompt.tmpl"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

type staticSkillSettingsStore struct {
	settings domain.WatchSettings
}

func (s staticSkillSettingsStore) Load(context.Context) (domain.WatchSettings, error) {
	return domain.NormalizeWatchSettings(s.settings), nil
}

func (s staticSkillSettingsStore) Save(context.Context, domain.WatchSettings) error {
	return nil
}

type fakeSkillAgent struct {
	run func(context.Context, AIRequest) (AIResponse, error)
}

func (a *fakeSkillAgent) Start(context.Context, domain.AIProvider, string) error { return nil }

func (a *fakeSkillAgent) Stop(context.Context) error { return nil }

func (a *fakeSkillAgent) Run(ctx context.Context, req AIRequest) (AIResponse, error) {
	return a.run(ctx, req)
}
