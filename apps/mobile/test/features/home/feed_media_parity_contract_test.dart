import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
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
import 'package:labuda/features/home/data/dto/feed_dto.dart';
import 'package:labuda/features/home/data/mappers/feed_mapper.dart';
import 'package:labuda/features/home/domain/entities/feed_item.dart';
import 'package:labuda/features/home/presentation/providers/feed_renderers.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show avatarCacheServiceProvider;
import 'package:labuda/domains/user/profile/data/services/avatar_cache_service.dart';
import 'package:labuda/shared/services/logger_service.dart';
import 'package:labuda/shared/widgets/stable_network_image.dart';

Map<String, dynamic> _feedJson({
  required String id,
  required List<Map<String, dynamic>> media,
}) {
  return <String, dynamic>{
    'id': id,
    'feed_item_kind': 'content',
    'status': 'active',
    'body': 'Feed body for $id',
    'caption': 'Feed body for $id',
    'author': <String, dynamic>{
      'id': '11111111-1111-1111-1111-111111111111',
      'username': 'alice',
      'avatar_url': 'https://example.com/avatar.jpg',
      'lifecycle': 'active',
    },
    'lifecycle': 'active',
    'is_hidden': false,
    'created_at': '2026-07-24T10:00:00.000Z',
    'updated_at': '2026-07-24T10:00:00.000Z',
    'media': media,
  };
}

FeedItem _feedItemWithMedia(List<MediaEntity> media, {String id = 'feed-1'}) {
  return FeedItem(
    id: id,
    content: 'Feed content for $id',
    author: const ContentAuthor(
      id: '11111111-1111-1111-1111-111111111111',
      username: 'alice',
      avatarUrl: 'https://example.com/avatar.jpg',
    ),
    type: FeedItemType.content,
    createdAt: DateTime.utc(2026, 7, 24, 10, 0),
    media: media,
    additionalData: const {
      'title': 'Feed content',
      'caption': 'Feed content',
      'status': 'active',
    },
  );
}

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

Widget _wrap(Widget child) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(_FakeAuthController.new),
      avatarCacheServiceProvider.overrideWith((_) => _NoOpAvatarCacheService()),
      presenceManagerProvider.overrideWith(_FakePresenceManager.new),
      presenceSubscriptionRegistryProvider.overrideWithValue(
        _FakePresenceRegistry(),
      ),
      loggerServiceProvider.overrideWithValue(LoggerService.instance),
    ],
    child: MaterialApp(home: Scaffold(body: child)),
  );
}

void main() {
  test('FeedMediaDto preserves kind, dimensions, and stable identity', () {
    final dto = FeedMediaDto.fromJson(<String, dynamic>{
      'url': 'https://cdn.example.com/content/alpha.jpg',
      'type': 'image',
      'kind': 'image',
      'position': 0,
      'width': 1200,
      'height': 800,
    });

    expect(dto.url, 'https://cdn.example.com/content/alpha.jpg');
    expect(dto.kind, 'image');
    expect(dto.position, 0);
    expect(dto.width, 1200);
    expect(dto.height, 800);

    final entity = dto.toMediaEntity();
    expect(entity.id, 'alpha');
    expect(entity.originalUrl, dto.url);
    expect(entity.type, MediaType.image);
    expect(entity.dimensions?.width, 1200);
    expect(entity.dimensions?.height, 800);
  });

  test('FeedItemMapper preserves ordered mixed media semantics', () {
    final dto = FeedItemDto.fromJson(
      _feedJson(
        id: 'content-ordered',
        media: <Map<String, dynamic>>[
          <String, dynamic>{
            'url': 'https://cdn.example.com/content/alpha.jpg',
            'type': 'image',
            'kind': 'image',
            'position': 0,
            'width': 1200,
            'height': 800,
          },
          <String, dynamic>{
            'url': 'https://cdn.example.com/content/bravo.mp4',
            'type': 'video',
            'kind': 'video',
            'position': 1,
            'width': 1920,
            'height': 1080,
          },
        ],
      ),
    );

    final item = dto.toFeedItem();

    expect(item.media, hasLength(2));
    expect(
      item.media[0].originalUrl,
      'https://cdn.example.com/content/alpha.jpg',
    );
    expect(item.media[0].type, MediaType.image);
    expect(item.media[0].dimensions?.width, 1200);
    expect(item.media[0].dimensions?.height, 800);
    expect(
      item.media[1].originalUrl,
      'https://cdn.example.com/content/bravo.mp4',
    );
    expect(item.media[1].type, MediaType.video);
    expect(item.media[1].dimensions?.width, 1920);
    expect(item.media[1].dimensions?.height, 1080);
  });

  testWidgets('FeedCard renders image media through StableNetworkImage', (
    tester,
  ) async {
    final item = _feedItemWithMedia(<MediaEntity>[
      MediaEntity(
        id: 'alpha',
        originalUrl: 'https://cdn.example.com/content/alpha.jpg',
        type: MediaType.image,
        createdAt: DateTime.fromMillisecondsSinceEpoch(0, isUtc: true),
      ),
    ]);

    await tester.pumpWidget(_wrap(FeedCard(item: item)));
    await tester.pumpAndSettle();

    expect(find.byType(StableNetworkImage), findsOneWidget);
    expect(
      find.descendant(
        of: find.byType(StableNetworkImage),
        matching: find.byType(Image),
      ),
      findsOneWidget,
    );
    expect(find.text('1 / 1'), findsNothing);
  });

  testWidgets('FeedCard renders video-first media as safe fallback', (
    tester,
  ) async {
    final item = _feedItemWithMedia(<MediaEntity>[
      MediaEntity(
        id: 'bravo',
        originalUrl: 'https://cdn.example.com/content/bravo.mp4',
        type: MediaType.video,
        createdAt: DateTime.fromMillisecondsSinceEpoch(0, isUtc: true),
      ),
      MediaEntity(
        id: 'alpha',
        originalUrl: 'https://cdn.example.com/content/alpha.jpg',
        type: MediaType.image,
        createdAt: DateTime.fromMillisecondsSinceEpoch(0, isUtc: true),
      ),
    ], id: 'feed-video-first');

    await tester.pumpWidget(_wrap(FeedCard(item: item)));
    await tester.pumpAndSettle();

    expect(find.byType(StableNetworkImage), findsNothing);
    expect(find.byIcon(Icons.videocam_outlined), findsOneWidget);
    expect(find.byIcon(Icons.play_circle_fill), findsOneWidget);
    expect(find.text('Feed content for feed-video-first'), findsOneWidget);
  });
}
