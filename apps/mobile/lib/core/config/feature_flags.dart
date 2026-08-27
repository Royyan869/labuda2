import 'env_config.dart';
import 'package:flutter/foundation.dart';
import 'package:labuda/shared/shared.dart';

/// Simplified Feature Flags for Go Backend Migration
///
/// Since this is a NEW app with NO existing users:
/// - Development: Uses Go API
/// - Staging: Uses Go API
/// - Production: Uses Go API
///
/// Firebase is ONLY used for:
/// - Authentication (social login, OTP)
/// - FCM (Push Notifications)
/// - Crashlytics
/// - Analytics
///
/// All data operations go through Go Backend API.
class FeatureFlags {
  FeatureFlags._();

  // ============ Master Switch ============
  //
  // Master switch for Go Backend API
  // All modules use Go API by default
  // Individual module flags for future flexibility if needed
  static bool get useGoBackend => true;

  // ============ Per-Module Flags ============
  //
  // All modules use Go API by default
  // Individual module flags for future flexibility if needed
  static bool get profile => useGoBackend;
  static bool get order => useGoBackend;
  static bool get payment => useGoBackend;
  static bool get listing => useGoBackend;
  static bool get auction => useGoBackend;
  static bool get follow => useGoBackend;
  static bool get like => useGoBackend;
  static bool get comment => useGoBackend;
  static bool get rating => useGoBackend;
  static bool get content => useGoBackend;
  static bool get notification => useGoBackend;
  static bool get support => useGoBackend;
  static bool get report => useGoBackend;
  static bool get verification => useGoBackend;
  static bool get seller => useGoBackend;
  static bool get shortlist => useGoBackend;
  static bool get orderConfirmation => useGoBackend;
  static bool get discount => useGoBackend;
  static bool get coins => useGoBackend;
  static bool get admin => useGoBackend;
  static bool get chat => useGoBackend;
  static bool get shipping => useGoBackend;
  static bool get share => useGoBackend;

  // ============ Helper Methods ============
  //
  /// Check if a specific module is enabled for Go API
  static bool isModuleEnabled(String module) {
    // All modules are enabled - new app uses Go API directly
    return useGoBackend;
  }

  /// Check if user should use Go API (always true for new app)
  static bool shouldUserUseModuleApi(String? userId, String module) {
    // New app - always use Go API
    return useGoBackend;
  }

  /// Get all feature flags as a map (for debugging)
  static Map<String, bool> getAllFlags() {
    return {
      'useGoBackend': useGoBackend,
      'profile': profile,
      'order': order,
      'payment': payment,
      'listing': listing,
      'auction': auction,
      'follow': follow,
      'like': like,
      'comment': comment,
      'rating': rating,
      'content': content,
      'notification': notification,
      'support': support,
      'report': report,
      'verification': verification,
      'seller': seller,
      'shortlist': shortlist,
      'orderConfirmation': orderConfirmation,
      'discount': discount,
      'coins': coins,
      'admin': admin,
      'chat': chat,
      'shipping': shipping,
      'share': share,
    };
  }

  /// Get enabled modules (for debugging)
  static List<String> getEnabledModules() {
    return getAllFlags().entries
        .where((e) => e.value && e.key != 'useGoBackend')
        .map((e) => e.key)
        .toList();
  }

  /// Get current environment
  static String get environment => EnvConfig.environmentCode;

  /// Print configuration summary (for debug)
  static void printConfigSummary() {
    if (kDebugMode) {
      LoggerService.instance.debug(
        '====================================================',
      );
      LoggerService.instance.debug('FEATURE FLAGS CONFIGURATION');
      LoggerService.instance.debug(
        '====================================================',
      );
      LoggerService.instance.debug('Environment: $environment');
      LoggerService.instance.debug('Use Go Backend: $useGoBackend');
      LoggerService.instance.debug(
        'All Modules: ${useGoBackend ? "ENABLED (Go API)" : "DISABLED"}',
      );
      LoggerService.instance.debug(
        '====================================================',
      );
    }
  }
}
