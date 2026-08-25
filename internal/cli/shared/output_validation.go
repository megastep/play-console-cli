package shared

import (
	"context"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// WrapCommandOutputValidation recursively wraps all commands' Exec functions
// to validate output format flags before execution. This prevents API calls
// when invalid output flags are passed.
func WrapCommandOutputValidation(cmd *ffcli.Command) {
	if cmd == nil {
		return
	}

	// Wrap subcommands recursively
	for _, sub := range cmd.Subcommands {
		WrapCommandOutputValidation(sub)
	}

	// Wrap this command's Exec
	originalExec := cmd.Exec
	if originalExec == nil {
		return
	}

	cmd.Exec = func(ctx context.Context, args []string) error {
		// Check if --output flag exists and validate its value
		if cmd.FlagSet != nil {
			outputFlag := cmd.FlagSet.Lookup("output")
			prettyFlag := cmd.FlagSet.Lookup("pretty")

			if outputFlag != nil {
				pretty := prettyFlag != nil && prettyFlag.Value.String() == "true"
				if err := ValidateStructuredOutputFlags(outputFlag.Value.String(), pretty); err != nil {
					return err
				}
			}
		}

		return originalExec(ctx, args)
	}
}
