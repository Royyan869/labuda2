// Profile-detail lifecycle redaction vocabulary.
//
// Thin delegates over the canonical [ContentLifecycleParse.publicRedactionLabel]
// getter — no local vocabulary lives here. Profile-detail policy differs from
// content-detail (which 404s on degraded). Per the governance constitution,
// profile-detail fail-OPENs on the target overlay: the screen renders the
// degraded identity card rather than a tombstone-404. These helpers shape
// what that degraded render looks like.

import 'package:labuda/shared/governance/content_lifecycle.dart';

/// Indonesian placeholder label for a degraded profile identity.
/// Returns null for active (caller should fall through to live data).
String? profileLifecycleRedactionLabel(ContentLifecycle lifecycle) {
  return lifecycle.isActive ? null : lifecycle.publicRedactionLabel;
}

/// True when the profile screen should suppress trust / verification badges,
/// presence indicators, bio, contact, social, and farm-info sections.
/// Removed users get the strictest suppression; unavailable users get the
/// same identity treatment but the surface MAY choose to keep skeleton
/// sections if they are otherwise safe (no contact / social leakage).
bool profileLifecycleSuppressesSensitiveSections(ContentLifecycle lifecycle) {
  return lifecycle.isDegraded;
}

/// True when target-user actions (follow, message, share, etc.) MUST be
/// disabled. Block / report actions remain available regardless — the
/// degraded card does not absolve abuse-investigation tooling.
bool profileLifecycleDisablesTargetActions(ContentLifecycle lifecycle) {
  return lifecycle.isDegraded;
}
