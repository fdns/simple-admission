package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidJob(t *testing.T) {

}

func TestInvalidJson(t *testing.T) {
	req, err := http.NewRequest("POST", "/health-check", bytes.NewReader([]byte("invalidjson")))
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := AdmissionHandler{
		RuntimeClass: "TestRuntimeClass",
	}

	handler.handler(rr, req)
	if rr.Code != 400 {
		t.Errorf("Handler returned wrong status code, expected 400, got %v", rr.Code)
	}
}
