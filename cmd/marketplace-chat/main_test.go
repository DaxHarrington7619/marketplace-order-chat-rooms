package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"example.com/marketplace-chat/internal/marketplace"
)

func TestWriteServiceErrorPreservesRejectionDetail(t *testing.T) {
	w := httptest.NewRecorder()
	writeServiceError(w, &marketplace.APIError{
		Status: http.StatusBadRequest,
		Code:   "INVALID_ARGUMENT",
		Detail: json.RawMessage(`{"code":"INVALID_ARGUMENT","message":"invalid type","field":"type"}`),
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", w.Code)
	}
	var response struct {
		Error  string `json:"error"`
		Detail struct {
			Message string `json:"message"`
			Field   string `json:"field"`
		} `json:"detail"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error != "INVALID_ARGUMENT" || response.Detail.Message != "invalid type" || response.Detail.Field != "type" {
		t.Fatalf("response = %+v", response)
	}
}
