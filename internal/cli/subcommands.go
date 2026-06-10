package cli

import (
	"context"
	"fmt"
	"io"
)

type subcommandHandler func(context.Context, []string) error

func runSubcommandGroup(
	ctx context.Context,
	args []string,
	errW io.Writer,
	group string,
	missingIsError bool,
	usageFn func(),
	handlers map[string]subcommandHandler,
) error {
	if len(args) == 0 {
		usageFn()
		if missingIsError {
			return UsageError("missing %s subcommand", group)
		}
		return nil
	}
	if isHelpToken(args[0]) {
		usageFn()
		return nil
	}
	h, ok := handlers[args[0]]
	if !ok {
		fmt.Fprintf(errW, "unknown %s subcommand %q\n", group, args[0])
		usageFn()
		return UsageError("unknown %s subcommand %q", group, args[0])
	}
	return h(ctx, args[1:])
}
