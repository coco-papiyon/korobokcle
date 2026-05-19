package github

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coco-papiyon/korobokcle/internal/config"
	"github.com/coco-papiyon/korobokcle/internal/domain"
)

type countingRepositoryLister struct {
	mu        sync.Mutex
	listCalls int
}

func (s *countingRepositoryLister) ListIssues(context.Context, config.WatchRule, string, time.Time) ([]domain.RepositoryItem, error) {
	s.mu.Lock()
	s.listCalls++
	s.mu.Unlock()
	return nil, nil
}

func (s *countingRepositoryLister) ListProjectIssues(context.Context, config.WatchRule, string, time.Time) ([]domain.RepositoryItem, error) {
	return nil, nil
}

func (s *countingRepositoryLister) ListPullRequests(context.Context, config.WatchRule, string, time.Time) ([]domain.RepositoryItem, error) {
	return nil, nil
}

func (s *countingRepositoryLister) ListPullRequestReviews(context.Context, string, time.Time) ([]domain.RepositoryItem, error) {
	return nil, nil
}

func TestWatcherSkipsPollingWhenIntervalIsZero(t *testing.T) {
	t.Parallel()

	lister := &countingRepositoryLister{}
	poller := NewPoller(lister, func() []config.WatchRule {
		return []config.WatchRule{
			{
				ID:           "rule-1",
				Enabled:      true,
				Repositories: []string{"owner/repo"},
				Target:       "issue",
			},
		}
	}, nil)

	var intervalNS atomic.Int64
	intervalNS.Store(int64(0))
	watcher := NewWatcher(poller, func() time.Duration {
		return time.Duration(intervalNS.Load())
	}, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan domain.DomainEvent, 1)
	done := make(chan error, 1)
	go func() {
		done <- watcher.Start(ctx, events)
	}()

	time.Sleep(500 * time.Millisecond)
	lister.mu.Lock()
	if lister.listCalls != 0 {
		lister.mu.Unlock()
		t.Fatalf("expected no polling while interval is zero, got %d calls", lister.listCalls)
	}
	lister.mu.Unlock()

	intervalNS.Store(int64(time.Second))
	time.Sleep(1500 * time.Millisecond)

	lister.mu.Lock()
	calls := lister.listCalls
	lister.mu.Unlock()
	if calls == 0 {
		t.Fatal("expected polling to resume after interval became positive")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("watcher returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop after context cancellation")
	}
}
