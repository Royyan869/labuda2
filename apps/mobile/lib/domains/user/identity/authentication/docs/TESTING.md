# Authentication Module Testing

## Overview

Keep auth tests focused on the current state machine and route-guard behavior.

## Current Checks

- `flutter test`
- `flutter analyze`
- targeted widget and provider tests under `apps/mobile/test`

## What to Cover

- session restoration on app launch
- unauthenticated access failing closed
- profile completion routing after auth
- sign-in success and failure paths
- Google sign-in and logout state transitions
- seller-upgrade entry points that reuse the same auth state

## Guidance

- Prefer behavior tests over architecture snapshots.
- Keep the assertions aligned with current controller and route-guard code.
- Remove old onboarding terminology from new tests.
