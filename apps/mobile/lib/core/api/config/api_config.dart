// API Configuration for different environments

import 'package:flutter/foundation.dart' show kIsWeb;

/// Environment types
enum ApiEnvironment {
  /// Local development (localhost)
  dev,

  /// Staging/testing server
  staging,

  /// Production server
  prod,
}

/// API Configuration
///
/// Manages base URLs, timeouts, and other API-related settings
/// for different environments.
class ApiConfig {
  // Private constructor to prevent instantiation
  ApiConfig._();

  /// Current environment - change this for different builds
  static ApiEnvironment _environment = ApiEnvironment.dev;

  /// Get current environment
  static ApiEnvironment get environment => _environment;

  /// Set environment (call during app initialization)
  static void setEnvironment(ApiEnvironment env) {
    _environment = env;
  }

  /// Build-time override for the REST API base URL.
  ///
  /// STAGE 3B: converges the dev/staging base URL through an explicit,
  /// versionable mechanism instead of a hard-coded LAN IP:
  ///
  ///     flutter run --dart-define=API_BASE_URL=http://192.168.1.50:8080/api/v1
  ///
  /// When unset the environment defaults below apply (localhost for dev).
  /// Physical devices require `adb reverse tcp:8080 tcp:8080`.
  /// `EnvConfig`/`ApiConfig` environment detection is unchanged.
  static const String _overrideBaseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: '',
  );

  /// Build-time override for the WebSocket URL (same mechanism).
  static const String _overrideWsUrl = String.fromEnvironment(
    'API_WS_URL',
    defaultValue: '',
  );

  /// Whether an explicit `--dart-define` override was provided.
  /// Used for fail-fast diagnostics on physical devices (no LAN IP fallback).
  static bool get hasOverrideBaseUrl => _overrideBaseUrl.isNotEmpty;
  static bool get hasOverrideWsUrl => _overrideWsUrl.isNotEmpty;

  // _isAndroid removed: 10.0.2.2 is an emulator-only alias that silently
  // breaks physical devices.  All dev platforms now use localhost,
  // paired with `adb reverse` for physical devices.

  /// Base URL for REST API
  static String get baseUrl {
    if (_overrideBaseUrl.isNotEmpty) return _overrideBaseUrl;
    switch (_environment) {
      case ApiEnvironment.dev:
        // CANONICAL dev default: localhost for all platforms.
        // Physical devices require: `adb reverse tcp:8080 tcp:8080`
        // before `flutter run`.  Override with --dart-define if needed.
        return 'http://localhost:8080/api/v1';
      case ApiEnvironment.staging:
        return 'https://staging-api.labuda.com/api/v1';
      case ApiEnvironment.prod:
        return 'https://api.labuda.com/api/v1';
    }
  }



  /// WebSocket URL for real-time features
  /// Backend route: GET /api/v1/ws
  static String get wsUrl {
    if (_overrideWsUrl.isNotEmpty) return _overrideWsUrl;
    switch (_environment) {
      case ApiEnvironment.dev:
        // CANONICAL dev default: localhost for all platforms.
        // Physical devices require: `adb reverse tcp:8080 tcp:8080`
        return 'ws://localhost:8080/api/v1/ws';
      case ApiEnvironment.staging:
        return 'wss://staging-api.labuda.com/api/v1/ws';
      case ApiEnvironment.prod:
        return 'wss://api.labuda.com/api/v1/ws';
    }
  }



  // ============ Timeouts ============

  /// Connection timeout in milliseconds
  static const int connectTimeout = 10000; // 10 seconds

  /// Receive timeout in milliseconds
  static const int receiveTimeout = 10000; // 10 seconds

  /// Send timeout in milliseconds
  static const int sendTimeout = 10000; // 10 seconds

  // ============ Headers ============

  /// Default headers for all requests
  static Map<String, String> get defaultHeaders => {
    'Content-Type': 'application/json',
    'Accept': 'application/json',
    'X-App-Version': appVersion,
    'X-Platform': platform,
  };

  /// App version (should be set during initialization)
  static String appVersion = '1.0.0';

  /// Platform identifier
  static String platform = 'flutter';

  // ============ Logging ============

  /// Enable request/response logging (disable in production)
  static bool get enableLogging {
    return _environment != ApiEnvironment.prod;
  }

  // ============ Helper Methods ============



  /// Check if current environment is development
  static bool get isDev => _environment == ApiEnvironment.dev;

  /// Check if current environment is staging
  static bool get isStaging => _environment == ApiEnvironment.staging;

  /// Check if current environment is production
  static bool get isProd => _environment == ApiEnvironment.prod;
}
