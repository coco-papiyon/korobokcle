package issuebody

import (
	"encoding/json"
	"fmt"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

const EventTypeRefreshed = "issue_body_refreshed"

type Snapshot struct {
	Body      string
	Author    string
	Labels    []string
	Assignees []string
}

func Resolve(events []domain.Event) (Snapshot, error) {
	var snapshot Snapshot
	bodyResolved := false
	metadataResolved := false
	var firstErr error

	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]

		switch event.EventType {
		case EventTypeRefreshed:
			if bodyResolved {
				continue
			}
			var payload struct {
				Body string `json:"body"`
			}
			if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("decode issue body refreshed payload: %w", err)
				}
				continue
			}
			snapshot.Body = payload.Body
			bodyResolved = true
		case string(domain.DomainEventIssueMatched):
			var payload struct {
				Body      string   `json:"body"`
				Author    string   `json:"author"`
				Labels    []string `json:"labels"`
				Assignees []string `json:"assignees"`
			}
			if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("decode issue matched payload: %w", err)
				}
				continue
			}
			if !bodyResolved {
				snapshot.Body = payload.Body
				bodyResolved = true
			}
			if !metadataResolved {
				snapshot.Author = payload.Author
				snapshot.Labels = append([]string(nil), payload.Labels...)
				snapshot.Assignees = append([]string(nil), payload.Assignees...)
				metadataResolved = true
			}
		}

		if bodyResolved && metadataResolved {
			break
		}
	}

	if bodyResolved || metadataResolved {
		return snapshot, nil
	}
	if firstErr != nil {
		return Snapshot{}, firstErr
	}
	return Snapshot{}, nil
}
