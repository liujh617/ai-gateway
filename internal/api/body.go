package api

import (
	"io"
	"net/http"
)

func (s *Server) requestBody(w http.ResponseWriter, r *http.Request) io.Reader {
	if s.maxBodyBytes <= 0 {
		return r.Body
	}
	return http.MaxBytesReader(w, r.Body, s.maxBodyBytes)
}
