package localapp

import (
	"net"
	"net/http"
	"strings"
	"testing"

	"downloader-2607/internal/service"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	app, err := New(service.NewYTDLP(service.Config{}), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func startTestServer(t *testing.T, app *App) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = app.server.Listener(listener) }()
	t.Cleanup(func() { _ = app.server.Shutdown() })
	return "http://" + listener.Addr().String()
}

func TestHomeIsLoopbackOnlyAndHardened(t *testing.T) {
	app := newTestApp(t)
	baseURL := startTestServer(t, app)
	response, err := http.Get(baseURL + "/")
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode)
	}
	if !strings.Contains(response.Header.Get("Content-Security-Policy"), "frame-ancestors 'none'") {
		t.Fatal("missing restrictive Content-Security-Policy")
	}
	if response.Header.Get("X-Frame-Options") != "DENY" {
		t.Fatal("missing X-Frame-Options DENY")
	}
}

func TestRejectsUntrustedHost(t *testing.T) {
	app := newTestApp(t)
	baseURL := startTestServer(t, app)
	req, _ := http.NewRequest(http.MethodGet, baseURL+"/", nil)
	req.Host = "attacker.example"
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.StatusCode)
	}
}

func TestAPIRequiresProcessToken(t *testing.T) {
	app := newTestApp(t)
	baseURL := startTestServer(t, app)
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/metadata", strings.NewReader(`{"url":"https://youtu.be/test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-TubeDown-Token", "wrong")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.StatusCode)
	}
}
