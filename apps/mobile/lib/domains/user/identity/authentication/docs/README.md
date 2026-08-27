# Authentication Module

## Overview

This module owns the current mobile authentication surface: sign-in, sign-up,
session state, email verification, password reset, profile completion after
auth, and seller-upgrade entry points.

Current authority lives in the auth controller, auth state, repositories, and
the route guards that consume that state.

## Current Model

- Authentication state is the source of truth for protected navigation.
- Profile completion happens after auth and is not a separate domain.
- Role changes for seller onboarding flow through the auth module state.
- Session state should be resolved before rendering protected UI.

## Relevant Code Locations

- `lib/src/providers/auth_controller.dart`
- `lib/src/providers/auth_state.dart`
- `lib/src/data/`
- `lib/src/domain/`

## Notes

- Keep this documentation aligned with the controller and repository code.
- If a claim conflicts with code, update the doc or remove the claim.
