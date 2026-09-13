// Command dev-firebase-admin provisions the Firebase Auth identity that
// corresponds to an existing seeded/bootstrapped Labuda DEVELOPMENT admin, so a
// developer can sign in to the Admin dashboard through the normal Firebase
// login flow.
//
// WHAT IT DOES
//   - creates (or reports) a real Firebase Auth user for --email
//
// WHAT IT DOES NOT DO
//   - it never creates a Labuda DB user
//   - it never grants capabilities or changes roles
//   - it never issues Labuda session tokens
//   - it never bypasses Firebase login, the exchange, or any middleware
//   - it never enables DEV_MOCK_FIREBASE_AUTH or any other bypass
//
// The canonical authentication chain is preserved end to end:
//
//	Firebase Auth account
//	  -> Firebase ID token
//	    -> POST /api/v1/auth/firebase/exchange
//	      -> existing DB identity mapping (by firebase_uid, falling back to email)
//	        -> existing admin role + capability authority
//
// DEVELOPMENT ONLY: refuses to run unless ENV=development.
//
// Usage:
//
//	cd backend
//	read -s PW                                   # silent, not in shell history
//	printf '%s' "$PW" | go run ./cmd/dev-firebase-admin --email admin@test.local --password-stdin
//
// The password is read from stdin only; it is never accepted as an argument, so
// it cannot leak through the process list or shell history.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/labuda/backend/internal/config"
	"github.com/labuda/backend/internal/platform/logger"
	"github.com/labuda/backend/pkg/firebase"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin))
}

// run is split out of main so flag/guard behavior is inspectable without
// executing the process.
func run(args []string, stdin io.Reader) int {
	fs := flag.NewFlagSet("dev-firebase-admin", flag.ContinueOnError)
	var email string
	var passwordStdin bool
	fs.StringVar(&email, "email", "", "Firebase email corresponding to an existing Labuda admin (e.g. admin@test.local)")
	fs.BoolVar(&passwordStdin, "password-stdin", false, "Read the development password from stdin (one line). Passwords must never be passed as arguments.")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if strings.TrimSpace(email) == "" || !passwordStdin {
		fmt.Fprintln(os.Stderr, "error: --email is required and --password-stdin must be set")
		fs.Usage()
		return 2
	}

	// Resolve environment + Firebase configuration through the canonical loader.
	// ENV defaults to "production" when unset, so the guard below fails closed.
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config load failed: %v\n", err)
		return 1
	}
	if err := guardDevelopment(cfg.Server.Env); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	password, err := readPassword(stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read password from stdin: %v\n", err)
		return 1
	}

	log, err := logger.New(cfg.Logging.Level, cfg.Logging.Format, cfg.Logging.Output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger init failed: %v\n", err)
		return 1
	}
	defer log.Sync()

	// Always use the REAL Firebase Admin client: provisioning must produce a real
	// Firebase identity, never a mock.
	fb, err := firebase.NewFirebaseClient(&cfg.Firebase, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "firebase init failed: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := Provision(ctx, realFirebaseAuth{client: fb}, Options{
		Email:    email,
		Password: password,
		Env:      cfg.Server.Env,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "provisioning failed: %v\n", err)
		return 1
	}

	fmt.Println("dev-firebase-admin: DEVELOPMENT ONLY")
	fmt.Printf("  project_id: %s\n", cfg.Firebase.ProjectID)
	fmt.Printf("  email: %s\n", result.Email)
	fmt.Printf("  status: %s\n", result.Status)
	fmt.Printf("  uid: %s\n", result.UID)

	switch result.Status {
	case StatusCreated:
		fmt.Println("  next: sign in to the Admin dashboard with this email/password through the normal Firebase login.")
	case StatusAlreadyExists:
		fmt.Println("  note: the Firebase user already existed; nothing was changed (password and attributes untouched).")
	case StatusExistsDisabled:
		fmt.Fprintln(os.Stderr, "  action: this Firebase user is disabled. Re-enable it in Firebase Authentication (or delete it and re-run) before signing in.")
		return 1
	}
	return 0
}

// readPassword reads one line from stdin and strips the trailing newline. It is
// deliberately stdin-only so the password never reaches argv.
func readPassword(stdin io.Reader) (string, error) {
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
