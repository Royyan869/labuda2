// =============================================================================
// ARCHITECTURE GUARDRAIL (R5) - CANONICAL DI PATH FOR CORE SERVICES
// =============================================================================
//
// **THIS IS THE ONLY VALID WAY to access core services in feature code.**
//
// **DO NOT USE THESE DEPRECATED PATTERNS:**
// - `sl<T>()` for service access in feature code
// - `ApiDI.apiClient` / `ApiDI.logger` in feature code
// - `ServiceLocator.getService<T>()` in feature code
// - Direct singleton `.instance` access where provider exists
//
// **USE THESE INSTEAD:**
// - `ref.watch(apiClientProvider)` for ApiClient
// - `ref.read(loggerServiceProvider)` for ILoggerService
// - `ref.watch(authServiceProvider)` for IAuthenticationService
// - `ref.read(localStorageServiceProvider)` for ILocalStorageService
// - etc.
//
// **IMPORT PATH:**
// `import 'package:labuda/core/providers/core_providers.dart';`
//
// The providers here are overridden in main.dart with actual service instances.
// This hybrid approach allows gradual migration while maintaining a canonical
// Riverpod path for consumers.
// =============================================================================

/// Core Providers - Riverpod providers for global core services
///
/// R4.1 DI ACCESS PATH STANDARDIZATION:
/// ======================================
/// **THIS IS THE CANONICAL DI PATH for core services.**
///
/// All feature code SHOULD use these providers via ref.watch() or ref.read():
/// - apiClientProvider for ApiClient
/// - loggerServiceProvider for ILoggerService
/// - authServiceProvider for IAuthenticationService
/// - localStorageServiceProvider for ILocalStorageService
/// - etc.
///
/// **DEPRECATED PATTERNS:**
/// - `sl<T>()` for service access in feature code
/// - ApiDI.apiClient / ApiDI.logger in feature code
/// - `ServiceLocator.getService<T>()` in feature code
/// - Direct singleton .instance access where provider exists
///
/// These patterns are now BOOTSTRAP ONLY - used only in:
/// - main.dart for provider overrides
/// - app_initializer.dart for service registration
/// - DI wrapper files that bridge to providers
///
/// The providers here are overridden in main.dart with actual service instances
/// that are registered via GetIt during bootstrap. This hybrid approach allows
/// gradual migration while maintaining a canonical Riverpod path for consumers.
library;

// Imports
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/navigation/navigation_handler.dart';
import 'package:labuda/core/src/interfaces/services/i_authentication_service.dart';
import 'package:labuda/core/src/interfaces/services/i_local_storage_service.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/core/src/interfaces/services/i_presence_service.dart';
import 'package:labuda/core/src/interfaces/services/i_validation_service.dart';
import 'package:labuda/core/src/websocket/chat_websocket_handler.dart';
import 'package:labuda/core/websocket/websocket_service.dart';
import 'package:labuda/core/src/interfaces/services/i_analytics_repository.dart';
import 'package:labuda/core/interfaces/i_notification_trigger.dart';
import 'package:labuda/core/src/navigation/i_navigation_registry.dart';
import 'package:labuda/core/services/s3_service.dart';

// Re-exports for convenience
export 'package:labuda/core/api/api_client.dart';
export 'package:labuda/core/navigation/navigation_handler.dart';
export 'package:labuda/core/src/interfaces/services/i_authentication_service.dart';
export 'package:labuda/core/src/interfaces/services/i_local_storage_service.dart';
export 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
export 'package:labuda/core/src/interfaces/services/i_presence_service.dart';
export 'package:labuda/core/src/interfaces/services/i_validation_service.dart';
export 'package:labuda/core/src/websocket/chat_websocket_handler.dart';
export 'package:labuda/core/websocket/websocket_service.dart';
export 'package:labuda/core/src/interfaces/services/i_analytics_repository.dart';
export 'package:labuda/core/interfaces/i_notification_trigger.dart';
export 'package:labuda/core/src/navigation/i_navigation_registry.dart';
export 'package:labuda/core/services/s3_service.dart';

// =============================================================================
// CORE SERVICE PROVIDERS (to be overridden in main.dart)
// =============================================================================

/// Provider for ApiClient
///
/// This must be overridden in main.dart with the actual ApiClient instance.
/// Example:
/// ```dart
/// apiClientProvider.overrideWithValue(apiClient)
/// ```
final apiClientProvider = Provider<ApiClient>((ref) {
  throw UnimplementedError(
    'ApiClient must be provided externally. '
    'Override apiClientProvider in main.dart.',
  );
});

/// Provider for ILoggerService
///
/// This must be overridden in main.dart with the actual logger instance.
final loggerServiceProvider = Provider<ILoggerService>((ref) {
  throw UnimplementedError(
    'ILoggerService must be provided externally. '
    'Override loggerServiceProvider in main.dart.',
  );
});

/// Provider for IAuthenticationService
///
/// This must be overridden in main.dart with the actual auth service instance.
final authServiceProvider = Provider<IAuthenticationService>((ref) {
  throw UnimplementedError(
    'IAuthenticationService must be provided externally. '
    'Override authServiceProvider in main.dart.',
  );
});

/// Provider for ILocalStorageService
///
/// This must be overridden in main.dart with the actual storage instance.
final localStorageServiceProvider = Provider<ILocalStorageService>((ref) {
  throw UnimplementedError(
    'ILocalStorageService must be provided externally. '
    'Override localStorageServiceProvider in main.dart.',
  );
});

/// Provider for IValidationService
///
/// This must be overridden in main.dart with the actual validation service.
final validationServiceProvider = Provider<IValidationService>((ref) {
  throw UnimplementedError(
    'IValidationService must be provided externally. '
    'Override validationServiceProvider in main.dart.',
  );
});

/// Provider for IPresenceService
///
/// This must be overridden in main.dart with the actual presence service.
final presenceServiceProvider = Provider<IPresenceService>((ref) {
  throw UnimplementedError(
    'IPresenceService must be provided externally. '
    'Override presenceServiceProvider in main.dart.',
  );
});

/// Provider for NavigationHandler
///
/// This must be overridden in main.dart with the actual navigation handler.
final navigationHandlerProvider = Provider<NavigationHandler>((ref) {
  throw UnimplementedError(
    'NavigationHandler must be provided externally. '
    'Override navigationHandlerProvider in main.dart.',
  );
});

/// Provider for INavigationRegistry
///
/// This must be overridden in main.dart with the actual navigation registry.
/// Used for feature modules to register tabs without direct dependencies.
final navigationRegistryProvider = Provider<INavigationRegistry>((ref) {
  throw UnimplementedError(
    'INavigationRegistry must be provided externally. '
    'Override navigationRegistryProvider in main.dart.',
  );
});

/// Provider for WebSocketService
///
/// This must be overridden in main.dart with the actual WebSocket service.
final webSocketServiceProvider = Provider<WebSocketService>((ref) {
  throw UnimplementedError(
    'WebSocketService must be provided externally. '
    'Override webSocketServiceProvider in main.dart.',
  );
});

/// Provider for ChatWebSocketHandler
///
/// This must be overridden in main.dart with the actual chat WebSocket handler.
final chatWebSocketHandlerProvider = Provider<ChatWebSocketHandler>((ref) {
  throw UnimplementedError(
    'ChatWebSocketHandler must be provided externally. '
    'Override chatWebSocketHandlerProvider in main.dart.',
  );
});

/// Provider for IAnalyticsRepository
///
/// This must be overridden in main.dart with the actual analytics implementation.
/// Typically provided externally at app bootstrap.
///
/// **Usage in feature modules:**
/// ```dart
/// final analytics = ref.watch(coreAnalyticsRepositoryProvider);
/// ```
final coreAnalyticsRepositoryProvider = Provider<IAnalyticsRepository>((ref) {
  throw UnimplementedError(
    'IAnalyticsRepository must be provided externally. '
    'Override coreAnalyticsRepositoryProvider in main.dart. '
    'Use: coreAnalyticsRepositoryProvider.overrideWithValue(ServiceLocator.getService<IAnalyticsRepository>())',
  );
});

/// Provider for INotificationTrigger
///
/// This must be overridden in main.dart with the actual notification trigger.
/// Typically provided via NotificationDI.register() which registers to ServiceLocator.
///
/// **Usage in feature modules:**
/// ```dart
/// final notificationTrigger = ref.watch(coreNotificationTriggerProvider);
/// ```
///
/// **Note:** This provider is nullable - if not overridden, returns null.
/// Features should handle null gracefully (notification is optional).
final coreNotificationTriggerProvider = Provider<INotificationTrigger?>((ref) {
  // Return null by default - override in main.dart to enable notifications
  return null;
});

/// Provider for S3Service
///
/// S3Service is a stateless service for uploading media to AWS S3.
/// No override needed - creates a new instance directly.
///
/// **Usage in feature modules:**
/// ```dart
/// final s3Service = ref.watch(s3ServiceProvider);
/// ```
final s3ServiceProvider = Provider<S3Service>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  S3Service.setApiClient(apiClient);
  return S3Service();
});
