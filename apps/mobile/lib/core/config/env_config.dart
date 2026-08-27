import 'package:flutter/foundation.dart';
import '../api/config/api_config.dart';

/// Application Environment Configuration
///
/// Centralizes environment detection and configuration for the app.
/// Works with ApiConfig to provide environment-based settings.
class EnvConfig {
  // Private constructor
  EnvConfig._();

  /// Current app environment
  static AppEnvironment _currentEnvironment = AppEnvironment.development;

  /// Force override environment (for testing)
  static AppEnvironment? _forcedEnvironment;

  /// Get current environment
  static AppEnvironment get current =>
      _forcedEnvironment ?? _currentEnvironment;

  /// Set environment (call during app initialization)
  static void setEnvironment(AppEnvironment env) {
    _currentEnvironment = env;
    // Sync with ApiConfig
    final apiEnv = _mapToApiEnvironment(env);
    ApiConfig.setEnvironment(apiEnv);
  }

  /// Force override environment (useful for testing)
  static void forceEnvironment(AppEnvironment env) {
    _forcedEnvironment = env;
    final apiEnv = _mapToApiEnvironment(env);
    ApiConfig.setEnvironment(apiEnv);
  }

  /// Clear forced environment
  static void clearForcedEnvironment() {
    _forcedEnvironment = null;
  }

  /// Detect environment from compilation constants
  static AppEnvironment detectEnvironment() {
    if (kReleaseMode) {
      return AppEnvironment.production;
    } else if (kProfileMode) {
      return AppEnvironment.staging;
    } else {
      return AppEnvironment.development;
    }
  }

  /// Initialize environment configuration
  ///
  /// Call this during app initialization (main.dart) before any API calls.
  /// If [environment] is not provided, will auto-detect from build mode.
  static void init([AppEnvironment? environment]) {
    final env = environment ?? detectEnvironment();
    setEnvironment(env);
  }

  // ============ Environment Check Helpers ============

  /// True if running in development mode
  static bool get isDevelopment => current == AppEnvironment.development;

  /// True if running in staging mode
  static bool get isStaging => current == AppEnvironment.staging;

  /// True if running in production mode
  static bool get isProduction => current == AppEnvironment.production;

  /// True if running in non-production environment
  static bool get isDevOrStaging => isDevelopment || isStaging;

  /// True if debug mode is enabled
  static bool get isDebugMode => kDebugMode;

  /// True if profile mode is enabled
  static bool get isProfileMode => kProfileMode;

  /// True if release mode is enabled
  static bool get isReleaseMode => kReleaseMode;

  // ============ Display Helpers ============

  /// Get environment name for display
  static String get environmentName {
    switch (current) {
      case AppEnvironment.development:
        return 'Development';
      case AppEnvironment.staging:
        return 'Staging';
      case AppEnvironment.production:
        return 'Production';
    }
  }

  /// Get environment short code
  static String get environmentCode {
    switch (current) {
      case AppEnvironment.development:
        return 'DEV';
      case AppEnvironment.staging:
        return 'STG';
      case AppEnvironment.production:
        return 'PROD';
    }
  }

  // ============ Private Methods ============

  static ApiEnvironment _mapToApiEnvironment(AppEnvironment env) {
    switch (env) {
      case AppEnvironment.development:
        return ApiEnvironment.dev;
      case AppEnvironment.staging:
        return ApiEnvironment.staging;
      case AppEnvironment.production:
        return ApiEnvironment.prod;
    }
  }
}

/// Application Environment Types
enum AppEnvironment {
  /// Local development
  development,

  /// Staging/testing environment
  staging,

  /// Production environment
  production,
}
