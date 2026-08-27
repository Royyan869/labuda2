import 'package:dio/dio.dart';
import 'package:dartz/dartz.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/domains/social/share/data/datasources/share_api_datasource.dart';
import 'package:labuda/domains/social/share/data/remote/native_share_service.dart';
import 'package:labuda/domains/social/share/data/repositories/share_repository_api.dart';
import 'package:labuda/domains/social/share/domain/entities/share_failure.dart';
import 'package:labuda/domains/social/share/domain/entities/share_destination.dart';
import 'package:labuda/domains/social/share/domain/entities/share_target.dart';

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
  Future<Either<ShareFailure, bool>> copyToClipboard({
    required String text,
  }) async {
    lastCopiedText = text;
    return Right(true);
  }

  @override
  Future<Either<ShareFailure, bool>> shareToInstagramStory({
    required String imageUrl,
  }) async {
    return Left(ShareFailure.unknown('unused'));
  }

  @override
  Future<Either<ShareFailure, bool>> shareToTelegram({
    required String text,
  }) async {
    return Left(ShareFailure.unknown('unused'));
  }

  @override
  Future<Either<ShareFailure, bool>> shareToWhatsApp({
    required String text,
  }) async {
    return Left(ShareFailure.unknown('unused'));
  }

  @override
  Future<Either<ShareFailure, bool>> shareViaDialog({
    required String text,
    String? subject,
  }) async {
    return Left(ShareFailure.unknown('unused'));
  }

  @override
  Future<Either<ShareFailure, bool>> shareViaEmail({
    required String subject,
    required String body,
  }) async {
    return Left(ShareFailure.unknown('unused'));
  }
}

class _RecordingShareApiDatasource extends ShareApiDatasource {
  String? lastCall;

  _RecordingShareApiDatasource() : super(_RecordingApiClient());

  @override
  Future<Map<String, dynamic>> createRepost({
    required String originalContentId,
    required String authorId,
    String? caption,
    required String originalAuthorId,
    required String originalContentTitle,
    String? originalContentImageURL,
  }) async {
    lastCall = 'createRepost';
    return <String, dynamic>{'id': 'repost-id'};
  }

  @override
  Future<Map<String, dynamic>> createShareReferencePost({
    required String authorId,
    String? authorUsername,
    String? authorAvatarUrl,
    required String content,
    required ShareTarget target,
    List<String> mediaUrls = const [],
  }) async {
    lastCall = 'createShareReferencePost';
    return <String, dynamic>{'id': 'shared-id'};
  }
}

void main() {
  group('share sheet rewire contract', () {
    test(
      'createShareReferencePost rewires non-content shares to canonical create-content route',
      () async {
        final client = _RecordingApiClient();
        final datasource = ShareApiDatasource(client);

        final cases = <ShareTarget>[
          ShareTarget(
            id: 'listing-1',
            type: ExternalShareType.listing,
            title: 'Listing title',
            description: 'Listing description',
            imageUrl: 'https://img.example/listing.jpg',
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
          await datasource.createShareReferencePost(
            authorId: 'user-1',
            content: 'Shared caption',
            target: target,
          );

          expect(client.lastPostPath, '/contents');
          final body = client.lastPostData as Map<String, dynamic>;
          expect(body['caption'], 'Shared caption');
          expect(body['type'], 'post');
          expect(body['visibility'], 'public');
          expect(body['allow_comments'], isTrue);

          final shareReference =
              body['share_reference'] as Map<String, dynamic>;
          expect(shareReference['targetType'], target.type.name);
          expect(shareReference['targetId'], target.id);

          final preview = shareReference['preview'] as Map<String, dynamic>;
          expect(preview['title'], target.title);
          expect(preview['imageUrl'], target.imageUrl);
          expect(preview['isAvailable'], isTrue);
          expect(preview['isSold'], isFalse);
          expect(preview['isClosed'], isFalse);
          expect(preview['isDeleted'], isFalse);
        }
      },
    );

    test('shareAsPost keeps post/request on canonical repost writer', () async {
      final datasource = _RecordingShareApiDatasource();
      final repo = ShareRepositoryApi(
        datasource: datasource,
        nativeShareService: _StubNativeShareService(),
      );

      final cases = <ExternalShareType>[
        ExternalShareType.post,
        ExternalShareType.request,
      ];

      for (final type in cases) {
        datasource.lastCall = null;
        final result = await repo.shareAsPost(
          target: ShareTarget(
            id: 'content-1',
            type: type,
            title: 'Content title',
            description: 'Content description',
            imageUrl: 'https://img.example/content.jpg',
          ),
          authorId: 'author-1',
          caption: 'Caption',
        );

        result.fold(
          (failure) => fail('unexpected failure: ${failure.message}'),
          (value) => expect(value, 'repost-id'),
        );

        expect(datasource.lastCall, 'createRepost');
      }
    });

    test(
      'shareAsPost rewires listing/auction/profile to createShareReferencePost',
      () async {
        final datasource = _RecordingShareApiDatasource();
        final repo = ShareRepositoryApi(
          datasource: datasource,
          nativeShareService: _StubNativeShareService(),
        );

        final cases = <ExternalShareType>[
          ExternalShareType.listing,
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
            (failure) => fail('unexpected failure: ${failure.message}'),
            (value) => expect(value, 'shared-id'),
          );

          expect(datasource.lastCall, 'createShareReferencePost');
        }
      },
    );

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

      result.fold((failure) => fail('unexpected failure: ${failure.message}'), (
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

      result.fold((failure) => fail('unexpected failure: ${failure.message}'), (
        shareResult,
      ) {
        expect(shareResult.success, isTrue);
        expect(shareResult.destination, ShareDestinationType.copyLink);
      });

      expect(
        nativeShareService.lastCopiedText,
        equals('$kPublicProfileBaseUrl/profile/profile-1'),
      );
    });
  });
}
