package util

import (
	"encoding/json"
	"net/http"
)

type ProxyErrorResponse struct {
	Error ProxyErrorDetail `json:"error"`
}

type ProxyErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

func WriteProxyError(w http.ResponseWriter, statusCode int, message, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ProxyErrorResponse{
		Error: ProxyErrorDetail{
			Message: message,
			Type:    "proxy_error",
			Code:    code,
		},
	})
}
