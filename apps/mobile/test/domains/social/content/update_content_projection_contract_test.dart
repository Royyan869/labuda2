import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/content/data/content_providers.dart';
import 'package:labuda/domains/social/content/data/content_repository_impl.dart';
import 'package:labuda/domains/social/content/data/dto/content_dto.dart';
import 'package:labuda/domains/social/content/data/dto/content_request_models.dart';
import 'package:labuda/domains/social/content/data/mappers/content_mapper.dart';
import 'package:labuda/domains/social/content/data/remote/content_api_datasource.dart';
import 'package:labuda/domains/social/content/domain/entities/content_resource_projection.dart';
import 'package:labuda/domains/social/content/presentation/providers/content_notifier.dart';
import 'package:labuda/domains/social/content/presentation/providers/content_state.dart';

class _NoopApiClient implements ApiClient {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

Map<String, dynamic> _profileProjectionJson({
  String state = 'LIVE',
  String resourceId = 'profile-77',
  String username = 'canonical-alice',
}) {
  final json = <String, dynamic>{
    'state': state,
    'resource_type': 'profile',
    'resource_id': resourceId,
  };
  if (state == 'LIVE') {
    json['profile'] = <String, dynamic>{
      'username': username,
      'avatar_url': 'https://example.com/profile.jpg',
      'lifecycle': 'active',
    };
  }
  return json;
}

Map<String, dynamic> _contentJson({
  required String id,
  required String caption,
  required Map<String, dynamic> resourceProjection,
}) {
  return <String, dynamic>{
    'id': id,
    'caption': caption,
    'author_id': 'author-1',
    'author_username': 'author',
    'author_city': null,
    'author_province': null,
    'type': 'post',
    'status': 'active',
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
    'resource_projection': resourceProjection,
    'card': <String, dynamic>{
      'id': id,
      'author': <String, dynamic>{
        'id': 'author-1',
        'username': 'author',
        'avatar_url': null,
        'lifecycle': 'active',
      },
    },
  };
}

class _RecordingUpdateDatasource extends ContentApiDatasource {
  _RecordingUpdateDatasource(this.response) : super(_NoopApiClient());

  final ContentDto response;
  UpdateContentDto? lastRequest;
  String? lastContentId;

  @override
  Future<ContentDto> updateContent(
    String contentId,
    UpdateContentDto request,
  ) async {
    lastContentId = contentId;
    lastRequest = request;
    return response;
  }
}

void main() {
  test(
    'update response preserves canonical projection through repository and notifier state',
    () async {
      final projectionJson = _profileProjectionJson();

      final currentContent = ContentMapper.toEntity(
        ContentDto.fromJson(
          _contentJson(
            id: 'content-77',
            caption: 'before update',
            resourceProjection: projectionJson,
          ),
        ),
      );

      final responseDto = ContentDto.fromJson(
        _contentJson(
          id: 'content-77',
          caption: 'after update',
          resourceProjection: projectionJson,
        ),
      );

      final datasource = _RecordingUpdateDatasource(responseDto);
      final repository = ContentRepositoryImpl(datasource, _NoopApiClient());
      final container = ProviderContainer(
        overrides: [
          contentRepositoryProvider.overrideWithValue(repository),
        ],
      );

      try {
        final notifier = container.read(contentActionsProvider.notifier);
        final result = await notifier.updateContent(
          currentContent.id,
          currentContent.copyWith(content: 'after update'),
        );

        expect(datasource.lastContentId, 'content-77');
        expect(datasource.lastRequest, isNotNull);

        final requestJson = jsonDecode(
          jsonEncode(datasource.lastRequest!.toJson()),
        ) as Map<String, dynamic>;
        expect(requestJson['caption'], 'after update');
        expect(requestJson.containsKey('share_reference'), isFalse);
        expect(requestJson.containsKey('resource_occurrence'), isFalse);
        expect(requestJson.containsKey('targetType'), isFalse);
        expect(requestJson.containsKey('targetId'), isFalse);
        expect(requestJson.containsKey('preview'), isFalse);

        final updated = result.dataOrThrow;
        expect(updated.resourceProjection, isNotNull);
        expect(
          updated.resourceProjection!.resourceType,
          ContentResourceProjectionType.profile,
        );
        expect(updated.resourceProjection!.state, ContentResourceProjectionState.live);
        expect(updated.resourceProjection!.canonicalPath, '/user/profile-77');
        expect(updated.resourceProjection!.titleText, '@canonical-alice');

        final state = container.read(contentActionsProvider);
        state.maybeMap(
          success: (success) {
            expect(success.content.resourceProjection, isNotNull);
            expect(
              success.content.resourceProjection!.canonicalPath,
              '/user/profile-77',
            );
          },
          orElse: () => fail('expected ContentFormState.success'),
        );
      } finally {
        container.dispose();
      }
    },
  );
}
