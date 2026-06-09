package http

import (
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
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
	// TestMain 不依赖 sqlite3/CGO，避免在 CI 环境中因 CGO_ENABLED=0 失败。
	// http 包的 handler 测试仅测试 HTTP 路由层，不需要真实数据库。
	log.Println("Running http package tests...")
	code := m.Run()
	os.Exit(code)
}
