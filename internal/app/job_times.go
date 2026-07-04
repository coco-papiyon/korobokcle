package app

import (
	"time"
	"strings"

	"github.com/coco-papiyon/korobokcle/internal/domain"
)

func markJobFetchedAt(job domain.Job) domain.Job {
	if job.FetchedAt.IsZero() {
		job.FetchedAt = time.Now().UTC()
	}
	return job
}

func markJobUpdatedAt(job domain.Job) domain.Job {
	job.UpdatedAt = time.Now().UTC()
	return job
}

func markJobState(job domain.Job, state domain.JobState) domain.Job {
	if job.State == state {
		return job
	}
	job.State = state
	job.UpdatedAt = time.Now().UTC()
	return job
}

func markJobSubStatus(job domain.Job, subStatus string) domain.Job {
	subStatus = strings.TrimSpace(subStatus)
	if job.SubStatus == subStatus {
		return job
	}
	job.SubStatus = subStatus
	job.UpdatedAt = time.Now().UTC()
	return job
}
