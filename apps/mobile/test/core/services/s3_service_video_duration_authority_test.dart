import 'dart:io';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/services/s3_service.dart';
import 'package:labuda/core/src/utils/constants/app_constants.dart';

class _RecordingApiClient extends ApiClient {
  _RecordingApiClient({required this.presignResponse}) : super.testing();

  final Map<String, dynamic> presignResponse;
  int postCalls = 0;

  @override
  Future<Response<T>> post<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    postCalls += 1;
    return Response<T>(
      requestOptions: RequestOptions(path: path),
      data: presignResponse as T,
    );
  }
}

class _VideoUploadFixture {
  _VideoUploadFixture._(this.server);

  final HttpServer server;
  int putCalls = 0;
  int getCalls = 0;
  Uint8List? lastUploadBody;

  static Future<_VideoUploadFixture> start() async {
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    final fixture = _VideoUploadFixture._(server);
    server.listen((request) async {
      if (request.method == 'PUT' && request.uri.path == '/upload/video') {
        fixture.putCalls += 1;
        final bytes = await request.fold<BytesBuilder>(
          BytesBuilder(copy: false),
          (builder, data) => builder..add(data),
        );
        fixture.lastUploadBody = bytes.takeBytes();
        request.response.statusCode = 200;
        await request.response.close();
        return;
      }

      if (request.method == 'GET' && request.uri.path == '/read/video') {
        fixture.getCalls += 1;
        request.response.statusCode = 200;
        request.response.headers.contentType = ContentType('video', 'mp4');
        request.response.add(const [1, 2, 3, 4]);
        await request.response.close();
        return;
      }

      request.response.statusCode = 404;
      await request.response.close();
    });
    return fixture;
  }

  Uri get uploadUrl =>
      Uri.parse('http://127.0.0.1:${server.port}/upload/video');

  Uri get readUrl => Uri.parse('http://127.0.0.1:${server.port}/read/video');

  Future<void> close() => server.close(force: true);
}

Future<File> _tempVideoFile(String name) async {
  final dir = await Directory.systemTemp.createTemp('labuda_video_duration_');
  final file = File('${dir.path}${Platform.pathSeparator}$name');
  await file.writeAsBytes(const [1, 2, 3, 4, 5, 6]);
  return file;
}

void main() {
  late _VideoUploadFixture fixture;

  setUpAll(() async {
    fixture = await _VideoUploadFixture.start();
  });

  tearDownAll(() async {
    await fixture.close();
  });

  test(
    'boundary helper accepts just below and exactly at the commerce limit',
    () {
      expect(
        S3Service.validateVideoDurationMessage(
          domainLabel: 'Listing',
          durationMs: AppConstants.maxCommerceVideoDurationMs - 1,
          maxDurationMs: AppConstants.maxCommerceVideoDurationMs,
        ),
        isNull,
      );

      expect(
        S3Service.validateVideoDurationMessage(
          domainLabel: 'Auction',
          durationMs: AppConstants.maxCommerceVideoDurationMs,
          maxDurationMs: AppConstants.maxCommerceVideoDurationMs,
        ),
        isNull,
      );
    },
  );

  test(
    'uploadVideo accepts boundary durations and keeps duration in ms',
    () async {
      S3Service.setApiClient(
        _RecordingApiClient(
          presignResponse: {
            'data': {
              'upload_url': fixture.uploadUrl.toString(),
              'storage_key': 'videos/test-upload.mp4',
              'read_url': fixture.readUrl.toString(),
            },
          },
        ),
      );

      final durations = <int>[
        AppConstants.maxCommerceVideoDurationMs - 1,
        AppConstants.maxCommerceVideoDurationMs,
      ];

      for (final durationMs in durations) {
        final service = S3Service(
          videoMetadataReader: (_) async => VideoMediaMetadata(
            width: 1920,
            height: 1080,
            durationMs: durationMs,
          ),
        );
        final file = await _tempVideoFile('boundary_$durationMs.mp4');

        final result = await service.uploadVideo(
          file,
          maxDurationMs: AppConstants.maxCommerceVideoDurationMs,
          domainLabel: 'Listing',
        );

        expect(result.isSuccess, isTrue);
        expect(result.data, 'videos/test-upload.mp4');
      }

      expect(fixture.putCalls, 2);
      expect(fixture.getCalls, 2);
    },
  );

  test('uploadVideo rejects just above the limit before presign', () async {
    final client = _RecordingApiClient(presignResponse: const {'data': {}});
    S3Service.setApiClient(client);

    final service = S3Service(
      videoMetadataReader: (_) async => VideoMediaMetadata(
        width: 1920,
        height: 1080,
        durationMs: AppConstants.maxCommerceVideoDurationMs + 1,
      ),
    );
    final file = await _tempVideoFile('too_long.mp4');

    final result = await service.uploadVideo(
      file,
      maxDurationMs: AppConstants.maxCommerceVideoDurationMs,
      domainLabel: 'Auction',
    );

    expect(result.isSuccess, isFalse);
    expect(
      result.error,
      'Durasi video Auction melebihi batas maksimal 3 menit (180000 ms).',
    );
    expect(client.postCalls, 0);
  });

  test('uploadVideo rejects missing duration metadata before presign', () async {
    final client = _RecordingApiClient(presignResponse: const {'data': {}});
    S3Service.setApiClient(client);

    final service = S3Service(
      videoMetadataReader: (_) async =>
          const VideoMediaMetadata(width: 1920, height: 1080),
    );
    final file = await _tempVideoFile('missing_duration.mp4');

    final result = await service.uploadVideo(
      file,
      maxDurationMs: AppConstants.maxCommerceVideoDurationMs,
      domainLabel: 'Listing',
    );

    expect(result.isSuccess, isFalse);
    expect(
      result.error,
      'Durasi video Listing tidak dapat dibaca. Batas maksimal 3 menit (180000 ms).',
    );
    expect(client.postCalls, 0);
  });

  test('content limit stays separate at 10 minutes', () {
    expect(
      S3Service.validateVideoDurationMessage(
        domainLabel: 'Content',
        durationMs: AppConstants.maxContentVideoDurationMs,
        maxDurationMs: AppConstants.maxContentVideoDurationMs,
      ),
      isNull,
    );
    expect(
      S3Service.validateVideoDurationMessage(
        domainLabel: 'Content',
        durationMs: AppConstants.maxContentVideoDurationMs + 1,
        maxDurationMs: AppConstants.maxContentVideoDurationMs,
      ),
      'Durasi video Content melebihi batas maksimal 10 menit (600000 ms).',
    );
  });
}
