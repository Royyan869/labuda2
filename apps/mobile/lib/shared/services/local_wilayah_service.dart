/// Local Wilayah Service
///
/// Service untuk load data wilayah dari local JSON assets
/// Menggantikan FirebaseWilayahServiceV2 untuk offline-first approach
library;

import 'dart:convert';
import 'package:flutter/services.dart';
import 'package:labuda/shared/models/wilayah_models.dart';

class LocalWilayahService {
  // Cache untuk data yang sudah di-load
  static List<Province>? _provincesCache;
  static final Map<String, List<City>> _citiesCache = {};
  static final Map<String, List<District>> _districtsCache = {};
  static final Map<String, List<Village>> _villagesCache = {};

  /// Load semua provinsi dari local JSON
  static Future<List<Province>> getProvinces() async {
    if (_provincesCache != null) {
      return _provincesCache!;
    }

    try {
      final jsonString = await rootBundle.loadString(
        'assets/data/provinces.json',
      );
      final List<dynamic> jsonList = json.decode(jsonString);

      _provincesCache = jsonList
          .map((json) => Province.fromJson(json as Map<String, dynamic>))
          .toList();

      return _provincesCache!;
    } catch (e) {
      throw Exception('Failed to load provinces: $e');
    }
  }

  /// Load cities berdasarkan provinsi
  static Future<List<City>> getCitiesByProvince(String provinceId) async {
    if (_citiesCache.containsKey(provinceId)) {
      return _citiesCache[provinceId]!;
    }

    try {
      final jsonString = await rootBundle.loadString(
        'assets/data/cities/$provinceId.json',
      );
      final List<dynamic> jsonList = json.decode(jsonString);

      final cities = jsonList
          .map((json) => City.fromJson(json as Map<String, dynamic>))
          .toList();

      _citiesCache[provinceId] = cities;
      return cities;
    } catch (e) {
      throw Exception('Failed to load cities for province $provinceId: $e');
    }
  }

  /// Load districts berdasarkan provinsi (akan di-filter by city di provider)
  static Future<List<District>> getDistrictsByCity(String cityId) async {
    // Extract province ID from city ID (e.g., "3201" -> "32")
    final provinceId = cityId.substring(0, 2);

    if (!_districtsCache.containsKey(provinceId)) {
      try {
        final jsonString = await rootBundle.loadString(
          'assets/data/districts/$provinceId.json',
        );
        final List<dynamic> jsonList = json.decode(jsonString);

        final districts = jsonList
            .map((json) => District.fromJson(json as Map<String, dynamic>))
            .toList();

        _districtsCache[provinceId] = districts;
      } catch (e) {
        throw Exception(
          'Failed to load districts for province $provinceId: $e',
        );
      }
    }

    // Filter districts by cityId
    return _districtsCache[provinceId]!
        .where((district) => district.cityId == cityId)
        .toList();
  }

  /// Load villages berdasarkan provinsi (akan di-filter by district di provider)
  static Future<List<Village>> getVillagesByDistrict(String districtId) async {
    // Extract province ID from district ID (e.g., "320101" -> "32")
    final provinceId = districtId.substring(0, 2);

    if (!_villagesCache.containsKey(provinceId)) {
      try {
        final jsonString = await rootBundle.loadString(
          'assets/data/villages/$provinceId.json',
        );
        final List<dynamic> jsonList = json.decode(jsonString);

        final villages = jsonList
            .map((json) => Village.fromJson(json as Map<String, dynamic>))
            .toList();

        _villagesCache[provinceId] = villages;
      } catch (e) {
        throw Exception('Failed to load villages for province $provinceId: $e');
      }
    }

    // Filter villages by districtId
    return _villagesCache[provinceId]!
        .where((village) => village.districtId == districtId)
        .toList();
  }

  /// Search villages berdasarkan query
  static Future<List<Village>> searchVillages(
    String districtId,
    String query,
  ) async {
    try {
      final villages = await getVillagesByDistrict(districtId);

      if (query.isEmpty) return villages;

      return villages
          .where(
            (village) =>
                village.name.toLowerCase().contains(query.toLowerCase()),
          )
          .toList();
    } catch (e) {
      throw Exception('Failed to search villages: $e');
    }
  }

  /// Get province by ID
  static Future<Province?> getProvinceById(String id) async {
    try {
      final provinces = await getProvinces();
      return provinces.where((p) => p.id == id).firstOrNull;
    } catch (e) {
      return null;
    }
  }

  /// Get city by ID
  static Future<City?> getCityById(String id) async {
    try {
      // Extract province ID from city ID
      final provinceId = id.substring(0, 2);
      final cities = await getCitiesByProvince(provinceId);
      return cities.where((c) => c.id == id).firstOrNull;
    } catch (e) {
      return null;
    }
  }

  /// Get district by ID
  static Future<District?> getDistrictById(String id) async {
    try {
      // Extract city ID from district ID (e.g., "320101" -> "3201")
      final cityId = id.substring(0, 4);

      final districts = await getDistrictsByCity(cityId);
      return districts.where((d) => d.id == id).firstOrNull;
    } catch (e) {
      return null;
    }
  }

  /// Get village by ID
  static Future<Village?> getVillageById(String id) async {
    try {
      // Extract district ID from village ID
      final districtId = id.substring(0, 6);

      final villages = await getVillagesByDistrict(districtId);
      return villages.where((v) => v.id == id).firstOrNull;
    } catch (e) {
      return null;
    }
  }

  /// Clear all caches (untuk development/testing)
  static void clearCache() {
    _provincesCache = null;
    _citiesCache.clear();
    _districtsCache.clear();
    _villagesCache.clear();
  }
}
