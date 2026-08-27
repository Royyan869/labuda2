// STAGE 4B-1: canonical URL validator behavior.
//
// Preserves the factual business behavior of the existing real consumer
// (ValidationService.validateUrl → UpdateProfileUseCase cover/farm URLs):
// HTTPS accepted; http://localhost development URLs accepted; plain http to
// non-localhost rejected. No network/domain validation.
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/helpers/canonical_url_validator.dart';

void main() {
  group('CanonicalUrlValidator.isValid', () {
    test('accepts HTTPS URLs', () {
      const valid = [
        'https://example.com',
        'https://example.com/path?q=1',
        'https://sub.example.co.id/photo.jpg',
        'https://storage.googleapis.com/bucket/key',
      ];
      for (final url in valid) {
        expect(CanonicalUrlValidator.isValid(url), isTrue, reason: url);
      }
    });

    test('accepts http://localhost development URLs', () {
      expect(CanonicalUrlValidator.isValid('http://localhost:8080'), isTrue);
      expect(CanonicalUrlValidator.isValid('http://localhost:8080/img.png'), isTrue);
    });

    test('rejects plain http to non-localhost hosts', () {
      expect(CanonicalUrlValidator.isValid('http://example.com'), isFalse);
      expect(CanonicalUrlValidator.isValid('http://192.168.1.8:8080'), isFalse);
    });

    test('rejects invalid schemes and non-URL strings', () {
      const invalid = [
        'example.com', // no scheme
        'ftp://example.com',
        'javascript:alert(1)',
        'www.example.com',
        'not a url',
        'https://', // nothing after scheme
      ];
      for (final url in invalid) {
        expect(CanonicalUrlValidator.isValid(url), isFalse, reason: url);
      }
    });

    test('rejects empty and whitespace-only input', () {
      expect(CanonicalUrlValidator.isValid(null), isFalse);
      expect(CanonicalUrlValidator.isValid(''), isFalse);
      expect(CanonicalUrlValidator.isValid('   '), isFalse);
    });

    test('trims surrounding whitespace', () {
      expect(CanonicalUrlValidator.isValid('  https://example.com  '), isTrue);
    });
  });

  group('CanonicalUrlValidator.validationMessage', () {
    test('returns null for valid URL', () {
      expect(
        CanonicalUrlValidator.validationMessage('https://example.com'),
        isNull,
      );
    });

    test('returns required message for empty input', () {
      expect(CanonicalUrlValidator.validationMessage(''), 'URL is required');
    });

    test('returns invalid-format message for malformed input', () {
      expect(
        CanonicalUrlValidator.validationMessage('http://example.com'),
        'Invalid URL format',
      );
    });
  });
}
