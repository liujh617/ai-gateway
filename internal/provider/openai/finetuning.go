package openai

import (
	"context"
	"fmt"
	"net/http"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/provider/httpx"
)

func (p *Provider) CreateFineTuningJob(ctx context.Context, req compat.FineTuningJobRequest) (*compat.FineTuningJob, error) {
	var out compat.FineTuningJob
	if err := p.doJSONRequest(ctx, "/fine_tuning/jobs", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *Provider) ListFineTuningJobs(ctx context.Context, after string, limit int) (*compat.FineTuningJobList, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.endpoint(fineTuningListPath("/fine_tuning/jobs", after, limit)), nil)
	if err != nil {
		return nil, err
	}
	p.setHeaders(httpReq)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, httpx.TransportError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, httpx.UpstreamError(resp)
	}
	if err := httpx.RequireJSONResponse(resp); err != nil {
		return nil, err
	}
	var out compat.FineTuningJobList
	if err := httpx.DecodeLimited(resp.Body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *Provider) RetrieveFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.endpoint("/fine_tuning/jobs/"+jobID), nil)
	if err != nil {
		return nil, err
	}
	p.setHeaders(httpReq)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, httpx.TransportError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, httpx.UpstreamError(resp)
	}
	if err := httpx.RequireJSONResponse(resp); err != nil {
		return nil, err
	}
	var out compat.FineTuningJob
	if err := httpx.DecodeLimited(resp.Body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *Provider) ListFineTuningJobEvents(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobEventList, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.endpoint(fineTuningListPath("/fine_tuning/jobs/"+jobID+"/events", after, limit)), nil)
	if err != nil {
		return nil, err
	}
	p.setHeaders(httpReq)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, httpx.TransportError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, httpx.UpstreamError(resp)
	}
	if err := httpx.RequireJSONResponse(resp); err != nil {
		return nil, err
	}
	var out compat.FineTuningJobEventList
	if err := httpx.DecodeLimited(resp.Body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *Provider) CancelFineTuningJob(ctx context.Context, jobID string) (*compat.FineTuningJob, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint("/fine_tuning/jobs/"+jobID+"/cancel"), nil)
	if err != nil {
		return nil, err
	}
	p.setJSONHeaders(httpReq)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, httpx.TransportError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, httpx.UpstreamError(resp)
	}
	if err := httpx.RequireJSONResponse(resp); err != nil {
		return nil, err
	}
	var out compat.FineTuningJob
	if err := httpx.DecodeLimited(resp.Body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *Provider) ListFineTuningJobCheckpoints(ctx context.Context, jobID string, after string, limit int) (*compat.FineTuningJobCheckpointList, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.endpoint(fineTuningListPath("/fine_tuning/jobs/"+jobID+"/checkpoints", after, limit)), nil)
	if err != nil {
		return nil, err
	}
	p.setHeaders(httpReq)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, httpx.TransportError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, httpx.UpstreamError(resp)
	}
	if err := httpx.RequireJSONResponse(resp); err != nil {
		return nil, err
	}
	var out compat.FineTuningJobCheckpointList
	if err := httpx.DecodeLimited(resp.Body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func fineTuningListPath(base string, after string, limit int) string {
	path := base
	sep := "?"
	if after != "" {
		path += sep + "after=" + after
		sep = "&"
	}
	if limit > 0 {
		path += fmt.Sprintf("%slimit=%d", sep, limit)
	}
	return path
}
