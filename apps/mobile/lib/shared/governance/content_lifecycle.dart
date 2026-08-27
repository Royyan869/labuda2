/// ContentLifecycle — canonical governance lifecycle vocabulary for the
/// mobile public-rendering layer.
///
/// PUBLIC LIFECYCLE IS 3-STATE — NOT 8.
///   - active       : fully visible, fully interactive
///   - unavailable  : degraded — greyed + CTAs disabled
///   - removed      : tombstone — minimal placeholder, drop where pagination permits
///
/// Mobile coarsens any wire vocabulary into this 3-state truth. The mapping
/// is DEFENSIVE — even if the backend already coarsens, mobile re-coarsens
/// so internal trust / seller states cannot leak through to the public UI.
///
/// Mapping rules (mobile):
///   active                                                    → active
///   deleted, banned, removed                                  → removed
///   unavailable, suspended, degraded, verification_pending,
///   subscription_expired, seller_revoked, limited, restricted → unavailable
///   null, empty, unknown                                      → unavailable  (FAIL CLOSED)
///
/// Lifecycle is SEPARATE from raw entity status; do not coerce one into the
/// other. Never send lifecycle back to the server — it is a governance-
/// derived projection.
library;

enum ContentLifecycle {
  /// Fully visible, fully interactive.
  active,

  /// Visible but degraded — moderation REDACT or business-rule unavailable.
  /// Discovery surfaces grey/mute; CTAs disable. This is also the
  /// FAIL-CLOSED default for null / empty / unknown wire values.
  unavailable,

  /// Governance TOMBSTONE — content is gone. Discovery surfaces should
  /// preferably drop from the list; fall back to a minimal placeholder when
  /// dropping would break pagination.
  removed,
}

extension ContentLifecycleParse on ContentLifecycle {
  /// Parse a wire string into a ContentLifecycle.
  ///
  /// Fails closed: null / empty / unknown → [ContentLifecycle.unavailable].
  /// Raw internal states (deleted/banned/suspended/etc.) are coarsened
  /// defensively so internal vocabulary cannot leak past this seam.
  /// Never throws.
  static ContentLifecycle fromWire(String? raw) {
    if (raw == null) return ContentLifecycle.unavailable;
    switch (raw.toLowerCase()) {
      case 'active':
        return ContentLifecycle.active;
      case 'removed':
      case 'deleted':
      case 'banned':
        return ContentLifecycle.removed;
      case 'unavailable':
      case 'suspended':
      case 'degraded':
      case 'verification_pending':
      case 'subscription_expired':
      case 'seller_revoked':
      case 'limited':
      case 'restricted':
        return ContentLifecycle.unavailable;
      default:
        // FAIL CLOSED — unknown vocabulary degrades the surface rather than
        // silently rendering it as fully active.
        return ContentLifecycle.unavailable;
    }
  }

  bool get isActive => this == ContentLifecycle.active;
  bool get isUnavailable => this == ContentLifecycle.unavailable;
  bool get isRemoved => this == ContentLifecycle.removed;

  /// True when the surface should render the card in a degraded state
  /// (greyed, CTAs disabled). Discovery surfaces typically combine this
  /// with [shouldDropFromList] to decide drop-vs-grey.
  bool get isDegraded => this != ContentLifecycle.active;

  /// True when discovery surfaces should preferably drop the item from the
  /// list rather than render any UI for it. Detail surfaces ignore this
  /// (they must always render a tombstone instead of dropping).
  bool get shouldDropFromList => this == ContentLifecycle.removed;

  /// Canonical user-facing redaction label for the public-rendering layer.
  ///
  ///   - [ContentLifecycle.removed]     → "Pengguna dihapus"
  ///   - [ContentLifecycle.unavailable] → "Pengguna tidak tersedia"
  ///   - [ContentLifecycle.active]      → empty string (caller renders real identity)
  ///
  /// Every public surface MUST route degraded identity rendering through
  /// this getter — no inline string switches, no per-domain helpers that
  /// invent their own vocabulary.
  String get publicRedactionLabel {
    switch (this) {
      case ContentLifecycle.removed:
        return 'Pengguna dihapus';
      case ContentLifecycle.unavailable:
        return 'Pengguna tidak tersedia';
      case ContentLifecycle.active:
        return '';
    }
  }
}
