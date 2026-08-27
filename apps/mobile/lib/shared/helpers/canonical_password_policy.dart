/// Canonical Labuda password policy — the single mobile source of truth for
/// password validity.
///
/// This is the APPLICATION password policy (owner/chatgpt-locked, Stage 2B):
///
///   MinLength:  8
///   Requires:   at least one uppercase letter [A-Z]
///               at least one lowercase letter [a-z]
///               at least one digit [0-9]
///
/// It is deliberately NOT the Firebase minimum (Firebase accepts 6+ chars).
/// Firebase's weaker floor is NOT our business/security policy; the app must
/// prevent submission of a password that fails this policy BEFORE calling
/// Firebase.
///
/// This class answers the POLICY VALIDITY question — "does this password
/// satisfy Labuda's minimum policy?" — only.  Strength tiers, entropy
/// scoring, zxcvbn-style engines, special-character requirements, maximum
/// length, and breached-password checks are intentionally NOT part of this
/// authority.
///
/// Do NOT add UI-specific rules here.  Screen-level code may wrap the result
/// with contextual error messages, but the yes/no validity answer comes from
/// here.
library;

class CanonicalPasswordPolicy {
  CanonicalPasswordPolicy._();

  static const int minLength = 8;

  static final RegExp _hasUppercase = RegExp(r'[A-Z]');
  static final RegExp _hasLowercase = RegExp(r'[a-z]');
  static final RegExp _hasDigit = RegExp(r'[0-9]');

  /// Returns `true` when [password] satisfies every canonical Labuda rule:
  /// at least 8 characters, containing at least one uppercase letter, one
  /// lowercase letter, and one digit.
  static bool isValid(String? password) {
    if (password == null) return false;
    if (password.length < minLength) return false;
    if (!_hasUppercase.hasMatch(password)) return false;
    if (!_hasLowercase.hasMatch(password)) return false;
    if (!_hasDigit.hasMatch(password)) return false;
    return true;
  }

  /// First canonical violation as a user-facing message, or `null` when the
  /// password is valid.  Screen-level validators can use this to surface a
  /// specific reason instead of a generic message.
  static String? validationMessage(String? password) {
    if (password == null || password.isEmpty) {
      return 'Password cannot be empty';
    }
    if (password.length < minLength) {
      return 'Password must be at least $minLength characters';
    }
    if (!_hasUppercase.hasMatch(password)) {
      return 'Password must contain at least one uppercase letter';
    }
    if (!_hasLowercase.hasMatch(password)) {
      return 'Password must contain at least one lowercase letter';
    }
    if (!_hasDigit.hasMatch(password)) {
      return 'Password must contain at least one digit';
    }
    return null;
  }
}
