// C1B2 — New-chat canonical user-search: contract and behavioral tests.
//
// Validates:
//   A) newChatUserSearchProvider — source contracts (canonical imports)
//   B) Cleanup contracts — old symbols removed from new-chat files
//   C) chatParticipantLabel canonical fallback — behavioral unit
//   D) Widget source contracts — identity label + avatar wiring
//   E) Profile search provider cleanup

import 'dart:io' as fs;

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/helpers/user_identity_formatter.dart';

// =============================================================================
// Tests
// =============================================================================

void main() {
  // ===========================================================================
  // A) newChatUserSearchProvider — source contracts
  // ===========================================================================

  group('C1B2 — newChatUserSearchProvider source contracts', () {
    test('provider imports canonical SearchApiService', () {
      final source = _codeOnly(_readProviderSource());
      expect(source, contains('searchApiServiceProvider'));
      expect(source, contains('search_api_service.dart'));
      expect(source, isNot(contains('/users/search')));
    });

    test('provider imports UserSearch from search domain', () {
      final source = _readProviderSource();
      expect(source, contains('user_search.dart'));
      expect(source, contains('List<UserSearch>'));
    });

    test('provider imports currentUserIdProvider for principal safety', () {
      final source = _readProviderSource();
      expect(source, contains('currentUserIdProvider'));
    });

    test('provider does NOT import ProfileEntity', () {
      final source = _codeOnly(_readProviderSource());
      expect(source, isNot(contains('ProfileEntity')));
      expect(source, isNot(contains('profile_entity')));
    });

    test('provider skips network call for blank input', () {
      final source = _readProviderSource();
      expect(source, contains('trimmed.isEmpty'));
      expect(source, contains('return <UserSearch>[]'));
    });

    test('provider uses ref.watch for principal (not captured closure)', () {
      final source = _readProviderSource();
      expect(source, contains('ref.watch(currentUserIdProvider)'));
    });
  });

  // ===========================================================================
  // B) Cleanup contracts
  // ===========================================================================

  group('C1B2 — Cleanup contracts', () {
    test('new_chat_screen.dart no longer imports ProfileEntity', () {
      final source = _readScreenSource();
      expect(source, isNot(contains('ProfileEntity')));
      expect(source, isNot(contains('profile.dart')));
    });

    test('new_chat_screen.dart imports newChatUserSearchProvider', () {
      final source = _readScreenSource();
      expect(source, contains('newChatUserSearchProvider'));
    });

    test('new_chat_screen.dart imports UserSearch from search domain', () {
      final source = _readScreenSource();
      expect(source, contains('user_search.dart'));
    });

    test('new_chat_user_list_widget.dart no longer imports ProfileEntity', () {
      final source = _codeOnly(_readWidgetSource());
      expect(source, isNot(contains('ProfileEntity')));
      expect(source, isNot(contains('profile.dart')));
    });

    test('new_chat_user_list_widget.dart has no username: null', () {
      final source = _readWidgetSource();
      expect(source, isNot(contains('username: null')));
    });

    test('new_chat_user_list_widget.dart has no profile.location identity', () {
      final source = _readWidgetSource();
      expect(source, isNot(contains('profile.location')));
    });

    test('new_chat_user_list_widget.dart uses canonical formatHandle', () {
      final source = _readWidgetSource();
      expect(source, contains('UserIdentityFormatter.formatHandle'));
    });

    test('new_chat_user_list_widget.dart passes username to UserAvatar', () {
      final source = _readWidgetSource();
      expect(source, contains('username: user.username'));
    });

    test('new_chat_user_list_widget.dart passes imageUrl', () {
      final source = _readWidgetSource();
      expect(source, contains('imageUrl: user.avatarUrl'));
    });

    test(
      'searchProfilesProvider removed from profile_search_provider.dart',
      () {
        final source = _readProfileSearchProviderSource();
        expect(source, isNot(contains('final searchProfilesProvider')));
      },
    );

    test('new-chat files never reference /users/search', () {
      final screenSource = _codeOnly(_readScreenSource());
      final widgetSource = _codeOnly(_readWidgetSource());
      final providerSource = _codeOnly(_readProviderSource());
      expect(screenSource, isNot(contains('/users/search')));
      expect(widgetSource, isNot(contains('/users/search')));
      expect(providerSource, isNot(contains('/users/search')));
    });

    test('no farmName identity in new-chat widget', () {
      final source = _readWidgetSource();
      expect(source, isNot(contains('farmName')));
    });

    test('no verified icon in new-chat widget', () {
      final source = _readWidgetSource();
      expect(source, isNot(contains('Icons.verified')));
    });

    test('UserAvatar size 40 is preserved', () {
      final source = _readWidgetSource();
      expect(source, contains('UserAvatar('));
      expect(source, contains('size: 40'));
    });

    test('new-chat screen no longer has client-side current-user filter', () {
      final source = _readScreenSource();
      // The old screen had: .where((profile) => profile.userId != currentUserId)
      // The new screen delegates self-exclusion to the provider.
      expect(source, isNot(contains('profile.userId != currentUserId')));
      expect(source, isNot(contains('user.userId != currentUserId')));
    });

    test('widget reads live principal at tap time (not captured prop)', () {
      final source = _readWidgetSource();
      expect(source, contains('ref.read(currentUserIdProvider)'));
    });

    test('widget prevents self-chat via live principal check', () {
      final source = _readWidgetSource();
      expect(source, contains('livePrincipalId == user.userId'));
    });

    test(
      'widget uses live principal in participantIds (not captured prop)',
      () {
        final source = _readWidgetSource();
        expect(source, contains('[livePrincipalId, user.userId]'));
      },
    );

    test('widget no longer accepts ProfileEntity typed parameter', () {
      final source = _readWidgetSource();
      expect(source, contains('final UserSearch user;'));
    });

    test('new-chat screen no longer accepts ProfileEntity in results', () {
      final source = _readScreenSource();
      expect(source, contains('final user = users[index]'));
      expect(source, contains('user: user,'));
    });
  });

  // ===========================================================================
  // C) chatParticipantLabel canonical fallback — behavioral unit
  // ===========================================================================

  group('C1B2 — chatParticipantLabel canonical fallback', () {
    test('valid username → @username', () {
      expect(UserIdentityFormatter.formatHandle('john_doe'), '@john_doe');
    });

    test('null → null (label producer uses "User")', () {
      expect(UserIdentityFormatter.formatHandle(null), isNull);
    });

    test('empty → null (label producer uses "User")', () {
      expect(UserIdentityFormatter.formatHandle(''), isNull);
    });

    test('whitespace → null (label producer uses "User")', () {
      expect(UserIdentityFormatter.formatHandle('   '), isNull);
    });

    test('never returns @User from formatter', () {
      expect(UserIdentityFormatter.formatHandle(null), isNot('@User'));
      expect(UserIdentityFormatter.formatHandle(''), isNot('@User'));
    });

    test('double-@ normalises to single @', () {
      expect(UserIdentityFormatter.formatHandle('@@alice'), '@alice');
    });

    test('underscore-separated username preserved', () {
      expect(UserIdentityFormatter.formatHandle('john_doe'), '@john_doe');
    });
  });

  // ===========================================================================
  // D) Widget source contracts — identity wiring
  // ===========================================================================

  group('C1B2 — Widget identity wiring contracts', () {
    test('widget uses formatHandle ?? User pattern for label', () {
      final source = _readWidgetSource();
      expect(
        source,
        contains("UserIdentityFormatter.formatHandle(user.username) ?? 'User'"),
      );
    });

    test('widget does NOT use location or farm as label', () {
      final source = _readWidgetSource();
      // The label production must not involve location.
      expect(source, isNot(contains('user.location')));
      expect(source, isNot(contains('user.farm')));
    });

    test('widget does NOT use email or phone as label', () {
      final source = _readWidgetSource();
      expect(source, isNot(contains('user.email')));
      expect(source, isNot(contains('user.phone')));
    });

    test('widget does NOT format raw user ID as label', () {
      final source = _readWidgetSource();
      // No raw userId interpolation in a Text widget.
      // The label is always formatHandle result or 'User'.
      expect(source, isNot(contains('user.userId as')));
    });
  });

  // ===========================================================================
  // E) Profile search provider cleanup
  // ===========================================================================

  group('C1B2 — Profile search provider cleanup', () {
    test('profile_search_provider.dart still exports other providers', () {
      final source = _readProfileSearchProviderSource();
      // Other providers must remain.
      expect(source, contains('multipleProfilesProvider'));
      expect(source, contains('trendingProfilesProvider'));
      expect(source, contains('verifiedSellersProvider'));
      expect(source, contains('profilesByTypeProvider'));
    });

    test('profile_search_provider.dart retains ProfileEntity import', () {
      final source = _readProfileSearchProviderSource();
      // Still needed by other providers in the file.
      expect(source, contains('profile_entity.dart'));
    });

    test('profile_notifier still has searchProfiles method', () {
      final source = _readFile(
        'lib/domains/user/profile/presentation/providers/notifiers/profile_notifier.dart',
      );
      // Must remain — profile_notifier has its own searchProfiles consumer.
      expect(source, contains('searchProfiles'));
    });
  });
}

// =============================================================================
// Helpers
// =============================================================================

/// Returns only non-comment lines from [source], skipping `//` and `///`
/// lines as well as block-comment contents.  Use when testing that production
/// code does NOT contain a pattern that may legitimately appear in docstrings.
String _codeOnly(String source) {
  return source
      .split('\n')
      .where((line) {
        final trimmed = line.trimLeft();
        return !trimmed.startsWith('//') &&
            !trimmed.startsWith('/*') &&
            !trimmed.startsWith('*');
      })
      .join('\n');
}

// =============================================================================
// Source readers
// =============================================================================

String _readProviderSource() {
  return _readFile(
    'lib/domains/chat/chat/presentation/providers/new_chat_user_search_provider.dart',
  );
}

String _readScreenSource() {
  return _readFile(
    'lib/domains/chat/chat/presentation/screens/new_chat_screen.dart',
  );
}

String _readWidgetSource() {
  return _readFile(
    'lib/domains/chat/chat/presentation/widgets/new_chat_user_list_widget.dart',
  );
}

String _readProfileSearchProviderSource() {
  return _readFile(
    'lib/domains/user/profile/presentation/providers/profile_search_provider.dart',
  );
}

String _readFile(String relativePath) {
  final file = fs.File(relativePath);
  if (!file.existsSync()) {
    throw Exception('File not found: $relativePath');
  }
  return file.readAsStringSync();
}
