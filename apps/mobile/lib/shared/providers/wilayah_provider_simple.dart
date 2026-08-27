/// Simple Wilayah Provider
///
/// Simplified version menggunakan FutureProvider saja
/// Untuk state management yang simple tanpa StateNotifier complexity
///
/// UPDATED: Using LocalWilayahService for offline-first approach
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/shared/models/wilayah_models.dart';
import 'package:labuda/shared/services/local_wilayah_service.dart';

/// Provider untuk list semua provinsi
final provincesProvider = FutureProvider<List<Province>>((ref) async {
  return await LocalWilayahService.getProvinces();
});

/// Provider untuk list kota berdasarkan provinsi yang dipilih
final citiesProvider = FutureProvider.family<List<City>, String?>((
  ref,
  provinceId,
) async {
  if (provinceId == null || provinceId.isEmpty) {
    return [];
  }
  return await LocalWilayahService.getCitiesByProvince(provinceId);
});

/// Provider untuk list kecamatan berdasarkan kota yang dipilih
final districtsProvider = FutureProvider.family<List<District>, String?>((
  ref,
  cityId,
) async {
  if (cityId == null || cityId.isEmpty) {
    return [];
  }
  return await LocalWilayahService.getDistrictsByCity(cityId);
});

/// Provider untuk list desa berdasarkan kecamatan yang dipilih
final villagesProvider = FutureProvider.family<List<Village>, String?>((
  ref,
  districtId,
) async {
  if (districtId == null || districtId.isEmpty) {
    return [];
  }
  return await LocalWilayahService.getVillagesByDistrict(districtId);
});

/// Provider untuk search desa berdasarkan query
final villageSearchProvider =
    FutureProvider.family<List<Village>, Map<String, String>>((
      ref,
      params,
    ) async {
      final districtId = params['districtId'];
      final query = params['query'] ?? '';

      if (districtId == null || districtId.isEmpty) {
        return [];
      }

      return await LocalWilayahService.searchVillages(districtId, query);
    });

/// Utility provider untuk get provinsi by ID
final provinceByIdProvider = FutureProvider.family<Province?, String>((
  ref,
  id,
) async {
  return await LocalWilayahService.getProvinceById(id);
});

/// Utility provider untuk get kota by ID
final cityByIdProvider = FutureProvider.family<City?, String>((ref, id) async {
  return await LocalWilayahService.getCityById(id);
});

/// Utility provider untuk get kecamatan by ID
final districtByIdProvider = FutureProvider.family<District?, String>((
  ref,
  id,
) async {
  return await LocalWilayahService.getDistrictById(id);
});

/// Utility provider untuk get desa by ID
final villageByIdProvider = FutureProvider.family<Village?, String>((
  ref,
  id,
) async {
  return await LocalWilayahService.getVillageById(id);
});
