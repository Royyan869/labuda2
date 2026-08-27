/// Canonical user identity formatter.
///
/// Centralized utility for consistent username presentation and avatar
/// fallback initials across all Labuda surfaces.
///
/// RULES:
/// - Initials derive ONLY from sanitized username.
/// - Never accept userId, email, phone, or farm name as identity input.
/// - Null return means "use generic icon" — no magic sentinel strings.
/// - This formatter replaces ad-hoc @-stripping and initial extraction
///   scattered across inline avatar consumers.
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

  /// Returns 1–2 uppercase letter initials from a sanitized username,
  /// or null when safe initials cannot be produced.
  ///
  /// Algorithm:
  /// 1. normalise the username (strip @, trim)
  /// 2. split on underscores and other non-alphanumeric separators
  /// 3. extract letters only from each token
  /// 4. when ≥ 2 letter-bearing tokens → first letter of first + last token
  /// 5. when 1 letter-bearing token with ≥ 2 letters → first two letters
  /// 6. when 1 letter-bearing token with 1 letter → that single letter
  /// 7. numeric-only / separator-only / empty / null → null
  ///
  /// Examples:
  /// | Input        | Result |
  /// |-------------|--------|
  /// | john_doe    | JD     |
  /// | @john_doe   | JD     |
  /// | alice       | AL     |
  /// | a           | A      |
  /// | a1b2        | AB     |
  /// | 123john     | JO     |
  /// | 12345       | null   |
  /// | @           | null   |
  /// | _           | null   |
  /// | null / ""   | null   |
  static String? avatarInitials(String? raw) {
    final normalized = normalizeUsername(raw);
    if (normalized == null) return null;

    // Split on non-alphanumeric characters (not letters or digits).
    // This handles underscores, hyphens, dots, spaces etc.
    final tokens = normalized.split(RegExp(r'[^a-zA-Z0-9]+'));

    // Extract letter-only portions from each token.
    final letterTokens = <String>[];
    for (final token in tokens) {
      final letters = token.replaceAll(RegExp(r'[^a-zA-Z]'), '');
      if (letters.isNotEmpty) {
        letterTokens.add(letters);
      }
    }

    if (letterTokens.isEmpty) return null;

    if (letterTokens.length >= 2) {
      // First letter of first token + first letter of last token
      return '${letterTokens.first[0]}${letterTokens.last[0]}'.toUpperCase();
    }

    // Single letter-bearing token
    final only = letterTokens.first;
    if (only.length >= 2) {
      return only.substring(0, 2).toUpperCase();
    }
    // exactly 1 letter
    return only.toUpperCase();
  }
}
