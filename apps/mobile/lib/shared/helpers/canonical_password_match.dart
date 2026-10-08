/// THE canonical confirm-password rule.
///
/// Owner decision 2026-10-05 (Input/Form convergence): confirmation must match
/// the password. Three independent implementations used to exist (sign-up
/// submit gate, change-password validator, confirm-field indicator); the rule
/// now lives here once so the indicator and every submit gate can never
/// disagree.
///
/// Matching is exact equality of TRIMMED values:
/// - empty confirmation (or empty password)         → not a match
/// - both non-empty and equal after trim            → match
/// - both non-empty and different after trim        → not a match
///
/// Business validity of the password itself is [CanonicalPasswordPolicy];
/// this class only owns the match.
library;

class CanonicalPasswordMatch {
  CanonicalPasswordMatch._();

  /// True when [confirm] is non-empty and equals [password] after trimming.
  /// Callers that want the neutral "nothing typed yet" state must check for an
  /// empty confirmation themselves (see [validationMessage]).
  static bool matches(String? confirm, String? password) {
    final c = (confirm ?? '').trim();
    return c.isNotEmpty && c == (password ?? '').trim();
  }

  /// Field-level validation message for a confirm-password field, or null.
  static String? validationMessage(String? confirm, String? password) {
    final c = (confirm ?? '').trim();
    if (c.isEmpty) return 'Please confirm your password';
    if (c != (password ?? '').trim()) return 'Passwords do not match';
    return null;
  }
}
