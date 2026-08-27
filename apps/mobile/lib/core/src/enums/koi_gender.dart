/// Koi Gender Enum
///
/// Represents the gender of a koi fish.
library;

enum KoiGender {
  /// Male koi
  male,

  /// Female koi
  female,

  /// Unknown/undetermined gender
  unknown;

  /// Get all gender values
  static List<KoiGender> get all => values;

  /// Get display name for UI
  String get displayName {
    switch (this) {
      case KoiGender.male:
        return 'Jantan';
      case KoiGender.female:
        return 'Betina';
      case KoiGender.unknown:
        return 'Tidak Diketahui';
    }
  }
}
