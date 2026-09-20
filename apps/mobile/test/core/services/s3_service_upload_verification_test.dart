import 'dart:convert';
import 'dart:io' as io;
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:image/image.dart' as img;
import 'package:labuda/core/core.dart';

Uint8List _jpegBytes() {
  final image = img.Image(width: 1, height: 1);
  image.setPixelRgba(0, 0, 255, 255, 255, 255);
  return img.encodeJpg(image, quality: 95);
}

/// Canonical test transport for the S3 upload contract.
///
/// Installed on `ApiClient.dio.httpClientAdapter` — the canonical test seam
/// proven by the fake-adapter isolation test in
/// `test/core/api/api_client_testing_test.dart`. Requests still traverse the
/// real `ApiClient`/Dio pipeline (interceptors included); only the transport is
/// replaced, because `flutter_test`'s `TestWidgetsFlutterBinding` (initialized
/// suite-wide by `test/flutter_test_config.dart`) installs an `HttpOverrides`
/// that answers every `dart:io HttpClient` request with an empty `400`.
///
/// It models ONLY the two canonical operations of the current production flow:
///
///   POST /media/upload-url -> { data: { upload_url, storage_key, read_url } }
///   PUT  the presigned url -> configurable status (bytes + content-type recorded)
///
/// There is deliberately no GET/HEAD read-back path: post-upload read_url
/// verification is NOT part of the canonical contract and must not be
/// reintroduced here.
class _UploadContractAdapter implements HttpClientAdapter {
  final requests = <String>[];
  int putStatus = 200;
  String storageKey = 'images/stores/user-1.jpg';
  Map<String, dynamic>? lastPresignBody;
  String? lastPresignUploadUrl;
  String? lastPutUrl;
  int? lastPutByteCount;
  String? lastPutContentType;

  // Error injection for presign leg.
  int? presignErrorStatus;
  String? presignErrorCode;
  String? presignErrorMessage;

  void reset() {
    requests.clear();
    putStatus = 200;
    storageKey = 'images/stores/user-1.jpg';
    lastPresignBody = null;
    lastPresignUploadUrl = null;
    lastPutUrl = null;
    lastPutByteCount = null;
    lastPutContentType = null;
    presignErrorStatus = null;
    presignErrorCode = null;
    presignErrorMessage = null;
  }

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<List<int>>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add('${options.method} ${options.uri.path}');

    if (options.method == 'POST' &&
        options.uri.path == '/media/upload-url') {
      final body = await _readBytes(options, requestStream);
      if (body.isNotEmpty) {
        final decoded = jsonDecode(utf8.decode(body)) as Map<String, dynamic>;
        lastPresignBody = decoded;
        final requestedKey = decoded['storage_key'] as String?;
        if (requestedKey != null && requestedKey.isNotEmpty) {
          storageKey = requestedKey;
        }
      }

      if (presignErrorStatus != null) {
        final envelope = <String, dynamic>{
          'success': false,
          'error': <String, dynamic>{
            'code': presignErrorCode ?? 'UNKNOWN',
            'message': presignErrorMessage ?? 'error',
          },
        };
        return ResponseBody.fromString(
          jsonEncode(envelope),
          presignErrorStatus!,
          headers: {
            Headers.contentTypeHeader: [Headers.jsonContentType],
          },
        );
      }

      lastPresignUploadUrl = 'https://upload.example.com/$storageKey';
      final envelope = <String, dynamic>{
        'data': <String, dynamic>{
          'upload_url': lastPresignUploadUrl,
          'storage_key': storageKey,
          'read_url': 'https://cdn.example.com/$storageKey',
        },
      };

      return ResponseBody.fromString(
        jsonEncode(envelope),
        200,
        headers: {
          Headers.contentTypeHeader: [Headers.jsonContentType],
        },
      );
    }

    if (options.method == 'PUT') {
      lastPutUrl = options.uri.toString();
      lastPutContentType = _header(options.headers, Headers.contentTypeHeader);
      final bytes = await _readBytes(options, requestStream);
      lastPutByteCount = bytes.length;
      return ResponseBody.fromString('', putStatus);
    }

    return ResponseBody.fromString('', 404);
  }

  @override
  void close({bool force = false}) {}

  Future<List<int>> _readBytes(
    RequestOptions options,
    Stream<List<int>>? requestStream,
  ) async {
    final bytes = <int>[];
    if (requestStream != null) {
      await for (final chunk in requestStream) {
        bytes.addAll(chunk);
      }
    }
    if (bytes.isNotEmpty) return bytes;

    final data = options.data;
    if (data is Map) return utf8.encode(jsonEncode(data));
    if (data is List<int>) return data;
    if (data is String) return utf8.encode(data);
    return bytes;
  }

  String? _header(Map<String, dynamic> headers, String name) {
    final direct = headers[name];
    if (direct != null) return direct.toString();
    final lower = name.toLowerCase();
    for (final entry in headers.entries) {
      if (entry.key.toLowerCase() == lower) {
        final value = entry.value;
        return value is List ? value.first.toString() : value?.toString();
      }
    }
    return null;
  }
}

void main() {
  late _UploadContractAdapter adapter;
  late S3Service s3Service;

  setUpAll(() {
    adapter = _UploadContractAdapter();

    // Presign leg — canonical `ApiClient.dio` seam, pipeline kept intact.
    final apiClient = ApiClient(
      logger: null,
      baseUrl: 'https://api.example.com',
    );
    apiClient.dio.httpClientAdapter = adapter;
    S3Service.setApiClient(apiClient);

    // PUT leg — presigned PUTs bypass ApiClient by design, so they use the
    // dedicated raw-Dio seam instead.
    final rawDio = Dio()..httpClientAdapter = adapter;

    s3Service = S3Service()..setRawDioForTest(rawDio);
  });

  Future<io.File> writeJpegFixture() async {
    final dir = await io.Directory.systemTemp.createTemp('s3-service-test');
    addTearDown(() async {
      await dir.delete(recursive: true);
    });

    final file = io.File('${dir.path}/fixture.jpg');
    await file.writeAsBytes(_jpegBytes());
    return file;
  }

  test(
    'uploadImageWithFixedKey presigns, PUTs the bytes, and returns the canonical S3UploadResult with read_url',
    () async {
      adapter.reset();
      final file = await writeJpegFixture();
      final expectedBytes = await file.length();

      final result = await s3Service.uploadImageWithFixedKey(
        file,
        'images/stores/user-1.jpg',
      );

      expect(result.isSuccess, isTrue, reason: 'error=${result.error} code=${result.errorCode}');
      expect(result.data, isNotNull);
      expect(result.data!.key, 'images/stores/user-1.jpg');
      expect(
        result.data!.url,
        'https://cdn.example.com/images/stores/user-1.jpg',
      );

      // Canonical flow is presign then PUT — exactly two requests, no read-back.
      expect(adapter.requests, contains('POST /media/upload-url'));
      expect(
        adapter.requests.where((r) => r.startsWith('PUT ')),
        hasLength(1),
        reason: 'requests=${adapter.requests}',
      );
      expect(adapter.requests, hasLength(2), reason: 'requests=${adapter.requests}');
      expect(
        adapter.requests.where((r) => r.startsWith('GET ')),
        isEmpty,
      );
      expect(
        adapter.requests.where((r) => r.startsWith('HEAD ')),
        isEmpty,
      );

      // Presign payload contract.
      expect(adapter.lastPresignBody, isNotNull);
      expect(adapter.lastPresignBody!['content_type'], 'image/jpeg');
      expect(adapter.lastPresignBody!['folder'], 'images');
      expect(
        adapter.lastPresignBody!['storage_key'],
        'images/stores/user-1.jpg',
      );

      // PUT contract — the bytes must go to the URL presign minted.
      expect(adapter.lastPutUrl, adapter.lastPresignUploadUrl);
      expect(
        adapter.lastPutUrl,
        'https://upload.example.com/images/stores/user-1.jpg',
      );
      expect(adapter.lastPutContentType, 'image/jpeg');
      expect(adapter.lastPutByteCount, expectedBytes);
    },
  );

  test(
    'failed PUT returns an upload error and a later attempt succeeds without restarting the service',
    () async {
      adapter.reset();
      adapter.putStatus = 500;
      final file = await writeJpegFixture();

      final first = await s3Service.uploadImageWithFixedKey(
        file,
        'images/stores/user-1.jpg',
      );

      expect(first.isError, isTrue);
      expect(first.error, contains('Upload'));
      expect(adapter.requests, contains('POST /media/upload-url'));
      expect(
        adapter.requests.where((r) => r.startsWith('PUT ')),
        hasLength(1),
        reason: 'requests=${adapter.requests}',
      );

      adapter.reset();
      final second = await s3Service.uploadImageWithFixedKey(
        file,
        'images/stores/user-1.jpg',
      );

      expect(second.isSuccess, isTrue);
      expect(second.data!.key, 'images/stores/user-1.jpg');
      expect(
        adapter.requests.where((r) => r.startsWith('PUT ')),
        hasLength(1),
        reason: 'requests=${adapter.requests}',
      );
    },
  );

  test('generic uploadImage uses read_url (no public_url fallback)', () async {
    adapter.reset();
    final file = await writeJpegFixture();
    final result = await s3Service.uploadImage(file);
    expect(result.isSuccess, isTrue);
    expect(result.data, 'https://cdn.example.com/images/stores/user-1.jpg');
    // Presign must NOT have been called with a storage_key override.
    // Adapter defaults to stored key when not provided; ensure presign body folder is images.
    expect(adapter.lastPresignBody!['folder'], 'images');
  });

  test('uploadImageWithMeta returns S3UploadResult with key and read_url', () async {
    adapter.reset();
    final file = await writeJpegFixture();
    final result = await s3Service.uploadImageWithMeta(file);
    expect(result.isSuccess, isTrue);
    expect(result.data!.key, isNotEmpty);
    expect(result.data!.url, startsWith('https://cdn.example.com/'));
  });

  test('uploadImageBytesWithFixedKey honors the fixed key and returns read_url', () async {
    adapter.reset();
    final bytes = _jpegBytes();
    final result = await s3Service.uploadImageBytesWithFixedKey(
      bytes,
      'images/avatars/user-1.jpg',
      contentType: 'image/jpeg',
    );
    expect(result.isSuccess, isTrue);
    expect(result.data!.key, 'images/avatars/user-1.jpg');
    expect(result.data!.url, 'https://cdn.example.com/images/avatars/user-1.jpg');
    expect(adapter.lastPresignBody!['storage_key'], 'images/avatars/user-1.jpg');
    expect(adapter.lastPresignBody!['folder'], 'images');
  });

  test('presign error envelope preserves structured code and message (INVALID_STORAGE_KEY)', () async {
    adapter.reset();
    adapter.presignErrorStatus = 400;
    adapter.presignErrorCode = 'INVALID_STORAGE_KEY';
    adapter.presignErrorMessage = 'storage_key must match images/avatars/{user_id}.jpg or .png, images/stores/{user_id}.jpg, images/profile-covers/{user_id}.jpg, or an owned commerce media key';
    final file = await writeJpegFixture();
    final result = await s3Service.uploadImageWithFixedKey(file, 'images/stores/another-user.jpg');
    expect(result.isError, isTrue);
    expect(result.errorCode, 'INVALID_STORAGE_KEY');
    expect(result.statusCode, 400);
    expect(result.error, contains('storage_key'));
    // No PUT should have occurred.
    expect(adapter.requests.where((r) => r.startsWith('PUT ')), isEmpty);
  });

  test('presign 503 UPLOAD_NOT_CONFIGURED is surfaced with code', () async {
    adapter.reset();
    adapter.presignErrorStatus = 503;
    adapter.presignErrorCode = 'UPLOAD_NOT_CONFIGURED';
    adapter.presignErrorMessage = 'Media upload service not configured';
    final file = await writeJpegFixture();
    final result = await s3Service.uploadImage(file);
    expect(result.isError, isTrue);
    expect(result.errorCode, 'UPLOAD_NOT_CONFIGURED');
    expect(result.statusCode, 503);
  });

  test('PUT failure preserves statusCode', () async {
    adapter.reset();
    adapter.putStatus = 500;
    final file = await writeJpegFixture();
    final result = await s3Service.uploadImage(file);
    expect(result.isError, isTrue);
    expect(result.statusCode, 500);
  });
}
