package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

type SkillAgent interface {
	AIRunner
	Start(context.Context, domain.AIProvider, string) error
	Stop(context.Context) error
}

type SkillAgentFactory func() SkillAgent

type skillDefinition struct {
	purpose     domain.SkillPurpose
	name        string
	displayName string
	keywords    [][]string
}

type discoveredSkill struct {
	path       string
	content    string
	normalized string
}

type skillMatchDecision struct {
	Matches    bool   `json:"matches"`
	Reason     string `json:"reason,omitempty"`
	Confidence string `json:"confidence,omitempty"`
}

type skillMatchRecord struct {
	AIExists  bool   `json:"aiExists"`
	Path      string `json:"path,omitempty"`
	Generated bool   `json:"generated,omitempty"`
}

var issueDrivenSkillDefinitions = []skillDefinition{
	{domain.SkillPurposeIssueDesign, "design-from-issue", "Issueをもとに設計", [][]string{{"issue", "design"}, {"issue", "設計"}}},
	{domain.SkillPurposeIssueImplementation, "implement-from-design", "設計結果をもとに実装", [][]string{{"design", "implement"}, {"設計", "実装"}}},
	{domain.SkillPurposeIssueVerification, "verifier-from-design", "設計結果をもとに検証", [][]string{{"design", "verify"}, {"design", "verification"}, {"設計", "検証"}}},
	{domain.SkillPurposePRReview, "review-pull-request", "PRのレビュー", [][]string{{"pull request", "review"}, {"pr", "レビュー"}}},
	{domain.SkillPurposePRAcceptance, "acceptance-test", "受入基準に基づく動作確認", [][]string{{"acceptance", "playwright"}, {"受入確認", "playwright"}}},
	{domain.SkillPurposeReviewFeedbackImplement, "review-comment-fix", "レビュー指摘の実装", [][]string{{"review", "feedback", "implement"}, {"レビュー", "指摘", "実装"}}},
	{domain.SkillPurposePRConflictResolution, "resolve-pr-conflicts", "PRのコンフリクト解消", [][]string{{"pull request", "conflict", "resolve"}, {"pr", "コンフリクト", "解消"}}},
}

type SkillGenerator struct {
	baseDir        string
	toolDir        string
	workDir        string
	skillLogDir    string
	settings       SettingsStore
	logger         workflowLogger
	factory        SkillAgentFactory
	matchCachePath string
	mu             sync.Mutex
}

func NewSkillGenerator(baseDir, toolDir, workDir string, settings SettingsStore, logger workflowLogger) *SkillGenerator {
	return NewSkillGeneratorWithFactory(baseDir, toolDir, workDir, settings, logger, func() SkillAgent {
		return NewHTTPAIRunner(nil, logger)
	})
}

func NewSkillGeneratorWithFactory(baseDir, toolDir, workDir string, settings SettingsStore, logger workflowLogger, factory SkillAgentFactory) *SkillGenerator {
	return &SkillGenerator{
		baseDir:        baseDir,
		toolDir:        toolDir,
		workDir:        workDir,
		skillLogDir:    filepath.Join(workDir, "logs", "skill"),
		settings:       settings,
		logger:         logger,
		factory:        factory,
		matchCachePath: filepath.Join(workDir, "state", "skill-matches.json"),
	}
}

func (g *SkillGenerator) SkillStatus(ctx context.Context) ([]domain.SkillStatus, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.scanSkills(ctx)
}

func (g *SkillGenerator) GenerateSkills(ctx context.Context, req domain.SkillGenerationRequest) (domain.SkillGenerationResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	logRunID := strconv.FormatInt(time.Now().UnixNano(), 10)
	g.appendSkillGenerationLog(logRunID, "start", strings.Join([]string{
		fmt.Sprintf("base_dir: %s", g.baseDir),
		fmt.Sprintf("tool_dir: %s", g.toolDir),
		fmt.Sprintf("work_dir: %s", g.workDir),
		fmt.Sprintf("project_context: %s", strings.TrimSpace(req.ProjectContext)),
		fmt.Sprintf("test_command: %s", strings.TrimSpace(req.TestCommand)),
		fmt.Sprintf("force_purposes: %v", req.ForcePurposes),
		fmt.Sprintf("overwrite_existing: %t", req.OverwriteExisting),
	}, "\n"))

	if g.settings == nil || g.factory == nil {
		g.appendSkillGenerationLog(logRunID, "error", "skill generator is not configured")
		return domain.SkillGenerationResult{}, fmt.Errorf("skill generator is not configured")
	}
	settings, err := g.settings.Load(ctx)
	if err != nil {
		g.appendSkillGenerationLog(logRunID, "error", fmt.Sprintf("load settings: %v", err))
		return domain.SkillGenerationResult{}, err
	}
	settings = domain.NormalizeWatchSettings(settings)
	req.ProjectContext = strings.TrimSpace(req.ProjectContext)
	req.TestCommand = strings.TrimSpace(req.TestCommand)
	if req.TestCommand == "" {
		g.appendSkillGenerationLog(logRunID, "error", "testCommand is required")
		return domain.SkillGenerationResult{}, fmt.Errorf("testCommand is required")
	}
	forceTargets := make(map[domain.SkillPurpose]struct{}, len(req.ForcePurposes))
	for _, purpose := range req.ForcePurposes {
		forceTargets[purpose] = struct{}{}
	}
	selectedOnly := len(forceTargets) > 0

	statuses, err := g.scanSkills(ctx)
	if err != nil {
		g.appendSkillGenerationLog(logRunID, "error", fmt.Sprintf("scan skills: %v", err))
		return domain.SkillGenerationResult{}, err
	}
	discovered, err := g.discoverSkills()
	if err != nil {
		g.appendSkillGenerationLog(logRunID, "error", fmt.Sprintf("discover skills: %v", err))
		return domain.SkillGenerationResult{}, err
	}
	candidates := buildSkillCandidates(discovered)
	cache, err := g.loadSkillMatchCache()
	if err != nil {
		g.appendSkillGenerationLog(logRunID, "error", fmt.Sprintf("load skill match cache: %v", err))
		return domain.SkillGenerationResult{}, err
	}

	toCreate := make([]skillDefinition, 0)
	evaluated := make(map[domain.SkillPurpose]skillMatchRecord)
	skippedExisting := 0
	for _, definition := range issueDrivenSkillDefinitions {
		candidateSkills := candidates[definition.purpose]
		forced := false
		if _, ok := forceTargets[definition.purpose]; ok {
			forced = true
		}
		if selectedOnly && !forced {
			continue
		}
		if len(candidateSkills) == 0 {
			toCreate = append(toCreate, definition)
			continue
		}
		if forced {
			evaluated[definition.purpose] = skillMatchRecord{
				AIExists:  true,
				Path:      candidateSkills[0].path,
				Generated: hasGeneratedMarker(candidateSkills[0].normalized),
			}
			targetDir := filepath.Join(g.baseDir, ".agents", "skills", definition.name)
			if !req.OverwriteExisting && directoryExists(targetDir) {
				skippedExisting++
				g.appendSkillGenerationLog(logRunID, "skip", fmt.Sprintf("forced purpose target exists and overwrite disabled: %s", targetDir))
				continue
			}
			toCreate = append(toCreate, definition)
			continue
		}
		aiExists, err := g.confirmSkillMatch(ctx, settings, definition, candidateSkills, logRunID)
		if err != nil {
			g.appendSkillGenerationLog(logRunID, "error", fmt.Sprintf("confirm skill match purpose=%s: %v", definition.purpose, err))
			return domain.SkillGenerationResult{}, err
		}
		evaluated[definition.purpose] = skillMatchRecord{
			AIExists:  aiExists,
			Path:      candidateSkills[0].path,
			Generated: hasGeneratedMarker(candidateSkills[0].normalized),
		}
		if !aiExists {
			toCreate = append(toCreate, definition)
		}
	}
	for purpose, record := range evaluated {
		cache[string(purpose)] = record
	}
	if err := g.saveSkillMatchCache(cache); err != nil {
		g.appendSkillGenerationLog(logRunID, "error", fmt.Sprintf("save skill match cache: %v", err))
		return domain.SkillGenerationResult{}, err
	}
	if len(toCreate) == 0 {
		statuses, err = g.scanSkills(ctx)
		if err != nil {
			g.appendSkillGenerationLog(logRunID, "error", fmt.Sprintf("rescan skills: %v", err))
			return domain.SkillGenerationResult{}, err
		}
		message := "同等のスキルがすでに存在します。"
		if skippedExisting > 0 {
			message = fmt.Sprintf("既存スキル %d 件をスキップしました。", skippedExisting)
		}
		g.appendSkillGenerationLog(logRunID, "complete", message)
		return domain.SkillGenerationResult{Provider: settings.AIProvider, Skills: statuses, Message: message}, nil
	}

	stageDir := filepath.Join(g.workDir, "workspace", "skill-generation", strconv.FormatInt(time.Now().UnixNano(), 10))
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		g.appendSkillGenerationLog(logRunID, "error", fmt.Sprintf("create skill staging directory: %v", err))
		return domain.SkillGenerationResult{}, fmt.Errorf("create skill staging directory: %w", err)
	}
	defer os.RemoveAll(stageDir)

	agent := g.factory()
	if err := agent.Start(ctx, settings.AIProvider, stageDir); err != nil {
		return domain.SkillGenerationResult{}, err
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = agent.Stop(stopCtx)
	}()

	model := selectedModel(settings, providerKey(settings.AIProvider))
	prompt, err := buildSkillGenerationPrompt(g.toolDir, settings.AIProvider, toCreate, req, stageDir)
	if err != nil {
		g.appendSkillGenerationLog(logRunID, "error", fmt.Sprintf("build prompt: %v", err))
		return domain.SkillGenerationResult{}, err
	}
	if g.logger != nil {
		g.logger.Debugf("skill generation request provider=%s model=%s missing=%d stage=%s", settings.AIProvider, displayModel(model), len(toCreate), stageDir)
	}
	g.appendSkillGenerationLog(logRunID, "request", strings.Join([]string{
		fmt.Sprintf("provider: %s", settings.AIProvider),
		fmt.Sprintf("model: %s", displayModel(model)),
		fmt.Sprintf("missing: %d", len(toCreate)),
		fmt.Sprintf("stage: %s", stageDir),
		"",
		prompt,
	}, "\n"))
	response, err := agent.Run(ctx, AIRequest{
		Provider:   settings.AIProvider,
		Model:      settingsModel(settings),
		System:     "Create concise, reusable Agent Skills. Follow the requested file paths exactly and do not modify files outside the staging directory.",
		Prompt:     prompt,
		WorkingDir: stageDir,
	})
	if err != nil {
		g.appendSkillGenerationLog(logRunID, "error", fmt.Sprintf("ai run: %v", err))
		return domain.SkillGenerationResult{}, err
	}
	if g.logger != nil {
		g.logger.Debugf("skill generation response provider=%s output=%s", settings.AIProvider, response.RawOutput)
	}
	g.appendSkillGenerationLog(logRunID, "response", strings.Join([]string{
		fmt.Sprintf("provider: %s", settings.AIProvider),
		"",
		response.RawOutput,
	}, "\n"))

	for _, definition := range toCreate {
		sourceDir := filepath.Join(stageDir, definition.name)
		if err := validateGeneratedSkill(sourceDir, definition, req); err != nil {
			g.appendSkillGenerationLog(logRunID, "error", fmt.Sprintf("validate generated skill %s: %v", definition.name, err))
			return domain.SkillGenerationResult{}, err
		}
		g.appendSkillGenerationLog(logRunID, "validated", fmt.Sprintf("skill: %s", definition.name))
	}
	for _, definition := range toCreate {
		sourceDir := filepath.Join(stageDir, definition.name)
		targetDir := filepath.Join(g.baseDir, ".agents", "skills", definition.name)
		if _, err := os.Stat(targetDir); err == nil {
			if req.OverwriteExisting {
				if err := os.RemoveAll(targetDir); err != nil {
					g.appendSkillGenerationLog(logRunID, "error", fmt.Sprintf("remove target %s: %v", targetDir, err))
					return domain.SkillGenerationResult{}, err
				}
				g.appendSkillGenerationLog(logRunID, "overwrite", fmt.Sprintf("removed existing target: %s", targetDir))
			} else {
				g.appendSkillGenerationLog(logRunID, "skip", fmt.Sprintf("target exists: %s", targetDir))
				continue
			}
		} else if !os.IsNotExist(err) {
			g.appendSkillGenerationLog(logRunID, "error", fmt.Sprintf("stat target %s: %v", targetDir, err))
			return domain.SkillGenerationResult{}, err
		}
		if err := copySkillDirectory(sourceDir, targetDir); err != nil {
			g.appendSkillGenerationLog(logRunID, "error", fmt.Sprintf("copy skill %s: %v", definition.name, err))
			return domain.SkillGenerationResult{}, err
		}
		g.appendSkillGenerationLog(logRunID, "copied", fmt.Sprintf("%s -> %s", sourceDir, targetDir))
	}

	statuses, err = g.scanSkills(ctx)
	if err != nil {
		g.appendSkillGenerationLog(logRunID, "error", fmt.Sprintf("final rescan skills: %v", err))
		return domain.SkillGenerationResult{}, err
	}
	message := "不足していたスキルを生成しました。"
	if req.OverwriteExisting {
		message = "選択したスキルを再生成して上書きしました。"
	} else if skippedExisting > 0 {
		message = fmt.Sprintf("不足スキルを生成し、既存スキル %d 件をスキップしました。", skippedExisting)
	}
	g.appendSkillGenerationLog(logRunID, "complete", message)
	return domain.SkillGenerationResult{Provider: settings.AIProvider, Skills: statuses, Message: message}, nil
}

func settingsModel(settings domain.WatchSettings) string {
	return selectedModel(settings, providerKey(settings.AIProvider))
}

func (g *SkillGenerator) scanSkills(ctx context.Context) ([]domain.SkillStatus, error) {
	discovered, err := g.discoverSkills()
	if err != nil {
		return nil, err
	}
	cache, err := g.loadSkillMatchCache()
	if err != nil {
		return nil, err
	}
	return buildSkillStatuses(g.baseDir, discovered, cache), nil
}

func (g *SkillGenerator) discoverSkills() ([]discoveredSkill, error) {
	discovered := make([]discoveredSkill, 0)
	for _, root := range []string{
		filepath.Join(g.baseDir, ".agents", "skills"),
		filepath.Join(g.baseDir, ".github", "skills"),
		filepath.Join(g.baseDir, ".codex", "skills"),
	} {
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			path := filepath.Join(root, entry.Name(), "SKILL.md")
			raw, err := os.ReadFile(path)
			if err == nil {
				discovered = append(discovered, discoveredSkill{
					path:       path,
					content:    string(raw),
					normalized: strings.ToLower(entry.Name() + "\n" + string(raw)),
				})
			}
		}
	}
	return discovered, nil
}

func skillMatchesDefinition(content string, definition skillDefinition) bool {
	if strings.Contains(content, "korobokcle-purpose: "+string(definition.purpose)) || strings.Contains(content, "name: "+definition.name) {
		return true
	}
	switch definition.purpose {
	case domain.SkillPurposeIssueDesign, domain.SkillPurposeReviewFeedbackDesign:
		if containsAnyFold(content, "implement", "implementation", "実装") {
			return false
		}
	case domain.SkillPurposePRReview:
		if containsAnyFold(content, "feedback fix", "review fix", "指摘修正", "指摘対応") {
			return false
		}
	}
	for _, keywordSet := range definition.keywords {
		matched := true
		for _, keyword := range keywordSet {
			if !strings.Contains(content, keyword) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func buildSkillCandidates(discovered []discoveredSkill) map[domain.SkillPurpose][]discoveredSkill {
	candidates := make(map[domain.SkillPurpose][]discoveredSkill, len(issueDrivenSkillDefinitions))
	for _, definition := range issueDrivenSkillDefinitions {
		for _, skill := range discovered {
			if skillMatchesDefinition(skill.normalized, definition) {
				candidates[definition.purpose] = append(candidates[definition.purpose], skill)
			}
		}
	}
	return candidates
}

func buildSkillStatuses(baseDir string, discovered []discoveredSkill, cache map[string]skillMatchRecord) []domain.SkillStatus {
	statuses := make([]domain.SkillStatus, 0, len(issueDrivenSkillDefinitions))
	for _, definition := range issueDrivenSkillDefinitions {
		status := domain.SkillStatus{Purpose: definition.purpose, Name: definition.name, DisplayName: definition.displayName}
		candidates := make([]discoveredSkill, 0)
		for _, skill := range discovered {
			if skillMatchesDefinition(skill.normalized, definition) {
				candidates = append(candidates, skill)
			}
		}
		if len(candidates) > 0 {
			status.Exists = true
			status.Path = relativeSkillPath(baseDir, candidates[0].path)
			status.Generated = hasGeneratedMarker(candidates[0].normalized)
		}
		if record, ok := cache[string(definition.purpose)]; ok && record.AIExists && len(candidates) > 0 {
			status.AIExists = true
		}
		statuses = append(statuses, status)
	}
	return statuses
}

func relativeSkillPath(baseDir string, path string) string {
	rel, err := filepath.Rel(baseDir, path)
	if err != nil {
		return path
	}
	return rel
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func hasGeneratedMarker(content string) bool {
	return strings.Contains(content, "generated-by: korobokcle")
}

func (g *SkillGenerator) loadSkillMatchCache() (map[string]skillMatchRecord, error) {
	raw, err := os.ReadFile(g.matchCachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]skillMatchRecord), nil
		}
		return nil, fmt.Errorf("read skill match cache: %w", err)
	}
	cache := make(map[string]skillMatchRecord)
	if err := json.Unmarshal(raw, &cache); err != nil {
		return nil, fmt.Errorf("decode skill match cache: %w", err)
	}
	return cache, nil
}

func (g *SkillGenerator) confirmSkillMatch(ctx context.Context, settings domain.WatchSettings, definition skillDefinition, candidates []discoveredSkill, logRunID string) (bool, error) {
	if g.factory == nil {
		return false, fmt.Errorf("skill generator is not configured")
	}
	agent := g.factory()
	if err := agent.Start(ctx, settings.AIProvider, g.baseDir); err != nil {
		return false, err
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = agent.Stop(stopCtx)
	}()

	for _, skill := range candidates {
		prompt := buildSkillMatchPrompt(definition, skill)
		response, err := agent.Run(ctx, AIRequest{
			Provider:   settings.AIProvider,
			Model:      settingsModel(settings),
			System:     "Classify whether an existing Agent Skill is equivalent to a canonical issue-driven skill. Return only JSON.",
			Prompt:     prompt,
			WorkingDir: g.baseDir,
		})
		if err != nil {
			g.appendSkillGenerationLog(logRunID, "error", fmt.Sprintf("skill match ai purpose=%s path=%s: %v", definition.purpose, skill.path, err))
			return false, err
		}
		decision, err := parseSkillMatchDecision(response.RawOutput)
		if err != nil {
			g.appendSkillGenerationLog(logRunID, "error", fmt.Sprintf("parse skill match decision purpose=%s path=%s: %v raw=%s", definition.purpose, skill.path, err, response.RawOutput))
			return false, err
		}
		if g.logger != nil {
			g.logger.Debugf("skill match ai decision purpose=%s path=%s matches=%t confidence=%s reason=%s", definition.purpose, skill.path, decision.Matches, decision.Confidence, decision.Reason)
		}
		g.appendSkillGenerationLog(logRunID, "match", fmt.Sprintf("purpose=%s path=%s matches=%t confidence=%s reason=%s", definition.purpose, skill.path, decision.Matches, decision.Confidence, decision.Reason))
		if decision.Matches {
			return true, nil
		}
	}
	return false, nil
}

func (g *SkillGenerator) saveSkillMatchCache(cache map[string]skillMatchRecord) error {
	if err := os.MkdirAll(filepath.Dir(g.matchCachePath), 0o755); err != nil {
		return fmt.Errorf("create skill match cache dir: %w", err)
	}
	raw, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("encode skill match cache: %w", err)
	}
	if err := os.WriteFile(g.matchCachePath, raw, 0o644); err != nil {
		return fmt.Errorf("write skill match cache: %w", err)
	}
	return nil
}

func missingSkillDefinitions(statuses []domain.SkillStatus) []skillDefinition {
	exists := make(map[domain.SkillPurpose]bool, len(statuses))
	for _, status := range statuses {
		exists[status.Purpose] = status.Exists
	}
	missing := make([]skillDefinition, 0)
	for _, definition := range issueDrivenSkillDefinitions {
		if !exists[definition.purpose] {
			missing = append(missing, definition)
		}
	}
	return missing
}

func buildSkillGenerationPrompt(toolDir string, provider domain.AIProvider, missing []skillDefinition, req domain.SkillGenerationRequest, stageDir string) (string, error) {
	items := make([]skillPromptDefinition, 0, len(missing))
	for _, definition := range missing {
		items = append(items, skillPromptDefinition{
			Name:        definition.name,
			Purpose:     string(definition.purpose),
			DisplayName: definition.displayName,
		})
	}
	prompt, err := renderSkillGenerationPrompt(filepath.Join(toolDir, "prompt", "skill_generation_prompt.tmpl"), skillGenerationPromptData{
		ProviderDisplayName: provider.DisplayName(),
		StageDir:            stageDir,
		ProjectContext:      req.ProjectContext,
		TestCommand:         req.TestCommand,
		Missing:             items,
		IsCodex:             provider == domain.AIProviderCodex,
	})
	if err != nil {
		return "", err
	}
	return prompt, nil
}

func buildSkillMatchPrompt(definition skillDefinition, skill discoveredSkill) string {
	return strings.Join([]string{
		"You are checking whether an existing Agent Skill should be treated as the same skill as a canonical issue-driven skill.",
		"Return only JSON in the format {\"matches\":true|false,\"reason\":\"...\",\"confidence\":\"low|medium|high\"}.",
		"Use semantic meaning, not exact wording.",
		"Treat it as a match only when the skill clearly serves the same workflow role.",
		"",
		"Canonical skill:",
		"Purpose: " + string(definition.purpose),
		"Name: " + definition.name,
		"Display: " + definition.displayName,
		"Intent: " + skillDefinitionIntent(definition.purpose),
		"",
		"Existing skill file:",
		"Path: " + skill.path,
		"Content:",
		skill.content,
	}, "\n")
}

func skillDefinitionIntent(purpose domain.SkillPurpose) string {
	switch purpose {
	case domain.SkillPurposeIssueDesign:
		return "Issue設計の必須出力形式を規定する: 概要、要件、設計、変更対象ファイル、テスト計画、リスク。"
	case domain.SkillPurposeIssueImplementation:
		return "実装結果の必須出力形式を規定する: 概要、変更内容、テスト結果、残課題。"
	case domain.SkillPurposeIssueVerification:
		return "設計に基づく検証結果の必須出力形式を規定する: 概要、確認内容、検証結果、残課題。"
	case domain.SkillPurposePRReview:
		return "重要度とファイル・行番号を含む、指摘事項優先のPull Requestレビュー形式を規定する。"
	case domain.SkillPurposePRAcceptance:
		return "Issueの受入基準に基づき、必要な変更ではアプリケーションを起動してPlaywrightで動作確認し、不要な変更では理由を示して省略する。"
	case domain.SkillPurposeReviewFeedbackDesign:
		return "設計修正の必須出力形式を規定する: 概要、要件、設計、変更対象ファイル、テスト計画、リスク。"
	case domain.SkillPurposeReviewFeedbackImplement:
		return "レビュー対応結果の必須出力形式を規定する: 概要、変更内容、テスト結果、残課題。"
	case domain.SkillPurposePRConflictResolution:
		return "PRコンフリクト解消結果の必須出力形式を規定する: 概要、確認した情報、解消方針、変更内容、テスト結果、残課題。"
	default:
		return ""
	}
}

func parseSkillMatchDecision(raw string) (skillMatchDecision, error) {
	raw = strings.TrimSpace(stripLeadingNoise(raw))
	if extracted, ok := extractFirstJSONObject(raw); ok {
		raw = extracted
	}
	var decision skillMatchDecision
	if err := json.Unmarshal([]byte(raw), &decision); err != nil {
		return skillMatchDecision{}, fmt.Errorf("decode skill match response: %w", err)
	}
	return decision, nil
}

func validateGeneratedSkill(dir string, definition skillDefinition, req domain.SkillGenerationRequest) error {
	path := filepath.Join(dir, "SKILL.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("generated skill %s is missing SKILL.md: %w", definition.name, err)
	}
	content := string(raw)
	if !strings.HasPrefix(strings.TrimSpace(content), "---") || !strings.Contains(content, "name: "+definition.name) || !strings.Contains(content, "description:") {
		return fmt.Errorf("generated skill %s has invalid frontmatter", definition.name)
	}
	if !strings.Contains(content, "korobokcle-purpose: "+string(definition.purpose)) {
		return fmt.Errorf("generated skill %s is missing purpose marker", definition.name)
	}
	if !strings.Contains(content, "必須出力形式") {
		return fmt.Errorf("generated skill %s is missing required output format", definition.name)
	}
	if definition.purpose == domain.SkillPurposeIssueDesign || definition.purpose == domain.SkillPurposeReviewFeedbackDesign {
		if !containsAllFold(content, "概要", "要件", "設計", "変更対象ファイル", "テスト計画", "リスク") {
			return fmt.Errorf("generated design skill %s is missing required output sections", definition.name)
		}
	}
	if definition.purpose == domain.SkillPurposeIssueImplementation || definition.purpose == domain.SkillPurposeReviewFeedbackImplement {
		if !containsAllFold(content, "概要", "変更内容", "テスト結果", "残課題") {
			return fmt.Errorf("generated implementation skill %s is missing required output sections or test command", definition.name)
		}
		if !containsAllCommandsFold(content, req.TestCommand) {
			return fmt.Errorf("generated implementation skill %s is missing required output sections or test command", definition.name)
		}
	}
	if definition.purpose == domain.SkillPurposeIssueVerification {
		if !containsAllFold(content, "判定結果", "確認内容", "検証結果", "残課題") {
			return fmt.Errorf("generated verification skill %s is missing required output sections or test command", definition.name)
		}
		if !containsAllCommandsFold(content, req.TestCommand) {
			return fmt.Errorf("generated verification skill %s is missing required output sections or test command", definition.name)
		}
	}
	if definition.purpose == domain.SkillPurposePRAcceptance {
		if !containsAllFold(content, "判定結果", "確認内容", "受入確認結果", "残課題", "Playwright", "動作確認が不要") {
			return fmt.Errorf("generated acceptance test skill %s is missing required output sections or acceptance instructions", definition.name)
		}
	}
	if definition.purpose == domain.SkillPurposePRConflictResolution {
		if !containsAllFold(content, "概要", "確認した情報", "解消方針", "変更内容", "テスト結果", "残課題") || !containsAllCommandsFold(content, req.TestCommand) {
			return fmt.Errorf("generated conflict resolution skill %s is missing required output sections or test command", definition.name)
		}
	}
	return nil
}

func containsAllFold(content string, values ...string) bool {
	content = strings.ToLower(content)
	for _, value := range values {
		if !strings.Contains(content, strings.ToLower(value)) {
			return false
		}
	}
	return true
}

func containsAnyFold(content string, values ...string) bool {
	content = strings.ToLower(content)
	for _, value := range values {
		if strings.Contains(content, strings.ToLower(value)) {
			return true
		}
	}
	return false
}

func containsAllCommandsFold(content string, commands string) bool {
	content = strings.ToLower(content)
	for _, command := range splitCommandLines(commands) {
		if !strings.Contains(content, strings.ToLower(command)) {
			return false
		}
	}
	return true
}

func splitCommandLines(commands string) []string {
	lines := strings.Split(strings.ReplaceAll(commands, "\r\n", "\n"), "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !looksLikeShellCommand(line) {
			continue
		}
		normalized = append(normalized, line)
	}
	return normalized
}

func looksLikeShellCommand(line string) bool {
	if strings.Contains(line, "&&") || strings.Contains(line, "|") || strings.Contains(line, ">") || strings.Contains(line, "<") {
		return true
	}
	if strings.HasPrefix(line, "cd ") || strings.HasPrefix(line, "go ") || strings.HasPrefix(line, "npm ") || strings.HasPrefix(line, "pnpm ") || strings.HasPrefix(line, "yarn ") || strings.HasPrefix(line, "bun ") || strings.HasPrefix(line, "make ") {
		return true
	}
	if strings.HasPrefix(line, "./") || strings.HasPrefix(line, ".\\") || strings.HasPrefix(line, "python ") || strings.HasPrefix(line, "node ") {
		return true
	}
	return false
}

func copySkillDirectory(source, target string) error {
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("generated skill contains unsupported symbolic link: %s", path)
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, raw, 0o644)
	})
}

func (g *SkillGenerator) appendSkillGenerationLog(runID string, section string, content string) {
	if g == nil {
		return
	}
	if err := os.MkdirAll(g.skillLogDir, 0o755); err != nil {
		return
	}
	logPath := filepath.Join(g.skillLogDir, runID+".log")
	entry := strings.Join([]string{
		fmt.Sprintf("=== %s %s ===", time.Now().Format(time.RFC3339), section),
		content,
		"",
	}, "\n")
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.WriteString(entry)
}
