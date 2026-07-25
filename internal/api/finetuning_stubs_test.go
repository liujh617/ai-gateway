package api_test

import (
	"context"
	"errors"

	"open-ai-gateway/internal/compat"
)

func (p *blockingProvider) CreateFineTuningJob(ctx context.Context, req compat.FineTuningJobRequest) (*compat.FineTuningJob, error) {
	return nil, errors.New("not implemented")
}
func (p *blockingProvider) ListFineTuningJobs(ctx context.Context, after string, limit int) (*compat.FineTuningJobList, error) {
	return nil, errors.New("not implemented")
}
func (p *blockingProvider) RetrieveFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) {
	return nil, errors.New("not implemented")
}
func (p *blockingProvider) ListFineTuningJobEvents(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobEventList, error) {
	return nil, errors.New("not implemented")
}
func (p *blockingProvider) CancelFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) {
	return nil, errors.New("not implemented")
}
func (p *blockingProvider) ListFineTuningJobCheckpoints(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobCheckpointList, error) {
	return nil, errors.New("not implemented")
}

func (p *captureProvider) CreateFineTuningJob(ctx context.Context, req compat.FineTuningJobRequest) (*compat.FineTuningJob, error) {
	return nil, errors.New("not implemented")
}
func (p *captureProvider) ListFineTuningJobs(ctx context.Context, after string, limit int) (*compat.FineTuningJobList, error) {
	return nil, errors.New("not implemented")
}
func (p *captureProvider) RetrieveFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) {
	return nil, errors.New("not implemented")
}
func (p *captureProvider) ListFineTuningJobEvents(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobEventList, error) {
	return nil, errors.New("not implemented")
}
func (p *captureProvider) CancelFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) {
	return nil, errors.New("not implemented")
}
func (p *captureProvider) ListFineTuningJobCheckpoints(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobCheckpointList, error) {
	return nil, errors.New("not implemented")
}

func (p *countingProvider) CreateFineTuningJob(ctx context.Context, req compat.FineTuningJobRequest) (*compat.FineTuningJob, error) {
	return nil, errors.New("not implemented")
}
func (p *countingProvider) ListFineTuningJobs(ctx context.Context, after string, limit int) (*compat.FineTuningJobList, error) {
	return nil, errors.New("not implemented")
}
func (p *countingProvider) RetrieveFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) {
	return nil, errors.New("not implemented")
}
func (p *countingProvider) ListFineTuningJobEvents(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobEventList, error) {
	return nil, errors.New("not implemented")
}
func (p *countingProvider) CancelFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) {
	return nil, errors.New("not implemented")
}
func (p *countingProvider) ListFineTuningJobCheckpoints(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobCheckpointList, error) {
	return nil, errors.New("not implemented")
}

func (p *usageStreamProvider) CreateFineTuningJob(ctx context.Context, req compat.FineTuningJobRequest) (*compat.FineTuningJob, error) {
	return nil, errors.New("not implemented")
}
func (p *usageStreamProvider) ListFineTuningJobs(ctx context.Context, after string, limit int) (*compat.FineTuningJobList, error) {
	return nil, errors.New("not implemented")
}
func (p *usageStreamProvider) RetrieveFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) {
	return nil, errors.New("not implemented")
}
func (p *usageStreamProvider) ListFineTuningJobEvents(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobEventList, error) {
	return nil, errors.New("not implemented")
}
func (p *usageStreamProvider) CancelFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) {
	return nil, errors.New("not implemented")
}
func (p *usageStreamProvider) ListFineTuningJobCheckpoints(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobCheckpointList, error) {
	return nil, errors.New("not implemented")
}

func (p *slowProvider) CreateFineTuningJob(ctx context.Context, req compat.FineTuningJobRequest) (*compat.FineTuningJob, error) {
	return nil, errors.New("not implemented")
}
func (p *slowProvider) ListFineTuningJobs(ctx context.Context, after string, limit int) (*compat.FineTuningJobList, error) {
	return nil, errors.New("not implemented")
}
func (p *slowProvider) RetrieveFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) {
	return nil, errors.New("not implemented")
}
func (p *slowProvider) ListFineTuningJobEvents(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobEventList, error) {
	return nil, errors.New("not implemented")
}
func (p *slowProvider) CancelFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) {
	return nil, errors.New("not implemented")
}
func (p *slowProvider) ListFineTuningJobCheckpoints(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobCheckpointList, error) {
	return nil, errors.New("not implemented")
}

func (p *delayedStreamProvider) CreateFineTuningJob(ctx context.Context, req compat.FineTuningJobRequest) (*compat.FineTuningJob, error) {
	return nil, errors.New("not implemented")
}
func (p *delayedStreamProvider) ListFineTuningJobs(ctx context.Context, after string, limit int) (*compat.FineTuningJobList, error) {
	return nil, errors.New("not implemented")
}
func (p *delayedStreamProvider) RetrieveFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) {
	return nil, errors.New("not implemented")
}
func (p *delayedStreamProvider) ListFineTuningJobEvents(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobEventList, error) {
	return nil, errors.New("not implemented")
}
func (p *delayedStreamProvider) CancelFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) {
	return nil, errors.New("not implemented")
}
func (p *delayedStreamProvider) ListFineTuningJobCheckpoints(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobCheckpointList, error) {
	return nil, errors.New("not implemented")
}

func (p *sleepyChatProvider) CreateFineTuningJob(ctx context.Context, req compat.FineTuningJobRequest) (*compat.FineTuningJob, error) {
	return nil, errors.New("not implemented")
}
func (p *sleepyChatProvider) ListFineTuningJobs(ctx context.Context, after string, limit int) (*compat.FineTuningJobList, error) {
	return nil, errors.New("not implemented")
}
func (p *sleepyChatProvider) RetrieveFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) {
	return nil, errors.New("not implemented")
}
func (p *sleepyChatProvider) ListFineTuningJobEvents(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobEventList, error) {
	return nil, errors.New("not implemented")
}
func (p *sleepyChatProvider) CancelFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) {
	return nil, errors.New("not implemented")
}
func (p *sleepyChatProvider) ListFineTuningJobCheckpoints(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobCheckpointList, error) {
	return nil, errors.New("not implemented")
}
