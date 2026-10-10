/// Canonical Geography Providers
///
/// Province/city/district/village state, backed by the ONE canonical Geography
/// API. There is no local dataset and no offline fallback — a second geography
/// authority is forbidden.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/models/wilayah_models.dart';
import 'package:hishumi/shared/services/geography_api_service.dart';

/// The canonical Geography API service.
final geographyApiServiceProvider = Provider<GeographyApiService>((ref) {
  return GeographyApiService(
    ref.watch(apiClientProvider),
    logger: ref.watch(loggerServiceProvider),
  );
});

/// All provinces.
final provincesProvider = FutureProvider<List<Province>>((ref) async {
  final result = await ref.watch(geographyApiServiceProvider).getProvinces();
  return result.fold((error) => throw Exception(error), (data) => data);
});

/// Regencies/cities of the selected province.
final citiesProvider = FutureProvider.family<List<City>, String?>((
  ref,
  provinceId,
) async {
  if (provinceId == null || provinceId.isEmpty) {
    return const [];
  }
  final result = await ref
      .watch(geographyApiServiceProvider)
      .getRegencies(provinceId);
  return result.fold((error) => throw Exception(error), (data) => data);
});

/// Districts of the selected regency/city.
final districtsProvider = FutureProvider.family<List<District>, String?>((
  ref,
  cityId,
) async {
  if (cityId == null || cityId.isEmpty) {
    return const [];
  }
  final result = await ref
      .watch(geographyApiServiceProvider)
      .getDistricts(cityId);
  return result.fold((error) => throw Exception(error), (data) => data);
});

/// Villages of the selected district (each with its canonical postal code).
final villagesProvider = FutureProvider.family<List<Village>, String?>((
  ref,
  districtId,
) async {
  if (districtId == null || districtId.isEmpty) {
    return const [];
  }
  final result = await ref
      .watch(geographyApiServiceProvider)
      .getVillages(districtId);
  return result.fold((error) => throw Exception(error), (data) => data);
});
