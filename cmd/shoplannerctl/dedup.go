// `shoplannerctl db dedup-logins` resolves duplicate logins in a shoplanner database.
//
// It is the answer to a server that refuses to start with
//
//	{"level":"fatal","error":"can't init users login index: users.login holds duplicates ..."}
//
// The GORM-era schema enforced no uniqueness on users.login (see internal/backend/user/repo/
// schema.sql), so databases from before the sqlc migration can hold two accounts under one
// login. The current code creates idx_users_login at boot and cannot do so over such rows.
package main

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"

	"go-backend/internal/backend/userdedup"
)

var (
	// errNoDBPath is a usage error: without a database there is nothing to inspect.
	errNoDBPath = errors.New("no database path: pass --db or set DB_PATH")
	// errUnresolved leaves a non-zero exit code behind when duplicates are still there, so
	// that this can be used as a check in a script or a preflight step.
	errUnresolved = errors.New("duplicate logins are still present; the server will not start")
	// errQuit unwinds the prompt loop when the operator asks to leave. Nothing is written.
	errQuit = errors.New("cancelled, nothing was changed")
)

func newDedupLoginsCmd() *cobra.Command {
	var (
		dbPath    string
		report    bool
		assumeYes bool
	)

	cmd := &cobra.Command{
		Use:   "dedup-logins",
		Short: "Resolve accounts that share a login",
		Long: "Lists every login held by more than one account, with how much data each account owns, " +
			"and asks which account keeps it. The others are renamed to <login>__dup__<id prefix>, or " +
			"deleted if they own nothing and that is what you ask for.\n\n" +
			"Nothing is written until every group has been decided, and then all of it in one " +
			"transaction. Afterwards the unique index the server needs is created, so a successful " +
			"run means the service will start.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDedupLogins(cmd, dbPath, report, assumeYes)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", os.Getenv("DB_PATH"), "path to the SQLite database (default $DB_PATH)")
	cmd.Flags().BoolVar(&report, "report", false, "list the duplicates and exit without changing anything")
	cmd.Flags().BoolVar(&assumeYes, "yes", false,
		"do not ask: in every group keep the account owning the most data and rename the others")

	return cmd
}

func runDedupLogins(cmd *cobra.Command, dbPath string, report, assumeYes bool) error {
	if dbPath == "" {
		return errNoDBPath
	}

	// sql.Open on a missing file would create an empty database and report no duplicates,
	// which is a confusing way to answer "your database is somewhere else".
	if _, err := os.Stat(dbPath); err != nil {
		return fmt.Errorf("can't open the database: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("can't open the database: %w", err)
	}
	defer func() { _ = db.Close() }()

	ctx := cmd.Context()
	out := cmd.OutOrStdout()

	groups, err := userdedup.Scan(ctx, db)
	if err != nil {
		return err //nolint:wrapcheck // Scan's errors already name the database operation
	}

	if len(groups) == 0 {
		fmt.Fprintln(out, "no duplicate logins.")

		return finish(ctx, db, out, report)
	}

	// In interactive mode the prompt prints each group as it asks about it, so listing them
	// all first would just say everything twice.
	if report || assumeYes {
		printGroups(out, groups)
	}

	if report {
		return errUnresolved
	}

	in := bufio.NewReader(cmd.InOrStdin())

	decisions, err := plan(out, groups, assumeYes, in)
	if err != nil {
		return err
	}

	if err = confirm(out, in, decisions, assumeYes); err != nil {
		return err
	}

	if err = applyDecisions(ctx, db, out, decisions); err != nil {
		return err
	}

	return finish(ctx, db, out, false)
}

// confirm shows the whole plan and waits for a yes. The per-group answers are given one group
// at a time with the others out of sight, which is the wrong moment to be sure about the shape
// of the whole change; this is the moment.
func confirm(out io.Writer, in *bufio.Reader, decisions []userdedup.Decision, assumeYes bool) error {
	if len(decisions) == 0 {
		return nil
	}

	fmt.Fprintln(out, "\nplanned changes:")

	for _, decision := range decisions {
		switch decision.Action {
		case userdedup.ActionRename:
			fmt.Fprintf(out, "  rename %s: %q -> %q\n",
				decision.Account.ID, decision.Account.Login, userdedup.ProposedLogin(decision.Account))
		case userdedup.ActionDelete:
			fmt.Fprintf(out, "  DELETE %s (%q), which owns nothing\n", decision.Account.ID, decision.Account.Login)
		}
	}

	if assumeYes {
		return nil
	}

	fmt.Fprint(out, "apply? [y/N]: ")

	line, err := in.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("can't read the answer: %w", err)
	}

	if strings.ToLower(strings.TrimSpace(line)) != "y" {
		return errQuit
	}

	return nil
}

// finish creates the index the server needs, so that the tool can answer the question the
// operator actually has: whether the service will come up now.
func finish(ctx context.Context, db *sql.DB, out io.Writer, report bool) error {
	if report {
		return nil
	}

	if err := userdedup.EnsureUniqueIndex(ctx, db); err != nil {
		return err //nolint:wrapcheck // EnsureUniqueIndex already names the index
	}

	fmt.Fprintln(out, "idx_users_login is in place — the server can start.")

	return nil
}

func applyDecisions(ctx context.Context, db *sql.DB, out io.Writer, decisions []userdedup.Decision) error {
	if len(decisions) == 0 {
		return errUnresolved
	}

	outcomes, err := userdedup.Apply(ctx, db, decisions)
	if err != nil {
		return err //nolint:wrapcheck // Apply's errors already name the account and the operation
	}

	fmt.Fprintln(out)

	for _, outcome := range outcomes {
		switch outcome.Action {
		case userdedup.ActionRename:
			fmt.Fprintf(out, "renamed %s: %q -> %q\n", outcome.Account.ID, outcome.Account.Login, outcome.NewLogin)
		case userdedup.ActionDelete:
			fmt.Fprintf(out, "deleted %s (%q), which owned nothing\n", outcome.Account.ID, outcome.Account.Login)
		}
	}

	remaining, err := userdedup.Scan(ctx, db)
	if err != nil {
		return err //nolint:wrapcheck // Scan's errors already name the database operation
	}

	if len(remaining) > 0 {
		fmt.Fprintln(out)
		printGroups(out, remaining)

		return errUnresolved
	}

	return nil
}

func printGroups(out io.Writer, groups []userdedup.Group) {
	for _, group := range groups {
		fmt.Fprintf(out, "\nlogin %q is shared by %d accounts:\n", string(group.Login), len(group.Accounts))

		for i, acc := range group.Accounts {
			fmt.Fprintf(out, "  %d) %s  role=%s  %s\n", i+1, acc.ID, acc.Role, describe(acc))
		}
	}
}

func describe(acc userdedup.Account) string {
	if acc.Usage.Total() == 0 {
		return "owns nothing"
	}

	return acc.Usage.String()
}

// plan turns the groups into the list of changes to make, either by asking or by taking the
// recommendation. It asks about every group before anything is written: a half-resolved
// database is worse than the one the operator started with.
func plan(out io.Writer, groups []userdedup.Group, assumeYes bool, in *bufio.Reader) ([]userdedup.Decision, error) {
	if assumeYes {
		decisions := []userdedup.Decision{}
		for _, group := range groups {
			decisions = append(decisions, group.Recommend()...)
		}

		return decisions, nil
	}

	decisions := []userdedup.Decision{}

	for _, group := range groups {
		chosen, err := ask(out, group, in)
		if err != nil {
			return nil, err
		}

		decisions = append(decisions, chosen...)
	}

	return decisions, nil
}

// ask prompts for one group until it gets an answer it understands.
func ask(out io.Writer, group userdedup.Group, in *bufio.Reader) ([]userdedup.Decision, error) {
	for {
		fmt.Fprintf(out, "\nlogin %q is shared by %d accounts:\n", string(group.Login), len(group.Accounts))

		for i, acc := range group.Accounts {
			fmt.Fprintf(out, "  %d) %s  role=%s  %s\n", i+1, acc.ID, acc.Role, describe(acc))
		}

		fmt.Fprintf(out, "keep which account? [1-%d] keeps it and renames the others, "+
			"append d to delete the ones owning nothing (e.g. 2d), s skips, q quits [1]: ", len(group.Accounts))

		line, err := in.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("can't read the answer: %w", err)
		}

		answer := strings.ToLower(strings.TrimSpace(line))
		if errors.Is(err, io.EOF) && answer == "" {
			return nil, errQuit
		}

		decisions, ok := parseAnswer(group, answer)
		if ok {
			return decisions, nil
		}

		if answer == "q" {
			return nil, errQuit
		}

		fmt.Fprintf(out, "did not understand %q.\n", answer)
	}
}

// parseAnswer maps one prompt answer onto the changes it stands for. The bool is false when the
// answer made no sense, which the caller turns back into another prompt.
func parseAnswer(group userdedup.Group, answer string) ([]userdedup.Decision, bool) {
	if answer == "s" {
		return nil, true
	}

	deleteEmpty := strings.HasSuffix(answer, "d")
	answer = strings.TrimSuffix(answer, "d")

	// An empty answer means the default, which is the first account: Scan sorted the group so
	// that the one owning the most data comes first.
	keep := 1

	if answer != "" {
		parsed, err := strconv.Atoi(answer)
		if err != nil || parsed < 1 || parsed > len(group.Accounts) {
			return nil, false
		}

		keep = parsed
	}

	return group.Resolve(group.Accounts[keep-1], deleteEmpty), true
}
