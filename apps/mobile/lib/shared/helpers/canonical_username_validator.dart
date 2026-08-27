/// Canonical username authority — the single mobile source of truth for
/// username normalization and validity.
///
/// Mirrors the backend [identityusername] rules exactly
/// (backend/internal/identity/username/validation.go):
///
///   Normalize:  strings.ToLower(strings.TrimSpace(value))
///   MinLength:  3
///   MaxLength:  30
///   Pattern:    ^[a-z0-9_]+$
///
/// Every persisted username satisfies these rules.  All mobile username
/// validation MUST delegate to this class for the validity decision.
///
/// Do NOT add mention-specific, registration-specific, or UI-specific
/// rules here.  Screen-level code may wrap the result with contextual
/// error messages, but the yes/no validity answer comes from here.
library;

class CanonicalUsernameValidator {
  CanonicalUsernameValidator._();

  static const int minLength = 3;
  static const int maxLength = 30;

  static final RegExp _pattern = RegExp(r'^[a-z0-9_]+$');

  // ---------------------------------------------------------------------------
  // Public API
  // ---------------------------------------------------------------------------

  /// Canonical normalization: strips leading `@`, trims whitespace, and
  /// lowercases.  Returns the raw canonical username (no `@` prefix), or
  /// `null` when the result is empty.
  ///
  /// This is the canonical identity normalisation — it differs from
  /// [UserIdentityFormatter.normalizeUsername] which preserves casing for
  /// display purposes.  Use THIS method when comparing usernames for
  /// identity or validating against backend rules.
  static String? normalize(String? raw) {
    if (raw == null) return null;
    var cleaned = raw.trim();
    if (cleaned.isEmpty) return null;
    while (cleaned.startsWith('@')) {
      cleaned = cleaned.substring(1);
    }
    cleaned = cleaned.trim().toLowerCase();
    return cleaned.isEmpty ? null : cleaned;
  }

  /// Returns `true` when [value] satisfies every backend username rule.
  ///
  /// Applies [normalize] first, so the input may be raw (including
  /// leading `@` and mixed case).  Returns `false` for null, empty,
  /// whitespace-only, under-length, over-length, or invalid characters.
  static bool isValid(String? value) {
    final canonical = normalize(value);
    if (canonical == null) return false;
    if (canonical.length < minLength || canonical.length > maxLength) {
      return false;
    }
    return _pattern.hasMatch(canonical);
  }

  /// Normalize + validate in one call.  Returns the canonical lowercase
  /// username when valid, or `null` when invalid.
  ///
  /// Use this when you need the normalized form for storage/comparison.
  static String? normalizeAndValidate(String? raw) {
    final canonical = normalize(raw);
    if (canonical == null) return null;
    if (canonical.length < minLength || canonical.length > maxLength) {
      return null;
    }
    if (!_pattern.hasMatch(canonical)) return null;
    return canonical;
  }
}
