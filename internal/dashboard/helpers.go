package dashboard

import (
	"encoding/json"
	"net/http"
	"time"
)

// writeJSON writes JSON response
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes error response
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error": message,
	})
}

// getCurrentTimestamp returns current timestamp in ISO 8601 format
func getCurrentTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// ParsePIIViolationFilter parses query parameters into PIIViolationFilter
func ParsePIIViolationFilter(r *interface{}) *PIIViolationFilter {
	// TODO: Parse from http.Request query parameters
	return &PIIViolationFilter{
		Page: 1,
		Size: 50,
	}
}

// ParseContentSafetyViolationFilter parses query parameters into ContentSafetyViolationFilter
func ParseContentSafetyViolationFilter(r *interface{}) *ContentSafetyViolationFilter {
	// TODO: Parse from http.Request query parameters
	return &ContentSafetyViolationFilter{
		Page: 1,
		Size: 50,
	}
}

// ParseAlertFilter parses query parameters into AlertFilter
func ParseAlertFilter(r *interface{}) *AlertFilter {
	// TODO: Parse from http.Request query parameters
	return &AlertFilter{
		Page: 1,
		Size: 50,
	}
}