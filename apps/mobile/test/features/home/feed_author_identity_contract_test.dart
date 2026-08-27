import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/providers/core_providers.dart';
import 'package:labuda/core/src/providers/presence_provider.dart'
    show
        PresenceAuthorityState,
        PresenceManager,
        PresenceState,
        PresenceSubscriptionHandle,
        PresenceSubscriptionRegistry,
        presenceManagerProvider,
        presenceSubscriptionRegistryProvider;
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/domains/social/share/presentation/widgets/share_bottom_sheet.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show avatarCacheServiceProvider;
import 'package:labuda/domains/user/profile/data/services/avatar_cache_service.dart';
import 'package:labuda/features/home/domain/entities/feed_item.dart';
import 'package:labuda/features/home/presentation/providers/feed_renderers.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/object/object_preview_provider.dart';
import 'package:labuda/shared/services/logger_service.dart';
import 'package:labuda/shared/widgets/profile_avatar.dart';
import 'package:labuda/shared/widgets/repost_attribution_bar.dart';

class _FakeAuthController extends AuthController {
  @override
  AuthState build() => const AuthStateUnauthenticated();
}

class _NoOpDatasource extends Fake implements UserApiDatasource {}

class _NoOpAvatarCacheService extends AvatarCacheService {
  _NoOpAvatarCacheService() : super(datasource: _NoOpDatasource());

  @override
  Future<String?> getUserAvatarUrl(String userId) async => null;
}

class _FakePresenceManager extends PresenceManager {
  @override
  PresenceAuthorityState build() => const PresenceAuthorityState.empty();
}

class _FakePresenceRegistry extends PresenceSubscriptionRegistry {
  @override
  PresenceSubscriptionHandle acquire(Set<String> userIds) {
    return PresenceSubscriptionHandle(() async {});
  }

  @override
  Future<void> prepareForLogout() async {}

  @override
  PresenceState? lookup(String userId) => null;

  @override
  Map<String, PresenceState?> lookupMany(Iterable<String> userIds) => {};

  @override
  Future<void> publishSelfPresence({required bool isOnline}) async {}

  @override
  Future<void> setForeground(bool isForeground) async {}
}

const _authorId = '00000000-0000-0000-0000-000000000123';
const _authorAvatar = 'https://example.com/avatar.jpg';

FeedItem _feedItem({
  required String id,
  required String content,
  required FeedItemType type,
  ContentAuthor? author,
  Map<String, dynamic> additionalData = const {},
}) {
  return FeedItem(
    id: id,
    content: content,
    author: author,
    type: type,
    createdAt: DateTime.utc(2026, 7, 23, 10, 0),
    additionalData: {
      'title': content,
      'caption': content,
      'status': 'active',
      ...additionalData,
    },
  );
}

ContentAuthor _activeAuthor({
  String id = _authorId,
  String? username = 'alice',
  String? avatarUrl = _authorAvatar,
}) {
  return ContentAuthor(
    id: id,
    username: username,
    avatarUrl: avatarUrl,
    lifecycle: ContentLifecycle.active,
  );
}

ContentAuthor _author({
  String id = _authorId,
  String? username = 'alice',
  String? avatarUrl = _authorAvatar,
  ContentLifecycle lifecycle = ContentLifecycle.active,
}) {
  return ContentAuthor(
    id: id,
    username: username,
    avatarUrl: avatarUrl,
    lifecycle: lifecycle,
  );
}

Widget _wrapWithRouter(Widget child) {
  final router = GoRouter(
    initialLocation: '/',
    routes: [
      GoRoute(
        path: '/',
        builder: (context, state) => Scaffold(body: child),
      ),
      GoRoute(
        path: '/user/:id',
        builder: (_, state) =>
            Scaffold(body: Text('profile:${state.pathParameters['id']}')),
      ),
      GoRoute(
        path: '/content/:id',
        builder: (_, state) =>
            Scaffold(body: Text('content:${state.pathParameters['id']}')),
      ),
      GoRoute(
        path: '/comment/content/:id',
        builder: (_, state) =>
            Scaffold(body: Text('comments:${state.pathParameters['id']}')),
      ),
      GoRoute(
        path: '/chat/new',
        builder: (context, state) => const Scaffold(body: Text('chat')),
      ),
    ],
  );

  return ProviderScope(
    overrides: [
      objectPreviewProvider.overrideWith((ref, reference) async => null),
      authControllerProvider.overrideWith(_FakeAuthController.new),
      avatarCacheServiceProvider.overrideWith(
        (_) => _NoOpAvatarCacheService(),
      ),
      presenceManagerProvider.overrideWith(_FakePresenceManager.new),
      presenceSubscriptionRegistryProvider.overrideWithValue(
        _FakePresenceRegistry(),
      ),
      loggerServiceProvider.overrideWithValue(LoggerService.instance),
    ],
    child: MaterialApp.router(routerConfig: router),
  );
}

Future<void> _openShareSheet(WidgetTester tester) async {
  await tester.tap(find.byIcon(Icons.share_outlined));
  await tester.pumpAndSettle();
}

void main() {
  test(
    'FeedItem exposes one typed author projection and no legacy getters',
    () {
      final source = File(
        'lib/features/home/domain/entities/feed_item.dart',
      ).readAsStringSync();

      expect(source, contains('final ContentAuthor? author;'));
      expect(source, contains('ContentAuthor? author,'));
      expect(source, isNot(contains('authorUsername')));
      expect(source, isNot(contains('authorAvatarUrl')));
      expect(source, isNot(contains('final ContentLifecycle authorLifecycle')));
      expect(source, isNot(contains('this.authorLifecycle')));
    },
  );

  test('Feed mapper uses the typed author projection and no flat identity', () {
    final source = File(
      'lib/features/home/data/mappers/feed_mapper.dart',
    ).readAsStringSync();

    expect(source, contains('author: _mapAuthor(author)'));
    expect(source, contains('ContentAuthor _mapAuthor(FeedAuthorDto? author)'));
    expect(source, isNot(contains('authorId:')));
    expect(source, isNot(contains('authorUsername:')));
    expect(source, isNot(contains('authorAvatarUrl:')));
  });

  test('Feed renderer no longer composes flat author identity strings', () {
    final source = File(
      'lib/features/home/presentation/providers/feed_renderers.dart',
    ).readAsStringSync();

    expect(source, isNot(contains('authorId:')));
    expect(source, isNot(contains('authorUsername:')));
    expect(source, isNot(contains('authorAvatarUrl:')));
    expect(source, isNot(contains(r'@${item.author')));
    expect(source, isNot(contains(r'@${author')));
  });

  testWidgets('active author shows handle and navigates to profile', (
    tester,
  ) async {
    final item = _feedItem(
      id: 'feed-1',
      content: 'hello world',
      type: FeedItemType.content,
      author: _activeAuthor(),
    );

    await tester.pumpWidget(_wrapWithRouter(FeedCard(item: item)));
    await tester.pumpAndSettle();

    final avatar = tester.widget<ProfileAvatar>(find.byType(ProfileAvatar));
    expect(avatar.imageUrl, _authorAvatar);
    expect(avatar.username, 'alice');
    expect(find.text('@alice'), findsOneWidget);

    await tester.tap(find.text('@alice'));
    await tester.pumpAndSettle();

    expect(find.text('profile:$_authorId'), findsOneWidget);
  });

  testWidgets('removed author redacts identity and blocks navigation', (
    tester,
  ) async {
    final item = _feedItem(
      id: 'feed-2',
      content: 'removed author content',
      type: FeedItemType.content,
      author: _author(lifecycle: ContentLifecycle.removed),
    );

    await tester.pumpWidget(_wrapWithRouter(FeedCard(item: item)));
    await tester.pumpAndSettle();

    final avatar = tester.widget<ProfileAvatar>(find.byType(ProfileAvatar));
    expect(avatar.imageUrl, isNull);
    expect(avatar.username, isNull);
    expect(find.text('Pengguna dihapus'), findsOneWidget);
    expect(find.text('@alice'), findsNothing);

    await tester.tap(find.text('Pengguna dihapus'));
    await tester.pumpAndSettle();

    expect(find.text('profile:$_authorId'), findsNothing);
  });

  testWidgets('unavailable author share sheet uses redaction label', (
    tester,
  ) async {
    final item = _feedItem(
      id: 'feed-3',
      content: 'unavailable author content',
      type: FeedItemType.content,
      author: _author(lifecycle: ContentLifecycle.unavailable),
    );

    await tester.pumpWidget(_wrapWithRouter(FeedCard(item: item)));
    await tester.pumpAndSettle();

    await _openShareSheet(tester);

    expect(
      find.descendant(
        of: find.byType(ShareBottomSheet),
        matching: find.text('Pengguna tidak tersedia'),
      ),
      findsOneWidget,
    );
    expect(find.text('@alice'), findsNothing);
  });

  testWidgets('malformed author fails closed and blocks navigation', (
    tester,
  ) async {
    final item = _feedItem(
      id: 'feed-4',
      content: 'malformed author content',
      type: FeedItemType.content,
      author: _author(username: 'user_1234abcd', avatarUrl: _authorAvatar),
    );

    await tester.pumpWidget(_wrapWithRouter(FeedCard(item: item)));
    await tester.pumpAndSettle();

    final avatar = tester.widget<ProfileAvatar>(find.byType(ProfileAvatar));
    expect(avatar.imageUrl, isNull);
    expect(avatar.username, isNull);
    expect(find.text('Pengguna tidak tersedia'), findsOneWidget);
    expect(find.text('@user_1234abcd'), findsNothing);

    await tester.tap(find.text('Pengguna tidak tersedia'));
    await tester.pumpAndSettle();

    expect(find.text('profile:$_authorId'), findsNothing);
  });

  testWidgets('active author share sheet uses current author projection', (
    tester,
  ) async {
    final item = _feedItem(
      id: 'feed-5',
      content: 'shareable content',
      type: FeedItemType.content,
      author: _activeAuthor(),
    );

    await tester.pumpWidget(_wrapWithRouter(FeedCard(item: item)));
    await tester.pumpAndSettle();

    await _openShareSheet(tester);

    expect(
      find.descendant(
        of: find.byType(ShareBottomSheet),
        matching: find.text('@alice'),
      ),
      findsOneWidget,
    );
    expect(
      find.descendant(
        of: find.byType(ShareBottomSheet),
        matching: find.text('shareable content'),
      ),
      findsOneWidget,
    );
  });

  testWidgets(
    'repost keeps original attribution separate from current author',
    (tester) async {
      final item = _feedItem(
        id: 'feed-6',
        content: 'repost content',
        type: FeedItemType.content,
        author: _activeAuthor(username: 'bob'),
        additionalData: {
          'isRepost': true,
          'originalAuthorId': 'author-original',
        },
      );

      await tester.pumpWidget(_wrapWithRouter(FeedCard(item: item)));
      await tester.pumpAndSettle();

      expect(find.byType(RepostAttributionBar), findsOneWidget);
      expect(find.text('Repost dari bob'), findsOneWidget);
      expect(find.text('@bob'), findsOneWidget);

      await _openShareSheet(tester);

      expect(
        find.descendant(
          of: find.byType(ShareBottomSheet),
          matching: find.text('@bob'),
        ),
        findsOneWidget,
      );
    },
  );
}
