package appupdate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckReportsAvailableRelease(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v0.2.0","html_url":"https://example.test/release","published_at":"2026-05-08T12:00:00Z"}`))
	}))
	defer server.Close()

	service := NewService(Options{
		AppVersion: "0.1.0",
		GitHubRepo: "Rammbloor/catenar",
		APIBaseURL: server.URL,
	})

	response := service.Check(context.Background())
	if !response.Ok {
		t.Fatalf("expected successful update check, got %#v", response.Error)
	}

	if response.Data == nil || !response.Data.UpdateAvailable {
		t.Fatalf("expected an available update, got %#v", response.Data)
	}
	if response.Data.LatestVersion != "0.2.0" {
		t.Fatalf("expected latest version 0.2.0, got %q", response.Data.LatestVersion)
	}
}

func TestCheckReportsCurrentRelease(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v0.2.0"}`))
	}))
	defer server.Close()

	service := NewService(Options{
		AppVersion: "0.2.0",
		GitHubRepo: "Rammbloor/catenar",
		APIBaseURL: server.URL,
	})

	response := service.Check(context.Background())
	if !response.Ok {
		t.Fatalf("expected successful update check, got %#v", response.Error)
	}
	if response.Data == nil || response.Data.UpdateAvailable {
		t.Fatalf("expected no available update, got %#v", response.Data)
	}
}

func TestCheckWrapsGitHubFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer server.Close()

	service := NewService(Options{
		AppVersion: "0.2.0",
		GitHubRepo: "Rammbloor/catenar",
		APIBaseURL: server.URL,
	})

	response := service.Check(context.Background())
	if response.Ok {
		t.Fatal("expected update check failure")
	}
	if response.Error == nil || response.Error.Code != "application.update_unavailable" {
		t.Fatalf("expected update_unavailable error, got %#v", response.Error)
	}
}

func TestCheckSelectsInstallerForRunningPlatform(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
          "tag_name":"v0.2.0",
          "html_url":"https://example.test/release",
          "assets":[
            {"name":"catenar-0.2.0-windows-amd64-installer.exe","browser_download_url":"https://example.test/catenar.exe"},
            {"name":"catenar-0.2.0-darwin-universal.zip","browser_download_url":"https://example.test/catenar.zip"}
          ]
        }`))
	}))
	defer server.Close()

	service := NewService(Options{
		AppVersion:   "0.1.0",
		GitHubRepo:   "Rammbloor/catenar",
		APIBaseURL:   server.URL,
		Platform:     "windows",
		Architecture: "amd64",
	})

	response := service.Check(context.Background())
	if !response.Ok || response.Data == nil {
		t.Fatalf("expected successful update check, got %#v", response)
	}
	if response.Data.DownloadName != "catenar-0.2.0-windows-amd64-installer.exe" || response.Data.DownloadURL != "https://example.test/catenar.exe" {
		t.Fatalf("expected Windows installer, got %#v", response.Data)
	}
}
