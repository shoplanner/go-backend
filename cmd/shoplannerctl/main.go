// Command shoplannerctl is the administrative CLI for a shoplanner installation.
//
// It is meant to be run on the server itself, next to the service, and is split accordingly:
//
//	shoplannerctl db ...     operates on the SQLite file directly, with the service stopped
//
// The API side (talking to a running server over HTTP) is not wired up yet; see client.go.
//
// Everything under `db` is for situations the server cannot get itself out of — today that is
// exactly one, duplicate logins left behind by the GORM-era schema, which stop it from creating
// idx_users_login and therefore from starting at all.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

func main() {
	os.Exit(run())
}

// run is separate from main so that the signal handler is torn down on the way out: os.Exit
// runs no deferred calls.
func run() int {
	// The db commands write in a transaction, so a Ctrl-C mid-run has to cancel the queries
	// rather than kill the process between two of them.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := newRootCmd().ExecuteContext(ctx); err != nil {
		// cobra has already printed the error to stderr.
		return 1
	}

	return 0
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "shoplannerctl",
		Short: "Administrative CLI for a shoplanner installation",
		// The errors these commands report are conditions, not usage mistakes; printing the
		// whole help text after each one buries them.
		SilenceUsage: true,
	}

	root.AddCommand(newDBCmd())

	return root
}

func newDBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Operate on the database file directly",
		Long: "Commands that open the SQLite file themselves. Stop shoplanner.service first: " +
			"the server holds the same file open, and these commands change rows underneath it.",
	}

	cmd.AddCommand(newDedupLoginsCmd())

	return cmd
}
