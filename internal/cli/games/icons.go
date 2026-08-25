package games

import (
	"context"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/tamtom/play-console-cli/internal/cli/shared"
	"github.com/tamtom/play-console-cli/internal/gamesclient"
)

const (
	playGamesIconDimension = 512
	playGamesIconMaxBytes  = 15 * 1024 * 1024
)

const deprecatedImageAPIHint = "Google has deprecated and removed this image upload method from the live discovery API, so the request may be rejected. Upload the icon in Play Console if that happens."

type validatedGameIcon struct {
	file        *os.File
	contentType string
	size        int64
}

func achievementsUploadIconCommand() *ffcli.Command {
	return gameIconUploadCommand("achievement", "achievement-id", gamesclient.AchievementIcon)
}

func leaderboardsUploadIconCommand() *ffcli.Command {
	return gameIconUploadCommand("leaderboard", "leaderboard-id", gamesclient.LeaderboardIcon)
}

func gameIconUploadCommand(resourceName, idFlagName, imageType string) *ffcli.Command {
	commandPath := "games " + resourceName + "s upload-icon"
	fs := flag.NewFlagSet(commandPath, flag.ExitOnError)
	resourceID := fs.String(idFlagName, "", strings.ToUpper(resourceName[:1])+resourceName[1:]+" ID")
	filePath := fs.String("file", "", "Path to a 512x512 PNG or JPEG icon")
	acknowledge := fs.Bool("acknowledge-deprecated-api", false, "Acknowledge that Google's deprecated image API may reject the request")
	outputFlag := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "upload-icon",
		ShortUsage: fmt.Sprintf("gplay %s --%s <id> --file <path> --acknowledge-deprecated-api [flags]", commandPath, idFlagName),
		ShortHelp:  fmt.Sprintf("Upload or replace the icon for a Play Games %s through Google's deprecated official API.", resourceName),
		LongHelp: fmt.Sprintf(`Upload or replace the icon for a Play Games %s.

Google has deprecated this official image upload API and removed it from the
live discovery document. Calls may fail even with valid credentials and image
data. The acknowledgement flag is required on every attempt.

The icon must be a decodable 512x512 PNG or JPEG no larger than 15 MB.`, resourceName),
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := shared.ValidateStructuredOutputFlags(*outputFlag, *pretty); err != nil {
				return err
			}
			if strings.TrimSpace(*resourceID) == "" {
				return shared.UsageErrorf("--%s is required", idFlagName)
			}
			if strings.TrimSpace(*filePath) == "" {
				return shared.UsageError("--file is required")
			}
			if !*acknowledge {
				return shared.UsageError("--acknowledge-deprecated-api is required because Google documents this upload method as deprecated and removed")
			}

			icon, err := openValidatedGameIcon(*filePath)
			if err != nil {
				return err
			}
			defer icon.file.Close()

			fmt.Fprintf(shared.Stderr(ctx), "WARNING: %s\n", deprecatedImageAPIHint)
			if shared.IsDryRun(ctx) {
				fmt.Fprintf(
					shared.Stderr(ctx),
					"[DRY RUN] Play Games icon: resource=%s imageType=%s file=%s contentType=%s bytes=%d dimensions=512x512\n",
					*resourceID,
					imageType,
					strings.TrimSpace(*filePath),
					icon.contentType,
					icon.size,
				)
			}

			uploader, err := gamesclient.NewLegacyImageUploader(ctx)
			if err != nil {
				return err
			}
			uploadCtx, cancel := shared.ContextWithUploadTimeout(ctx, uploader.Cfg)
			defer cancel()

			resp, err := uploader.UploadImage(uploadCtx, strings.TrimSpace(*resourceID), imageType, icon.file, icon.size, icon.contentType)
			if err != nil {
				return shared.WrapActionable(err, "upload Play Games "+resourceName+" icon through deprecated official API", deprecatedImageAPIHint)
			}
			return shared.PrintOutputContext(ctx, resp, *outputFlag, *pretty)
		},
	}
}

func openValidatedGameIcon(path string) (*validatedGameIcon, error) {
	trimmedPath := strings.TrimSpace(path)
	file, err := os.Open(trimmedPath)
	if err != nil {
		return nil, shared.WrapActionable(err, "failed to open Play Games icon", "Check that --file exists and is readable.")
	}
	fail := func(err error) (*validatedGameIcon, error) {
		_ = file.Close()
		return nil, err
	}

	info, err := file.Stat()
	if err != nil {
		return fail(shared.WrapActionable(err, "failed to inspect Play Games icon", "Check that --file is a readable regular file."))
	}
	if !info.Mode().IsRegular() {
		return fail(shared.NewValidationError("invalid Play Games icon", fmt.Errorf("%s is not a regular file", trimmedPath), "Provide a 512x512 PNG or JPEG file."))
	}
	if info.Size() > playGamesIconMaxBytes {
		return fail(shared.NewValidationError("invalid Play Games icon", fmt.Errorf("file size %d bytes exceeds the 15 MB API limit", info.Size()), "Reduce the icon to 15 MB or less."))
	}

	config, format, err := image.DecodeConfig(file)
	if err != nil || (format != "png" && format != "jpeg") {
		if err == nil {
			err = fmt.Errorf("unsupported decoded image format %q", format)
		}
		return fail(shared.NewValidationError("invalid Play Games icon", err, "Provide a valid PNG or JPEG image."))
	}
	if config.Width != playGamesIconDimension || config.Height != playGamesIconDimension {
		return fail(shared.NewValidationError(
			"invalid Play Games icon",
			fmt.Errorf("dimensions are %dx%d; Google requires 512x512", config.Width, config.Height),
			"Resize the icon to exactly 512x512 pixels.",
		))
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fail(shared.WrapActionable(err, "failed to rewind Play Games icon", "Check that --file is a regular file."))
	}
	if _, _, err := image.Decode(file); err != nil {
		return fail(shared.NewValidationError("invalid Play Games icon", err, "Provide a valid PNG or JPEG image that is fully decodable."))
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fail(shared.WrapActionable(err, "failed to rewind Play Games icon", "Check that --file is a regular file."))
	}

	contentType := "image/png"
	if format == "jpeg" {
		contentType = "image/jpeg"
	}
	return &validatedGameIcon{
		file:        file,
		contentType: contentType,
		size:        info.Size(),
	}, nil
}
