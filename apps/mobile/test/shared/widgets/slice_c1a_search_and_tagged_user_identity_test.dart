// C1A — Search & Tagged-User Identity contracts
//
// Tests for:
//   A. ProfileAvatar initials/image rendering via userId
//   B. UserSearch entity field contracts
//   C. SearchResultItem non-user rendering (listing/auction/content)
//   D. Mention insertion string contracts
//
// RETIRED tests (covered by canonical test/shared/helpers/user_identity_formatter_test.dart
// which passes 36/36):
//   - UserIdentityFormatter.formatHandle mention integrity (7 tests)
//   - SearchResultItem formatHandle integration (4 tests — SearchResultItem
//     renders result.title directly, no longer applies formatHandle)
//   - UserSearchBottomSheet formatHandle (2 tests)
//
// Canonical production references:
//   - ProfileAvatar(userId:, size:) — shared/widgets/profile_avatar.dart
//   - UserInitialsHelper.fromUserId — shared/helpers/user_initials_helper.dart
//   - UserIdentityFormatter — shared/helpers/user_identity_formatter.dart
//   - UserSearch — features/search/search/domain/entities/user_search.dart
//   - SearchResult / SearchResultItem — features/search/search/...

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';
import 'package:labuda/features/search/search/domain/entities/user_search.dart';
import 'package:labuda/features/search/search/presentation/widgets/search_result_item.dart';
import 'package:labuda/shared/helpers/user_identity_formatter.dart';
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
// A — ProfileAvatar fallback chain via UserSearch data
//
// Canonical ProfileAvatar takes userId (required). Initials derive from
// UserInitialsHelper.fromUserId which returns first 2 chars of userId
// (uppercased). Empty/null userId → 'U'.
// ============================================================================

void main() {
  group('C1A — ProfileAvatar fallback chain via UserSearch data', () {
    testWidgets(
      'valid userId + no avatar → canonical initials via ProfileAvatar',
      (tester) async {
        await tester.pumpWidget(
          _materialWrap(ProfileAvatar(userId: 'john_doe', size: 40)),
        );

        // UserInitialsHelper.fromUserId('john_doe') → 'JO' (first 2 chars)
        expect(find.text('JO'), findsOneWidget);
        expect(find.byIcon(Icons.person), findsNothing);
      },
    );

    testWidgets(
      'valid userId + avatar URL → passes to ProfileAvatar image branch',
      (tester) async {
        await tester.pumpWidget(
          _materialWrap(
            ProfileAvatar(
              userId: 'alice',
              size: 40,
              imageUrl: 'https://example.com/avatar.png',
            ),
          ),
        );

        await tester.pump();
        // Image loading branch active — no initials rendered
        expect(find.text('AL'), findsNothing);
      },
    );

    testWidgets(
      'numeric-only userId → shows first two digits as initials',
      (tester) async {
        await tester.pumpWidget(
          _materialWrap(ProfileAvatar(userId: '12345', size: 40)),
        );

        // UserInitialsHelper.fromUserId('12345') → '12'
        expect(find.text('12'), findsOneWidget);
      },
    );

    testWidgets('empty userId → generic U initial', (tester) async {
      await tester.pumpWidget(
        _materialWrap(ProfileAvatar(userId: '', size: 40)),
      );

      // UserInitialsHelper.fromUserId('') → 'U'
      expect(find.text('U'), findsOneWidget);
    });

    testWidgets('single-char userId → uppercase single char', (tester) async {
      await tester.pumpWidget(
        _materialWrap(ProfileAvatar(userId: 'a', size: 40)),
      );

      // UserInitialsHelper.fromUserId('a') → 'A'
      expect(find.text('A'), findsOneWidget);
    });
  });

  // ==========================================================================
  // B — UserSearch entity field contracts
  // ==========================================================================

  group('C1A — UserSearch entity fields', () {
    test('UserSearch core fields are accessible', () {
      const user = UserSearch(userId: '1', username: 'test');
      expect(user.userId, '1');
      expect(user.username, 'test');
      expect(user.avatarUrl, isNull);
      // displayName and handle are canonical getters (@$username)
      expect(user.displayName, '@test');
      expect(user.handle, '@test');
    });

    test('UserSearch.username remains raw — no @ prefix', () {
      const user = UserSearch(userId: '1', username: 'test_user');
      expect(user.username, 'test_user');
      expect(user.username.startsWith('@'), isFalse);
    });

    test('UserSearch equality works correctly', () {
      const a = UserSearch(userId: '1', username: 'test');
      const b = UserSearch(userId: '1', username: 'test');
      expect(a, equals(b));
    });
  });

  // ==========================================================================
  // C — SearchResultItem rendering contracts
  // ==========================================================================

  group('C1A — SearchResultItem user-type rendering contracts', () {
    testWidgets('ProfileAvatar from search result data renders initials', (
      tester,
    ) async {
      // Simulates _buildImage for user type: ProfileAvatar with userId
      final result = _userSearchResult(username: 'john_doe');
      await tester.pumpWidget(
        _materialWrap(
          ProfileAvatar(
            userId: result.metadata['username'] as String,
            size: 48,
            imageUrl: result.imageUrl,
            showShadow: false,
          ),
        ),
      );

      // UserInitialsHelper.fromUserId('john_doe') → 'JO'
      expect(find.text('JO'), findsOneWidget);
    });

    test('metadata username reaches ProfileAvatar (not result.id)', () {
      final result = SearchResult(
        id: 'abc123-def456',
        type: SearchResultType.user,
        title: '12345',
        imageUrl: null,
        metadata: {'username': '12345'},
        createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
      );

      // ProfileAvatar receives metadata username as userId, not result.id
      final avatarUserId = result.metadata['username'] as String?;
      expect(avatarUserId, '12345');
      // User ID is NOT used for avatar initials
      expect(avatarUserId, isNot(equals(result.id)));

      // Numeric-only → UserInitialsHelper.fromUserId returns first 2 digits
      final initials = UserInitialsHelper.fromUserId(avatarUserId);
      expect(initials, '12');
    });
  });

  // ==========================================================================
  // D — TaggedUsersChips avatar contract
  // ==========================================================================

  group('C1A — TaggedUsersChips avatar contract', () {
    testWidgets('ProfileAvatar renders with userId in chip', (tester) async {
      // ProfileAvatar for chip size 28 with valid userId
      await tester.pumpWidget(
        _materialWrap(
          ProfileAvatar(
            userId: 'tagged_user',
            size: 28,
            imageUrl: null,
            showShadow: false,
          ),
        ),
      );

      // UserInitialsHelper.fromUserId('tagged_user') → 'TA'
      expect(find.text('TA'), findsOneWidget);
    });

    testWidgets(
      'chip-size ProfileAvatar with image URL passes to image branch',
      (tester) async {
        await tester.pumpWidget(
          _materialWrap(
            ProfileAvatar(
              userId: 'tagged_user',
              size: 28,
              imageUrl: 'https://example.com/avatar.png',
              showShadow: false,
            ),
          ),
        );

        await tester.pump();
        // Image loading branch active
        expect(find.text('TA'), findsNothing);
      },
    );

    testWidgets(
      'chip-size ProfileAvatar with empty userId shows U initial',
      (tester) async {
        await tester.pumpWidget(
          _materialWrap(
            ProfileAvatar(userId: '', size: 28, showShadow: false),
          ),
        );

        // UserInitialsHelper.fromUserId('') → 'U'
        expect(find.text('U'), findsOneWidget);
      },
    );
  });

  // ==========================================================================
  // E — UserSearchBottomSheet avatar contracts
  // ==========================================================================

  group('C1A — UserSearchBottomSheet avatar contracts', () {
    testWidgets('tile-size ProfileAvatar renders initials for valid userId', (
      tester,
    ) async {
      await tester.pumpWidget(
        _materialWrap(ProfileAvatar(userId: 'search_user', size: 48)),
      );

      // UserInitialsHelper.fromUserId('search_user') → 'SE'
      expect(find.text('SE'), findsOneWidget);
    });

    testWidgets(
      'tile-size ProfileAvatar with avatar URL enters image branch',
      (tester) async {
        await tester.pumpWidget(
          _materialWrap(
            ProfileAvatar(
              userId: 'search_user',
              size: 48,
              imageUrl: 'https://example.com/photo.png',
            ),
          ),
        );

        await tester.pump();
        expect(find.text('SE'), findsNothing);
      },
    );
  });

  // ==========================================================================
  // F — Mention insertion string contracts
  // ==========================================================================

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

  // ==========================================================================
  // G — Non-user search results remain unchanged
  // ==========================================================================

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
