package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"open-ai-gateway/internal/audit"
	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/middleware"
	"open-ai-gateway/internal/router"
	"open-ai-gateway/internal/routes"
)

const maxAudioUploadBytes = 25 << 20 // 25 MiB

func (s *Server) handleAudioTranscriptions(w http.ResponseWriter, r *http.Request) {
	req, err := s.parseAudioMultipartForm(r, w)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	if validationErr := req.ValidateTextOnly(); validationErr != nil {
		s.writeError(w, r, validationErr)
		return
	}
	if !s.modelAllowedForRequest(r, req.Model) {
		middleware.SetLogRoute(r.Context(), req.Model, "", "")
		s.writeError(w, r, compat.ModelNotFound(req.Model))
		return
	}
	route, resolveErr := s.router.ResolveFor(req.Model, "transcriptions")
	if resolveErr != nil {
		middleware.SetLogRoute(r.Context(), req.Model, "", "")
		s.writeError(w, r, resolveErr)
		return
	}
	externalModel := req.Model
	requestEvent := s.auditBaseEvent(r, audit.EventRequest, routes.AudioTranscriptionsPath, externalModel)
	requestEvent.Body = rawBody(audioReqSummary(req.Filename, len(req.File)))
	s.audit.Record(r.Context(), requestEvent)

	ctx, cancel := context.WithTimeout(r.Context(), s.requestTimeout)
	defer cancel()
	resp, providerName, upstreamModel, fbErr := s.createTranscriptionWithFallback(ctx, r, route, externalModel, req)
	if fbErr != nil {
		s.writeAuditedError(w, r, routes.AudioTranscriptionsPath, externalModel, providerError(fbErr))
		return
	}
	responseEvent := s.auditBaseEvent(r, audit.EventResponse, routes.AudioTranscriptionsPath, externalModel)
	responseEvent.Provider = providerName
	responseEvent.UpstreamModel = upstreamModel
	responseEvent.Status = http.StatusOK
	responseEvent.Body = rawBody(resp)
	s.audit.Record(r.Context(), responseEvent)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleAudioTranslations(w http.ResponseWriter, r *http.Request) {
	req, err := s.parseAudioMultipartForm(r, w)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	translationReq := compat.AudioTranslationRequest{
		Model:          req.Model,
		File:           req.File,
		Filename:       req.Filename,
		Prompt:         req.Prompt,
		ResponseFormat: req.ResponseFormat,
		Temperature:    req.Temperature,
	}
	if validationErr := translationReq.ValidateTextOnly(); validationErr != nil {
		s.writeError(w, r, validationErr)
		return
	}
	if !s.modelAllowedForRequest(r, translationReq.Model) {
		middleware.SetLogRoute(r.Context(), translationReq.Model, "", "")
		s.writeError(w, r, compat.ModelNotFound(translationReq.Model))
		return
	}
	route, resolveErr := s.router.ResolveFor(translationReq.Model, "translations")
	if resolveErr != nil {
		middleware.SetLogRoute(r.Context(), translationReq.Model, "", "")
		s.writeError(w, r, resolveErr)
		return
	}
	externalModel := translationReq.Model
	requestEvent := s.auditBaseEvent(r, audit.EventRequest, routes.AudioTranslationsPath, externalModel)
	requestEvent.Body = rawBody(audioReqSummary(req.Filename, len(req.File)))
	s.audit.Record(r.Context(), requestEvent)

	ctx, cancel := context.WithTimeout(r.Context(), s.requestTimeout)
	defer cancel()
	resp, providerName, upstreamModel, fbErr := s.createTranslationWithFallback(ctx, r, route, externalModel, translationReq)
	if fbErr != nil {
		s.writeAuditedError(w, r, routes.AudioTranslationsPath, externalModel, providerError(fbErr))
		return
	}
	responseEvent := s.auditBaseEvent(r, audit.EventResponse, routes.AudioTranslationsPath, externalModel)
	responseEvent.Provider = providerName
	responseEvent.UpstreamModel = upstreamModel
	responseEvent.Status = http.StatusOK
	responseEvent.Body = rawBody(resp)
	s.audit.Record(r.Context(), responseEvent)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleAudioSpeech(w http.ResponseWriter, r *http.Request) {
	if err := requireJSONContentType(r); err != nil {
		s.writeError(w, r, err)
		return
	}
	var req compat.SpeechRequest
	if err := decodeJSONBody(s.requestBody(w, r), &req); err != nil {
		s.writeError(w, r, decodeError(err))
		return
	}
	if validationErr := req.Validate(); validationErr != nil {
		s.writeError(w, r, validationErr)
		return
	}
	if !s.modelAllowedForRequest(r, req.Model) {
		middleware.SetLogRoute(r.Context(), req.Model, "", "")
		s.writeError(w, r, compat.ModelNotFound(req.Model))
		return
	}
	route, resolveErr := s.router.ResolveFor(req.Model, "speech")
	if resolveErr != nil {
		middleware.SetLogRoute(r.Context(), req.Model, "", "")
		s.writeError(w, r, resolveErr)
		return
	}
	externalModel := req.Model
	requestEvent := s.auditBaseEvent(r, audit.EventRequest, routes.AudioSpeechPath, externalModel)
	requestEvent.Body = rawBody(speechReqSummary(req.Input))
	s.audit.Record(r.Context(), requestEvent)

	ctx, cancel := context.WithTimeout(r.Context(), s.requestTimeout)
	defer cancel()
	resp, providerName, upstreamModel, err := s.createSpeechWithFallback(ctx, r, route, externalModel, req)
	if err != nil {
		s.writeAuditedError(w, r, routes.AudioSpeechPath, externalModel, providerError(err))
		return
	}
	responseEvent := s.auditBaseEvent(r, audit.EventResponse, routes.AudioSpeechPath, externalModel)
	responseEvent.Provider = providerName
	responseEvent.UpstreamModel = upstreamModel
	responseEvent.Status = http.StatusOK
	responseEvent.Body = rawBody(speechRespSummary(len(resp.Data), resp.ContentType))
	s.audit.Record(r.Context(), responseEvent)
	w.Header().Set("Content-Type", resp.ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(resp.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp.Data)
}

func (s *Server) parseAudioMultipartForm(r *http.Request, w http.ResponseWriter) (*compat.AudioTranscriptionRequest, *compat.Error) {
	ct := r.Header.Get("Content-Type")
	if ct == "" || !strings.HasPrefix(strings.ToLower(ct), "multipart/form-data") {
		return nil, compat.UnsupportedMediaType("Content-Type must be multipart/form-data")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAudioUploadBytes)
	if err := r.ParseMultipartForm(maxAudioUploadBytes); err != nil {
		return nil, compat.InvalidRequest("failed to parse multipart form: "+err.Error(), "body")
	}
	defer r.MultipartForm.RemoveAll()

	model := strings.TrimSpace(r.FormValue("model"))
	if model == "" {
		return nil, compat.InvalidRequest("missing required field: model", "model")
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		return nil, compat.InvalidRequest("missing required field: file", "file")
	}
	defer file.Close()
	fileData, err := io.ReadAll(file)
	if err != nil {
		return nil, compat.InvalidRequest("failed to read audio file: "+err.Error(), "file")
	}
	if len(fileData) == 0 {
		return nil, compat.InvalidRequest("uploaded file is empty", "file")
	}

	lang := strings.TrimSpace(r.FormValue("language"))
	prompt := strings.TrimSpace(r.FormValue("prompt"))
	responseFormat := strings.TrimSpace(r.FormValue("response_format"))
	var temp *float64
	if ts := strings.TrimSpace(r.FormValue("temperature")); ts != "" {
		if t, parseErr := strconv.ParseFloat(ts, 64); parseErr == nil {
			temp = &t
		}
	}

	filename := ""
	if handler != nil {
		filename = handler.Filename
	}

	return &compat.AudioTranscriptionRequest{
		Model:          model,
		File:           fileData,
		Filename:       filename,
		Language:       lang,
		Prompt:         prompt,
		ResponseFormat: responseFormat,
		Temperature:    temp,
	}, nil
}

func audioReqSummary(filename string, sizeBytes int) map[string]any {
	return map[string]any{
		"audio_file":       filename,
		"audio_file_bytes": sizeBytes,
	}
}

func speechReqSummary(input string) map[string]any {
	const maxPreview = 200
	preview := input
	if len(preview) > maxPreview {
		preview = preview[:maxPreview] + "..."
	}
	return map[string]any{"input_preview": preview}
}

func speechRespSummary(dataSize int, contentType string) map[string]any {
	return map[string]any{
		"audio_content_type": contentType,
		"audio_bytes":        dataSize,
	}
}

func (s *Server) createTranscriptionWithFallback(ctx context.Context, r *http.Request, route router.ModelRoute, externalModel string, req *compat.AudioTranscriptionRequest) (*compat.AudioTranscriptionResponse, string, string, error) {
	var lastErr error
	var skippedFrom string
	attempts := route.Attempts()
	for index, attempt := range attempts {
		if !s.providerHealth.Healthy(attempt.ProviderName) {
			s.observeProviderHealth(attempt.ProviderName)
			s.observeProviderCircuitOpen(r.Context(), routes.AudioTranscriptionsPath, externalModel, attempt.ProviderName)
			if skippedFrom == "" {
				skippedFrom = attempt.ProviderName
			}
			continue
		}
		if skippedFrom != "" {
			s.observeProviderFallback(r.Context(), routes.AudioTranscriptionsPath, externalModel, skippedFrom, attempt.ProviderName)
			skippedFrom = ""
		}
		attemptReq := *req
		attemptReq.Model = attempt.UpstreamModel
		middleware.SetLogRoute(r.Context(), externalModel, attempt.ProviderName, attempt.UpstreamModel)
		resp, err := attempt.Provider.CreateTranscription(ctx, attemptReq)
		if err == nil {
			s.providerHealth.MarkSuccess(attempt.ProviderName)
			s.observeProviderHealth(attempt.ProviderName)
			return resp, attempt.ProviderName, attempt.UpstreamModel, nil
		}
		lastErr = err
		if canFallbackProviderError(err) {
			s.providerHealth.MarkFailure(attempt.ProviderName)
			s.observeProviderHealth(attempt.ProviderName)
		}
		if index == len(attempts)-1 || !canFallbackProviderError(err) {
			return nil, "", "", err
		}
		if nextProviderName := s.nextHealthyProviderName(attempts[index+1:]); nextProviderName != "" {
			s.observeProviderFallback(r.Context(), routes.AudioTranscriptionsPath, externalModel, attempt.ProviderName, nextProviderName)
		}
	}
	return nil, "", "", lastErr
}

func (s *Server) createTranslationWithFallback(ctx context.Context, r *http.Request, route router.ModelRoute, externalModel string, req compat.AudioTranslationRequest) (*compat.AudioTranslationResponse, string, string, error) {
	var lastErr error
	var skippedFrom string
	attempts := route.Attempts()
	for index, attempt := range attempts {
		if !s.providerHealth.Healthy(attempt.ProviderName) {
			s.observeProviderHealth(attempt.ProviderName)
			s.observeProviderCircuitOpen(r.Context(), routes.AudioTranslationsPath, externalModel, attempt.ProviderName)
			if skippedFrom == "" {
				skippedFrom = attempt.ProviderName
			}
			continue
		}
		if skippedFrom != "" {
			s.observeProviderFallback(r.Context(), routes.AudioTranslationsPath, externalModel, skippedFrom, attempt.ProviderName)
			skippedFrom = ""
		}
		attemptReq := req
		attemptReq.Model = attempt.UpstreamModel
		middleware.SetLogRoute(r.Context(), externalModel, attempt.ProviderName, attempt.UpstreamModel)
		resp, err := attempt.Provider.CreateTranslation(ctx, attemptReq)
		if err == nil {
			s.providerHealth.MarkSuccess(attempt.ProviderName)
			s.observeProviderHealth(attempt.ProviderName)
			return resp, attempt.ProviderName, attempt.UpstreamModel, nil
		}
		lastErr = err
		if canFallbackProviderError(err) {
			s.providerHealth.MarkFailure(attempt.ProviderName)
			s.observeProviderHealth(attempt.ProviderName)
		}
		if index == len(attempts)-1 || !canFallbackProviderError(err) {
			return nil, "", "", err
		}
		if nextProviderName := s.nextHealthyProviderName(attempts[index+1:]); nextProviderName != "" {
			s.observeProviderFallback(r.Context(), routes.AudioTranslationsPath, externalModel, attempt.ProviderName, nextProviderName)
		}
	}
	return nil, "", "", lastErr
}

func (s *Server) createSpeechWithFallback(ctx context.Context, r *http.Request, route router.ModelRoute, externalModel string, req compat.SpeechRequest) (*compat.SpeechResponse, string, string, error) {
	var lastErr error
	var skippedFrom string
	attempts := route.Attempts()
	for index, attempt := range attempts {
		if !s.providerHealth.Healthy(attempt.ProviderName) {
			s.observeProviderHealth(attempt.ProviderName)
			s.observeProviderCircuitOpen(r.Context(), routes.AudioSpeechPath, externalModel, attempt.ProviderName)
			if skippedFrom == "" {
				skippedFrom = attempt.ProviderName
			}
			continue
		}
		if skippedFrom != "" {
			s.observeProviderFallback(r.Context(), routes.AudioSpeechPath, externalModel, skippedFrom, attempt.ProviderName)
			skippedFrom = ""
		}
		attemptReq := req
		attemptReq.Model = attempt.UpstreamModel
		middleware.SetLogRoute(r.Context(), externalModel, attempt.ProviderName, attempt.UpstreamModel)
		resp, err := attempt.Provider.CreateSpeech(ctx, attemptReq)
		if err == nil {
			s.providerHealth.MarkSuccess(attempt.ProviderName)
			s.observeProviderHealth(attempt.ProviderName)
			return resp, attempt.ProviderName, attempt.UpstreamModel, nil
		}
		lastErr = err
		if canFallbackProviderError(err) {
			s.providerHealth.MarkFailure(attempt.ProviderName)
			s.observeProviderHealth(attempt.ProviderName)
		}
		if index == len(attempts)-1 || !canFallbackProviderError(err) {
			return nil, "", "", err
		}
		if nextProviderName := s.nextHealthyProviderName(attempts[index+1:]); nextProviderName != "" {
			s.observeProviderFallback(r.Context(), routes.AudioSpeechPath, externalModel, attempt.ProviderName, nextProviderName)
		}
	}
	return nil, "", "", lastErr
}
