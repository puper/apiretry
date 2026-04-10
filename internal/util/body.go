package util

import (
	"bytes"
	"io"
	"net/http"
)

func ReadAndCacheBody(r *http.Request, maxBytes int64) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	limited := io.LimitReader(r.Body, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, &BodyTooLargeError{MaxBytes: maxBytes}
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	return data, nil
}

func DrainBody(body io.ReadCloser, maxBytes int64) {
	if body == nil {
		return
	}
	io.Copy(io.Discard, io.LimitReader(body, maxBytes))
	body.Close()
}

type BodyTooLargeError struct {
	MaxBytes int64
}

func (e *BodyTooLargeError) Error() string {
	return "request body too large"
}
