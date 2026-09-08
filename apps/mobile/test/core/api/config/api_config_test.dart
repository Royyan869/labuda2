// STAGE 3B / CANONICAL FIX: API base-URL convergence.
//
// All dev platforms use localhost:8080.  The Android emulator alias
// 10.0.2.2 has been removed — it silently broke physical devices.
// Physical devices use `adb reverse tcp:8080 tcp:8080` so localhost works.
// An explicit --dart-define=API_BASE_URL / API_WS_URL override takes
// precedence when the default is unsuitable.
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

    test('dev default no longer uses emulator-only 10.0.2.2', () {
      ApiConfig.setEnvironment(ApiEnvironment.dev);
      expect(ApiConfig.baseUrl.contains('10.0.2.2'), isFalse);
      expect(ApiConfig.wsUrl.contains('10.0.2.2'), isFalse);
    });

    test('dev default is localhost:8080 on all platforms', () {
      ApiConfig.setEnvironment(ApiEnvironment.dev);
      expect(ApiConfig.baseUrl, equals('http://localhost:8080/api/v1'));
      expect(ApiConfig.wsUrl, equals('ws://localhost:8080/api/v1/ws'));
    });

    test('prod and staging URLs unchanged', () {
      ApiConfig.setEnvironment(ApiEnvironment.prod);
      expect(ApiConfig.baseUrl, equals('https://api.labuda.com/api/v1'));

      ApiConfig.setEnvironment(ApiEnvironment.staging);
      expect(
        ApiConfig.baseUrl,
        equals('https://staging-api.labuda.com/api/v1'),
      );
    });
  });
}
