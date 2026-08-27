import 'package:get_it/get_it.dart';
import 'package:labuda/core/src/interfaces/services/i_local_storage_service.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/core/src/interfaces/services/i_presence_service.dart';
import 'package:labuda/core/src/interfaces/services/i_validation_service.dart';
import 'package:labuda/core/src/navigation/i_navigation_registry.dart';
import 'package:labuda/core/src/navigation/navigation_registry_impl.dart';
import 'package:labuda/core/navigation/navigation_handler.dart';
import 'package:labuda/core/websocket/websocket_service.dart';
import 'package:labuda/core/src/websocket/chat_websocket_handler.dart';

// =============================================================================
// ARCHITECTURE GUARDRAIL (R5) - SERVICE LOCATOR USAGE RULES
// =============================================================================
//
// **CRITICAL:** This ServiceLocator is now **BOOTSTRAP/INFRASTRUCTURE ONLY**.
//
// **THESE ARE DANGEROUS PATHS - DO NOT USE IN FEATURE CODE:**
// - generic `sl` access in feature widgets/providers
// - generic `ServiceLocator.getService` access in feature layers
// - Any direct GetIt access in business logic
//
// **USE THESE INSTEAD (Riverpod Providers):**
// - `apiClientProvider` for ApiClient
// - `loggerServiceProvider` for ILoggerService
// - `authServiceProvider` for IAuthenticationService
// - `localStorageServiceProvider` for ILocalStorageService
// - etc.
//
// **ALLOWED USE CASES for generic `sl` access:**
// 1. App initialization/bootstrap in main.dart, app_initializer.dart
// 2. DI wrapper files that bridge to Riverpod (e.g., NotificationDI.register())
// 3. Bridge providers that internally use generic `sl` but expose Provider API
//
// Import canonical providers from:
// - `package:labuda/core/providers/core_providers.dart` (core services)
// - `package:labuda/features/feature_name/data/feature_providers.dart` (feature data)
// =============================================================================

/// Clean Service Locator sesuai GUIDELINES.md
///
/// R4.1 DI ACCESS PATH STANDARDIZATION:
/// ======================================
/// This ServiceLocator is now **BOOTSTRAP/INFRASTRUCTURE ONLY**.
///
/// **CANONICAL DI PATH:** Riverpod Providers (ref.watch/ref.read)
/// - Use loggerServiceProvider for ILoggerService
/// - Use apiClientProvider for ApiClient
/// - Use presenceServiceProvider for IPresenceService
/// - etc.
///
/// **ALLOWED USE CASES for generic `sl` access:**
/// 1. App initialization/bootstrap in main.dart, app_initializer.dart
/// 2. DI wrapper files that bridge to Riverpod (e.g., NotificationDI.register())
/// 3. Bridge providers that internally use generic `sl` but expose Provider API
///
/// **DEPRECATED USE CASES:**
/// - Direct generic `sl` access in feature widgets/providers
/// - Direct generic `sl` access in business logic
/// - Any consumer-facing code should use Riverpod providers instead
///
/// Rules yang diikuti:
/// - Tidak ada circular dependency
/// - Tidak ada cross-import ke src/ modul lain
/// - Core hanya mengelola core services
/// - Feature modules register dependencies sendiri
final GetIt sl = GetIt.instance;

class ServiceLocator {
  static Future<void> init() async {
    // Hanya init core services
    await _initCoreServices();
  }

  static Future<void> _initCoreServices() async {
    // Register navigation registry
    if (!sl.isRegistered<INavigationRegistry>()) {
      sl.registerSingleton<INavigationRegistry>(NavigationRegistryImpl());
    }

    // Core services registration akan dilakukan dari main.dart
    // Tidak ada hardcoded dependencies di sini
  }

  /// Register core services dari main.dart
  static void registerLogger(ILoggerService logger) {
    if (!sl.isRegistered<ILoggerService>()) {
      sl.registerSingleton<ILoggerService>(logger);
    }
  }

  static void registerValidation(IValidationService validation) {
    if (!sl.isRegistered<IValidationService>()) {
      sl.registerSingleton<IValidationService>(validation);
    }
  }

  static void registerLocalStorage(ILocalStorageService storage) {
    if (!sl.isRegistered<ILocalStorageService>()) {
      sl.registerSingleton<ILocalStorageService>(storage);
    }
  }

  static void registerNavigationHandler(NavigationHandler handler) {
    if (!sl.isRegistered<NavigationHandler>()) {
      sl.registerSingleton<NavigationHandler>(handler);
    }
  }

  static void registerPresenceService(IPresenceService presenceService) {
    if (!sl.isRegistered<IPresenceService>()) {
      sl.registerSingleton<IPresenceService>(presenceService);
    }
  }

  static void registerWebSocketService(WebSocketService webSocketService) {
    if (!sl.isRegistered<WebSocketService>()) {
      sl.registerSingleton<WebSocketService>(webSocketService);
    }
  }

  static void registerChatWebSocketHandler(ChatWebSocketHandler chatHandler) {
    if (!sl.isRegistered<ChatWebSocketHandler>()) {
      sl.registerSingleton<ChatWebSocketHandler>(chatHandler);
    }
  }

  /// Feature modules register their own dependencies
  static void registerService<T extends Object>(T service) {
    if (!sl.isRegistered<T>()) {
      sl.registerSingleton<T>(service);
    }
  }

  static void registerLazyService<T extends Object>(T Function() factory) {
    if (!sl.isRegistered<T>()) {
      sl.registerLazySingleton<T>(factory);
    }
  }

  /// Get service instance
  ///
  /// **DEPRECATED FOR FEATURE USE - DO NOT USE IN FEATURE CODE**
  /// This method is for bootstrap/infrastructure only.
  ///
  /// In feature widgets/providers, use Riverpod providers instead:
  /// - `ref.watch(apiClientProvider)` instead of `sl<ApiClient>()`
  /// - `ref.read(loggerServiceProvider)` instead of `sl<ILoggerService>()`
  ///
  /// Allowed contexts:
  /// - main.dart initialization
  /// - app_initializer.dart
  /// - DI wrapper files that bridge to Riverpod
  static T getService<T extends Object>() {
    return sl.get<T>();
  }

  /// Check if service is registered
  ///
  /// **DEPRECATED FOR FEATURE USE - DO NOT USE IN FEATURE CODE**
  /// This method is for bootstrap/infrastructure only.
  static bool isServiceRegistered<T extends Object>() {
    return sl.isRegistered<T>();
  }

  /// Reset service locator
  ///
  /// **DANGER:** Only use this in tests. Never call this in production.
  static void reset() {
    sl.reset();
  }
}

/// Extension untuk convenience
extension ServiceLocatorExtensions on GetIt {
  T getService<T extends Object>() => get<T>();
  bool isServiceRegistered<T extends Object>() => isRegistered<T>();
}
