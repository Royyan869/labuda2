import 'dart:io';

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
        contains('ApiClient({ILoggerService? logger, String? baseUrl})'),
      );
      expect(source, contains('_createDio(baseUrl);'));
      expect(source, contains('AuthInterceptor(logger: _logger)'));
      expect(source, contains('DetailedLoggingInterceptor()'));
      expect(source, contains('ErrorInterceptor(logger: _logger)'));
      expect(source, contains('if (ApiConfig.enableLogging)'));
      expect(source, contains('LogInterceptor('));
    });

    test('testing constructor disables production interceptors', () {
      final source = _readApiClientSource();

      expect(source, contains('ApiClient.testing'));
      expect(source, contains('includeInterceptors: false'));
      expect(source, contains('bool includeInterceptors = true'));
    });

    test('testing constructor creates a bare Dio instance', () {
      final client = ApiClient.testing(baseUrl: 'https://example.com');

      expect(client.dio.options.baseUrl, 'https://example.com');
      expect(client.dio.interceptors, hasLength(1));
      expect(
        client.dio.interceptors.single.runtimeType.toString(),
        contains('ImplyContentTypeInterceptor'),
      );
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
