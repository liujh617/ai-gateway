package httpx

import (
	"bytes"
	"mime/multipart"
	"strconv"
)

// BuildAudioMultipartBody constructs a multipart/form-data body for audio
// transcription and translation requests. Fields are conditionally added:
// language, prompt, and response_format are skipped when empty; temperature
// is skipped when nil.
func BuildAudioMultipartBody(fileData []byte, filename, model, language, prompt, responseFormat string, temperature *float64) ([]byte, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("model", model); err != nil {
		return nil, "", err
	}
	if language != "" {
		if err := w.WriteField("language", language); err != nil {
			return nil, "", err
		}
	}
	if prompt != "" {
		if err := w.WriteField("prompt", prompt); err != nil {
			return nil, "", err
		}
	}
	if responseFormat != "" {
		if err := w.WriteField("response_format", responseFormat); err != nil {
			return nil, "", err
		}
	}
	if temperature != nil {
		if err := w.WriteField("temperature", Ftoa(*temperature)); err != nil {
			return nil, "", err
		}
	}
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		return nil, "", err
	}
	if _, err := fw.Write(fileData); err != nil {
		return nil, "", err
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), w.FormDataContentType(), nil
}

// Ftoa formats a float64 as a string for use in multipart form fields.
func Ftoa(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
