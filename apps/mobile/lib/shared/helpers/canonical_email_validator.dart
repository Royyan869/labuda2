/// Canonical email authority — the single mobile source of truth for email
/// format validity.
///
/// Owner/ChatGPT-locked (Stage 4B-1):
/// - Deterministic, pure, UI-independent format check only.
/// - Accepts normal valid user emails.
/// - Rejects clearly invalid input (empty, no @, no dot-TLD, spaces).
/// - Deliberately NOT a full RFC implementation; performs NO network/domain
///   verification.
///
/// This class answers the FORMAT VALIDITY question ("is this a well-formed
/// email address?") only. Availability, MX/DNS checks, and deliverability are
/// explicitly out of scope.
///
/// Do NOT add UI-specific rules here. Screen-level code may wrap the result
/// with contextual error messages, but the yes/no validity answer comes from
/// here.
library;

class CanonicalEmailValidator {
  CanonicalEmailValidator._();

  /// Matches the common `local@domain.tld` shape without attempting full RFC
  /// correctness: 1+ chars before @, a domain with at least one dot, and a
  /// 2+ letter TLD.
  static final RegExp _pattern = RegExp(
    r'^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$',
  );

  /// Returns `true` when [value] is a well-formed email address.
  ///
  /// Returns `false` for null, empty, or whitespace-only input, and for
  /// strings that do not match the canonical shape.
  static bool isValid(String? value) {
    if (value == null) return false;
    final trimmed = value.trim();
    if (trimmed.isEmpty) return false;
    return _pattern.hasMatch(trimmed);
  }

  /// First canonical violation as a user-facing message, or `null` when the
  /// email is valid. Screen-level validators can use this to surface a
  /// specific reason instead of a generic message.
  static String? validationMessage(String? value) {
    if (value == null || value.trim().isEmpty) {
      return 'Email is required';
    }
    if (!isValid(value)) {
      return 'Invalid email format';
    }
    return null;
  }
}
