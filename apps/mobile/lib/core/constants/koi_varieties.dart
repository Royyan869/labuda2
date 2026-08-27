/// Koi Varieties Constants - Centralized Source of Truth
///
/// Single source of truth untuk semua koi varieties di aplikasi.
/// Digunakan oleh modul katalog dan discovery (listing/auction/promotion).
///
/// Data reference: VARIETY_KOI.md
class KoiVarieties {
  KoiVarieties._();

  /// Complete list of all koi varieties
  static const List<String> all = [
    // Gosanke (Big 3)
    'Kohaku',
    'Sanke',
    'Showa',

    // Hikarimono (Metallic)
    'Ogon',
    'Kujaku',
    'Hariwake',
    'Kikusui',
    'Yamato Nishiki',
    'Kin Showa',

    // Utsurimono & Bekko
    'Shiro Utsuri',
    'Hi/Ki Utsuri',
    'Bekko',

    // Koromo & Goshiki
    'Koromo',
    'Goshiki',

    // Asagi & Shusui
    'Asagi',
    'Shusui',

    // Tancho
    'Tancho',

    // Kawarimono (Non-metallic single colors & variations)
    'Chagoi',
    'Soragoi',
    'Karamel',
    'Soramel',
    'Ochiba',
    'Karashugoi',
    'Midorigoi',
    'Benigoi',
    'Aka Muji',
    'Shiro Muji',
    'Kagami-goi',
    'Karashi',

    // Kawarimono (Black-based)
    'Kikokuryu',
    'Kumonryu',
    'Hajiro',
    'Matsukawabake',
    'Matsuba',

    // Special types
    'Doitsu',
    'Gin Rin',
    'Butterfly',
    'Body-Long',

    // Others
    'Kijiro',
    'Heisei Nishiki',
    'Lainnya',
  ];

  /// Variety groups for organized selection
  /// Useful for filters, category pickers, etc.
  static const Map<String, List<String>> groups = {
    'Gosanke': ['Kohaku', 'Sanke', 'Showa'],
    'Hikarimono': [
      'Ogon',
      'Kujaku',
      'Hariwake',
      'Kikusui',
      'Yamato Nishiki',
      'Kin Showa',
    ],
    'Utsurimono & Bekko': ['Shiro Utsuri', 'Hi/Ki Utsuri', 'Bekko'],
    'Koromo & Goshiki': ['Koromo', 'Goshiki'],
    'Asagi & Shusui': ['Asagi', 'Shusui'],
    'Tancho': ['Tancho'],
    'Kawarimono': [
      'Chagoi',
      'Soragoi',
      'Karamel',
      'Soramel',
      'Ochiba',
      'Karashugoi',
      'Midorigoi',
      'Benigoi',
      'Aka Muji',
      'Shiro Muji',
      'Kagami-goi',
      'Karashi',
      'Kikokuryu',
      'Kumonryu',
      'Hajiro',
      'Matsukawabake',
      'Matsuba',
    ],
    'Special Types': ['Doitsu', 'Gin Rin', 'Butterfly', 'Body-Long'],
    'Others': ['Kijiro', 'Heisei Nishiki', 'Lainnya'],
  };

  /// Popular varieties for quick selection
  /// Based on common varieties in Indonesian koi market
  static const List<String> popular = [
    'Kohaku',
    'Sanke',
    'Showa',
    'Ogon',
    'Chagoi',
    'Soragoi',
    'Asagi',
    'Shusui',
  ];

  /// Preset categories for listing/auction filters
  /// Simplified grouping untuk category creation
  static const Map<String, List<String>> categoryPresets = {
    'Gosanke': ['Kohaku', 'Sanke', 'Showa'],
    'Hikarimono': [
      'Ogon',
      'Kujaku',
      'Hariwake',
      'Kikusui',
      'Yamato Nishiki',
      'Kin Showa',
    ],
    'Kawarimono': [
      'Chagoi',
      'Soragoi',
      'Karamel',
      'Soramel',
      'Ochiba',
      'Midorigoi',
      'Benigoi',
    ],
    'Utsurimono': ['Shiro Utsuri', 'Hi/Ki Utsuri', 'Bekko'],
    'Asagi & Shusui': ['Asagi', 'Shusui'],
  };

  /// Validate if a variety name is valid
  static bool isValid(String variety) {
    return all.contains(variety);
  }

  /// Get group name for a variety
  static String? getGroupName(String variety) {
    for (final entry in groups.entries) {
      if (entry.value.contains(variety)) {
        return entry.key;
      }
    }
    return null;
  }

  /// Search varieties by keyword
  static List<String> search(String keyword) {
    final lowerKeyword = keyword.toLowerCase();
    return all.where((v) => v.toLowerCase().contains(lowerKeyword)).toList();
  }
}
