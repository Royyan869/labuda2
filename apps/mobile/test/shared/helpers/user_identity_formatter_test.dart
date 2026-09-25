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

  // REMOVED: avatarInitials group — initials no longer exist (Owner decision 2026-09-24:
  // avatars are ALWAYS the user photo or user icon; the method was killed at the source).
}
