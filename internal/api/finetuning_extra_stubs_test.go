package api_test

import (
	"context"
	"errors"

	"open-ai-gateway/internal/compat"
)

func (p *captureCompletionProvider) CreateFineTuningJob(ctx context.Context, req compat.FineTuningJobRequest) (*compat.FineTuningJob, error) { return nil, errors.New("not implemented") }
func (p *captureCompletionProvider) ListFineTuningJobs(ctx context.Context, after string, limit int) (*compat.FineTuningJobList, error) { return nil, errors.New("not implemented") }
func (p *captureCompletionProvider) RetrieveFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) { return nil, errors.New("not implemented") }
func (p *captureCompletionProvider) ListFineTuningJobEvents(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobEventList, error) { return nil, errors.New("not implemented") }
func (p *captureCompletionProvider) CancelFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) { return nil, errors.New("not implemented") }
func (p *captureCompletionProvider) ListFineTuningJobCheckpoints(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobCheckpointList, error) { return nil, errors.New("not implemented") }

func (p *countingCompletionProvider) CreateFineTuningJob(ctx context.Context, req compat.FineTuningJobRequest) (*compat.FineTuningJob, error) { return nil, errors.New("not implemented") }
func (p *countingCompletionProvider) ListFineTuningJobs(ctx context.Context, after string, limit int) (*compat.FineTuningJobList, error) { return nil, errors.New("not implemented") }
func (p *countingCompletionProvider) RetrieveFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) { return nil, errors.New("not implemented") }
func (p *countingCompletionProvider) ListFineTuningJobEvents(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobEventList, error) { return nil, errors.New("not implemented") }
func (p *countingCompletionProvider) CancelFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) { return nil, errors.New("not implemented") }
func (p *countingCompletionProvider) ListFineTuningJobCheckpoints(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobCheckpointList, error) { return nil, errors.New("not implemented") }

func (p *slowCompletionProvider) CreateFineTuningJob(ctx context.Context, req compat.FineTuningJobRequest) (*compat.FineTuningJob, error) { return nil, errors.New("not implemented") }
func (p *slowCompletionProvider) ListFineTuningJobs(ctx context.Context, after string, limit int) (*compat.FineTuningJobList, error) { return nil, errors.New("not implemented") }
func (p *slowCompletionProvider) RetrieveFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) { return nil, errors.New("not implemented") }
func (p *slowCompletionProvider) ListFineTuningJobEvents(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobEventList, error) { return nil, errors.New("not implemented") }
func (p *slowCompletionProvider) CancelFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) { return nil, errors.New("not implemented") }
func (p *slowCompletionProvider) ListFineTuningJobCheckpoints(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobCheckpointList, error) { return nil, errors.New("not implemented") }

func (p *blockingCompletionProvider) CreateFineTuningJob(ctx context.Context, req compat.FineTuningJobRequest) (*compat.FineTuningJob, error) { return nil, errors.New("not implemented") }
func (p *blockingCompletionProvider) ListFineTuningJobs(ctx context.Context, after string, limit int) (*compat.FineTuningJobList, error) { return nil, errors.New("not implemented") }
func (p *blockingCompletionProvider) RetrieveFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) { return nil, errors.New("not implemented") }
func (p *blockingCompletionProvider) ListFineTuningJobEvents(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobEventList, error) { return nil, errors.New("not implemented") }
func (p *blockingCompletionProvider) CancelFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) { return nil, errors.New("not implemented") }
func (p *blockingCompletionProvider) ListFineTuningJobCheckpoints(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobCheckpointList, error) { return nil, errors.New("not implemented") }

func (p *responseFunctionStateProvider) CreateFineTuningJob(ctx context.Context, req compat.FineTuningJobRequest) (*compat.FineTuningJob, error) { return nil, errors.New("not implemented") }
func (p *responseFunctionStateProvider) ListFineTuningJobs(ctx context.Context, after string, limit int) (*compat.FineTuningJobList, error) { return nil, errors.New("not implemented") }
func (p *responseFunctionStateProvider) RetrieveFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) { return nil, errors.New("not implemented") }
func (p *responseFunctionStateProvider) ListFineTuningJobEvents(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobEventList, error) { return nil, errors.New("not implemented") }
func (p *responseFunctionStateProvider) CancelFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) { return nil, errors.New("not implemented") }
func (p *responseFunctionStateProvider) ListFineTuningJobCheckpoints(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobCheckpointList, error) { return nil, errors.New("not implemented") }

func (p *responseStateProvider) CreateFineTuningJob(ctx context.Context, req compat.FineTuningJobRequest) (*compat.FineTuningJob, error) { return nil, errors.New("not implemented") }
func (p *responseStateProvider) ListFineTuningJobs(ctx context.Context, after string, limit int) (*compat.FineTuningJobList, error) { return nil, errors.New("not implemented") }
func (p *responseStateProvider) RetrieveFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) { return nil, errors.New("not implemented") }
func (p *responseStateProvider) ListFineTuningJobEvents(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobEventList, error) { return nil, errors.New("not implemented") }
func (p *responseStateProvider) CancelFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) { return nil, errors.New("not implemented") }
func (p *responseStateProvider) ListFineTuningJobCheckpoints(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobCheckpointList, error) { return nil, errors.New("not implemented") }

func (p *responseStateStreamProvider) CreateFineTuningJob(ctx context.Context, req compat.FineTuningJobRequest) (*compat.FineTuningJob, error) { return nil, errors.New("not implemented") }
func (p *responseStateStreamProvider) ListFineTuningJobs(ctx context.Context, after string, limit int) (*compat.FineTuningJobList, error) { return nil, errors.New("not implemented") }
func (p *responseStateStreamProvider) RetrieveFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) { return nil, errors.New("not implemented") }
func (p *responseStateStreamProvider) ListFineTuningJobEvents(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobEventList, error) { return nil, errors.New("not implemented") }
func (p *responseStateStreamProvider) CancelFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) { return nil, errors.New("not implemented") }
func (p *responseStateStreamProvider) ListFineTuningJobCheckpoints(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobCheckpointList, error) { return nil, errors.New("not implemented") }

func (p *functionStreamProvider) CreateFineTuningJob(ctx context.Context, req compat.FineTuningJobRequest) (*compat.FineTuningJob, error) { return nil, errors.New("not implemented") }
func (p *functionStreamProvider) ListFineTuningJobs(ctx context.Context, after string, limit int) (*compat.FineTuningJobList, error) { return nil, errors.New("not implemented") }
func (p *functionStreamProvider) RetrieveFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) { return nil, errors.New("not implemented") }
func (p *functionStreamProvider) ListFineTuningJobEvents(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobEventList, error) { return nil, errors.New("not implemented") }
func (p *functionStreamProvider) CancelFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) { return nil, errors.New("not implemented") }
func (p *functionStreamProvider) ListFineTuningJobCheckpoints(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobCheckpointList, error) { return nil, errors.New("not implemented") }
