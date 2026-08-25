package cmdtest_test

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGames_Help(t *testing.T) {
	stdout, stderr, err := runCommand(t, "games")
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected help error, got %v", err)
	}
	combined := stdout + stderr
	for _, want := range []string{"games", "achievements", "leaderboards", "scores", "events", "players"} {
		if !strings.Contains(combined, want) {
			t.Fatalf("help should mention %q, got: %q", want, combined)
		}
	}
}

func TestGamesAchievements_Help(t *testing.T) {
	stdout, stderr, err := runCommand(t, "games", "achievements")
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected help error, got %v", err)
	}
	combined := stdout + stderr
	for _, want := range []string{"list", "get", "create", "update", "delete", "upload-icon", "reset", "reset-for-all-players"} {
		if !strings.Contains(combined, want) {
			t.Fatalf("achievements help should mention %q, got: %q", want, combined)
		}
	}
}

func TestGamesLeaderboards_Help(t *testing.T) {
	stdout, stderr, err := runCommand(t, "games", "leaderboards")
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected help error, got %v", err)
	}
	combined := stdout + stderr
	for _, want := range []string{"list", "get", "create", "update", "delete", "upload-icon"} {
		if !strings.Contains(combined, want) {
			t.Fatalf("leaderboards help should mention %q, got: %q", want, combined)
		}
	}
}

func TestGamesAchievementsUploadIcon_MissingID(t *testing.T) {
	_, stderr, err := runCommand(t, "games", "achievements", "upload-icon")
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
	if !strings.Contains(stderr, "--achievement-id is required") {
		t.Fatalf("stderr should mention missing achievement id, got: %q", stderr)
	}
}

func TestGamesLeaderboardsUploadIcon_MissingFile(t *testing.T) {
	_, stderr, err := runCommand(t, "games", "leaderboards", "upload-icon", "--leaderboard-id", "leaderboard-1")
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
	if !strings.Contains(stderr, "--file is required") {
		t.Fatalf("stderr should mention missing file, got: %q", stderr)
	}
}

func TestGamesAchievementsUploadIcon_RequiresDeprecatedAPIAcknowledgement(t *testing.T) {
	_, stderr, err := runCommand(t,
		"games", "achievements", "upload-icon",
		"--achievement-id", "achievement-1",
		"--file", "icon.png",
	)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
	if !strings.Contains(stderr, "--acknowledge-deprecated-api") {
		t.Fatalf("stderr should mention deprecated API acknowledgement, got: %q", stderr)
	}
}

func TestGamesAchievementsUploadIcon_ValidatesBeforeAuthentication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "icon.png")
	if err := os.WriteFile(path, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, stderr, err := runCommand(t,
		"games", "achievements", "upload-icon",
		"--achievement-id", "achievement-1",
		"--file", path,
		"--acknowledge-deprecated-api",
	)
	if err == nil {
		t.Fatal("expected invalid image error")
	}
	combined := stderr + err.Error()
	if !strings.Contains(combined, "valid PNG or JPEG") {
		t.Fatalf("error should report image validation before authentication, got: %q", combined)
	}
	if strings.Contains(combined, "authentication failed") {
		t.Fatalf("validation should happen before authentication, got: %q", combined)
	}
}

func TestGamesAchievementsUploadIcon_RejectsInvalidOutputBeforeUpload(t *testing.T) {
	_, stderr, err := runCommand(t,
		"games", "achievements", "upload-icon",
		"--achievement-id", "achievement-1",
		"--file", "missing.png",
		"--acknowledge-deprecated-api",
		"--output", "yaml",
	)
	if err == nil {
		t.Fatal("expected unsupported output error")
	}
	combined := stderr + err.Error()
	if !strings.Contains(combined, "unsupported format") {
		t.Fatalf("error should report unsupported output, got: %q", combined)
	}
	if strings.Contains(combined, "failed to open") || strings.Contains(combined, "authentication failed") {
		t.Fatalf("output validation should happen before file or API work, got: %q", combined)
	}
}

func TestGamesAchievementsList_MissingApplicationID(t *testing.T) {
	t.Setenv("GPLAY_GAMES_APP_ID", "")
	_, stderr, err := runCommand(t, "games", "achievements", "list")
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
	if !strings.Contains(stderr, "--application-id is required") {
		t.Fatalf("stderr should mention missing application id, got: %q", stderr)
	}
}

func TestGamesAchievementsGet_MissingID(t *testing.T) {
	_, stderr, err := runCommand(t, "games", "achievements", "get")
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
	if !strings.Contains(stderr, "--achievement-id is required") {
		t.Fatalf("stderr should mention missing achievement id, got: %q", stderr)
	}
}

func TestGamesAchievementsCreate_MissingData(t *testing.T) {
	_, stderr, err := runCommand(t, "games", "achievements", "create", "--application-id", "123")
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
	if !strings.Contains(stderr, "--data is required") {
		t.Fatalf("stderr should mention missing data, got: %q", stderr)
	}
}

func TestGamesAchievementsDelete_RequiresConfirm(t *testing.T) {
	_, stderr, err := runCommand(t, "games", "achievements", "delete", "--achievement-id", "abc")
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
	if !strings.Contains(stderr, "--confirm") {
		t.Fatalf("stderr should mention --confirm, got: %q", stderr)
	}
}

func TestGamesLeaderboardsList_MissingApplicationID(t *testing.T) {
	t.Setenv("GPLAY_GAMES_APP_ID", "")
	_, stderr, err := runCommand(t, "games", "leaderboards", "list")
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
	if !strings.Contains(stderr, "--application-id is required") {
		t.Fatalf("stderr should mention missing application id, got: %q", stderr)
	}
}

func TestGamesScoresReset_MissingLeaderboardID(t *testing.T) {
	_, stderr, err := runCommand(t, "games", "scores", "reset")
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
	if !strings.Contains(stderr, "--leaderboard-id is required") {
		t.Fatalf("stderr should mention missing leaderboard id, got: %q", stderr)
	}
}

func TestGamesEventsResetForAll_RequiresConfirm(t *testing.T) {
	_, stderr, err := runCommand(t, "games", "events", "reset-for-all-players", "--event-id", "e1")
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
	if !strings.Contains(stderr, "--confirm") {
		t.Fatalf("stderr should mention --confirm, got: %q", stderr)
	}
}

func TestGamesPlayersHide_MissingPlayerID(t *testing.T) {
	_, stderr, err := runCommand(t, "games", "players", "hide", "--application-id", "123")
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
	if !strings.Contains(stderr, "--player-id is required") {
		t.Fatalf("stderr should mention missing player id, got: %q", stderr)
	}
}
