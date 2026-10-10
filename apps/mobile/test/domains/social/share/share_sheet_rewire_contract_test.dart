import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/social/share/data/datasources/share_api_datasource.dart';
import 'package:hishumi/domains/social/share/data/remote/native_share_service.dart';
import 'package:hishumi/domains/social/share/data/repositories/share_repository_api.dart';
import 'package:hishumi/domains/social/share/domain/entities/share_destination.dart';
import 'package:hishumi/domains/social/share/domain/entities/share_target.dart';

class _RecordingApiClient implements ApiClient {
  String? lastPostPath;
  dynamic lastPostData;
  dynamic postPayload = <String, dynamic>{
    'data': <String, dynamic>{'id': 'ok'},
  };

  @override
  Future<Response<T>> post<T>(
    String path, {
    data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    lastPostPath = path;
    lastPostData = data;
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      data: postPayload as T,
      statusCode: 200,
    );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _StubNativeShareService implements NativeShareService {
  String? lastCopiedText;

  @override
  Future<Result<bool>> copyToClipboard({required String text}) async {
    lastCopiedText = text;
    return Result.success(true);
  }

  @override
  Future<Result<bool>> shareViaDialog({
    required String text,
    String? subject,
  }) async => Result.success(true);

  @override
  Future<Result<bool>> shareViaEmail({
    required String subject,
    required String body,
  }) async => Result.success(true);

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _RecordingShareApiDatasource extends ShareApiDatasource {
  String? lastCall;

  _RecordingShareApiDatasource() : super(_RecordingApiClient());

  @override
  Future<Map<String, dynamic>> createRepost({
    required String originalContentId,
    required String authorId,
    String? caption,
    String? originalAuthorId,
    String? originalContentTitle,
    String? originalContentImageURL,
    String? targetType,
    String? targetId,
  }) async {
    lastCall = 'createRepost';
    return <String, dynamic>{'id': 'repost-id'};
  }
}

void main() {
  group('share sheet rewire contract', () {
    test(
      'createRepost sends target_type and target_id for non-content shares',
      () async {
        final client = _RecordingApiClient();
        final datasource = ShareApiDatasource(client);

        final cases = <ShareTarget>[
          ShareTarget(
            id: 'forSale-1',
            type: ExternalShareType.forSale,
            title: 'ForSale title',
            description: 'ForSale description',
            imageUrl: 'https://img.example/forSale.jpg',
          ),
          ShareTarget(
            id: 'auction-1',
            type: ExternalShareType.auction,
            title: 'Auction title',
            description: 'Auction description',
            imageUrl: 'https://img.example/auction.jpg',
          ),
          ShareTarget(
            id: 'profile-1',
            type: ExternalShareType.profile,
            title: 'Profile title',
            description: 'Profile description',
            imageUrl: 'https://img.example/profile.jpg',
          ),
        ];

        for (final target in cases) {
          await datasource.createRepost(
            originalContentId: target.id,
            authorId: 'user-1',
            caption: 'Shared caption',
            originalContentTitle: target.title,
            originalContentImageURL: target.imageUrl,
            targetType: target.type.name == 'forSale'
                ? 'for_sale'
                : target.type.name,
            targetId: target.id,
          );

          expect(client.lastPostPath, '/contents/${target.id}/repost');
          final body = client.lastPostData as Map<String, dynamic>;
          expect(body['caption'], 'Shared caption');
          expect(body['target_type'], isNotNull);
          expect(body['target_id'], target.id);
        }
      },
    );

    test('shareAsPost routes ALL share types through createRepost', () async {
      final datasource = _RecordingShareApiDatasource();
      final repo = ShareRepositoryApi(
        datasource: datasource,
        nativeShareService: _StubNativeShareService(),
      );

      final cases = <ExternalShareType>[
        ExternalShareType.post,
        ExternalShareType.request,
        ExternalShareType.forSale,
        ExternalShareType.auction,
        ExternalShareType.profile,
      ];

      for (final type in cases) {
        datasource.lastCall = null;
        final result = await repo.shareAsPost(
          target: ShareTarget(
            id: '${type.name}-1',
            type: type,
            title: '${type.name} title',
            description: '${type.name} description',
            imageUrl: 'https://img.example/${type.name}.jpg',
          ),
          authorId: 'author-1',
          caption: 'Caption',
        );

        result.fold(
          (error) => fail('unexpected failure: $error'),
          (value) => expect(value, 'repost-id'),
        );

        expect(datasource.lastCall, 'createRepost');
      }
    });

    test('sendToChat remains unimplemented', () async {
      final repo = ShareRepositoryApi(
        datasource: _RecordingShareApiDatasource(),
        nativeShareService: _StubNativeShareService(),
      );

      final result = await repo.sendToChat(
        target: ShareTarget(
          id: 'profile-1',
          type: ExternalShareType.profile,
          title: 'Profile title',
          description: 'Profile description',
          imageUrl: 'https://img.example/profile.jpg',
        ),
        recipientUserId: 'recipient-1',
        message: 'hello',
      );

      result.fold((error) => fail('unexpected failure: $error'), (
        shareResult,
      ) {
        expect(shareResult.success, isFalse);
        expect(shareResult.destination, ShareDestinationType.sendToChat);
        expect(shareResult.error, 'Coming soon');
      });
    });

    test('copyLink for profile shares emits the public profile URL', () async {
      final nativeShareService = _StubNativeShareService();
      final repo = ShareRepositoryApi(
        datasource: _RecordingShareApiDatasource(),
        nativeShareService: nativeShareService,
      );

      final result = await repo.shareViaExternal(
        target: ShareTarget(
          id: 'profile-1',
          type: ExternalShareType.profile,
          title: 'Profile title',
          description: 'Profile description',
        ),
        destination: ShareDestinationType.copyLink,
      );

      result.fold((error) => fail('unexpected failure: $error'), (
        shareResult,
      ) {
        expect(shareResult.success, isTrue);
        expect(shareResult.destination, ShareDestinationType.copyLink);
      });

      expect(
        nativeShareService.lastCopiedText,
        equals('$kPublicBaseUrl/profile/profile-1'),
      );
    });
  });
}
