package compat

import "net/http"

type Error struct {
	Status  int
	Message string
	Type    string
	Param   *string
	Code    *string
}

func (e *Error) Error() string {
	return e.Message
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Param   *string `json:"param"`
	Code    *string `json:"code"`
}

func NewError(status int, typ, message string, param *string) *Error {
	return &Error{
		Status:  status,
		Message: message,
		Type:    typ,
		Param:   param,
	}
}

func InvalidRequest(message, param string) *Error {
	return NewError(http.StatusBadRequest, "invalid_request_error", message, stringPtr(param))
}

func Authentication(message string) *Error {
	return NewError(http.StatusUnauthorized, "authentication_error", message, nil)
}

func ModelNotFound(model string) *Error {
	return NewError(http.StatusNotFound, "invalid_request_error", "model not found: "+model, stringPtr("model"))
}

func ServerError(status int, message string) *Error {
	return NewError(status, "server_error", message, nil)
}

func RateLimit(message string) *Error {
	return NewError(http.StatusTooManyRequests, "rate_limit_error", message, nil)
}

func ErrorResponseFor(err *Error) ErrorResponse {
	return ErrorResponse{
		Error: ErrorBody{
			Message: err.Message,
			Type:    err.Type,
			Param:   err.Param,
			Code:    err.Code,
		},
	}
}

func stringPtr(s string) *string {
	return &s
}
