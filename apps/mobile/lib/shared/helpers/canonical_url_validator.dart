/// Canonical URL authority — the single mobile source of truth for URL FORMAT
/// validity.
///
/// Stage 4B-1: preserves the factual business behavior already used by the
/// existing real consumer (`ValidationService.validateUrl`, consumed by
/// `UpdateProfileUseCase` for cover-photo and farm-website URLs):
/// - `https://` URLs are accepted.
/// - `http://localhost` development URLs are accepted (existing behavior).
/// - `http://` (non-localhost) is rejected (must be HTTPS).
/// - No network/domain validation is performed.
///
/// This class answers the FORMAT/POLICY VALIDITY question only.
///
/// Do NOT add UI-specific rules here. Screen-level code may wrap the result
/// with contextual error messages, but the yes/no validity answer comes from
/// here.
library;

class CanonicalUrlValidator {
  CanonicalUrlValidator._();

  static final RegExp _urlPattern = RegExp(
    r'^https?:\/\/[^\s]+$',
    caseSensitive: false,
  );

  /// Returns `true` when [value] is a canonically valid URL:
  /// non-empty, `http(s)://` shape, and (for plain http) localhost only.
  static bool isValid(String? value) {
    if (value == null) return false;
    final trimmed = value.trim();
    if (trimmed.isEmpty) return false;
    if (!_urlPattern.hasMatch(trimmed)) return false;
    // Plain http is only acceptable for localhost development URLs.
    if (!trimmed.toLowerCase().startsWith('https://') &&
        !trimmed.toLowerCase().startsWith('http://localhost')) {
      return false;
    }
    return true;
  }

  /// First canonical violation as a user-facing message, or `null` when the
  /// URL is valid. Screen-level validators can use this to surface a specific
  /// reason instead of a generic message.
  static String? validationMessage(String? value) {
    if (value == null || value.trim().isEmpty) {
      return 'URL is required';
    }
    if (!isValid(value)) {
      return 'Invalid URL format';
    }
    return null;
  }
}
