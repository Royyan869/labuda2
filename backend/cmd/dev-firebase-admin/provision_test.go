package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"firebase.google.com/go/v4/auth"
)

// fakeFirebaseAuth is a test double for the firebaseAuth seam. It records calls
// so tests can prove exactly which Firebase operations (and no others) ran.
type fakeFirebaseAuth struct {
	existing  *auth.UserRecord
	findErr   error
	createErr error
	created   *auth.UserRecord
	calls     []string
}

func (f *fakeFirebaseAuth) FindByEmail(_ context.Context, _ string) (*auth.UserRecord, bool, error) {
	f.calls = append(f.calls, "FindByEmail")
	if f.findErr != nil {
		return nil, false, f.findErr
	}
	if f.existing == nil {
		return nil, false, nil
	}
	return f.existing, true, nil
}

func (f *fakeFirebaseAuth) Create(_ context.Context, email, _ string) (*auth.UserRecord, error) {
	f.calls = append(f.calls, "Create")
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.created != nil {
		return f.created, nil
	}
	return userRecord("new-uid", email, false), nil
}

func userRecord(uid, email string, disabled bool) *auth.UserRecord {
	return &auth.UserRecord{
		UserInfo: &auth.UserInfo{UID: uid, Email: email},
		Disabled: disabled,
	}
}

const devEnv = "development"

func devOptions() Options {
	return Options{Email: "admin@test.local", Password: "devpass123", Env: devEnv}
}

// ── Environment safety ──────────────────────────────────────────────────────

func TestGuardDevelopment_OnlyAllowsDevelopment(t *testing.T) {
	if err := guardDevelopment("development"); err != nil {
		t.Fatalf("development must be allowed: %v", err)
	}
	for _, env := range []string{"production", "staging", "", "  ", "Production", "DEVELOPMENT"} {
		if err := guardDevelopment(env); err == nil {
			t.Fatalf("env %q must be refused", env)
		}
	}
}

func TestProvision_RefusesNonDevelopmentWithoutTouchingFirebase(t *testing.T) {
	for _, env := range []string{"production", "staging", ""} {
		fb := &fakeFirebaseAuth{}
		opts := devOptions()
		opts.Env = env

		res, err := Provision(context.Background(), fb, opts)

		if err == nil {
			t.Fatalf("env %q: expected refusal", env)
		}
		if res != nil {
			t.Fatalf("env %q: expected nil result on refusal", env)
		}
		if len(fb.calls) != 0 {
			t.Fatalf("env %q: no Firebase call may run before the environment guard (got %v)", env, fb.calls)
		}
	}
}

// ── Input validation ────────────────────────────────────────────────────────

func TestProvision_RejectsInvalidInputWithoutTouchingFirebase(t *testing.T) {
	cases := map[string]Options{
		"missing email":   {Email: "", Password: "devpass123", Env: devEnv},
		"invalid email":   {Email: "not-an-email", Password: "devpass123", Env: devEnv},
		"missing password": {Email: "admin@test.local", Password: "", Env: devEnv},
		"short password":  {Email: "admin@test.local", Password: "12345", Env: devEnv},
	}
	for name, opts := range cases {
		fb := &fakeFirebaseAuth{}
		res, err := Provision(context.Background(), fb, opts)

		if err == nil {
			t.Fatalf("%s: expected validation error", name)
		}
		if res != nil {
			t.Fatalf("%s: expected nil result", name)
		}
		if len(fb.calls) != 0 {
			t.Fatalf("%s: no Firebase call may run before input validation (got %v)", name, fb.calls)
		}
	}
}

// ── Idempotency / outcomes ──────────────────────────────────────────────────

func TestProvision_CreatesWhenFirebaseUserAbsent(t *testing.T) {
	fb := &fakeFirebaseAuth{}
	res, err := Provision(context.Background(), fb, devOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusCreated {
		t.Fatalf("status = %q, want %q", res.Status, StatusCreated)
	}
	if res.UID != "new-uid" {
		t.Fatalf("uid = %q, want new-uid", res.UID)
	}
	if got := strings.Join(fb.calls, ","); got != "FindByEmail,Create" {
		t.Fatalf("calls = %q, want FindByEmail,Create", got)
	}
}

func TestProvision_AlreadyExistsDoesNotOverwrite(t *testing.T) {
	fb := &fakeFirebaseAuth{existing: userRecord("existing-uid", "admin@test.local", false)}
	res, err := Provision(context.Background(), fb, devOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusAlreadyExists {
		t.Fatalf("status = %q, want %q", res.Status, StatusAlreadyExists)
	}
	if res.UID != "existing-uid" {
		t.Fatalf("uid = %q, want existing-uid", res.UID)
	}
	if got := strings.Join(fb.calls, ","); got != "FindByEmail" {
		t.Fatalf("calls = %q, want only FindByEmail (password/attributes must be untouched)", got)
	}
}

func TestProvision_DisabledUserIsReportedNotEnabled(t *testing.T) {
	fb := &fakeFirebaseAuth{existing: userRecord("disabled-uid", "admin@test.local", true)}
	res, err := Provision(context.Background(), fb, devOptions())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusExistsDisabled {
		t.Fatalf("status = %q, want %q", res.Status, StatusExistsDisabled)
	}
	if got := strings.Join(fb.calls, ","); got != "FindByEmail" {
		t.Fatalf("calls = %q, want only FindByEmail (a disabled user must not be silently enabled)", got)
	}
}

// ── Error propagation ───────────────────────────────────────────────────────

func TestProvision_PropagatesLookupError(t *testing.T) {
	fb := &fakeFirebaseAuth{findErr: errors.New("lookup boom")}
	res, err := Provision(context.Background(), fb, devOptions())
	if err == nil {
		t.Fatal("expected error")
	}
	if res != nil {
		t.Fatal("expected nil result")
	}
	if !strings.Contains(err.Error(), "firebase lookup failed") || !strings.Contains(err.Error(), "lookup boom") {
		t.Fatalf("error must wrap the cause, got: %v", err)
	}
	if strings.Contains(strings.Join(fb.calls, ","), "Create") {
		t.Fatal("must not attempt creation after a lookup failure")
	}
}

func TestProvision_PropagatesCreateError(t *testing.T) {
	fb := &fakeFirebaseAuth{createErr: errors.New("create boom")}
	res, err := Provision(context.Background(), fb, devOptions())
	if err == nil {
		t.Fatal("expected error")
	}
	if res != nil {
		t.Fatal("expected nil result")
	}
	if !strings.Contains(err.Error(), "firebase create failed") || !strings.Contains(err.Error(), "create boom") {
		t.Fatalf("error must wrap the cause, got: %v", err)
	}
}

// ── Password input ──────────────────────────────────────────────────────────

func TestReadPassword_TrimsNewlineOnly(t *testing.T) {
	cases := map[string]string{
		"devpass123\n":   "devpass123",
		"devpass123\r\n": "devpass123",
		"devpass123":     "devpass123",
		"  spaced  \n":   "  spaced  ", // only the newline is stripped
	}
	for in, want := range cases {
		got, err := readPassword(strings.NewReader(in))
		if err != nil {
			t.Fatalf("input %q: unexpected error %v", in, err)
		}
		if got != want {
			t.Fatalf("input %q: got %q, want %q", in, got, want)
		}
	}
}

func TestRun_RequiresEmailAndPasswordStdin(t *testing.T) {
	if code := run([]string{}, strings.NewReader("")); code != 2 {
		t.Fatalf("no flags: exit = %d, want 2", code)
	}
	if code := run([]string{"--email", "admin@test.local"}, strings.NewReader("")); code != 2 {
		t.Fatalf("missing --password-stdin: exit = %d, want 2", code)
	}
	if code := run([]string{"--password-stdin"}, strings.NewReader("")); code != 2 {
		t.Fatalf("missing --email: exit = %d, want 2", code)
	}
}

// ── Scope guard: never touch Labuda DB / role / capability authority ────────

func TestCommand_NeverTouchesLabudaDatabaseOrAuthority(t *testing.T) {
	forbidden := []string{
		"github.com/labuda/backend/pkg/database",
		"github.com/jackc/pgx",
		"user_capabilities",
		"AllCapabilityStrings",
		"SetRole(",
	}
	for _, file := range []string{"main.go", "provision.go"} {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		src := string(raw)
		for _, needle := range forbidden {
			if strings.Contains(src, needle) {
				t.Fatalf("%s must not reference %q — this command provisions Firebase identity only", file, needle)
			}
		}
	}
}
