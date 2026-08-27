# Authentication API Documentation

## Overview

This is a high-level reference for the current authentication surface. Treat
the controller, repository, and route-guard code as the source of truth.

## Core Surfaces

- sign in with email/password
- sign up with email/password
- sign in with Google
- observe auth state
- log out
- send and confirm email verification
- reset password
- complete profile after auth
- update user role for seller upgrade flows

## Contract Notes

- Auth state must be resolved before protected screens render.
- Profile completion is a post-auth state, not a separate onboarding domain.
- Route guards should fail closed when auth state is missing or unresolved.
- Do not duplicate auth authority in docs that conflict with the controller.

## Data and State

- Auth state, user profile state, and role state are separate concerns.
- Session restoration should be driven by the persisted auth state only.
- Keep user-facing errors explicit rather than treating failures as success.
