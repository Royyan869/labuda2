// Package main provides the dev-firebase-admin command.
//
// This file holds the pure, testable provisioning core. It knows nothing about
// the Labuda database, roles, or capabilities: it only provisions the Firebase
// Auth identity that lets an EXISTING Labuda development admin sign in through
// the normal Firebase login flow.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"firebase.google.com/go/v4/auth"

	"github.com/labuda/backend/pkg/firebase"
)

// Status is the non-sensitive outcome of a provisioning attempt.
type Status string

const (
	// StatusCreated means a new Firebase Auth user was provisioned.
	StatusCreated Status = "created"

	// StatusAlreadyExists means a Firebase Auth user already existed and was
	// left completely untouched (password and attributes preserved).
	StatusAlreadyExists Status = "already_exists"

	// StatusExistsDisabled means a Firebase Auth user already existed but is
	// disabled. It is reported, never silently re-enabled.
	StatusExistsDisabled Status = "exists_disabled"
)

// Options are the validated, non-secret inputs for one provisioning run.
type Options struct {
	// Email is the Firebase email. It must match the email of an existing
	// seeded/bootstrapped Labuda admin. Provisioning alone does not bind
	// anything: FirebaseExchange binds the provisioned UID to that admin row on
	// the first login, because a Firebase-VERIFIED email is the account key.
	Email string

	// Password is the development password to set when creating the Firebase
	// user. It is never logged, printed, or persisted.
	Password string

	// Env is the resolved environment identity (config.Server.Env / ENV).
	Env string
}

// Result is the non-sensitive outcome of a provisioning run.
type Result struct {
	Status Status
	UID    string
	Email  string
}

// firebaseAuth is the minimal seam the provisioner needs. It is satisfied by
// realFirebaseAuth, which delegates entirely to the existing canonical
// *firebase.Client. It exists so the provisioning logic can be unit-tested
// without real Firebase credentials.
type firebaseAuth interface {
	// FindByEmail returns (nil, false, nil) when no Firebase user exists.
	FindByEmail(ctx context.Context, email string) (*auth.UserRecord, bool, error)

	// Create provisions a new Firebase user with an explicit email-verified
	// attribute.
	Create(ctx context.Context, email, password string, emailVerified bool) (*auth.UserRecord, error)
}

// realFirebaseAuth adapts the existing canonical Firebase client to firebaseAuth.
// It is a consumer-side seam, not a second SDK wrapper: every call still goes
// through *firebase.Client.
type realFirebaseAuth struct {
	client *firebase.Client
}

// FindByEmail looks the user up through the canonical client and classifies a
// not-found result as (nil, false, nil).
//
// NOTE: the canonical client wraps SDK errors with fmt.Errorf("%w"), while the
// SDK's auth.IsUserNotFound uses a direct type assertion (not errors.As). The
// wrapped error must therefore be unwrapped before the check, or a genuinely
// absent user would be misreported as a lookup failure.
func (r realFirebaseAuth) FindByEmail(ctx context.Context, email string) (*auth.UserRecord, bool, error) {
	user, err := r.client.GetUserByEmail(ctx, email)
	if err != nil {
		if isFirebaseUserNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return user, true, nil
}

// Create provisions a new Firebase user through the canonical client.
func (r realFirebaseAuth) Create(ctx context.Context, email, password string, emailVerified bool) (*auth.UserRecord, error) {
	return r.client.CreateUser(ctx, email, password, emailVerified)
}

// isFirebaseUserNotFound reports whether the error chain contains a Firebase
// "user not found" error.
func isFirebaseUserNotFound(err error) bool {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if auth.IsUserNotFound(e) {
			return true
		}
	}
	return false
}

// guardDevelopment refuses to run outside ENV=development.
//
// ENV is the repository's canonical environment identity. config.Load()
// defaults an unset ENV to "production", so an unconfigured or misconfigured
// environment fails closed here rather than provisioning in production.
func guardDevelopment(env string) error {
	if strings.TrimSpace(env) != "development" {
		return fmt.Errorf(
			"refusing to run: dev-firebase-admin provisions a development Firebase identity and only runs when ENV=development (got %q)",
			env,
		)
	}
	return nil
}

// validateInputs rejects missing or malformed input before any external call.
func validateInputs(opts Options) error {
	email := strings.TrimSpace(opts.Email)
	if email == "" {
		return errors.New("--email is required")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return fmt.Errorf("invalid --email: %w", err)
	}
	if opts.Password == "" {
		return errors.New("password is required (supply it on stdin with --password-stdin)")
	}
	if len(opts.Password) < 6 {
		return errors.New("password must be at least 6 characters (Firebase Auth minimum)")
	}
	return nil
}

// Provision creates the Firebase Auth identity for opts.Email when it is absent.
//
// Idempotent behavior:
//   - absent        → create the user, status "created"
//   - present       → do not touch it, status "already_exists"
//   - present+disabled → do not enable it, status "exists_disabled"
//
// It never creates a Labuda DB user, never changes roles/capabilities, and never
// issues a Labuda session token. All Firebase errors are propagated to the caller.
func Provision(ctx context.Context, fb firebaseAuth, opts Options) (*Result, error) {
	if err := guardDevelopment(opts.Env); err != nil {
		return nil, err
	}
	if err := validateInputs(opts); err != nil {
		return nil, err
	}

	email := strings.TrimSpace(opts.Email)

	existing, found, err := fb.FindByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("firebase lookup failed: %w", err)
	}
	if found {
		if existing.Disabled {
			// Never silently re-enable a disabled account: that is an
			// administrative decision, not a provisioning side effect.
			return &Result{Status: StatusExistsDisabled, UID: existing.UID, Email: email}, nil
		}
		return &Result{Status: StatusAlreadyExists, UID: existing.UID, Email: email}, nil
	}

	// Development provisioning always marks the address verified: the canonical
	// exchange binds a Firebase identity to an existing Labuda account only when
	// the email is verified, and a development fixture address (admin@test.local)
	// has no mailbox to click a verification link in.
	created, err := fb.Create(ctx, email, opts.Password, true)
	if err != nil {
		return nil, fmt.Errorf("firebase create failed: %w", err)
	}
	return &Result{Status: StatusCreated, UID: created.UID, Email: email}, nil
}
