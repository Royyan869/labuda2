// STAGE 3B: locks the API base-URL convergence.
//
// The hard-coded dev LAN IP (192.168.1.8:8080) is no longer an authority.
// Dev defaults are platform-aware (Android emulator -> 10.0.2.2, everything
// else -> localhost) and an explicit --dart-define=API_BASE_URL /
// API_WS_URL override takes precedence for physical devices / other hosts.
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/config/api_config.dart';

void main() {
  tearDown(() {
    // Restore the canonical environment after each test.
    ApiConfig.setEnvironment(ApiEnvironment.dev);
  });

  group('ApiConfig dev default (no override)', () {
    test('dev default no longer references the hard-coded LAN IP', () {
      ApiConfig.setEnvironment(ApiEnvironment.dev);
      expect(ApiConfig.baseUrl.contains('192.168.1.8'), isFalse);
      expect(ApiConfig.wsUrl.contains('192.168.1.8'), isFalse);
    });

    test('dev default is a localhost:8080 base URL on non-Android', () {
      ApiConfig.setEnvironment(ApiEnvironment.dev);
      // The Android emulator host mapping (10.0.2.2) is decided by
      // defaultTargetPlatform at runtime; the non-Android shape must be
      // the plain localhost form.
      expect(
        ApiConfig.baseUrl,
        anyOf(
          equals('http://10.0.2.2:8080/api/v1'),
          equals('http://localhost:8080/api/v1'),
        ),
      );
      expect(ApiConfig.baseUrlIOS, equals('http://localhost:8080/api/v1'));
      expect(ApiConfig.wsUrlIOS, equals('ws://localhost:8080/api/v1/ws'));
    });

    test('prod and staging URLs unchanged', () {
      ApiConfig.setEnvironment(ApiEnvironment.prod);
      expect(ApiConfig.baseUrl, equals('https://api.labuda.com/api/v1'));

      ApiConfig.setEnvironment(ApiEnvironment.staging);
      expect(ApiConfig.baseUrl, equals('https://staging-api.labuda.com/api/v1'));
    });
  });
}
