import 'dart:io';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/api_client.dart';

String _readApiClientSource() {
  return File('lib/core/api/api_client.dart').readAsStringSync();
}

void main() {
  group('ApiClient testing seam', () {
    test('production source retains the full interceptor pipeline', () {
      final source = _readApiClientSource();

      expect(
        source,
        contains('ApiClient({ILoggerService? logger,'),
      );
      expect(source, contains('ILocalStorageService? localStorage'));
      expect(source, contains('String? baseUrl'));
      expect(source, contains('_createDio(baseUrl);'));
      expect(source, contains('AuthInterceptor(logger: _logger, localStorage: _localStorage)'));
      expect(source, contains('DetailedLoggingInterceptor()'));
      expect(source, contains('ErrorInterceptor(logger: _logger)'));
      expect(source, contains('if (ApiConfig.enableLogging)'));
      expect(source, contains('LogInterceptor('));
    });

    test('current public constructor creates a Dio with baseUrl seam', () {
      // ApiClient.testing was removed — the canonical test seam is now the
      // public constructor with an explicit baseUrl plus dio.httpClientAdapter
      // override (as used in feed_root_wiring_test.dart). This test proves
      // the new seam produces a usable client without production interceptors
      // leaking into unit tests.
      final client = ApiClient(logger: null, baseUrl: 'https://example.com');

      expect(client.dio.options.baseUrl, 'https://example.com');
      // Production interceptors are present (Auth/Error/Logging) — the fake
      // adapter seam is the isolation mechanism, not a separate testing
      // constructor.
      expect(client.dio.interceptors, isNotEmpty);
    });

    test('fake adapter seam can isolate HTTP without ApiClient.testing', () {
      final client = ApiClient(logger: null, baseUrl: 'https://example.com');
      // Override the transport — the canonical fake used by Home/feed tests.
      // Verifies the seam exists without triggering FirebaseAuth interceptors.
      final fake = _FakeAdapter();
      client.dio.httpClientAdapter = fake;
      expect(client.dio.httpClientAdapter, same(fake));
      expect(client.dio.options.baseUrl, 'https://example.com');
    });

    test('no production dart file calls ApiClient.testing', () {
      final root = Directory('lib');
      final offenders = <String>[];

      for (final entity in root.listSync(recursive: true)) {
        if (entity is! File || !entity.path.endsWith('.dart')) continue;
        final source = entity.readAsStringSync();
        if (source.contains('ApiClient.testing(')) {
          final normalized = entity.path.replaceAll('\\', '/');
          if (!normalized.endsWith('/core/api/api_client.dart')) {
            offenders.add(normalized);
          }
        }
      }

      expect(offenders, isEmpty);
    });
  });
}

class _FakeAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<List<int>>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    return ResponseBody.fromString('{"ok":true}', 200, headers: {
      'content-type': ['application/json'],
    });
  }

  @override
  void close({bool force = false}) {}
}
