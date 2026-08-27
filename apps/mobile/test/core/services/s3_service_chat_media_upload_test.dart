import 'dart:async';
import 'dart:convert';
import 'dart:io' as io;
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:image/image.dart' as img;
import 'package:labuda/core/core.dart';

class _ChatMediaProbeServer {
  late final io.HttpServer server;
  final methods = <String>[];

  int putStatus = 200;
  String mediaAssetId = 'chat-media-asset-1';
  String storageKey = 'chat/media/asset-1.png';
  String readUrl = 'https://cdn.example.invalid/chat/media/asset-1.png';
  int? lastPutByteCount;
  String? lastPutContentType;
  Map<String, dynamic>? lastPresignBody;
  Map<String, dynamic>? lastFinalizeBody;
  bool cancelCalled = false;

  Uri get baseUri => Uri.parse(
    'http://${server.address.address}:${server.port}',
  );

  Future<void> start() async {
    server = await io.HttpServer.bind(io.InternetAddress.loopbackIPv4, 0);
    server.listen(_handle);
  }

  Future<void> stop() async {
    await server.close(force: true);
  }

  void reset() {
    methods.clear();
    putStatus = 200;
    mediaAssetId = 'chat-media-asset-1';
    storageKey = 'chat/media/asset-1.png';
    readUrl = 'https://cdn.example.invalid/chat/media/asset-1.png';
    lastPutByteCount = null;
    lastPutContentType = null;
    lastPresignBody = null;
    lastFinalizeBody = null;
    cancelCalled = false;
  }

  Future<void> _handle(io.HttpRequest request) async {
    methods.add('${request.method} ${request.uri.path}');

    if (request.method == 'POST' &&
        request.uri.path.endsWith('/media/upload-url')) {
      final rawBody = await utf8.decoder.bind(request).join();
      lastPresignBody = rawBody.isEmpty
          ? null
          : jsonDecode(rawBody) as Map<String, dynamic>;

      final payload = <String, dynamic>{
        'data': <String, dynamic>{
          'media_asset_id': mediaAssetId,
          'upload_url': '${baseUri.toString()}/upload',
          'storage_key': storageKey,
          'read_url': readUrl,
          'expires_at': '2026-07-31T23:59:59.000Z',
        },
      };

      request.response.statusCode = 200;
      request.response.headers.contentType = io.ContentType.json;
      request.response.write(jsonEncode(payload));
      await request.response.close();
      return;
    }

    if (request.method == 'PUT' && request.uri.path == '/upload') {
      lastPutContentType = request.headers.value(
        io.HttpHeaders.contentTypeHeader,
      );
      try {
        final bytes = await request.fold<List<int>>(<int>[], (previous, chunk) {
          previous.addAll(chunk);
          return previous;
        });
        lastPutByteCount = bytes.length;
        request.response.statusCode = putStatus;
        await request.response.close();
      } catch (_) {
        lastPutByteCount = null;
      }
      return;
    }

    if (request.method == 'POST' &&
        request.uri.path.endsWith('/media/finalize')) {
      final rawBody = await utf8.decoder.bind(request).join();
      lastFinalizeBody = rawBody.isEmpty
          ? null
          : jsonDecode(rawBody) as Map<String, dynamic>;

      final payload = <String, dynamic>{
        'data': <String, dynamic>{
          'id': mediaAssetId,
          'room_id': 'room-1',
          'media_type': 'image/png',
          'content_type': 'image/png',
          'storage_key': storageKey,
          'url': readUrl,
          'byte_size': lastPutByteCount ?? 0,
          'width': 128,
          'height': 96,
          'thumbnail_storage_key': null,
          'thumbnail_url': null,
          'duration_ms': null,
        },
      };

      request.response.statusCode = 200;
      request.response.headers.contentType = io.ContentType.json;
      request.response.write(jsonEncode(payload));
      await request.response.close();
      return;
    }

    if (request.method == 'POST' &&
        request.uri.path.endsWith('/media/$mediaAssetId/cancel')) {
      cancelCalled = true;
      request.response.statusCode = 200;
      request.response.headers.contentType = io.ContentType.json;
      request.response.write(jsonEncode({'data': {'ok': true}}));
      await request.response.close();
      return;
    }

    request.response.statusCode = 404;
    await request.response.close();
  }
}

Uint8List _pngBytes({int width = 128, int height = 96}) {
  final image = img.Image(width: width, height: height);
  for (var y = 0; y < height; y++) {
    for (var x = 0; x < width; x++) {
      final r = (x * 13 + y * 7) % 256;
      final g = (x * 5 + y * 17) % 256;
      final b = (x * 19 + y * 3) % 256;
      image.setPixelRgba(x, y, r, g, b, 255);
    }
  }
  return img.encodePng(image);
}

Future<io.File> _writePngFixture({required String name}) async {
  final dir = await io.Directory.systemTemp.createTemp('s3-chat-upload-test');
  final file = io.File('${dir.path}/$name');
  await file.writeAsBytes(_pngBytes(width: 1024, height: 1024));
  return file;
}

void main() {
  late _ChatMediaProbeServer server;
  late S3Service service;

  setUpAll(() async {
    server = _ChatMediaProbeServer();
    await server.start();
    S3Service.setApiClient(ApiClient.testing(baseUrl: server.baseUri.toString()));
    service = S3Service();
  });

  tearDownAll(() async {
    await server.stop();
  });

  test('uploadChatMedia emits progress and finalizes canonical media', () async {
    server.reset();
    final file = await _writePngFixture(name: 'chat-upload-success.png');
    addTearDown(() async => file.parent.delete(recursive: true));

    final progressEvents = <(int sent, int total)>[];
    final result = await service.uploadChatMedia(
      file,
      roomId: 'room-1',
      onSendProgress: (sent, total) {
        progressEvents.add((sent, total));
      },
    );

    expect(result.isSuccess, isTrue);
    final data = result.data!;
    expect(data.mediaAssetId, 'chat-media-asset-1');
    expect(data.roomId, 'room-1');
    expect(data.mediaType, 'image/png');
    expect(data.contentType, 'image/png');
    expect(data.storageKey, 'chat/media/asset-1.png');
    expect(data.readUrl, server.readUrl);
    expect(data.byteSize, greaterThan(0));
    expect(progressEvents, isNotEmpty);
    expect(progressEvents.last.$2, greaterThan(0));
    expect(server.methods, contains('POST /chat/rooms/room-1/media/upload-url'));
    expect(server.methods, contains('PUT /upload'));
    expect(server.methods, contains('POST /chat/rooms/room-1/media/finalize'));
    expect(server.cancelCalled, isFalse);
    expect(server.lastPresignBody?['content_type'], 'image/png');
    expect(server.lastFinalizeBody?['media_asset_id'], 'chat-media-asset-1');
    expect(server.lastPutContentType, 'image/png');
    expect(server.lastPutByteCount, greaterThan(0));
  });

  test('uploadChatMedia retries after upload failure and cancels draft', () async {
    server.reset();
    final file = await _writePngFixture(name: 'chat-upload-retry.png');
    addTearDown(() async => file.parent.delete(recursive: true));

    server.putStatus = 500;
    final first = await service.uploadChatMedia(file, roomId: 'room-1');
    expect(first.isError, isTrue);
    expect(first.error, contains('Upload chat media error'));
    expect(server.cancelCalled, isTrue);
    expect(server.methods, contains('POST /chat/rooms/room-1/media/chat-media-asset-1/cancel'));

    server.reset();
    final second = await service.uploadChatMedia(file, roomId: 'room-1');
    expect(second.isSuccess, isTrue);
    expect(second.data?.mediaAssetId, 'chat-media-asset-1');
  });

  test('uploadChatMedia cancel token aborts and invokes cleanup', () async {
    server.reset();
    final file = await _writePngFixture(name: 'chat-upload-cancel.png');
    addTearDown(() async => file.parent.delete(recursive: true));

    final cancelToken = CancelToken();
    final cancelCompleter = Completer<void>();
    final resultFuture = service.uploadChatMedia(
      file,
      roomId: 'room-1',
      cancelToken: cancelToken,
      onSendProgress: (sent, total) {
        if (sent > 0 && !cancelCompleter.isCompleted) {
          cancelCompleter.complete();
        }
      },
    );

    await cancelCompleter.future.timeout(const Duration(seconds: 5));
    cancelToken.cancel('stop');

    final result = await resultFuture;
    expect(result.isError, isTrue);
    expect(result.error, contains('Upload dibatalkan'));
    expect(server.cancelCalled, isTrue);
    expect(server.methods, contains('POST /chat/rooms/room-1/media/chat-media-asset-1/cancel'));
  });
}
