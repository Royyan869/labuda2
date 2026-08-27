# Authentication Flow

## Overview

This module controls the mobile app flow from app launch to an authenticated
session. It is the current source of truth for sign-in, sign-up, profile
completion, and seller upgrade entry points.

## Canonical Flow

1. App starts and resolves any stored session.
2. If the user is unauthenticated, only public screens are available.
3. If the user is authenticated but profile completion is required, route to
   the profile-completion screen.
4. If the user is authenticated and complete, allow normal app navigation.
5. Seller-upgrade paths should reuse the same auth state instead of creating a
   separate identity flow.
6. Logout clears the session and returns the app to the unauthenticated state.

## Guard Rules

- Protected routes must fail closed when auth state is missing.
- Profile completion is part of auth, not a separate onboarding domain.
- Role and session checks should come from the controller state, not duplicate
  local flags.

## Notes

- Keep this document aligned with the current controller and route-guard code.
- If the flow changes in code, update this document at the same time.
