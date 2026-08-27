import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

/// Service untuk postal code lookup
/// Auto-fill wilayah berdasarkan kode pos
class PostalCodeService {
  static const String _postalCodesAsset = 'assets/data/postal_codes.json';
  static List<Map<String, dynamic>>? _postalCodes;

  /// Initialize postal codes data
  static Future<void> init() async {
    if (_postalCodes != null) return;

    try {
      final String jsonString = await rootBundle.loadString(_postalCodesAsset);
      final List<dynamic> jsonList = json.decode(jsonString);
      _postalCodes = jsonList.cast<Map<String, dynamic>>();
    } catch (e) {
      debugPrint('PostalCodeService: Error loading postal codes - $e');
      _postalCodes = [];
    }
  }

  /// Lookup wilayah by postal code
  /// Returns: {provinceId, cityId, districtId, area} or null if not found
  static Future<PostalCodeData?> lookup(String postalCode) async {
    // Ensure data is loaded
    await init();

    // Clean postal code (remove spaces, trim)
    final cleanCode = postalCode.replaceAll(' ', '').trim();

    // Must be 5 digits
    if (cleanCode.length != 5) return null;

    try {
      // Find matching postal code
      final match = _postalCodes?.firstWhere(
        (item) => item['postalCode'] == cleanCode,
        orElse: () => {},
      );

      if (match == null || match.isEmpty) return null;

      return PostalCodeData(
        postalCode: cleanCode,
        provinceId: match['provinceId'],
        cityId: match['cityId'],
        districtId: match['districtId'],
        villageId: match['villageId'],
        area: match['area'],
      );
    } catch (e) {
      debugPrint('PostalCodeService: Error in lookup for $cleanCode - $e');
      return null;
    }
  }

  /// Get suggestions for partial postal code
  static Future<List<PostalCodeData>> getSuggestions(String partial) async {
    await init();

    if (partial.length < 2) return [];

    final cleanPartial = partial.replaceAll(' ', '').trim();

    try {
      final suggestions =
          _postalCodes
              ?.where(
                (item) =>
                    item['postalCode'].toString().startsWith(cleanPartial),
              )
              .take(5)
              .map(
                (item) => PostalCodeData(
                  postalCode: item['postalCode'],
                  provinceId: item['provinceId'],
                  cityId: item['cityId'],
                  districtId: item['districtId'],
                  villageId: item['villageId'],
                  area: item['area'],
                ),
              )
              .toList() ??
          [];

      return suggestions;
    } catch (e) {
      debugPrint(
        'PostalCodeService: Error getting suggestions for $cleanPartial - $e',
      );
      return [];
    }
  }

  /// Reverse lookup: Find postal code by wilayah IDs (village-level)
  /// Returns postal code string or null if not found
  static Future<String?> getPostalCodeByWilayah({
    required String provinceId,
    required String cityId,
    required String districtId,
    String? villageId,
  }) async {
    await init();

    try {
      // If villageId provided, find exact match
      if (villageId != null && villageId.isNotEmpty) {
        final match = _postalCodes?.firstWhere(
          (item) => item['villageId'] == villageId,
          orElse: () => {},
        );

        if (match != null && match.isNotEmpty) {
          return match['postalCode'] as String?;
        }
      }

      // Fallback: Find first matching district (if village not found or not provided)
      final match = _postalCodes?.firstWhere(
        (item) =>
            item['provinceId'] == provinceId &&
            item['cityId'] == cityId &&
            item['districtId'] == districtId,
        orElse: () => {},
      );

      if (match == null || match.isEmpty) return null;

      return match['postalCode'] as String?;
    } catch (e) {
      debugPrint(
        'PostalCodeService: Error finding postal code for wilayah - $e',
      );
      return null;
    }
  }
}

/// Data class for postal code lookup result
class PostalCodeData {
  final String postalCode;
  final String provinceId;
  final String cityId;
  final String districtId;
  final String? villageId;
  final String area;

  PostalCodeData({
    required this.postalCode,
    required this.provinceId,
    required this.cityId,
    required this.districtId,
    this.villageId,
    required this.area,
  });

  @override
  String toString() => '$postalCode - $area';
}
