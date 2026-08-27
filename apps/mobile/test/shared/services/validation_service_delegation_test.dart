// STAGE 4B-1: ValidationService delegates email/phone/URL validation to the
// canonical field validators (thin delegating wrapper — no policy of its
// own for those fields).
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/services/validation_service.dart';

void main() {
  const service = ValidationService();

  group('ValidationService.validateEmail delegates to canonical email', () {
    test('accepts the same valid inputs as CanonicalEmailValidator', () async {
      for (final email in ['user@example.com', 'first.last@example.co.id']) {
        final result = await service.validateEmail(email);
        expect(result.isSuccess, isTrue, reason: email);
      }
    });

    test('rejects the same invalid inputs as CanonicalEmailValidator',
        () async {
      for (final email in ['plainaddress', 'user@nodot', '']) {
        final result = await service.validateEmail(email);
        expect(result.isError, isTrue, reason: email);
      }
    });
  });

  group('ValidationService.validatePhoneNumber delegates to canonical phone', () {
    test('accepts canonical prefix variants (format only)', () async {
      for (final phone in ['+6281234567890', '6281234567890', '081234567890']) {
        final result = await service.validatePhoneNumber(phone);
        expect(result.isSuccess, isTrue, reason: phone);
      }
    });

    test('rejects out-of-range digit counts and foreign prefixes', () async {
      for (final phone in [
        '0812345', // 7 digits
        '08123456789012', // 13 digits
        '+6512345678', // foreign prefix
      ]) {
        final result = await service.validatePhoneNumber(phone);
        expect(result.isError, isTrue, reason: phone);
      }
    });
  });

  group('ValidationService.validateUrl delegates to canonical URL', () {
    test('accepts HTTPS and localhost, rejects plain http elsewhere', () async {
      expect(
        (await service.validateUrl('https://example.com')).isSuccess,
        isTrue,
      );
      expect(
        (await service.validateUrl('http://localhost:8080/img.png')).isSuccess,
        isTrue,
      );
      expect(
        (await service.validateUrl('http://example.com')).isError,
        isTrue,
      );
      expect((await service.validateUrl('example.com')).isError, isTrue);
    });
  });
}
