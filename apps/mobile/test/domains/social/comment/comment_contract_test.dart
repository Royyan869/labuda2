// ignore_for_file: use_null_aware_elements

import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/social/comment/data/dto/comment_dto.dart';
import 'package:labuda/domains/social/comment/data/mappers/comment_mapper.dart';
import 'package:labuda/domains/social/comment/domain/entities/comment.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

Map<String, dynamic> _commentJson({
  required String id,
  required String contentId,
  String? parentId,
  String type = 'normal',
  String authorId = 'author-1',
  String authorUsername = 'alice',
  String? authorAvatarUrl,
  String? authorLifecycle = 'active',
  String? body = 'hello',
}) {
  return <String, dynamic>{
    'id': id,
    // C-IDENT — canonical wire key is `target_id` on the backend
    // CommentResponse; CommentDto maps it to local contentId.
    'target_id': contentId,
    'author_id': authorId,
    'author_username': authorUsername,
    if (authorAvatarUrl != null) 'author_avatar_url': authorAvatarUrl,
    if (authorLifecycle != null) 'author_lifecycle': authorLifecycle,
    'author': <String, dynamic>{
      if (authorLifecycle != null) 'lifecycle': authorLifecycle,
    },
    if (body != null) 'body': body,
    'type': type,
    if (parentId != null) 'parent_id': parentId,
    'created_at': '2026-07-23T00:00:00.000Z',
    'updated_at': '2026-07-23T00:00:00.000Z',
  };
}

String _normalize(String path) =>
    File(path).readAsStringSync().replaceAll('\r\n', '\n');

void main() {
  group('CommentDto contract', () {
    test('parses the canonical flat author response for root comments', () {
      final dto = CommentDto.fromJson(
        _commentJson(id: 'comment-1', contentId: 'content-1'),
      );

      expect(dto.id, 'comment-1');
      expect(dto.contentId, 'content-1');
      expect(dto.authorId, 'author-1');
      expect(dto.authorUsername, 'alice');
      expect(dto.isTopLevel, isTrue);
      expect(dto.isReply, isFalse);
    });

    test('parses the canonical flat author response for replies', () {
      final dto = CommentDto.fromJson(
        _commentJson(
          id: 'reply-1',
          contentId: 'content-1',
          parentId: 'comment-1',
        ),
      );

      expect(dto.parentId, 'comment-1');
      expect(dto.isReply, isTrue);
      expect(dto.authorUsername, 'alice');
    });

    test('fails closed when required flat author fields are missing', () {
      expect(
        () => CommentDto.fromJson(<String, dynamic>{
          'id': 'comment-1',
          'target_id': 'content-1',
          'author_username': 'alice',
          'body': 'hello',
          'type': 'normal',
          'created_at': '2026-07-23T00:00:00.000Z',
        }),
        throwsA(isA<TypeError>()),
      );
    });

    test('create and reply requests use the canonical JSON keys', () {
      const request = CreateCommentDto(
        targetId: 'content-1',
        targetType: 'content',
        content: 'hello',
        parentId: 'comment-1',
      );
      final requestJson = request.toJson();

      expect(requestJson, containsPair('target_id', 'content-1'));
      expect(requestJson, containsPair('target_type', 'content'));
      expect(requestJson, containsPair('body', 'hello'));
      expect(requestJson, containsPair('parent_id', 'comment-1'));
      expect(requestJson.containsKey('content_id'), isFalse);

      const commerceRequest = CreateCommerceReferenceCommentDto(
        resourceReference: ResourceReferenceRequest(
          resourceType: 'for_sale',
          resourceId: 'listing-1',
        ),
        body: 'seller response',
      );
      final listingJson = commerceRequest.toJson();

      final resourceRef = listingJson['resource_reference'] as Map<String, dynamic>;
      expect(resourceRef, containsPair('resource_id', 'listing-1'));
      expect(resourceRef, containsPair('resource_type', 'for_sale'));
      expect(listingJson, containsPair('body', 'seller response'));
      expect(listingJson.containsKey('share_reference'), isFalse);
      expect(listingJson.containsKey('content_id'), isFalse);
    });

    test('empty pagination metadata parses safely', () {
      final dto = ListCommentsDto.fromJson(<String, dynamic>{
        'comments': const [],
        'limit': 20,
      });

      expect(dto.comments, isEmpty);
      expect(dto.nextCursor, isNull);
      expect(dto.limit, 20);
    });
  });

  group('CommentMapper contract', () {
    test('root and reply map from the same flat-author projection', () {
      final root = CommentMapper.toEntity(
        CommentDto.fromJson(
          _commentJson(id: 'comment-1', contentId: 'content-1'),
        ),
      );
      final reply = CommentMapper.toEntity(
        CommentDto.fromJson(
          _commentJson(
            id: 'reply-1',
            contentId: 'content-1',
            parentId: 'comment-1',
          ),
        ),
      );

      expect(root.authorId, 'author-1');
      expect(root.authorUsername, 'alice');
      expect(root.authorLifecycle, ContentLifecycle.active);
      expect(reply.authorId, 'author-1');
      expect(reply.authorUsername, 'alice');
      expect(reply.authorLifecycle, ContentLifecycle.active);
    });

    test('removed author stays redacted through the canonical mapper', () {
      final entity = CommentMapper.toEntity(
        CommentDto.fromJson(
          _commentJson(
            id: 'comment-2',
            contentId: 'content-1',
            authorUsername: 'alice',
            authorAvatarUrl: 'https://cdn.example/avatar.png',
            authorLifecycle: 'removed',
          ),
        ),
      );

      expect(entity.authorLifecycle, ContentLifecycle.removed);
      expect(entity.authorUsername, 'alice');
      expect(entity.authorAvatarUrl, 'https://cdn.example/avatar.png');
    });
  });

  group('Comment source contract', () {
    test('CommentDto source keeps the canonical flat author fields', () {
      final source = _normalize(
        'lib/domains/social/comment/data/dto/comment_dto.dart',
      );

      for (final needle in [
        'authorUsername',
        'authorAvatarUrl',
        'authorLifecycle',
        'contentId',
        'CreateCommerceReferenceCommentDto',
      ]) {
        expect(source.contains(needle), isTrue, reason: needle);
      }
    });

    test('comment models only expose commerce reference helpers', () {
      final entitySource = _normalize(
        'lib/domains/social/comment/domain/entities/comment.dart',
      );
      final dtoSource = _normalize(
        'lib/domains/social/comment/data/dto/comment_dto.dart',
      );

      expect(entitySource.contains('isCommerceReference'), isTrue);
      expect(entitySource.contains("commerceReference('commerce_reference')"), isTrue);
      expect(dtoSource.contains('isCommerceReference'), isTrue);

      for (final source in [entitySource, dtoSource]) {
        expect(source.contains('isListingReference'), isFalse);
        expect(source.contains("listingReference('listing_reference')"), isFalse);
      }
    });

    test('comment API uses the canonical commerce reference endpoint', () {
      final datasource = _normalize(
        'lib/domains/social/comment/data/remote/comment_api_datasource.dart',
      );
      final repository = _normalize(
        'lib/domains/social/comment/data/comment_repository_impl.dart',
      );

      expect(datasource.contains('/comments/reference'), isTrue);
      expect(datasource.contains('createCommerceReferenceComment'), isTrue);
      expect(repository.contains('createCommerceReferenceComment'), isTrue);
    });

    test('DiscussionScreen uses the commerce composer path', () {
      final source = _normalize(
        'lib/domains/social/comment/presentation/screens/discussion_screen.dart',
      );

      expect(source.contains('CommentInputWithCommerceReference'), isTrue);
      expect(source.contains('createCommerceReferenceComment'), isTrue);
    });

    test('repository no longer walks pages to count comments', () {
      final source = _normalize(
        'lib/domains/social/comment/data/comment_repository_impl.dart',
      );

      expect(source.contains('_countCommentsByCursor'), isFalse);
      expect(source.contains('while (true)'), isFalse);
    });
  });

  group('Comment cursor contract', () {
    test('CommentDto reads contentId from the canonical target_id key', () {
      final dto = CommentDto.fromJson(<String, dynamic>{
        'id': 'comment-1',
        'target_id': 'content-9',
        'author_id': 'author-1',
        'author_username': 'alice',
        'body': 'hello',
        'type': 'normal',
        'created_at': '2026-07-23T00:00:00.000Z',
      });

      expect(dto.contentId, 'content-9');
    });

    test('CommentPage surfaces the backend next_cursor for pagination', () {
      final dto = ListCommentsDto.fromJson(<String, dynamic>{
        'comments': [
          {
            'id': 'c1',
            'target_id': 'content-1',
            'author_id': 'a1',
            'author_username': 'alice',
            'body': 'x',
            'type': 'normal',
            'created_at': '2026-07-23T00:00:00.000Z',
          },
        ],
        'limit': 20,
        'next_cursor': '2026-07-23T00:00:01.000Z',
      });

      final page = CommentPage(
        comments: CommentMapper.toEntityList(dto.comments),
        nextCursor: dto.nextCursor,
      );
      expect(page.comments, hasLength(1));
      expect(page.comments.first.id, 'c1');
      expect(page.nextCursor, '2026-07-23T00:00:01.000Z');
    });

    test('repository consumes the canonical cursor endpoint', () {
      final repo = _normalize(
        'lib/domains/social/comment/data/comment_repository_impl.dart',
      );
      final datasource = _normalize(
        'lib/domains/social/comment/data/remote/comment_api_datasource.dart',
      );
      final notifier = _normalize(
        'lib/domains/social/comment/presentation/providers/comment_notifier.dart',
      );

      // C-CURSOR: getComments walks GET /contents/:id/comments via cursor.
      expect(repo.contains('listContentComments'), isTrue);
      expect(repo.contains('cursor'), isTrue);
      expect(datasource.contains('\$contentId/comments'), isTrue);
      expect(notifier.contains('nextCursor'), isTrue);
      // DTO identity: single canonical wire key target_id.
      expect(
        _normalize('lib/domains/social/comment/data/dto/comment_dto.dart'),
        contains("json['target_id']"),
      );
    });

    test('notifier appends new comments in ASC tail order, never prepends', () {
      final notifier = _normalize(
        'lib/domains/social/comment/presentation/providers/comment_notifier.dart',
      );

      expect(notifier.contains('[...state.comments, newComment]'), isTrue);
      expect(
        notifier.contains('[newComment, ...state.comments]'),
        isFalse,
        reason: 'C-ORDER: newest comment must append at the tail (ASC), not the top',
      );
    });
  });
}
