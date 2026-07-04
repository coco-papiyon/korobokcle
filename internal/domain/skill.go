package domain

import (
	"encoding/json"
	"fmt"
)

type SkillPurpose string

const (
	SkillPurposeIssueDesign             SkillPurpose = "issue_design"
	SkillPurposeIssueImplementation     SkillPurpose = "issue_implementation"
	SkillPurposeIssueVerification       SkillPurpose = "issue_verification"
	SkillPurposePRReview                SkillPurpose = "pr_review"
	SkillPurposePRConflictResolution    SkillPurpose = "pr_conflict_resolution"
	SkillPurposeReviewFeedbackDesign    SkillPurpose = "review_feedback_design"
	SkillPurposeReviewFeedbackImplement SkillPurpose = "review_feedback_implementation"
)

type SkillStatus struct {
	Purpose     SkillPurpose `json:"purpose"`
	Name        string       `json:"name"`
	DisplayName string       `json:"displayName"`
	Exists      bool         `json:"exists"`
	AIExists    bool         `json:"aiExists"`
	Generated   bool         `json:"generated"`
	Path        string       `json:"path,omitempty"`
}

type SkillGenerationRequest struct {
	ProjectContext    string        `json:"projectContext"`
	TestCommand       string        `json:"testCommand"`
	MaxFixLoops       int           `json:"maxFixLoops"`
	ForcePurposes     SkillPurposes `json:"forcePurposes,omitempty"`
	OverwriteExisting bool          `json:"overwriteExisting,omitempty"`
}

type SkillPurposes []SkillPurpose

func (p *SkillPurposes) UnmarshalJSON(data []byte) error {
	trimmed := string(data)
	if trimmed == "null" {
		*p = nil
		return nil
	}

	var list []SkillPurpose
	if err := json.Unmarshal(data, &list); err == nil {
		*p = list
		return nil
	}

	var single SkillPurpose
	if err := json.Unmarshal(data, &single); err == nil {
		*p = SkillPurposes{single}
		return nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err == nil {
		for _, key := range []string{"purpose", "value", "name"} {
			raw, ok := obj[key]
			if !ok {
				continue
			}
			if err := json.Unmarshal(raw, &single); err != nil {
				return fmt.Errorf("decode forcePurposes.%s: %w", key, err)
			}
			*p = SkillPurposes{single}
			return nil
		}

		purposes := make([]SkillPurpose, 0, len(obj))
		for key, raw := range obj {
			if key == "" {
				continue
			}
			var enabled bool
			if err := json.Unmarshal(raw, &enabled); err == nil {
				if enabled {
					purposes = append(purposes, SkillPurpose(key))
				}
				continue
			}
			var value string
			if err := json.Unmarshal(raw, &value); err == nil {
				value = string(SkillPurpose(value))
				switch value {
				case "", "false", "0":
					continue
				}
				purposes = append(purposes, SkillPurpose(key))
				continue
			}
			if string(raw) != "null" {
				purposes = append(purposes, SkillPurpose(key))
			}
		}
		if len(purposes) > 0 {
			*p = purposes
			return nil
		}
	}

	return fmt.Errorf("decode forcePurposes: expected array, string, or object")
}

func (p SkillPurposes) MarshalJSON() ([]byte, error) {
	return json.Marshal([]SkillPurpose(p))
}

type SkillGenerationResult struct {
	Provider AIProvider    `json:"provider"`
	Skills   []SkillStatus `json:"skills"`
	Message  string        `json:"message"`
}
