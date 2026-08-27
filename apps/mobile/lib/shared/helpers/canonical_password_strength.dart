/// Canonical password strength evaluator — the single mobile source of truth
/// for password-strength UX classification.
///
/// THIS IS UX FEEDBACK ONLY.  Password *validity* is decided exclusively by
/// [CanonicalPasswordPolicy] (min 8 + uppercase + lowercase + digit).  A
/// password can be policy-valid but not Strong, and a password can receive a
/// strength classification while being policy-invalid.  The strength score
/// MUST never be used as a submission gate.
///
/// Deterministic model (owner-locked, Stage 2C):
///
///   Criteria (each satisfied = 1 point):
///     1. length >= 8
///     2. length >= 12
///     3. contains uppercase [A-Z]
///     4. contains lowercase [a-z]
///     5. contains digit [0-9]
///     6. contains special character (anything outside [A-Za-z0-9])
///
///   Score → classification:
///     0–2 → weak
///     3–4 → medium
///     5–6 → strong
///
/// Special characters contribute to the STRENGTH score but are NOT required
/// for password validity.  No entropy, dictionary, breached-password,
/// history, or maximum-length rules exist here.
library;

/// User-facing password strength classification.
enum PasswordStrengthLevel {
  weak,
  medium,
  strong;

  /// Canonical English label used by all strength UIs.
  String get label {
    switch (this) {
      case PasswordStrengthLevel.weak:
        return 'Weak';
      case PasswordStrengthLevel.medium:
        return 'Medium';
      case PasswordStrengthLevel.strong:
        return 'Strong';
    }
  }
}

class CanonicalPasswordStrength {
  CanonicalPasswordStrength._();

  static final RegExp _hasUppercase = RegExp(r'[A-Z]');
  static final RegExp _hasLowercase = RegExp(r'[a-z]');
  static final RegExp _hasDigit = RegExp(r'[0-9]');
  static final RegExp _hasSpecial = RegExp(r'[^A-Za-z0-9]');

  /// Evaluate the strength classification for [password].
  ///
  /// An empty password returns `null` — no strength classification is shown
  /// for empty input (neutral state, no misleading "Weak").
  static PasswordStrengthLevel? evaluate(String? password) {
    if (password == null || password.isEmpty) return null;
    return classify(score(password));
  }

  /// Deterministic 0–6 score for [password] (see class doc for criteria).
  static int score(String password) {
    var score = 0;
    if (password.length >= 8) score++;
    if (password.length >= 12) score++;
    if (_hasUppercase.hasMatch(password)) score++;
    if (_hasLowercase.hasMatch(password)) score++;
    if (_hasDigit.hasMatch(password)) score++;
    if (_hasSpecial.hasMatch(password)) score++;
    return score;
  }

  /// Map a 0–6 score to the canonical three-state classification.
  static PasswordStrengthLevel classify(int score) {
    if (score <= 2) return PasswordStrengthLevel.weak;
    if (score <= 4) return PasswordStrengthLevel.medium;
    return PasswordStrengthLevel.strong;
  }

  /// Canonical progress-bar fraction (0.0–1.0) for [password].
  ///
  /// Empty input maps to 0.0 (no visible strength).
  static double progress(String? password) {
    if (password == null || password.isEmpty) return 0;
    return score(password) / 6;
  }
}
