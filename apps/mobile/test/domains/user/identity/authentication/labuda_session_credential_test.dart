import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/labuda_session_credential.dart';

void main() {
  group('LabudaSessionCredential', () {
    test('holds both tokens', () {
      const cred = LabudaSessionCredential(
        accessToken: 'access-123',
        refreshToken: 'refresh-456',
      );
      expect(cred.accessToken, 'access-123');
      expect(cred.refreshToken, 'refresh-456');
    });

    test('equality by value', () {
      const a = LabudaSessionCredential(
        accessToken: 'a',
        refreshToken: 'b',
      );
      const b = LabudaSessionCredential(
        accessToken: 'a',
        refreshToken: 'b',
      );
      expect(a, equals(b));
      expect(a.hashCode, b.hashCode);
    });

    test('inequality when tokens differ', () {
      const a = LabudaSessionCredential(
        accessToken: 'a',
        refreshToken: 'b',
      );
      const b = LabudaSessionCredential(
        accessToken: 'a',
        refreshToken: 'c',
      );
      expect(a, isNot(equals(b)));
    });

    test('toString does not leak token values', () {
      const cred = LabudaSessionCredential(
        accessToken: 'secret-access',
        refreshToken: 'secret-refresh',
      );
      final str = cred.toString();
      expect(str, isNot(contains('secret-access')));
      expect(str, isNot(contains('secret-refresh')));
      expect(str, contains('[REDACTED]'));
    });
  });
}
