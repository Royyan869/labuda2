/// Wilayah Service - Local JSON Only
///
/// Service untuk load data wilayah dari local JSON assets.
/// Offline-first access tanpa Firebase dependency.
///
/// MIGRATION STATUS: Fully migrated to local JSON
/// - Firebase queries removed
/// - Only local JSON access
library;

import 'dart:convert';
import 'package:flutter/services.dart';
import 'package:labuda/shared/models/wilayah_models.dart';
import 'package:labuda/shared/services/logger_service.dart';

class FirebaseWilayahService {
  // Firebase migration: Now using local JSON only
  static const bool USE_FIREBASE = false;

  // Local JSON assets paths
  static const String _provincesAsset = 'assets/data/full/provinces.json';
  static const String _citiesAsset = 'assets/data/full/cities.json';
  static const String _districtsAsset = 'assets/data/full/districts.json';
  static const String _villagesAsset = 'assets/data/full/villages.json';

  /// Load semua provinsi dari local JSON
  static Future<List<Province>> getProvinces() async {
    return _getProvincesFromLocal();
  }

  /// Get provinces from JSON
  static Future<List<Province>> _getProvincesFromLocal() async {
    try {
      LoggerService.instance.logInfo('📁 Loading provinces from local JSON');
      final String jsonString = await rootBundle.loadString(_provincesAsset);
      final List<dynamic> jsonList = json.decode(jsonString);

      final provinces = jsonList
          .map((json) => Province.fromJson(json as Map<String, dynamic>))
          .toList();

      LoggerService.instance.logInfo(
        '✅ Loaded ${provinces.length} provinces from local',
      );
      provinces.sort((a, b) => a.name.compareTo(b.name));
      return provinces;
    } catch (e) {
      LoggerService.instance.logError('Failed to load provinces from local', e);
      throw Exception('Failed to load provinces from local: $e');
    }
  }

  /// Load kota berdasarkan provinsi ID
  static Future<List<City>> getCitiesByProvince(String provinceId) async {
    return _getCitiesFromLocal(provinceId);
  }

  /// Get cities from JSON
  static Future<List<City>> _getCitiesFromLocal(String provinceId) async {
    try {
      LoggerService.instance.logInfo(
        '📁 Loading cities from local JSON for province: $provinceId',
      );
      final String jsonString = await rootBundle.loadString(_citiesAsset);
      final List<dynamic> jsonList = json.decode(jsonString);

      final cities = jsonList
          .where((json) => json['province_id'] == provinceId)
          .map(
            (json) => City.fromJson({
              'id': json['id'],
              'name': json['name'],
              'provinceId': json['province_id'],
            }),
          )
          .toList();

      LoggerService.instance.logInfo(
        '✅ Loaded ${cities.length} cities from local',
      );
      cities.sort((a, b) => a.name.compareTo(b.name));
      return cities;
    } catch (e) {
      LoggerService.instance.logError(
        'Failed to load cities from local for province $provinceId',
        e,
      );
      return [];
    }
  }

  /// Load kecamatan berdasarkan kota ID
  static Future<List<District>> getDistrictsByCity(String cityId) async {
    return _getDistrictsFromLocal(cityId);
  }

  /// Get districts from JSON
  static Future<List<District>> _getDistrictsFromLocal(String cityId) async {
    try {
      LoggerService.instance.logInfo(
        '📁 Loading districts from local JSON for city: $cityId',
      );
      final String jsonString = await rootBundle.loadString(_districtsAsset);
      final List<dynamic> jsonList = json.decode(jsonString);

      final districts = jsonList
          .where((json) => json['city_id'] == cityId)
          .map(
            (json) => District.fromJson({
              'id': json['id'],
              'name': json['name'],
              'cityId': json['city_id'],
            }),
          )
          .toList();

      LoggerService.instance.logInfo(
        '✅ Loaded ${districts.length} districts from local',
      );
      districts.sort((a, b) => a.name.compareTo(b.name));
      return districts;
    } catch (e) {
      LoggerService.instance.logError(
        'Failed to load districts from local for city $cityId',
        e,
      );
      return [];
    }
  }

  /// Load desa berdasarkan kecamatan ID
  static Future<List<Village>> getVillagesByDistrict(String districtId) async {
    return _getVillagesFromLocal(districtId);
  }

  /// Get villages from JSON
  static Future<List<Village>> _getVillagesFromLocal(String districtId) async {
    try {
      LoggerService.instance.logInfo(
        '📁 Loading villages from local JSON for district: $districtId',
      );
      final String jsonString = await rootBundle.loadString(_villagesAsset);
      final List<dynamic> jsonList = json.decode(jsonString);

      final villages = jsonList
          .where((json) => json['district_id'] == districtId)
          .map(
            (json) => Village.fromJson({
              'id': json['id'],
              'name': json['name'],
              'districtId': json['district_id'],
            }),
          )
          .toList();

      LoggerService.instance.logInfo(
        '✅ Loaded ${villages.length} villages from local',
      );
      villages.sort((a, b) => a.name.compareTo(b.name));
      return villages;
    } catch (e) {
      LoggerService.instance.logError(
        'Failed to load villages from local for district $districtId',
        e,
      );
      return [];
    }
  }

  /// Search villages berdasarkan query dan district
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

  /// Get provinsi by ID
  static Future<Province?> getProvinceById(String id) async {
    try {
      final provinces = await getProvinces();
      return provinces.where((p) => p.id == id).firstOrNull;
    } catch (e) {
      return null;
    }
  }

  /// Test connection (always returns true for local JSON)
  static Future<bool> testConnection() async {
    return true;
  }
}
