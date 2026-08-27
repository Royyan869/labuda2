/// Validators and utility functions for edit profile screen
class EditProfileValidators {
  /// Mask email for privacy: "user@example.com" -> "us***@example.com"
  static String maskEmail(String email) {
    final parts = email.split('@');
    if (parts.length != 2) return email;

    final username = parts[0];
    final domain = parts[1];

    if (username.length <= 2) {
      return '${username[0]}***@$domain';
    }

    final visiblePart = username.substring(0, 2);
    return '$visiblePart***@$domain';
  }

  /// Mask phone for privacy: "081234567890" -> "081***890"
  static String maskPhone(String phone) {
    if (phone.length <= 6) return phone;

    final start = phone.substring(0, 3);
    final end = phone.substring(phone.length - 3);
    return '$start***$end';
  }

  /// Validate display name
  static String? validateDisplayName(String? value) {
    if (value == null || value.trim().isEmpty) {
      return 'Display name is required';
    }
    return null;
  }

  /// Validate farm name (seller only)
  static String? validateFarmName(String? value) {
    if (value == null || value.trim().isEmpty) {
      return 'Farm name is required';
    }
    return null;
  }
}
