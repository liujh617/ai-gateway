package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"open-ai-gateway/internal/compat"
	"open-ai-gateway/internal/router"
	"open-ai-gateway/internal/routes"
)

const maxFileUploadBytes = 512 << 20 // 512 MiB

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleUploadFile(w, r)
	case http.MethodGet:
		s.handleListFiles(w, r)
	default:
		s.writeError(w, r, compat.ServerError(http.StatusMethodNotAllowed, "method not allowed"))
	}
}

func (s *Server) handleUploadFile(w http.ResponseWriter, r *http.Request) {
	ct := r.Header.Get("Content-Type")
	if ct == "" || !strings.HasPrefix(strings.ToLower(ct), "multipart/form-data") {
		s.writeError(w, r, compat.UnsupportedMediaType("Content-Type must be multipart/form-data"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxFileUploadBytes)
	if err := r.ParseMultipartForm(maxFileUploadBytes); err != nil {
		s.writeError(w, r, compat.InvalidRequest("failed to parse multipart form: "+err.Error(), "body"))
		return
	}
	defer r.MultipartForm.RemoveAll()

	purpose := strings.TrimSpace(r.FormValue("purpose"))
	file, handler, err := r.FormFile("file")
	if err != nil {
		s.writeError(w, r, compat.InvalidRequest("missing required field: file", "file"))
		return
	}
	defer file.Close()
	fileData, err := io.ReadAll(file)
	if err != nil {
		s.writeError(w, r, compat.InvalidRequest("failed to read file: "+err.Error(), "file"))
		return
	}
	filename := ""
	if handler != nil {
		filename = handler.Filename
	}

	req := compat.FileUploadRequest{
		File:     fileData,
		Filename: filename,
		Purpose:  purpose,
	}
	if err := req.Validate(); err != nil {
		s.writeAuditedError(w, r, routes.FilesPath, "", err)
		return
	}

	route, ok := s.resolveCapabilityRoute(w, r, "files")
	if !ok {
		return
	}
	fobj, err := route.Provider.UploadFile(r.Context(), req)
	if err != nil {
		s.writeAuditedError(w, r, routes.FilesPath, "", providerError(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(fobj)
}

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	route, ok := s.resolveCapabilityRoute(w, r, "files")
	if !ok {
		return
	}
	list, err := route.Provider.ListFiles(r.Context())
	if err != nil {
		s.writeAuditedError(w, r, routes.FilesPath, "", providerError(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func (s *Server) handleRetrieveFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if strings.HasSuffix(path, "/content") {
		fileID := strings.TrimSuffix(strings.TrimPrefix(path, routes.FilesPath+"/"), "/content")
		if fileID == "" || fileID == path {
			s.writeError(w, r, compat.InvalidRequest("missing file id", "file_id"))
			return
		}
		s.handleDownloadFile(w, r, fileID)
		return
	}
	fileID := strings.TrimPrefix(path, routes.FilesPath+"/")
	if fileID == "" || fileID == path {
		s.writeError(w, r, compat.InvalidRequest("missing file id", "file_id"))
		return
	}
	switch r.Method {
	case http.MethodGet:
		route, ok := s.resolveCapabilityRoute(w, r, "files")
		if !ok {
			return
		}
		fobj, err := route.Provider.RetrieveFile(r.Context(), fileID)
		if err != nil {
			s.writeAuditedError(w, r, routes.FilesRetrievePath, "", providerError(err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fobj)
	case http.MethodDelete:
		route, ok := s.resolveCapabilityRoute(w, r, "files")
		if !ok {
			return
		}
		resp, err := route.Provider.DeleteFile(r.Context(), fileID)
		if err != nil {
			s.writeAuditedError(w, r, routes.FilesRetrievePath, "", providerError(err))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	default:
		s.writeError(w, r, compat.ServerError(http.StatusMethodNotAllowed, "method not allowed"))
	}
}

func (s *Server) handleDownloadFile(w http.ResponseWriter, r *http.Request, fileID string) {
	route, ok := s.resolveCapabilityRoute(w, r, "files")
	if !ok {
		return
	}
	data, ct, err := route.Provider.DownloadFile(r.Context(), fileID)
	if err != nil {
		s.writeAuditedError(w, r, routes.FilesContentPath, "", providerError(err))
		return
	}
	w.Header().Set("Content-Type", ct)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) resolveCapabilityRoute(w http.ResponseWriter, r *http.Request, capability string) (router.ModelRoute, bool) {
	route, err := s.router.ResolveByCapability(capability)
	if err != nil {
		s.writeError(w, r, err)
		return router.ModelRoute{}, false
	}
	return route, true
}

