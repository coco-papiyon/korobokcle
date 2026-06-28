package app

import (
	"context"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

type CompositeJobSource struct {
	sources []JobSource
}

func NewCompositeJobSource(sources ...JobSource) *CompositeJobSource {
	return &CompositeJobSource{sources: append([]JobSource(nil), sources...)}
}

func (s *CompositeJobSource) List(ctx context.Context) ([]domain.Job, error) {
	jobs := make([]domain.Job, 0)
	for _, source := range s.sources {
		if source == nil {
			continue
		}
		items, err := source.List(ctx)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, items...)
	}
	return jobs, nil
}
