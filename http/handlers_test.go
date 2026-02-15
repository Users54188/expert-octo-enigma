package http

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"cloudquant/db"
)

func TestHealthHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/api/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleHealth)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expected := `{"status":"ok"}`
	if rr.Body.String() != expected+"\n" && rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}

func TestMain(m *testing.M) {
	// Setup
	dbPath := "./test.db"
	db.InitDB(dbPath)

	code := m.Run()

	// Teardown
	os.Remove(dbPath)
	os.Exit(code)
}
