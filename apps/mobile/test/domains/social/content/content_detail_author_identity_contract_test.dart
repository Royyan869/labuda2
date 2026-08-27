import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/content/data/dto/content_dto.dart';
import 'package:labuda/domains/social/content/data/mappers/content_mapper.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/domains/social/content/presentation/widgets/content_author_identity.dart';
import 'package:labuda/domains/user/profile/data/datasources/user_api_datasource.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show avatarCacheServiceProvider;
import 'package:labuda/domains/user/profile/data/services/avatar_cache_service.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/widgets/profile_avatar.dart';

Map<String, dynamic> _contentJson({
  required String authorId,
  required String authorUsername,
  required String authorAvatar,
  required String authorLifecycle,
  Map<String, dynamic>? cardAuthor,
}) {
  return <String, dynamic>{
    'id': 'content-1',
    'caption': 'hello identity',
    'author_city': null,
    'author_province': null,
    'lifecycle': 'active',
    'visibility': 'public',
    'media': <Map<String, dynamic>>[],
    'tags': <String>[],
    'location': null,
    'engagement': <String, dynamic>{
      'viewCount': 0,
      'likeCount': 0,
      'commentCount': 0,
      'shareCount': 0,
      'saveCount': 0,
      'reportCount': 0,
    },
    'moderation_info': null,
    'published_at': null,
    'created_at': '2026-07-23T00:00:00.000Z',
    'updated_at': '2026-07-23T00:00:00.000Z',
    'is_liked': null,
    'is_saved': null,
    'original_author_id': null,
    'share_reference': null,
    'card': <String, dynamic>{
      'id': 'content-card-1',
      'author': <String, dynamic>{
        'id': authorId,
        'username': authorUsername,
        'avatar_url': authorAvatar,
        'lifecycle': authorLifecycle,
        if (cardAuthor != null) ...cardAuthor,
      },
    },
  };
}

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _NoOpUserApiDatasource extends Fake implements UserApiDatasource {}

class _NoOpAvatarCacheService extends AvatarCacheService {
  _NoOpAvatarCacheService() : super(datasource: _NoOpUserApiDatasource());

  @override
  Future<String?> getUserAvatarUrl(String userId) async => null;
}

class _NoOpPresenceRegistry extends PresenceSubscriptionRegistry {
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

Widget _wrap(Widget child, {required String userId}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(
        () => _FakeAuthController(const AuthState.unauthenticated()),
      ),
      avatarCacheServiceProvider.overrideWithValue(_NoOpAvatarCacheService()),
      presenceSubscriptionRegistryProvider.overrideWithValue(
        _NoOpPresenceRegistry(),
      ),
      userOnlineStatusProvider(userId).overrideWithValue(false),
    ],
    child: MaterialApp(
      home: Scaffold(body: Center(child: child)),
    ),
  );
}

ContentAuthor _author({
  required String id,
  required String username,
  String? avatarUrl,
  ContentLifecycle lifecycle = ContentLifecycle.active,
}) {
  return ContentAuthor(
    id: id,
    username: username,
    avatarUrl: avatarUrl,
    lifecycle: lifecycle,
  );
}

void main() {
  const canonicalId = '00000000-0000-0000-0000-000000000123';

  test('ContentDto and mapper prefer the embedded card.author projection', () {
    final dto = ContentDto.fromJson(
      _contentJson(
        authorId: 'flat-author-id',
        authorUsername: 'flat_author',
        authorAvatar: 'https://example.com/flat.png',
        authorLifecycle: 'active',
        cardAuthor: <String, dynamic>{
          'id': canonicalId,
          'username': 'alice',
          'avatar_url': 'https://example.com/canonical.png',
        },
      ),
    );

    expect(dto.author, isNotNull);
    expect(dto.author!.id, canonicalId);
    expect(dto.author!.username, 'alice');
    expect(dto.author!.avatarUrl, 'https://example.com/canonical.png');

    final entity = ContentMapper.toEntity(dto);
    expect(entity.author.id, canonicalId);
    expect(entity.author.username, 'alice');
    expect(entity.author.avatarUrl, 'https://example.com/canonical.png');
    expect(entity.author.lifecycle, ContentLifecycle.active);
    expect(entity.author.canOpenProfile, isTrue);
  });

  testWidgets('active author shows canonical handle and allows navigation', (
    tester,
  ) async {
    var tapped = false;

    await tester.pumpWidget(
      _wrap(
        ContentAuthorIdentity(
          author: _author(id: canonicalId, username: 'alice'),
          onTap: () => tapped = true,
        ),
        userId: canonicalId,
      ),
    );

    expect(find.text('@alice'), findsOneWidget);
    expect(find.byType(ProfileAvatar), findsOneWidget);

    await tester.tap(find.byType(ContentAuthorIdentity));
    await tester.pump();

    expect(tapped, isTrue);
  });

  testWidgets('unavailable author redacts and blocks navigation', (
    tester,
  ) async {
    var tapped = false;

    await tester.pumpWidget(
      _wrap(
        ContentAuthorIdentity(
          author: _author(
            id: canonicalId,
            username: 'alice',
            lifecycle: ContentLifecycle.unavailable,
          ),
          onTap: () => tapped = true,
        ),
        userId: canonicalId,
      ),
    );

    expect(find.text('Pengguna tidak tersedia'), findsOneWidget);
    expect(find.text('@alice'), findsNothing);

    await tester.tap(find.byType(ContentAuthorIdentity));
    await tester.pump();

    expect(tapped, isFalse);
  });

  testWidgets('malformed author redacts and blocks navigation', (tester) async {
    var tapped = false;

    await tester.pumpWidget(
      _wrap(
        ContentAuthorIdentity(
          author: _author(id: canonicalId, username: 'user_1234abcd'),
          onTap: () => tapped = true,
        ),
        userId: canonicalId,
      ),
    );

    expect(find.text('Pengguna tidak tersedia'), findsOneWidget);
    expect(find.text('@user_1234abcd'), findsNothing);

    await tester.tap(find.byType(ContentAuthorIdentity));
    await tester.pump();

    expect(tapped, isFalse);
  });

  testWidgets('refresh rebuild updates active author into a redacted one', (
    tester,
  ) async {
    var tapped = false;

    await tester.pumpWidget(
      _wrap(
        ContentAuthorIdentity(
          author: _author(id: canonicalId, username: 'alice'),
          onTap: () => tapped = true,
        ),
        userId: canonicalId,
      ),
    );
    expect(find.text('@alice'), findsOneWidget);

    await tester.pumpWidget(
      _wrap(
        ContentAuthorIdentity(
          author: _author(
            id: canonicalId,
            username: 'alice',
            lifecycle: ContentLifecycle.removed,
          ),
          onTap: () => tapped = true,
        ),
        userId: canonicalId,
      ),
    );
    await tester.pump();

    expect(find.text('Pengguna dihapus'), findsOneWidget);
    expect(find.text('@alice'), findsNothing);

    await tester.tap(find.byType(ContentAuthorIdentity));
    await tester.pump();

    expect(tapped, isFalse);
  });

  test('content detail screen no longer reads flat author identity fields', () {
    final source = File(
      'lib/domains/social/content/presentation/screens/content_detail_screen.dart',
    ).readAsStringSync();

    expect(source, isNot(contains('content.authorUsername')));
    expect(source, isNot(contains('content.authorAvatarUrl')));
    expect(source, isNot(contains('content.authorId')));
    expect(source, contains('author.id'));
  });
}
