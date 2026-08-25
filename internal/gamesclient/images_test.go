package gamesclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/api/googleapi"
)

func TestUploadImage_UsesOfficialLegacyMediaEndpoint(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got, want := r.URL.EscapedPath(), "/upload/games/v1configuration/images/achievement%2Fone/imageType/ACHIEVEMENT_ICON"; got != want {
			t.Errorf("escaped path = %q, want %q", got, want)
		}
		if got := r.URL.Query().Get("uploadType"); got != "media" {
			t.Errorf("uploadType = %q, want media", got)
		}
		if got := r.Header.Get("Content-Type"); got != "image/png" {
			t.Errorf("Content-Type = %q, want image/png", got)
		}
		if got := r.ContentLength; got != 7 {
			t.Errorf("Content-Length = %d, want 7", got)
		}
		body, _ := io.ReadAll(r.Body)
		if got := string(body); got != "pngdata" {
			t.Errorf("body = %q, want pngdata", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"kind":"gamesConfiguration#imageConfiguration","url":"https://example/icon.png","resourceId":"achievement/one","imageType":"ACHIEVEMENT_ICON"}`)
	}))
	defer server.Close()

	uploader := NewLegacyImageUploaderWithClient(server.Client(), server.URL+"/")

	for range 2 {
		got, err := uploader.UploadImage(context.Background(), "achievement/one", AchievementIcon, strings.NewReader("pngdata"), 7, "image/png")
		if err != nil {
			t.Fatal(err)
		}
		if got.ResourceID != "achievement/one" || got.ImageType != AchievementIcon || got.URL != "https://example/icon.png" {
			t.Fatalf("response = %#v", got)
		}
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2 replacement uploads", requests)
	}
}

func TestUploadImage_RejectsUnsupportedImageTypeWithoutRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer server.Close()
	uploader := NewLegacyImageUploaderWithClient(server.Client(), server.URL+"/")

	_, err := uploader.UploadImage(context.Background(), "resource", "GAME_ICON", strings.NewReader("pngdata"), 7, "image/png")
	if err == nil || !strings.Contains(err.Error(), "unsupported Play Games image type") {
		t.Fatalf("error = %v", err)
	}
	if requests != 0 {
		t.Fatalf("requests = %d, want 0", requests)
	}
}

func TestUploadImage_ReturnsGoogleAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusGone)
		_, _ = io.WriteString(w, `{"error":{"code":410,"message":"image upload removed"}}`)
	}))
	defer server.Close()
	uploader := NewLegacyImageUploaderWithClient(server.Client(), server.URL+"/")

	_, err := uploader.UploadImage(context.Background(), "leaderboard", LeaderboardIcon, strings.NewReader("jpegdata"), 8, "image/jpeg")
	var apiErr *googleapi.Error
	if !errors.As(err, &apiErr) || apiErr.Code != http.StatusGone {
		t.Fatalf("error = %#v, want googleapi.Error code 410", err)
	}
}
