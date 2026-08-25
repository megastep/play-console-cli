package gamesclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"google.golang.org/api/googleapi"

	"github.com/tamtom/play-console-cli/internal/config"
	"github.com/tamtom/play-console-cli/internal/playclient"
)

const legacyImageUploadBaseURL = "https://www.googleapis.com/"

// Legacy Play Games image types. Google has removed this method from the live
// discovery document, but its deprecated REST reference still documents these
// two values.
const (
	AchievementIcon = "ACHIEVEMENT_ICON"
	LeaderboardIcon = "LEADERBOARD_ICON"
)

// ImageConfiguration is the response returned by the legacy official image
// upload endpoint.
type ImageConfiguration struct {
	Kind       string `json:"kind,omitempty"`
	URL        string `json:"url,omitempty"`
	ResourceID string `json:"resourceId,omitempty"`
	ImageType  string `json:"imageType,omitempty"`
}

// LegacyImageUploader isolates the raw HTTP transport required by Google's
// deprecated image endpoint from the generated Games API services.
type LegacyImageUploader struct {
	Cfg     *config.Config
	client  *http.Client
	baseURL string
}

type (
	LegacyImageUploaderFactory           func(context.Context) (*LegacyImageUploader, error)
	legacyImageUploaderFactoryContextKey struct{}
)

// ContextWithLegacyImageUploaderFactory installs a scoped uploader factory for tests.
func ContextWithLegacyImageUploaderFactory(ctx context.Context, factory LegacyImageUploaderFactory) context.Context {
	if factory == nil {
		return ctx
	}
	return context.WithValue(ctx, legacyImageUploaderFactoryContextKey{}, factory)
}

// NewLegacyImageUploader creates an uploader authenticated only with the scope
// required by the Play Games publishing endpoint.
func NewLegacyImageUploader(ctx context.Context) (*LegacyImageUploader, error) {
	if factory, ok := ctx.Value(legacyImageUploaderFactoryContextKey{}).(LegacyImageUploaderFactory); ok && factory != nil {
		return factory(ctx)
	}
	client, cfg, err := playclient.NewAuthenticatedClientWithScopes(ctx, scopeAndroidPublisher)
	if err != nil {
		return nil, err
	}
	return &LegacyImageUploader{Cfg: cfg, client: client, baseURL: legacyImageUploadBaseURL}, nil
}

// NewLegacyImageUploaderWithClient builds a test uploader on a caller-provided transport.
func NewLegacyImageUploaderWithClient(client *http.Client, baseURL string) *LegacyImageUploader {
	return &LegacyImageUploader{Cfg: &config.Config{}, client: client, baseURL: baseURL}
}

// UploadImage replaces the icon for an achievement or leaderboard through
// Google's deprecated Play Games Services Publishing media endpoint.
func (u *LegacyImageUploader) UploadImage(ctx context.Context, resourceID, imageType string, body io.Reader, size int64, contentType string) (*ImageConfiguration, error) {
	if strings.TrimSpace(resourceID) == "" {
		return nil, fmt.Errorf("Play Games image resource ID is required")
	}
	if imageType != AchievementIcon && imageType != LeaderboardIcon {
		return nil, fmt.Errorf("unsupported Play Games image type %q", imageType)
	}
	if u == nil || u.client == nil {
		return nil, fmt.Errorf("Play Games upload client is unavailable")
	}

	base := strings.TrimRight(u.baseURL, "/")
	endpoint := fmt.Sprintf(
		"%s/upload/games/v1configuration/images/%s/imageType/%s?uploadType=media",
		base,
		url.PathEscape(resourceID),
		imageType,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("build Play Games image upload request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = size

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := googleapi.CheckResponse(resp); err != nil {
		return nil, err
	}

	var result ImageConfiguration
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode Play Games image upload response: %w", err)
	}
	return &result, nil
}
