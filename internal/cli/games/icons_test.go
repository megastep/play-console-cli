package games

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/tamtom/play-console-cli/internal/cli/shared"
	"github.com/tamtom/play-console-cli/internal/gamesclient"
)

func TestOpenValidatedGameIcon_AcceptsGoogleIconFormats(t *testing.T) {
	tests := []struct {
		name        string
		format      string
		contentType string
	}{
		{name: "PNG", format: "png", contentType: "image/png"},
		{name: "JPEG", format: "jpeg", contentType: "image/jpeg"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := writeGameIcon(t, tc.format, 512, 512)
			icon, err := openValidatedGameIcon(path)
			if err != nil {
				t.Fatal(err)
			}
			defer icon.file.Close()
			if icon.contentType != tc.contentType {
				t.Fatalf("content type = %q, want %q", icon.contentType, tc.contentType)
			}
			if icon.size <= 0 {
				t.Fatalf("size = %d, want positive", icon.size)
			}
			if offset, err := icon.file.Seek(0, io.SeekCurrent); err != nil || offset != 0 {
				t.Fatalf("file offset = %d, err = %v; want rewound", offset, err)
			}
		})
	}
}

func TestOpenValidatedGameIcon_RejectsInvalidFiles(t *testing.T) {
	corrupt := filepath.Join(t.TempDir(), "corrupt.png")
	if err := os.WriteFile(corrupt, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	oversized := filepath.Join(t.TempDir(), "oversized.png")
	oversizedFile, err := os.Create(oversized)
	if err != nil {
		t.Fatal(err)
	}
	if err := oversizedFile.Truncate(playGamesIconMaxBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err := oversizedFile.Close(); err != nil {
		t.Fatal(err)
	}
	completePNG := writeGameIcon(t, "png", 512, 512)
	completePNGData, err := os.ReadFile(completePNG)
	if err != nil {
		t.Fatal(err)
	}
	truncatedPNG := filepath.Join(t.TempDir(), "truncated.png")
	truncatedPNGData := completePNGData[:len(completePNGData)/2]
	if _, _, err := image.DecodeConfig(bytes.NewReader(truncatedPNGData)); err != nil {
		t.Fatalf("truncated fixture must retain a readable PNG header: %v", err)
	}
	if _, _, err := image.Decode(bytes.NewReader(truncatedPNGData)); err == nil {
		t.Fatal("truncated fixture must fail full image decoding")
	}
	if err := os.WriteFile(truncatedPNG, truncatedPNGData, 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "missing", path: filepath.Join(t.TempDir(), "missing.png"), want: "failed to open"},
		{name: "corrupt", path: corrupt, want: "valid PNG or JPEG"},
		{name: "truncated", path: truncatedPNG, want: "fully decodable"},
		{name: "oversized dimensions before full decode", path: writePNGHeaderOnly(t, 2048, 2048), want: "dimensions are 2048x2048"},
		{name: "wrong dimensions", path: writeGameIcon(t, "png", 511, 512), want: "dimensions are 511x512"},
		{name: "oversized", path: oversized, want: "exceeds the 15 MB API limit"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := openValidatedGameIcon(tc.path)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestGameIconUploadCommands_UseResourceSpecificImageTypes(t *testing.T) {
	tests := []struct {
		name         string
		command      func() *ffcli.Command
		idFlag       string
		resourceID   string
		imageType    string
		expectedPath string
	}{
		{
			name:         "achievement",
			command:      achievementsUploadIconCommand,
			idFlag:       "--achievement-id",
			resourceID:   "achievement-1",
			imageType:    gamesclient.AchievementIcon,
			expectedPath: "/upload/games/v1configuration/images/achievement-1/imageType/ACHIEVEMENT_ICON",
		},
		{
			name:         "leaderboard",
			command:      leaderboardsUploadIconCommand,
			idFlag:       "--leaderboard-id",
			resourceID:   "leaderboard-1",
			imageType:    gamesclient.LeaderboardIcon,
			expectedPath: "/upload/games/v1configuration/images/leaderboard-1/imageType/LEADERBOARD_ICON",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := writeGameIcon(t, "png", 512, 512)
			var gotPath, gotType string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotType = r.Header.Get("Content-Type")
				_, _ = io.Copy(io.Discard, r.Body)
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"kind":"gamesConfiguration#imageConfiguration","url":"https://example/icon.png","resourceId":"`+tc.resourceID+`","imageType":"`+tc.imageType+`"}`)
			}))
			defer server.Close()

			cmd := tc.command()
			if err := cmd.FlagSet.Parse([]string{tc.idFlag, tc.resourceID, "--file", path, "--acknowledge-deprecated-api"}); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			ctx := shared.ContextWithIO(context.Background(), &stdout, &stderr)
			ctx = gamesclient.ContextWithServiceFactory(ctx, func(context.Context) (*gamesclient.Service, error) {
				return nil, errors.New("aggregate Games service must not be created for icon uploads")
			})
			ctx = gamesclient.ContextWithLegacyImageUploaderFactory(ctx, func(context.Context) (*gamesclient.LegacyImageUploader, error) {
				return gamesclient.NewLegacyImageUploaderWithClient(server.Client(), server.URL+"/"), nil
			})
			if err := cmd.Exec(ctx, nil); err != nil {
				t.Fatal(err)
			}
			if gotPath != tc.expectedPath {
				t.Fatalf("path = %q, want %q", gotPath, tc.expectedPath)
			}
			if gotType != "image/png" {
				t.Fatalf("Content-Type = %q, want image/png", gotType)
			}
			if !strings.Contains(stdout.String(), `"imageType":"`+tc.imageType+`"`) {
				t.Fatalf("stdout = %q", stdout.String())
			}
			if !strings.Contains(stderr.String(), "WARNING:") || !strings.Contains(stderr.String(), "deprecated") {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
	}
}

func TestGameIconUploadCommand_ReturnsActionableDeprecationError(t *testing.T) {
	path := writeGameIcon(t, "png", 512, 512)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusGone)
		_, _ = io.WriteString(w, `{"error":{"code":410,"message":"removed"}}`)
	}))
	defer server.Close()
	cmd := achievementsUploadIconCommand()
	if err := cmd.FlagSet.Parse([]string{"--achievement-id", "achievement-1", "--file", path, "--acknowledge-deprecated-api"}); err != nil {
		t.Fatal(err)
	}
	ctx := gamesclient.ContextWithServiceFactory(context.Background(), func(context.Context) (*gamesclient.Service, error) {
		return nil, errors.New("aggregate Games service must not be created for icon uploads")
	})
	ctx = gamesclient.ContextWithLegacyImageUploaderFactory(ctx, func(context.Context) (*gamesclient.LegacyImageUploader, error) {
		return gamesclient.NewLegacyImageUploaderWithClient(server.Client(), server.URL+"/"), nil
	})
	err := cmd.Exec(ctx, nil)
	if err == nil || !strings.Contains(err.Error(), "Upload the icon in Play Console") {
		t.Fatalf("error = %v", err)
	}
}

func TestGameIconUploadCommand_DryRunMakesNoRequestAndRedactsBody(t *testing.T) {
	path := writeGameIcon(t, "png", 512, 512)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests++ }))
	defer server.Close()
	var transportLog, stdout, stderr bytes.Buffer
	client := server.Client()
	client.Transport = &shared.DryRunTransport{Base: client.Transport, Writer: &transportLog}

	cmd := leaderboardsUploadIconCommand()
	if err := cmd.FlagSet.Parse([]string{"--leaderboard-id", "leaderboard-1", "--file", path, "--acknowledge-deprecated-api"}); err != nil {
		t.Fatal(err)
	}
	ctx := shared.ContextWithDryRun(context.Background(), true)
	ctx = shared.ContextWithIO(ctx, &stdout, &stderr)
	ctx = gamesclient.ContextWithServiceFactory(ctx, func(context.Context) (*gamesclient.Service, error) {
		return nil, errors.New("aggregate Games service must not be created for icon uploads")
	})
	ctx = gamesclient.ContextWithLegacyImageUploaderFactory(ctx, func(context.Context) (*gamesclient.LegacyImageUploader, error) {
		return gamesclient.NewLegacyImageUploaderWithClient(client, server.URL+"/"), nil
	})
	if err := cmd.Exec(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if requests != 0 {
		t.Fatalf("requests = %d, want 0", requests)
	}
	if !strings.Contains(stderr.String(), "[DRY RUN] Play Games icon:") || !strings.Contains(stderr.String(), "dimensions=512x512") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if !strings.Contains(transportLog.String(), "non-text body omitted") {
		t.Fatalf("transport log = %q", transportLog.String())
	}
}

func writeGameIcon(t *testing.T, format string, width, height int) string {
	t.Helper()
	ext := ".png"
	if format == "jpeg" {
		ext = ".jpg"
	}
	path := filepath.Join(t.TempDir(), "icon"+ext)
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 0x7f, A: 0xff})
		}
	}
	if format == "jpeg" {
		err = jpeg.Encode(file, img, &jpeg.Options{Quality: 90})
	} else {
		err = png.Encode(file, img)
	}
	closeErr := file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	return path
}

func writePNGHeaderOnly(t *testing.T, width, height uint32) string {
	t.Helper()
	var data bytes.Buffer
	data.Write([]byte("\x89PNG\r\n\x1a\n"))
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], 13)
	data.Write(length[:])
	chunk := make([]byte, 17)
	copy(chunk, "IHDR")
	binary.BigEndian.PutUint32(chunk[4:8], width)
	binary.BigEndian.PutUint32(chunk[8:12], height)
	chunk[12] = 8
	chunk[13] = 6
	data.Write(chunk)
	var checksum [4]byte
	binary.BigEndian.PutUint32(checksum[:], crc32.ChecksumIEEE(chunk))
	data.Write(checksum[:])

	path := filepath.Join(t.TempDir(), "header-only.png")
	if err := os.WriteFile(path, data.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
