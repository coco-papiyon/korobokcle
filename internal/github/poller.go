package github

import (
	"context"
	"log"
	"sort"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
)

type RepositoryLister interface {
	ListIssues(ctx context.Context, repository string, since time.Time) ([]domain.RepositoryItem, error)
	ListPullRequests(ctx context.Context, repository string, since time.Time) ([]domain.RepositoryItem, error)
}

type WatchRuleProvider func() []config.WatchRule

type Poller struct {
	client     RepositoryLister
	rules      WatchRuleProvider
	lastSeenAt map[string]time.Time
	debug      *log.Logger
}

func NewPoller(client RepositoryLister, rules WatchRuleProvider, debug *log.Logger) *Poller {
	return &Poller{
		client:     client,
		rules:      rules,
		lastSeenAt: make(map[string]time.Time),
		debug:      debug,
	}
}

func (p *Poller) Poll(ctx context.Context) ([]domain.DomainEvent, error) {
	var events []domain.DomainEvent
	rules := p.rules()
	p.debugf("poll cycle started rules=%d", len(rules))

	for _, rule := range rules {
		if !rule.Enabled {
			p.debugf("poll rule skipped disabled rule=%s", rule.ID)
			continue
		}
		for _, repository := range rule.Repositories {
			key := repository + ":" + rule.Target
			since := p.lastSeenAt[key]
			p.debugf("poll repository start rule=%s target=%s repository=%s since=%s", rule.ID, rule.Target, repository, formatSince(since))

			items, err := p.listItems(ctx, repository, rule.Target, since)
			if err != nil {
				log.Printf("poller skipped repository=%q target=%q rule=%q: %v", repository, rule.Target, rule.ID, err)
				p.debugf("poll repository failed rule=%s target=%s repository=%s error=%v", rule.ID, rule.Target, repository, err)
				continue
			}
			p.debugf("poll repository fetched rule=%s target=%s repository=%s items=%d", rule.ID, rule.Target, repository, len(items))

			matchedCount := 0
			for _, item := range items {
				result := domain.EvaluateWatchRule(rule, item)
				if result.Status != domain.MatchStatusMatched {
					p.debugf("poll item ignored rule=%s repository=%s number=%d reason=%s", rule.ID, item.Repository, item.Number, result.Reason)
					continue
				}
				matchedCount++
				events = append(events, domain.DomainEvent{
					Type:      eventTypeFor(item.Target),
					RuleID:    rule.ID,
					RuleName:  rule.Name,
					Item:      item,
					MatchedAt: time.Now().UTC(),
				})
				p.debugf("poll item matched rule=%s repository=%s number=%d target=%s title=%q", rule.ID, item.Repository, item.Number, item.Target, item.Title)
			}
			p.debugf("poll repository result rule=%s repository=%s matched=%d", rule.ID, repository, matchedCount)

			if latest := latestUpdatedAt(items); latest.After(since) {
				p.lastSeenAt[key] = latest
				p.debugf("poll repository checkpoint updated rule=%s repository=%s updatedAt=%s", rule.ID, repository, latest.Format(time.RFC3339))
			}
		}
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Item.UpdatedAt.Before(events[j].Item.UpdatedAt)
	})
	p.debugf("poll cycle completed events=%d", len(events))
	return events, nil
}

type Watcher struct {
	poller   *Poller
	interval time.Duration
	logger   *log.Logger
	debug    *log.Logger
}

func NewWatcher(poller *Poller, interval time.Duration, logger *log.Logger, debug *log.Logger) *Watcher {
	return &Watcher{poller: poller, interval: interval, logger: logger, debug: debug}
}

func (w *Watcher) Start(ctx context.Context, out chan<- domain.DomainEvent) error {
	w.debugf("watcher started interval=%s", w.interval)
	if err := w.pollOnce(ctx, out); err != nil {
		return err
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.debugf("watcher stopped by context")
			return nil
		case <-ticker.C:
			w.debugf("watcher ticker fired")
			if err := w.pollOnce(ctx, out); err != nil {
				return err
			}
		}
	}
}

func (w *Watcher) pollOnce(ctx context.Context, out chan<- domain.DomainEvent) error {
	w.debugf("watcher pollOnce begin")
	events, err := w.poller.Poll(ctx)
	if err != nil {
		return err
	}
	w.debugf("watcher pollOnce result events=%d", len(events))

	for _, event := range events {
		if w.logger != nil {
			w.logger.Printf("watcher matched %s %s#%d via rule=%s", event.Item.Target, event.Item.Repository, event.Item.Number, event.RuleID)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case out <- event:
			w.debugf("watcher event queued repository=%s number=%d rule=%s", event.Item.Repository, event.Item.Number, event.RuleID)
		}
	}
	return nil
}

func (p *Poller) debugf(format string, args ...any) {
	if p.debug != nil {
		p.debug.Printf(format, args...)
	}
}

func (w *Watcher) debugf(format string, args ...any) {
	if w.debug != nil {
		w.debug.Printf(format, args...)
	}
}

func formatSince(value time.Time) string {
	if value.IsZero() {
		return "zero"
	}
	return value.Format(time.RFC3339)
}

func (p *Poller) listItems(ctx context.Context, repository string, target string, since time.Time) ([]domain.RepositoryItem, error) {
	switch target {
	case string(domain.TargetIssue):
		return p.client.ListIssues(ctx, repository, since)
	case string(domain.TargetPullRequest):
		return p.client.ListPullRequests(ctx, repository, since)
	default:
		return nil, nil
	}
}

func latestUpdatedAt(items []domain.RepositoryItem) time.Time {
	var latest time.Time
	for _, item := range items {
		if item.UpdatedAt.After(latest) {
			latest = item.UpdatedAt
		}
	}
	return latest
}

func eventTypeFor(target domain.MonitoredTarget) domain.DomainEventType {
	if target == domain.TargetPullRequest {
		return domain.DomainEventPRMatched
	}
	return domain.DomainEventIssueMatched
}
