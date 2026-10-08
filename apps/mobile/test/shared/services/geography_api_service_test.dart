// Canonical Geography API access-path contract.
//
// Proves the mobile app reads Province -> Regency -> District -> Village from
// the ONE canonical Geography API and maps the canonical wire fields
// (code/name/parent_code/postal_code). No local dataset exists.

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api.dart';
import 'package:labuda/shared/services/geography_api_service.dart';

class _FakeApiClient implements ApiClient {
  final List<String> calls = [];

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    calls.add(path);
    final Map<String, dynamic> body;
    switch (path) {
      case '/geographies/provinces':
        body = {
          'success': true,
          'data': [
            {'code': '32', 'level': 'province', 'name': 'Jawa Barat'},
          ],
        };
      case '/geographies/provinces/32/regencies':
        body = {
          'success': true,
          'data': [
            {
              'code': '3204',
              'level': 'regency',
              'name': 'Kabupaten Bandung',
              'parent_code': '32',
            },
          ],
        };
      case '/geographies/regencies/3204/districts':
        body = {
          'success': true,
          'data': [
            {
              'code': '320401',
              'level': 'district',
              'name': 'Ciwidey',
              'parent_code': '3204',
            },
          ],
        };
      case '/geographies/districts/320401/villages':
        body = {
          'success': true,
          'data': [
            {
              'code': '3204012001',
              'level': 'village',
              'name': 'Ciwidey',
              'parent_code': '320401',
              'postal_code': '40973',
            },
          ],
        };
      default:
        body = {'success': true, 'data': <dynamic>[]};
    }
    return Response<dynamic>(
          requestOptions: RequestOptions(path: path),
          data: body,
          statusCode: 200,
        )
        as Response<T>;
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  test('reads the canonical hierarchy from the ONE Geography API', () async {
    final client = _FakeApiClient();
    final service = GeographyApiService(client);

    final provinces = await service.getProvinces();
    expect(provinces.isSuccess, isTrue);
    expect(provinces.data!.single.id, '32');
    expect(provinces.data!.single.name, 'Jawa Barat');

    final regencies = await service.getRegencies('32');
    expect(regencies.data!.single.id, '3204');
    expect(regencies.data!.single.provinceId, '32');

    final districts = await service.getDistricts('3204');
    expect(districts.data!.single.id, '320401');
    expect(districts.data!.single.cityId, '3204');

    final villages = await service.getVillages('320401');
    expect(villages.data!.single.id, '3204012001');
    expect(villages.data!.single.districtId, '320401');
    expect(villages.data!.single.postalCode, '40973');

    expect(client.calls, [
      '/geographies/provinces',
      '/geographies/provinces/32/regencies',
      '/geographies/regencies/3204/districts',
      '/geographies/districts/320401/villages',
    ]);
  });
}
