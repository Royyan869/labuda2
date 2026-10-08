/// Canonical Geography API Service
///
/// The ONE runtime access path to Labuda's Geography Master. Province, regency,
/// district and village data are read from the backend canonical Geography API
/// — never from a local dataset. There is no offline fallback: a geography
/// authority second copy is forbidden.
library;

import 'package:labuda/core/api/api.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/shared/models/wilayah_models.dart';

class GeographyApiService extends BaseApiRepository {
  GeographyApiService(super.apiClient, {super.logger});

  /// Provinces of Indonesia.
  Future<Result<List<Province>>> getProvinces() {
    return executeListRequest(
      () => apiClient.get('/geographies/provinces'),
      itemParser: (json) => Province(
        id: json['code'] as String,
        name: json['name'] as String,
      ),
    );
  }

  /// Regencies/cities of a province.
  Future<Result<List<City>>> getRegencies(String provinceCode) {
    return executeListRequest(
      () => apiClient.get('/geographies/provinces/$provinceCode/regencies'),
      itemParser: (json) => City(
        id: json['code'] as String,
        name: json['name'] as String,
        provinceId: (json['parent_code'] as String?) ?? provinceCode,
      ),
    );
  }

  /// Districts of a regency/city.
  Future<Result<List<District>>> getDistricts(String regencyCode) {
    return executeListRequest(
      () => apiClient.get('/geographies/regencies/$regencyCode/districts'),
      itemParser: (json) => District(
        id: json['code'] as String,
        name: json['name'] as String,
        cityId: (json['parent_code'] as String?) ?? regencyCode,
      ),
    );
  }

  /// Villages of a district (each carries its canonical postal code).
  Future<Result<List<Village>>> getVillages(String districtCode) {
    return executeListRequest(
      () => apiClient.get('/geographies/districts/$districtCode/villages'),
      itemParser: (json) => Village(
        id: json['code'] as String,
        name: json['name'] as String,
        districtId: (json['parent_code'] as String?) ?? districtCode,
        postalCode: json['postal_code'] as String?,
      ),
    );
  }
}
