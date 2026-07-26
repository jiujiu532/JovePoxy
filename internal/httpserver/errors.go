package httpserver

import (
	"encoding/json"
	"net/http"
)

type openAIErrorResponse struct {
	Error openAIError `json:"error"`
}

type openAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param,omitempty"`
	Code    string `json:"code,omitempty"`
}

func writeOpenAIError(writer http.ResponseWriter, status int, message, errorType, param, code string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(openAIErrorResponse{Error: openAIError{
		Message: message, Type: errorType, Param: param, Code: code,
	}})
}
