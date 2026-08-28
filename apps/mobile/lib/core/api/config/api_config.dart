// API Configuration for different environments

import 'package:flutter/foundation.dart'
    show TargetPlatform, defaultTargetPlatform, kIsWeb;

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
  /// When unset the environment defaults below apply (platform-aware for
  /// dev so the Android emulator host mapping is handled without editing
  /// source). `EnvConfig`/`ApiConfig` environment detection is unchanged.
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

  /// Android emulators reach the host machine via 10.0.2.2; every other
  /// platform (iOS simulator, desktop, web) uses localhost.
  static bool get _isAndroid {
    if (kIsWeb) return false;
    return defaultTargetPlatform == TargetPlatform.android;
  }

  /// Base URL for REST API
  static String get baseUrl {
    if (_overrideBaseUrl.isNotEmpty) return _overrideBaseUrl;
    switch (_environment) {
      case ApiEnvironment.dev:
        // STAGE 3B: platform-aware dev default — Android emulators reach
        // the host machine via 10.0.2.2; everything else (iOS simulator,
        // desktop, web) uses localhost. A physical device still needs the
        // explicit --dart-define=API_BASE_URL=<host-LAN-IP> override; no
        // hard-coded LAN IP is the authority anymore.
        return 'http://${_isAndroid ? '10.0.2.2' : 'localhost'}:8080/api/v1';
      case ApiEnvironment.staging:
        return 'http://localhost:8081/api/v1';
      case ApiEnvironment.prod:
        return 'https://api.labuda.com/api/v1';
    }
  }

  /// Base URL for iOS simulator (uses different localhost address)
  static String get baseUrlIOS {
    if (_overrideBaseUrl.isNotEmpty) return _overrideBaseUrl;
    switch (_environment) {
      case ApiEnvironment.dev:
        return 'http://localhost:8080/api/v1';
      case ApiEnvironment.staging:
        return 'http://localhost:8081/api/v1';
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
        // STAGE 3B: platform-aware dev default (see [baseUrl]).
        return 'ws://${_isAndroid ? '10.0.2.2' : 'localhost'}:8080/api/v1/ws';
      case ApiEnvironment.staging:
        return 'wss://staging-api.labuda.com/api/v1/ws';
      case ApiEnvironment.prod:
        return 'wss://api.labuda.com/api/v1/ws';
    }
  }

  /// WebSocket URL for iOS simulator
  /// Backend route: GET /api/v1/ws
  static String get wsUrlIOS {
    if (_overrideWsUrl.isNotEmpty) return _overrideWsUrl;
    switch (_environment) {
      case ApiEnvironment.dev:
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

  /// Get base URL based on current platform
  static String getBaseUrl({bool isIOS = false}) {
    return isIOS ? baseUrlIOS : baseUrl;
  }

  /// Get WebSocket URL based on current platform
  static String getWsUrl({bool isIOS = false}) {
    return isIOS ? wsUrlIOS : wsUrl;
  }

  /// Check if current environment is development
  static bool get isDev => _environment == ApiEnvironment.dev;

  /// Check if current environment is staging
  static bool get isStaging => _environment == ApiEnvironment.staging;

  /// Check if current environment is production
  static bool get isProd => _environment == ApiEnvironment.prod;
}
