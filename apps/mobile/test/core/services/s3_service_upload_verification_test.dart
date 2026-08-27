import 'dart:convert';
import 'dart:io' as io;
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:image/image.dart' as img;

Uint8List _jpegBytes() {
  final image = img.Image(width: 1, height: 1);
  image.setPixelRgba(0, 0, 255, 255, 255, 255);
  return img.encodeJpg(image, quality: 95);
}

class _ProbeServer {
  late final io.HttpServer server;
  final methods = <String>[];

  int putStatus = 200;
  int getStatus = 200;
  int headStatus = 200;
  String getContentType = 'image/jpeg';
  Uint8List getBody = _jpegBytes();
  String storageKey = 'images/stores/user-1.jpg';
  int? lastPutByteCount;
  String? lastPutContentType;

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
    getStatus = 200;
    headStatus = 200;
    getContentType = 'image/jpeg';
    getBody = _jpegBytes();
    storageKey = 'images/stores/user-1.jpg';
    lastPutByteCount = null;
    lastPutContentType = null;
  }

  Future<void> _handle(io.HttpRequest request) async {
    methods.add('${request.method} ${request.uri.path}');

    if (request.method == 'POST' && request.uri.path == '/media/upload-url') {
      final rawBody = await utf8.decoder.bind(request).join();
      if (rawBody.isNotEmpty) {
        final body = jsonDecode(rawBody) as Map<String, dynamic>;
        final requestedKey = body['storage_key'] as String?;
        if (requestedKey != null && requestedKey.isNotEmpty) {
          storageKey = requestedKey;
        }
      }

      final payload = <String, dynamic>{
        'data': <String, dynamic>{
          'upload_url': '${baseUri.toString()}/upload',
          'read_url': '${baseUri.toString()}/read',
          'storage_key': storageKey,
        },
      };

      request.response.statusCode = 200;
      request.response.headers.contentType = io.ContentType(
        'application',
        'json',
      );
      request.response.write(jsonEncode(payload));
      await request.response.close();
      return;
    }

    if (request.method == 'PUT' && request.uri.path == '/upload') {
      lastPutContentType = request.headers.value(io.HttpHeaders.contentTypeHeader);
      final bytes = await request.fold<List<int>>(<int>[], (previous, chunk) {
        previous.addAll(chunk);
        return previous;
      });
      lastPutByteCount = bytes.length;
      request.response.statusCode = putStatus;
      await request.response.close();
      return;
    }

    if (request.uri.path == '/read' && request.method == 'GET') {
      request.response.statusCode = getStatus;
      final contentType = _parseContentType(getContentType);
      if (contentType != null) {
        request.response.headers.contentType = contentType;
      }
      if (getStatus >= 200 && getStatus < 300) {
        request.response.add(getBody);
      }
      await request.response.close();
      return;
    }

    if (request.uri.path == '/read' && request.method == 'HEAD') {
      request.response.statusCode = headStatus;
      final contentType = _parseContentType(getContentType);
      if (contentType != null) {
        request.response.headers.contentType = contentType;
      }
      await request.response.close();
      return;
    }

    request.response.statusCode = 404;
    await request.response.close();
  }

  io.ContentType? _parseContentType(String value) {
    final parts = value.split(';');
    final typeParts = parts.first.trim().split('/');
    if (typeParts.length != 2) {
      return null;
    }
    final charsetPart = parts.length > 1 ? parts.last.trim() : null;
    if (charsetPart != null && !charsetPart.startsWith('charset=')) {
      return null;
    }
    return io.ContentType(typeParts[0].trim(), typeParts[1].trim());
  }
}

void main() {
  late _ProbeServer server;
  late S3Service s3Service;

  setUpAll(() async {
    server = _ProbeServer();
    await server.start();
    S3Service.setApiClient(ApiClient.testing(baseUrl: server.baseUri.toString()));
    s3Service = S3Service();
  });

  tearDownAll(() async {
    await server.stop();
  });

  Future<io.File> writeJpegFixture() async {
    final dir = await io.Directory.systemTemp.createTemp('s3-service-test');
    addTearDown(() async {
      await dir.delete(recursive: true);
    });

    final file = io.File('${dir.path}/fixture.jpg');
    final bytes = _jpegBytes();
    await file.writeAsBytes(bytes);
    return file;
  }

  test(
    'uploadImageWithFixedKey returns the storage key after PUT 2xx and signed GET 2xx',
    () async {
      server.reset();
      final file = await writeJpegFixture();

      final result = await s3Service.uploadImageWithFixedKey(
        file,
        'images/stores/user-1.jpg',
      );

      expect(result.isSuccess, isTrue);
      expect(result.data, 'images/stores/user-1.jpg');
      expect(server.methods, contains('POST /media/upload-url'));
      expect(server.methods, contains('PUT /upload'));
      expect(server.methods, contains('GET /read'));
      expect(server.methods.where((m) => m.startsWith('HEAD ')), isEmpty);
      expect(server.lastPutContentType, 'image/jpeg');
      expect(server.lastPutByteCount, greaterThan(0));
    },
  );

  test(
    'HEAD is not accepted when GET returns 403',
    () async {
      server.reset();
      server.getStatus = 403;
      server.headStatus = 200;
      final file = await writeJpegFixture();

      final result = await s3Service.uploadImageWithFixedKey(
        file,
        'images/stores/user-1.jpg',
      );

      expect(result.isError, isTrue);
      expect(result.error, contains('diverifikasi'));
      expect(server.methods, contains('GET /read'));
      expect(server.methods.where((m) => m.startsWith('HEAD ')), isEmpty);
    },
  );

  test(
    'empty signed GET body fails verification',
    () async {
      server.reset();
      server.getBody = Uint8List(0);
      final file = await writeJpegFixture();

      final result = await s3Service.uploadImageWithFixedKey(
        file,
        'images/stores/user-1.jpg',
      );

      expect(result.isError, isTrue);
      expect(result.error, contains('diverifikasi'));
    },
  );

  test(
    'non-image signed GET body fails verification',
    () async {
      server.reset();
      server.getContentType = 'text/plain';
      server.getBody = Uint8List.fromList(utf8.encode('not an image'));
      final file = await writeJpegFixture();

      final result = await s3Service.uploadImageWithFixedKey(
        file,
        'images/stores/user-1.jpg',
      );

      expect(result.isError, isTrue);
      expect(result.error, contains('diverifikasi'));
    },
  );

  test(
    'retry succeeds without restarting the service after a failed verification',
    () async {
      server.reset();
      server.getStatus = 403;
      final file = await writeJpegFixture();

      final first = await s3Service.uploadImageWithFixedKey(
        file,
        'images/stores/user-1.jpg',
      );
      expect(first.isError, isTrue);

      server.reset();
      final second = await s3Service.uploadImageWithFixedKey(
        file,
        'images/stores/user-1.jpg',
      );

      expect(second.isSuccess, isTrue);
      expect(second.data, 'images/stores/user-1.jpg');
    },
  );
}
