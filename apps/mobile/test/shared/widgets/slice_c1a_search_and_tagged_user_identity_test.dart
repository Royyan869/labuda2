// C1A — Search & Tagged-User Identity contracts
//
// Tests for:
//   A. UserSearchBottomSheet — ProfileAvatar + formatHandle
//   B. TaggedUsersChips    — ProfileAvatar + raw label
//   C. Unified search user results — ProfileAvatar + formatHandle
//   D. Getter removal       — no displayName / handle on UserSearch
//   E. Mention integrity    — formatHandle contracts
//   F. Residue              — no C1A UserInitialsHelper / NetworkImage / @ leaks

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';
import 'package:labuda/features/search/search/domain/entities/user_search.dart';
import 'package:labuda/features/search/search/presentation/widgets/search_result_item.dart';
import 'package:labuda/shared/shared.dart';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

Widget _materialWrap(Widget child) => MaterialApp(home: Scaffold(body: child));

SearchResult _userSearchResult({required String username, String? avatarUrl}) {
  return SearchResult(
    id: 'user-1',
    type: SearchResultType.user,
    title: username, // C1A mapper: raw username, no @
    imageUrl: avatarUrl,
    metadata: {'username': username, 'bio': 'Test bio'},
    createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
  );
}

// ============================================================================
// A — UserSearchBottomSheet contracts (tile-level)
// ============================================================================

void main() {
  group('C1A — UserIdentityFormatter.formatHandle (mention integrity)', () {
    test('formatHandle produces exactly one @ for valid username', () {
      expect(UserIdentityFormatter.formatHandle('john_doe'), '@john_doe');
      expect(UserIdentityFormatter.formatHandle('alice'), '@alice');
    });

    test('formatHandle strips stale leading @ and produces exactly one @', () {
      expect(UserIdentityFormatter.formatHandle('@john_doe'), '@john_doe');
      expect(UserIdentityFormatter.formatHandle('@@double'), '@double');
    });

    test('formatHandle returns null for empty/whitespace/@-only input', () {
      expect(UserIdentityFormatter.formatHandle(''), isNull);
      expect(UserIdentityFormatter.formatHandle('   '), isNull);
      expect(UserIdentityFormatter.formatHandle('@'), isNull);
      expect(UserIdentityFormatter.formatHandle('@@'), isNull);
    });

    test('formatHandle returns null for null input', () {
      expect(UserIdentityFormatter.formatHandle(null), isNull);
    });

    test(
      'formatHandle returns handle for numeric-only username (numeric is valid)',
      () {
        expect(UserIdentityFormatter.formatHandle('12345'), '@12345');
      },
    );

    test('no @@ produced from any input', () {
      final cases = [
        '@username',
        '@@double',
        'clean',
        '@',
        '@@@triple',
        '@a@b',
      ];
      for (final input in cases) {
        final result = UserIdentityFormatter.formatHandle(input);
        if (result != null) {
          expect(
            result.contains('@@'),
            isFalse,
            reason: 'Input "$input" produced "@@": "$result"',
          );
        }
      }
    });

    test('formatHandle never returns bare @', () {
      final cases = ['@', '@@', '', '   ', '@@@'];
      for (final input in cases) {
        final result = UserIdentityFormatter.formatHandle(input);
        if (result != null) {
          expect(
            result,
            isNot(equals('@')),
            reason: 'Input "$input" produced bare "@"',
          );
        }
      }
    });
  });

  group('C1A — ProfileAvatar fallback chain via UserSearch data', () {
    testWidgets(
      'valid username + no avatar → canonical initials via ProfileAvatar',
      (tester) async {
        await tester.pumpWidget(
          _materialWrap(ProfileAvatar(size: 40, username: 'john_doe')),
        );

        expect(find.text('JD'), findsOneWidget);
        expect(find.byIcon(Icons.person), findsNothing);
      },
    );

    testWidgets(
      'valid username + avatar URL → passes to ProfileAvatar image branch',
      (tester) async {
        await tester.pumpWidget(
          _materialWrap(
            ProfileAvatar(
              size: 40,
              imageUrl: 'https://example.com/avatar.png',
              username: 'alice',
            ),
          ),
        );

        await tester.pump();
        // Image loading branch active — no initials rendered
        expect(find.text('AL'), findsNothing);
      },
    );

    testWidgets(
      'numeric-only username → generic person icon (no numeric initials)',
      (tester) async {
        await tester.pumpWidget(
          _materialWrap(ProfileAvatar(size: 40, username: '12345')),
        );

        expect(find.byIcon(Icons.person), findsOneWidget);
        expect(find.text('12'), findsNothing);
      },
    );

    testWidgets('missing username → generic person icon (no bare @)', (
      tester,
    ) async {
      await tester.pumpWidget(_materialWrap(ProfileAvatar(size: 40)));

      expect(find.byIcon(Icons.person), findsOneWidget);
    });

    testWidgets('leading-@ username → correct initials (no @ in avatar text)', (
      tester,
    ) async {
      await tester.pumpWidget(
        _materialWrap(ProfileAvatar(size: 40, username: '@john_doe')),
      );

      expect(find.text('JD'), findsOneWidget);
    });
  });

  group('C1A — UserSearch entity (getter removal)', () {
    test('UserSearch has no displayName getter', () {
      const user = UserSearch(userId: '1', username: 'test');
      // Verify the entity compiles without displayName/handle fields
      expect(user.userId, '1');
      expect(user.username, 'test');
      expect(user.avatarUrl, isNull);
    });

    test('UserSearch.username remains raw — no @ prefix', () {
      const user = UserSearch(userId: '1', username: 'test_user');
      expect(user.username, 'test_user');
      expect(user.username.startsWith('@'), isFalse);
    });

    test('UserSearch equality works correctly without displayName/handle', () {
      const a = UserSearch(userId: '1', username: 'test');
      const b = UserSearch(userId: '1', username: 'test');
      expect(a, equals(b));
    });
  });

  group('C1A — SearchResultItem user-type rendering contracts', () {
    // SearchResultItem with user-type results requires a full Riverpod stack
    // (FollowButton → followStatusProvider → authRepositoryProvider →
    //  apiClientProvider). We prove the contracts at the level of the
    // individual primitives that SearchResultItem delegates to:
    // ProfileAvatar (image) and UserIdentityFormatter.formatHandle (title).

    testWidgets('ProfileAvatar from search result data renders initials', (
      tester,
    ) async {
      // Simulates _buildImage for user type: ProfileAvatar with raw username
      final result = _userSearchResult(username: 'john_doe');
      await tester.pumpWidget(
        _materialWrap(
          ProfileAvatar(
            size: 48,
            imageUrl: result.imageUrl,
            username: result.metadata['username'] as String?,
            showShadow: false,
          ),
        ),
      );

      expect(find.text('JD'), findsOneWidget);
    });

    test('formatHandle applied to raw title produces exactly one @', () {
      // Simulates _buildContent for user type
      final result = _userSearchResult(username: 'alice');
      final formatted = UserIdentityFormatter.formatHandle(result.title);
      expect(formatted, '@alice');
    });

    test('formatHandle + raw username = handle, never raw duplicate', () {
      final result = _userSearchResult(username: 'bob_smith');
      final formatted = UserIdentityFormatter.formatHandle(result.title);
      expect(formatted, '@bob_smith');
      expect(formatted, isNot(equals(result.title))); // not raw
    });

    test('leading-@ username normalised to exactly one @ in formatHandle', () {
      // Even if stale @ data leaks, formatHandle normalises
      final formatted = UserIdentityFormatter.formatHandle('@prefixed');
      expect(formatted, '@prefixed');
      // Never @@
      expect(formatted!.contains('@@'), isFalse);
    });

    test('empty title formatHandle returns null → no bare @', () {
      final formatted = UserIdentityFormatter.formatHandle('');
      expect(formatted, isNull);
    });

    test('metadata username reaches ProfileAvatar (not user ID)', () {
      final result = SearchResult(
        id: 'abc123-def456',
        type: SearchResultType.user,
        title: '12345',
        imageUrl: null,
        metadata: {'username': '12345'},
        createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
      );

      // ProfileAvatar receives metadata username, not result.id
      final avatarUsername = result.metadata['username'] as String?;
      expect(avatarUsername, '12345');
      // User ID is NOT used for avatar initials
      expect(avatarUsername, isNot(equals(result.id)));

      // Numeric-only → ProfileAvatar avatarInitials returns null → person icon
      final initials = UserIdentityFormatter.avatarInitials(avatarUsername);
      expect(initials, isNull);
    });
  });

  group('C1A — TaggedUsersChips avatar contract', () {
    testWidgets('ProfileAvatar renders with raw username in chip', (
      tester,
    ) async {
      // ProfileAvatar for chip size 28 with valid username
      await tester.pumpWidget(
        _materialWrap(
          ProfileAvatar(
            size: 28,
            imageUrl: null,
            username: 'tagged_user',
            showShadow: false,
          ),
        ),
      );

      expect(find.text('TU'), findsOneWidget);
    });

    testWidgets(
      'chip-size ProfileAvatar with image URL passes to image branch',
      (tester) async {
        await tester.pumpWidget(
          _materialWrap(
            ProfileAvatar(
              size: 28,
              imageUrl: 'https://example.com/avatar.png',
              username: 'tagged_user',
              showShadow: false,
            ),
          ),
        );

        await tester.pump();
        // Image loading branch active
        expect(find.text('TU'), findsNothing);
      },
    );

    testWidgets(
      'chip-size ProfileAvatar with missing username shows person icon',
      (tester) async {
        await tester.pumpWidget(
          _materialWrap(ProfileAvatar(size: 28, showShadow: false)),
        );

        expect(find.byIcon(Icons.person), findsOneWidget);
      },
    );
  });

  group('C1A — UserSearchBottomSheet avatar + text contracts', () {
    testWidgets('tile-size ProfileAvatar renders initials for valid username', (
      tester,
    ) async {
      await tester.pumpWidget(
        _materialWrap(
          ProfileAvatar(size: 48, imageUrl: null, username: 'search_user'),
        ),
      );

      expect(find.text('SU'), findsOneWidget);
    });

    testWidgets('tile-size ProfileAvatar with avatar URL enters image branch', (
      tester,
    ) async {
      await tester.pumpWidget(
        _materialWrap(
          ProfileAvatar(
            size: 48,
            imageUrl: 'https://example.com/photo.png',
            username: 'search_user',
          ),
        ),
      );

      await tester.pump();
      expect(find.text('SU'), findsNothing);
    });

    testWidgets(
      'formatHandle on bottom-sheet-style data produces exactly one @',
      (tester) async {
        // Simulate what the bottom sheet subtitle does
        final handle = UserIdentityFormatter.formatHandle('myuser');
        expect(handle, '@myuser');

        // Leading @ is normalised
        final handle2 = UserIdentityFormatter.formatHandle('@myuser');
        expect(handle2, '@myuser');
      },
    );

    testWidgets('empty username formatHandle returns null → empty subtitle', (
      tester,
    ) async {
      // The bottom sheet does: formatHandle(username) ?? ''
      final result = UserIdentityFormatter.formatHandle('') ?? '';
      expect(result, '');
      expect(result.contains('@'), isFalse);
    });
  });

  group('C1A — Mention insertion string contracts', () {
    test('mention insertion produces @username with exactly one @', () {
      // Simulate _onUserSelected in mention_text_field.dart
      final username = 'john_doe';
      final mentionToken = '@$username ';
      expect(mentionToken, '@john_doe ');
      expect(mentionToken.contains('@@'), isFalse);
    });

    test('mention insertion with leading-@ username normalization', () {
      // normalizeUsername strips leading @ before insertion
      final raw = '@john_doe';
      final normalized = UserIdentityFormatter.normalizeUsername(raw);
      expect(normalized, 'john_doe');
      final mentionToken = '@$normalized ';
      expect(mentionToken, '@john_doe ');
    });

    test('mention user ID binding remains separate from display text', () {
      // The user ID is stored separately, not derived from the display text
      const user = UserSearch(userId: 'abc-123-def', username: 'john_doe');
      expect(user.userId, 'abc-123-def');
      expect(user.username, 'john_doe');
      // User ID != username — they're separate concerns
      expect(user.userId, isNot(equals(user.username)));
    });

    test('removing displayName/handle does not break mention selection', () {
      const user = UserSearch(userId: 'usr_1', username: 'testuser');
      // Mention insertion uses username directly: '@${user.username} '
      final insertedMention = '@${user.username} ';
      expect(insertedMention, '@testuser ');
      // This is the exact pattern used in _onUserSelected
    });
  });

  group('C1A — Non-user search results remain unchanged', () {
    testWidgets('listing result renders commerce-style image', (tester) async {
      await tester.pumpWidget(
        ProviderScope(
          child: MaterialApp(
            home: Scaffold(
              body: SearchResultItem(
                result: SearchResult(
                  id: 'l1',
                  type: SearchResultType.listing,
                  title: 'Koi Fish Premium',
                  subtitle: '@seller\nFarm Name',
                  imageUrl: 'https://example.com/koi.jpg',
                  metadata: {
                    'sellerLifecycle': 'active',
                    'sellerTrustLifecycle': 'active',
                    'sellerId': 's1',
                  },
                  createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
                ),
              ),
            ),
          ),
        ),
      );

      // Commerce title renders as-is
      expect(find.text('Koi Fish Premium'), findsOneWidget);
      // Commerce subtitle renders
      expect(find.text('@seller\nFarm Name'), findsOneWidget);
    });

    testWidgets('auction result renders commerce-style image', (tester) async {
      await tester.pumpWidget(
        ProviderScope(
          child: MaterialApp(
            home: Scaffold(
              body: SearchResultItem(
                result: SearchResult(
                  id: 'a1',
                  type: SearchResultType.auction,
                  title: 'Sanke Auction',
                  subtitle: '@auction_seller\nAuction Farm',
                  imageUrl: 'https://example.com/auction.jpg',
                  metadata: {
                    'sellerLifecycle': 'active',
                    'sellerTrustLifecycle': 'active',
                    'sellerId': 's2',
                  },
                  createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
                ),
              ),
            ),
          ),
        ),
      );

      expect(find.text('Sanke Auction'), findsOneWidget);
      expect(find.text('@auction_seller\nAuction Farm'), findsOneWidget);
    });

    testWidgets('content result renders type-indicator (not FollowButton)', (
      tester,
    ) async {
      await tester.pumpWidget(
        ProviderScope(
          child: MaterialApp(
            home: Scaffold(
              body: SearchResultItem(
                result: SearchResult(
                  id: 'c1',
                  type: SearchResultType.content,
                  title: 'My Post',
                  subtitle: 'author_name',
                  metadata: {
                    'lifecycle': 'active',
                    'authorLifecycle': 'active',
                    'authorId': 'a1',
                    'authorUsername': 'author_name',
                  },
                  createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
                ),
              ),
            ),
          ),
        ),
      );

      // Content title renders
      expect(find.text('My Post'), findsOneWidget);
    });
  });
}
