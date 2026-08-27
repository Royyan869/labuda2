/// Helper untuk generate user initials secara konsisten
///
/// Centralized utility untuk menghindari inconsistency di berbagai tempat.
/// Semua avatar display harus menggunakan helper ini untuk generate initials.
class UserInitialsHelper {
  /// Generate initials dari full name user
  ///
  /// Rules:
  /// - Jika name ada 2+ kata: Ambil huruf pertama dari kata pertama + kata terakhir
  /// - Jika name 1 kata dengan 2+ karakter: Ambil 2 huruf pertama
  /// - Jika name 1 karakter: Gunakan karakter tersebut
  /// - Jika name kosong: Return 'U' (Unknown/User)
  ///
  /// Contoh:
  /// - "John Doe" → "JD"
  /// - "Muhammad Ali Akbar" → "MA"
  /// - "Alice" → "AL"
  /// - "X" → "X"
  /// - "" → "U"
  static String fromName(String? name) {
    if (name == null || name.trim().isEmpty) {
      return 'U';
    }

    final trimmedName = name.trim();
    final words = trimmedName.split(RegExp(r'\s+'));

    if (words.length >= 2) {
      // Multiple words: First char of first word + first char of last word
      final firstInitial = words.first.isNotEmpty ? words.first[0] : '';
      final lastInitial = words.last.isNotEmpty ? words.last[0] : '';
      return '$firstInitial$lastInitial'.toUpperCase();
    } else if (trimmedName.length >= 2) {
      // Single word with 2+ chars: First 2 chars
      return trimmedName.substring(0, 2).toUpperCase();
    } else if (trimmedName.length == 1) {
      // Single char: Use that char
      return trimmedName.toUpperCase();
    } else {
      return 'U';
    }
  }

  /// Generate initials dari userId (fallback)
  ///
  /// Digunakan ketika user belum set nama atau data user tidak tersedia.
  /// Rules:
  /// - Jika userId ada 2+ karakter: Ambil 2 karakter pertama
  /// - Jika userId 1 karakter: Gunakan karakter tersebut
  /// - Jika userId kosong: Return 'U'
  ///
  /// Contoh:
  /// - "abc123" → "AB"
  /// - "xyz" → "XY"
  /// - "A" → "A"
  /// - "" → "U"
  static String fromUserId(String? userId) {
    if (userId == null || userId.isEmpty) {
      return 'U';
    }

    if (userId.length >= 2) {
      return userId.substring(0, 2).toUpperCase();
    } else {
      return userId.toUpperCase();
    }
  }

  /// Get initials with priority: name > userId > fallback
  ///
  /// Smart helper yang mencoba name dulu, fallback ke userId jika name kosong.
  ///
  /// Contoh usage:
  /// ```dart
  /// final initials = UserInitialsHelper.get(
  ///   name: user.fullName,
  ///   userId: user.id,
  /// );
  /// ```
  static String get({String? name, String? userId}) {
    if (name != null && name.trim().isNotEmpty) {
      return fromName(name);
    } else if (userId != null && userId.isNotEmpty) {
      return fromUserId(userId);
    } else {
      return 'U';
    }
  }
}
