/// Canonical user ID validator.
///
/// The mobile app treats canonical user IDs as UUID-shaped strings.
library;

class CanonicalUserIdValidator {
  CanonicalUserIdValidator._();

  static bool isValid(String? value) {
    final trimmed = value?.trim();
    if (trimmed == null || trimmed.isEmpty) return false;
    if (trimmed.length != 36) return false;
    return RegExp(
      r'^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$',
    ).hasMatch(trimmed);
  }

  static String? normalize(String? value) {
    final trimmed = value?.trim();
    if (trimmed == null || trimmed.isEmpty) return null;
    return isValid(trimmed) ? trimmed : null;
  }
}
