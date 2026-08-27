/// Canonical Indonesian phone authority — the single mobile source of truth
/// for phone number FORMAT validity.
///
/// Owner/ChatGPT-locked rule (Stage 4B-1):
/// - Accepted prefixes: `+62...`, `62...`, `0...` (Indonesian national /
///   international shapes).
/// - Canonical digit rule: 9–12 digits AFTER the applicable prefix.
/// - Validation is FORMAT ONLY.
///
/// This class deliberately does NOT:
/// - normalize to E.164 (that is `PhoneVerificationService.formatPhoneNumber`)
/// - perform Firebase verification / OTP sending
/// - make network calls
///
/// Prefix accounting (canonical, explicit):
///   `+62`  -> prefix is `+62`  (3 chars) -> 9-12 digits follow
///   `62`   -> prefix is `62`   (2 chars) -> 9-12 digits follow
///   `0`    -> prefix is `0`    (1 char)  -> 9-12 digits follow
///
/// Whitespace and hyphens between digits are tolerated (stripped before
/// matching) so formatted input like `0812-3456-7890` is accepted; leading/
/// trailing whitespace is trimmed.
///
/// Do NOT add UI-specific rules here. Screen-level code may wrap the result
/// with contextual error messages, but the yes/no validity answer comes from
/// here.
library;

class CanonicalPhoneValidator {
  CanonicalPhoneValidator._();

  /// Minimum digits allowed after the prefix.
  static const int minDigitsAfterPrefix = 9;

  /// Maximum digits allowed after the prefix.
  static const int maxDigitsAfterPrefix = 12;

  static final RegExp _pattern = RegExp(
    r'^(?:\+62|62|0)([0-9]{9,12})$',
  );

  /// Returns `true` when [value] is a canonically valid Indonesian phone
  /// number (format only — no normalization, no verification).
  ///
  /// Returns `false` for null, empty, or whitespace-only input, and for
  /// strings that do not match the canonical prefix + digit rule.
  static bool isValid(String? value) {
    if (value == null) return false;
    final trimmed = value.trim();
    if (trimmed.isEmpty) return false;
    // Tolerate spaces/hyphens between digits (e.g. 0812-3456-7890).
    final cleaned = trimmed.replaceAll(RegExp(r'[\s-]'), '');
    return _pattern.hasMatch(cleaned);
  }

  /// First canonical violation as a user-facing message, or `null` when the
  /// phone number is valid. Screen-level validators can use this to surface a
  /// specific reason instead of a generic message.
  static String? validationMessage(String? value) {
    if (value == null || value.trim().isEmpty) {
      return 'Phone number is required';
    }
    if (!isValid(value)) {
      return 'Invalid phone number format';
    }
    return null;
  }
}
