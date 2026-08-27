import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/user/profile/data/mappers/user_api_mapper.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';

Map<String, dynamic> _baseUserJson({
  String? location,
  Map<String, dynamic>? profile,
}) {
  return {
    'id': 'user-1',
    'email': 'seller@example.com',
    'account_status': 'active',
    'roles': ['seller'],
    'created_at': '2026-01-01T00:00:00.000Z',
    'updated_at': '2026-01-01T00:00:00.000Z',
    ...(location == null ? const <String, dynamic>{} : {'location': location}),
    ...(profile == null ? const <String, dynamic>{} : {'profile': profile}),
  };
}

Map<String, dynamic> _profileJson({String? location}) {
  return {
    'id': 'profile-1',
    'username': 'sellerone',
    'preferred_lang': 'id',
    ...(location == null ? const <String, dynamic>{} : {'location': location}),
  };
}

void main() {
  group('User API location mapping', () {
    test('UserApiResponse parses top-level public location', () {
      final response = UserApiResponse.fromJson(
        _baseUserJson(location: '  Bogor, West Java  '),
      );

      expect(response.location, 'Bogor, West Java');
    });

    test(
      'ProfileEntity prefers top-level location over nested profile location',
      () {
        final response = UserApiResponse.fromJson(
          _baseUserJson(
            location: '  Depok, West Java  ',
            profile: _profileJson(location: 'Bandung, West Java'),
          ),
        );

        final profileEntity = UserApiMapper.toProfileEntity(response);

        expect(profileEntity.location, 'Depok, West Java');
      },
    );

    test(
      'ProfileEntity falls back to nested profile location when top-level location is absent',
      () {
        final response = UserApiResponse.fromJson(
          _baseUserJson(profile: _profileJson(location: 'Bandung, West Java')),
        );

        final profileEntity = UserApiMapper.toProfileEntity(response);

        expect(profileEntity.location, 'Bandung, West Java');
      },
    );
  });
}
