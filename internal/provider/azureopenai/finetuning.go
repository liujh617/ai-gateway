package azureopenai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/provider/httpx"
)

func (p *Provider) CreateFineTuningJob(ctx context.Context, req compat.FineTuningJobRequest) (*compat.FineTuningJob, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/fine_tuning/jobs?api-version="+p.apiVersion, bytes.NewReader(body))
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

func (p *Provider) ListFineTuningJobs(ctx context.Context, after string, limit int) (*compat.FineTuningJobList, error) {
	path := p.baseURL + "/fine_tuning/jobs?api-version=" + p.apiVersion
	if after != "" {
		path += "&after=" + after
	}
	if limit > 0 {
		path += fmt.Sprintf("&limit=%d", limit)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
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
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/fine_tuning/jobs/"+jobID+"?api-version="+p.apiVersion, nil)
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
	path := p.baseURL + "/fine_tuning/jobs/" + jobID + "/events?api-version=" + p.apiVersion
	if after != "" {
		path += "&after=" + after
	}
	if limit > 0 {
		path += fmt.Sprintf("&limit=%d", limit)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
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
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/fine_tuning/jobs/"+jobID+"/cancel?api-version="+p.apiVersion, nil)
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
	path := p.baseURL + "/fine_tuning/jobs/" + jobID + "/checkpoints?api-version=" + p.apiVersion
	if after != "" {
		path += "&after=" + after
	}
	if limit > 0 {
		path += fmt.Sprintf("&limit=%d", limit)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
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
