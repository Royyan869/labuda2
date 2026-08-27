import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/helpers/user_identity_formatter.dart';

void main() {
  group('UserIdentityFormatter.normalizeUsername', () {
    test('strips leading @', () {
      expect(UserIdentityFormatter.normalizeUsername('@john_doe'), 'john_doe');
    });

    test('strips multiple leading @', () {
      expect(UserIdentityFormatter.normalizeUsername('@@john_doe'), 'john_doe');
    });

    test('plain username passes through', () {
      expect(UserIdentityFormatter.normalizeUsername('john_doe'), 'john_doe');
    });

    test('null returns null', () {
      expect(UserIdentityFormatter.normalizeUsername(null), isNull);
    });

    test('empty string returns null', () {
      expect(UserIdentityFormatter.normalizeUsername(''), isNull);
    });

    test('whitespace-only returns null', () {
      expect(UserIdentityFormatter.normalizeUsername('   '), isNull);
    });

    test('only @ returns null', () {
      expect(UserIdentityFormatter.normalizeUsername('@'), isNull);
    });

    test('trims whitespace around username', () {
      expect(
        UserIdentityFormatter.normalizeUsername('  john_doe  '),
        'john_doe',
      );
    });
  });

  group('UserIdentityFormatter.formatHandle', () {
    test('produces exactly one @', () {
      expect(UserIdentityFormatter.formatHandle('john_doe'), '@john_doe');
    });

    test('normalises stale leading @', () {
      expect(UserIdentityFormatter.formatHandle('@john_doe'), '@john_doe');
    });

    test('normalises multiple leading @', () {
      expect(UserIdentityFormatter.formatHandle('@@john_doe'), '@john_doe');
    });

    test('null returns null', () {
      expect(UserIdentityFormatter.formatHandle(null), isNull);
    });

    test('empty returns null (not bare @)', () {
      expect(UserIdentityFormatter.formatHandle(''), isNull);
    });

    test('whitespace returns null', () {
      expect(UserIdentityFormatter.formatHandle('   '), isNull);
    });

    test('only @ returns null', () {
      expect(UserIdentityFormatter.formatHandle('@'), isNull);
    });
  });

  group('UserIdentityFormatter.avatarInitials', () {
    // --- canonical examples from the spec ---
    test('john_doe → JD', () {
      expect(UserIdentityFormatter.avatarInitials('john_doe'), 'JD');
    });

    test('@john_doe → JD (strips @)', () {
      expect(UserIdentityFormatter.avatarInitials('@john_doe'), 'JD');
    });

    test('alice → AL', () {
      expect(UserIdentityFormatter.avatarInitials('alice'), 'AL');
    });

    test('a → A (single letter)', () {
      expect(UserIdentityFormatter.avatarInitials('a'), 'A');
    });

    test('a1b2 → AB (digits stripped, letters remain)', () {
      expect(UserIdentityFormatter.avatarInitials('a1b2'), 'AB');
    });

    test('123john → JO (leading digits stripped from token, letters used)', () {
      expect(UserIdentityFormatter.avatarInitials('123john'), 'JO');
    });

    test('numeric-only → null', () {
      expect(UserIdentityFormatter.avatarInitials('12345'), isNull);
    });

    test('@ → null', () {
      expect(UserIdentityFormatter.avatarInitials('@'), isNull);
    });

    test('empty string → null', () {
      expect(UserIdentityFormatter.avatarInitials(''), isNull);
    });

    test('null → null', () {
      expect(UserIdentityFormatter.avatarInitials(null), isNull);
    });

    // --- additional edge cases ---
    test('separator-only (_) → null', () {
      expect(UserIdentityFormatter.avatarInitials('_'), isNull);
    });

    test('multiple separators only → null', () {
      expect(UserIdentityFormatter.avatarInitials('__--..'), isNull);
    });

    test('output never exceeds two characters', () {
      final result = UserIdentityFormatter.avatarInitials('abcdefghij');
      expect(result, hasLength(2));
    });

    test('whitespace-only → null', () {
      expect(UserIdentityFormatter.avatarInitials('   '), isNull);
    });

    test('two-letter token with underscores → first+last token initials', () {
      // "john_doe" → tokens ["john","doe"] → "J"+"D" → "JD"
      expect(UserIdentityFormatter.avatarInitials('john_doe'), 'JD');
    });

    test('hyphenated username → initials from letter tokens', () {
      expect(UserIdentityFormatter.avatarInitials('john-doe'), 'JD');
    });

    test('dot-separated username → initials from letter tokens', () {
      expect(UserIdentityFormatter.avatarInitials('john.doe'), 'JD');
    });

    test('three tokens → first+last', () {
      // "muhammad_ali_akbar" → "M"+"A" → "MA"
      expect(UserIdentityFormatter.avatarInitials('muhammad_ali_akbar'), 'MA');
    });

    test('uppercase output', () {
      final result = UserIdentityFormatter.avatarInitials('john_doe');
      expect(result, 'JD'); // already uppercase assertion
    });

    test('mixed case input produces uppercase output', () {
      expect(UserIdentityFormatter.avatarInitials('JoHn_DoE'), 'JD');
    });

    test('trailing @ inside token is treated as letter after stripping', () {
      // "@@john" → normalize strips both @ → "john" → 4 letters → "JO"
      expect(UserIdentityFormatter.avatarInitials('@@john'), 'JO');
    });
  });
}
