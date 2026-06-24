package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func (s *Server) requestBody(w http.ResponseWriter, r *http.Request) io.Reader {
	if s.maxBodyBytes <= 0 {
		return r.Body
	}
	return http.MaxBytesReader(w, r.Body, s.maxBodyBytes)
}

func decodeJSONBody(r io.Reader, out any) error {
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("request body must contain a single JSON value")
		}
		return err
	}
	return nil
}
