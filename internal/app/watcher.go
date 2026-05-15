package app

import (
	"context"
	"log"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
	gh "github.com/coco-papiyon/korobokcle/internal/github"
	"github.com/coco-papiyon/korobokcle/internal/orchestrator"
)

func startWatcher(ctx context.Context, cfg *config.Service, orch *orchestrator.Orchestrator, logger *log.Logger, debugLogger *log.Logger) {
	tokenProvider := gh.NewGHTokenProvider(10 * time.Minute)
	client := gh.NewClient(tokenProvider, debugLogger)
	poller := gh.NewPoller(client, func() []config.WatchRule {
		return cfg.WatchRules().Rules
	}, debugLogger)
	watcher := gh.NewWatcher(poller, cfg.App().PollInterval, logger, debugLogger)
	events := make(chan domain.DomainEvent, 16)

	go func() {
		if err := watcher.Start(ctx, events); err != nil && ctx.Err() == nil {
			logger.Printf("watcher stopped with error: %v", err)
		}
		close(events)
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-events:
				if !ok {
					return
				}
				rulesByID := make(map[string]config.WatchRule)
				for _, rule := range cfg.WatchRules().Rules {
					rulesByID[rule.ID] = rule
				}
				rule, exists := rulesByID[event.RuleID]
				if !exists {
					logger.Printf("matched event ignored: rule %q not found", event.RuleID)
					continue
				}
				if debugLogger != nil {
					debugLogger.Printf("processing matched event type=%s jobTarget=%s repository=%s number=%d rule=%s", event.Type, event.Item.Target, event.Item.Repository, event.Item.Number, event.RuleID)
				}
				if err := orch.ProcessMatch(ctx, rule, event); err != nil {
					logger.Printf("process match failed for %s#%d: %v", event.Item.Repository, event.Item.Number, err)
					if debugLogger != nil {
						debugLogger.Printf("processing matched event failed repository=%s number=%d error=%v", event.Item.Repository, event.Item.Number, err)
					}
					continue
				}
				if debugLogger != nil {
					debugLogger.Printf("processing matched event completed repository=%s number=%d rule=%s", event.Item.Repository, event.Item.Number, event.RuleID)
				}
			}
		}
	}()
}
