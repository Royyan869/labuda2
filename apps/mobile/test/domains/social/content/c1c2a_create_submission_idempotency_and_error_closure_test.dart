import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api.dart';
import 'package:labuda/core/api/exceptions/api_exception.dart';
import 'package:labuda/domains/social/content/data/content_repository_impl.dart';
import 'package:labuda/domains/social/content/data/dto/content_dto.dart';
import 'package:labuda/domains/social/content/data/dto/content_request_models.dart';
import 'package:labuda/domains/social/content/data/remote/content_api_datasource.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/domains/social/content/presentation/widgets/create_content/content_submission_handler.dart';

Map<String, dynamic> _contentJson({
  String id = 'content-1',
  String caption = 'hello content',
}) {
  return <String, dynamic>{
    'id': id,
    'caption': caption,
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
    'moderation_info': <String, dynamic>{
      'isApproved': true,
      'hasBeenModerated': false,
      'flagCount': 0,
      'moderatorId': null,
      'moderatedAt': null,
      'moderationNote': null,
      'lastAction': null,
    },
    'published_at': null,
    'created_at': '2026-06-02T00:00:00.000Z',
    'updated_at': '2026-06-02T00:00:00.000Z',
    'is_liked': null,
    'is_saved': null,
    'original_author_id': null,
    'share_reference': null,
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

Map<String, dynamic> _successEnvelope() {
  return <String, dynamic>{
    'success': true,
    'data': _contentJson(),
    'timestamp': '2026-06-02T00:00:00.000Z',
  };
}

Map<String, dynamic> _errorEnvelope({
  required String code,
  required String message,
  Map<String, dynamic>? details,
}) {
  return <String, dynamic>{
    'success': false,
    'error': <String, dynamic>{
      'code': code,
      'message': message,
      if (details != null) 'details': details,
    },
    'timestamp': '2026-06-02T00:00:00.000Z',
  };
}

class _ContentTransportAdapter implements HttpClientAdapter {
  final int statusCode;
  final Map<String, dynamic> body;

  Map<String, dynamic>? lastHeaders;
  dynamic lastData;
  String? lastPath;
  int fetchCount = 0;

  _ContentTransportAdapter({required this.statusCode, required this.body});

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    fetchCount += 1;
    lastHeaders = Map<String, dynamic>.from(options.headers);
    lastData = options.data;
    lastPath = options.path;

    return ResponseBody.fromBytes(
      utf8.encode(jsonEncode(body)),
      statusCode,
      headers: {
        Headers.contentTypeHeader: ['application/json; charset=utf-8'],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

class _RecordingCreateContentDatasource extends ContentApiDatasource {
  String? lastIdempotencyKey;
  ContentCreateRequest? lastRequest;

  _RecordingCreateContentDatasource(super.apiClient);

  @override
  Future<ContentDto> createContentWithRequest(
    ContentCreateRequest request, {
    required String idempotencyKey,
  }) async {
    lastRequest = request;
    lastIdempotencyKey = idempotencyKey;
    return ContentDto.fromJson(
      _contentJson(caption: request.content),
    );
  }
}

class _ThrowingCreateContentDatasource extends ContentApiDatasource {
  _ThrowingCreateContentDatasource(super.apiClient);

  @override
  Future<ContentDto> createContentWithRequest(
    ContentCreateRequest request, {
    required String idempotencyKey,
  }) async {
    throw BadRequestException(
      message: 'Idempotency-Key header required',
      code: 'BAD_REQUEST',
    );
  }
}

ApiClient _clientWithAdapter(_ContentTransportAdapter adapter) {
  final client = ApiClient.testing(baseUrl: 'https://example.com/api/v1');
  client.dio.httpClientAdapter = adapter;
  return client;
}

void main() {
  group('ContentApiDatasource createContent', () {
    test(
      'sends Idempotency-Key and parses a canonical 201 envelope without type',
      () async {
        final adapter = _ContentTransportAdapter(
          statusCode: 201,
          body: _successEnvelope(),
        );
        final client = _clientWithAdapter(adapter);
        final datasource = ContentApiDatasource(client);

        final dto = await datasource.createContent(
          const CreateContentDto(
            content: 'hello',
            visibility: ContentVisibility.public,
          ),
          idempotencyKey: 'create-key',
        );

        final request = adapter.lastData as Map<String, dynamic>;
        expect(adapter.fetchCount, 1);
        expect(adapter.lastPath, '/contents');
        expect(adapter.lastHeaders?['Idempotency-Key'], 'create-key');
        expect(request['caption'], 'hello');
        expect(request.containsKey('type'), isFalse);
        expect(dto.id, 'content-1');
      },
    );

    for (final testCase
        in <({int statusCode, Matcher matcher, String message, String code})>[
          (
            statusCode: 400,
            matcher: isA<BadRequestException>().having(
              (e) => e.code,
              'code',
              'BAD_REQUEST',
            ),
            message: 'Idempotency-Key header required',
            code: 'BAD_REQUEST',
          ),
          (
            statusCode: 401,
            matcher: isA<UnauthorizedException>().having(
              (e) => e.code,
              'code',
              'UNAUTHORIZED',
            ),
            message: 'Authentication required',
            code: 'UNAUTHORIZED',
          ),
          (
            statusCode: 403,
            matcher: isA<ForbiddenException>().having(
              (e) => e.code,
              'code',
              'FORBIDDEN',
            ),
            message: 'Forbidden',
            code: 'FORBIDDEN',
          ),
          (
            statusCode: 422,
            matcher: isA<ValidationException>().having(
              (e) => e.code,
              'code',
              'VALIDATION_ERROR',
            ),
            message: 'Validation failed',
            code: 'VALIDATION_ERROR',
          ),
        ]) {
      test(
        'maps HTTP ${testCase.statusCode} error envelopes to canonical failures without parsing ContentDto',
        () async {
          final adapter = _ContentTransportAdapter(
            statusCode: testCase.statusCode,
            body: _errorEnvelope(
              code: testCase.code,
              message: testCase.message,
              details: testCase.statusCode == 422
                  ? <String, dynamic>{
                      'field_errors': <String, dynamic>{
                        'caption': <String>['required'],
                      },
                    }
                  : null,
            ),
          );
          final client = _clientWithAdapter(adapter);
          final datasource = ContentApiDatasource(client);

          await expectLater(
            datasource.createContent(
              const CreateContentDto(content: 'hello'),
              idempotencyKey: 'error-key-${testCase.statusCode}',
            ),
            throwsA(testCase.matcher),
          );

          expect(adapter.fetchCount, 1);
          expect(
            adapter.lastHeaders?['Idempotency-Key'],
            'error-key-${testCase.statusCode}',
          );
          expect(adapter.lastPath, '/contents');
        },
      );
    }

    test(
      'fails clearly when a 2xx response is missing the data object',
      () async {
        final adapter = _ContentTransportAdapter(
          statusCode: 201,
          body: <String, dynamic>{
            'success': true,
            'timestamp': '2026-06-02T00:00:00.000Z',
          },
        );
        final client = _clientWithAdapter(adapter);
        final datasource = ContentApiDatasource(client);

        await expectLater(
          datasource.createContent(
            const CreateContentDto(content: 'hello'),
            idempotencyKey: 'malformed-key',
          ),
          throwsA(
            isA<UnknownApiException>().having(
              (e) => e.code,
              'code',
              'MALFORMED_RESPONSE',
            ),
          ),
        );

        expect(adapter.fetchCount, 1);
      },
    );
  });

  group('ContentRepositoryImpl createContent', () {
    test('passes the same idempotency key through to the datasource', () async {
      final client = ApiClient.testing(baseUrl: 'https://example.com/api/v1');
      final datasource = _RecordingCreateContentDatasource(client);
      final repository = ContentRepositoryImpl(datasource, client);

      final result = await repository.createContent(
        authorId: 'author-1',
        authorUsername: 'author',
        authorAvatarUrl: null,
        content: 'hello repo',
        idempotencyKey: 'repo-key-1',
      );

      expect(datasource.lastIdempotencyKey, 'repo-key-1');
      expect(datasource.lastRequest?.content, 'hello repo');
      expect(result.isSuccess, isTrue);
      result.fold((error) => fail('unexpected error: $error'), (content) {
        expect(content.content, 'hello repo');
      });
    });

    test('surfaces canonical API failures from the datasource', () async {
      final client = ApiClient.testing(baseUrl: 'https://example.com/api/v1');
      final datasource = _ThrowingCreateContentDatasource(client);
      final repository = ContentRepositoryImpl(datasource, client);

      final result = await repository.createContent(
        authorId: 'author-1',
        authorUsername: 'author',
        authorAvatarUrl: null,
        content: 'hello repo',
        idempotencyKey: 'repo-key-2',
      );

      expect(result.isSuccess, isFalse);
      expect(result.error, contains('Idempotency-Key header required'));
      expect(result.error, contains('BAD_REQUEST'));
    });
  });

  group('Submission key lifecycle', () {
    test('separate logical submissions receive different keys', () {
      final first = ContentSubmissionHandler.createIdempotencyKey();
      final second = ContentSubmissionHandler.createIdempotencyKey();

      expect(first, isNotEmpty);
      expect(second, isNotEmpty);
      expect(first, isNot(equals(second)));
    });
  });
}
