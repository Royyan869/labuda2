import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/user/profile/data/models/api/user_api_models.dart';

void main() {
  group('Firebase session response contract', () {
    test('incomplete exchange parses session-only payload', () {
      final response = FirebaseExchangeResponse.fromJson({
        'user_id': 'user-3',
        'access_token': 'restricted-token',
        'expires_at': '2026-06-14T00:00:00Z',
        'requires_profile_completion': true,
        'created': false,
        'email': 'charlie@example.com',
      });

      expect(response.userId, equals('user-3'));
      expect(response.requiresProfileCompletion, isTrue);
      expect(response.email, equals('charlie@example.com'));
      expect(response.refreshToken, isNull);
      expect(response.asCompleteResponse, isNull);
    });

    test(
      'complete exchange parses refresh tokens without embedded user data',
      () {
        final response = FirebaseExchangeResponse.fromJson({
          'user_id': 'user-3',
          'access_token': 'access-token',
          'refresh_token': 'refresh-token',
          'expires_at': '2026-06-14T00:00:00Z',
          'refresh_expires_at': '2026-07-14T00:00:00Z',
          'created': true,
          'requires_profile_completion': false,
        });

        expect(response.isComplete, isTrue);
        expect(response.asCompleteResponse, isNotNull);
        expect(
          response.asCompleteResponse!.refreshToken,
          equals('refresh-token'),
        );
        expect(response.asCompleteResponse!.userId, equals('user-3'));
      },
    );

    test('complete response parser only requires session fields', () {
      final response = FirebaseExchangeCompleteResponse.fromJson({
        'user_id': 'user-4',
        'access_token': 'access-token',
        'refresh_token': 'refresh-token',
        'expires_at': '2026-06-14T00:00:00Z',
        'refresh_expires_at': '2026-07-14T00:00:00Z',
        'created': false,
        'requires_profile_completion': false,
      });

      expect(response.userId, equals('user-4'));
      expect(response.accessToken, equals('access-token'));
      expect(response.refreshToken, equals('refresh-token'));
      expect(response.requiresProfileCompletion, isFalse);
      expect(response.created, isFalse);
    });
  });
}
