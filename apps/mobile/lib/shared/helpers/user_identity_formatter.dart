/// Canonical user identity formatter.
///
/// Canonical user identity formatter.
///
/// Business truth (Owner decision 2026-09-24): user avatars are ALWAYS the
/// user's photo or the user icon — initials do not exist in this app.
/// This formatter centralizes username presentation (handles) only.
library;

class UserIdentityFormatter {
  UserIdentityFormatter._(); // non-instantiable

  // --- public API ---

  /// Returns a clean username WITHOUT a leading '@', or null.
  ///
  /// - trims whitespace
  /// - strips all leading '@' characters
  /// - returns null when the result is empty
  static String? normalizeUsername(String? raw) {
    if (raw == null) return null;
    var cleaned = raw.trim();
    if (cleaned.isEmpty) return null;
    while (cleaned.startsWith('@')) {
      cleaned = cleaned.substring(1);
    }
    cleaned = cleaned.trim();
    return cleaned.isEmpty ? null : cleaned;
  }

  /// Returns a handle with EXACTLY one leading '@', or null.
  ///
  /// - null / empty / whitespace / only '@' → null
  /// - stale leading '@' is normalised
  /// - never returns bare '@' or '@@username'
  static String? formatHandle(String? raw) {
    final normalized = normalizeUsername(raw);
    if (normalized == null || normalized.isEmpty) return null;
    return '@$normalized';
  }

}
