// refresh_token_contract_test.dart â€” mobile contract regression proof for
// the refresh token rotation feature (F3C).
//
// Invariants covered (matching TASK C matrix):
//   C-1: BackendRefreshResponse.fromJson parses refresh_token correctly
//   C-2: BackendRefreshResponse.fromJson parses all four fields
//   C-3: BackendRefreshResponse.fromJson handles missing fields with safe defaults
//   C-4: FirebaseExchangeResponse.fromJson parses refresh_expires_at correctly
//   C-5: FirebaseExchangeResponse.fromJson handles missing refresh_expires_at safely
//
// NOTE â€” Mobile storage-after-refresh contract (TASK C invariant 2):
//   refreshPlatformToken() in AuthApiDatasource returns a Result<BackendRefreshResponse>
//   but does NOT automatically persist the tokens. The caller is responsible for
//   calling setAuthToken() and setRefreshToken() after a successful refresh.
//   This is documented as P2 infrastructure debt (F3D+).
//   The AuthInterceptor handles 401 via Firebase token force-refresh (not platform JWT).

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';

void main() {
  group('BackendRefreshResponse.fromJson', () {
    // C-1 / C-2: all four fields parsed correctly
    test('parses all fields from canonical response', () {
      final json = {
        'access_token': 'new_access_token_xyz',
        'refresh_token': 'new_refresh_token_abc',
        'expires_at': '2026-06-14T12:00:00Z',
        'refresh_expires_at': '2026-07-14T12:00:00Z',
      };

      final resp = BackendRefreshResponse.fromJson(json);

      expect(resp.accessToken, equals('new_access_token_xyz'));
      expect(resp.refreshToken, equals('new_refresh_token_abc'));
      expect(resp.expiresAt, equals('2026-06-14T12:00:00Z'));
      expect(resp.refreshExpiresAt, equals('2026-07-14T12:00:00Z'));
    });

    // C-1: refresh_token specifically parsed (not conflated with access_token)
    test('refresh_token and access_token are distinct fields', () {
      final json = {
        'access_token': 'access_aaa',
        'refresh_token': 'refresh_bbb',
        'expires_at': '2026-06-14T00:00:00Z',
        'refresh_expires_at': '2026-07-14T00:00:00Z',
      };

      final resp = BackendRefreshResponse.fromJson(json);

      expect(resp.accessToken, isNot(equals(resp.refreshToken)));
      expect(resp.accessToken, equals('access_aaa'));
      expect(resp.refreshToken, equals('refresh_bbb'));
    });

    // C-3: missing fields fall back to empty string (safe default)
    test('missing refresh_token defaults to empty string', () {
      final json = <String, dynamic>{
        'access_token': 'access_token_only',
        // refresh_token deliberately absent
        'expires_at': '2026-06-14T00:00:00Z',
        'refresh_expires_at': '2026-07-14T00:00:00Z',
      };

      final resp = BackendRefreshResponse.fromJson(json);

      // Empty string is the safe default â€” callers must check isNotEmpty
      // before persisting.
      expect(resp.refreshToken, equals(''));
    });

    test('completely empty JSON returns safe empty-string defaults', () {
      final resp = BackendRefreshResponse.fromJson({});

      expect(resp.accessToken, equals(''));
      expect(resp.refreshToken, equals(''));
      expect(resp.expiresAt, equals(''));
      expect(resp.refreshExpiresAt, equals(''));
    });

    test('null field values fall back to empty string', () {
      final json = <String, dynamic>{
        'access_token': null,
        'refresh_token': null,
        'expires_at': null,
        'refresh_expires_at': null,
      };

      final resp = BackendRefreshResponse.fromJson(json);

      expect(resp.accessToken, equals(''));
      expect(resp.refreshToken, equals(''));
      expect(resp.expiresAt, equals(''));
      expect(resp.refreshExpiresAt, equals(''));
    });
  });

  group('FirebaseExchangeResponse.fromJson â€” refresh_expires_at', () {
    // C-4: refresh_expires_at parsed from login response
    test('parses refresh_expires_at from login response', () {
      final json = {
        'access_token': 'access_token_from_login',
        'refresh_token': 'refresh_token_from_login',
        'expires_at': '2026-06-14T01:00:00Z',
        'refresh_expires_at': '2026-07-14T01:00:00Z',
        'created': false,
        'profile_complete': true,
      };

      final resp = FirebaseExchangeResponse.fromJson(json);

      expect(resp.refreshToken, equals('refresh_token_from_login'));
      expect(resp.refreshExpiresAt, equals('2026-07-14T01:00:00Z'));
    });

    // C-5: missing refresh_expires_at handled safely
    test('missing refresh_expires_at defaults to null', () {
      final json = {
        'access_token': 'at',
        'refresh_token': 'rt',
        'expires_at': '2026-06-14T00:00:00Z',
        // refresh_expires_at deliberately absent
        'created': false,
        'profile_complete': false,
      };

      final resp = FirebaseExchangeResponse.fromJson(json);

      expect(resp.refreshExpiresAt, isNull);
    });

    // Both AT and RT are present in login response
    test('login response has distinct access_token and refresh_token', () {
      final json = {
        'access_token': 'at_value',
        'refresh_token': 'rt_value',
        'expires_at': '2026-06-14T00:00:00Z',
        'refresh_expires_at': '2026-07-14T00:00:00Z',
        'created': true,
        'profile_complete': false,
      };

      final resp = FirebaseExchangeResponse.fromJson(json);

      expect(resp.accessToken, equals('at_value'));
      expect(resp.refreshToken, equals('rt_value'));
      expect(resp.accessToken, isNot(equals(resp.refreshToken)));
    });
  });
}
