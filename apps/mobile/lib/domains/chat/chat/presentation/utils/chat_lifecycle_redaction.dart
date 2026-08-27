/// Chat-side governance lifecycle redaction helper.
///
/// Thin delegate over the canonical [ContentLifecycleParse.publicRedactionLabel]
/// getter. Empty-string-on-active is the chat-domain contract so callers can
/// write:
///
///     final label = lifecycle.isDegraded
///         ? chatLifecycleRedactionLabel(lifecycle)
///         : actualName;
///
/// without nullability juggling.
///
/// Doctrine reminder: slot-persistence is preserved at the rendering
/// layer too — chat rooms and messages remain visible for degraded
/// identities; only the participant/sender display switches to the
/// redaction placeholder + neutral avatar.
library;

import 'package:labuda/shared/governance/content_lifecycle.dart';

/// Returns the user-facing redaction label for a degraded chat identity.
/// Active → empty string (caller falls back to the real name). Never throws.
String chatLifecycleRedactionLabel(ContentLifecycle lifecycle) =>
    lifecycle.publicRedactionLabel;
