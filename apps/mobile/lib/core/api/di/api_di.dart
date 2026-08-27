import 'package:get_it/get_it.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/api/config/api_config.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/core/src/services/service_locator.dart';

/// API Layer Dependency Injection
///
/// R4.1 DI ACCESS PATH STANDARDIZATION:
/// ======================================
/// This ApiDI is now **BOOTSTRAP/INFRASTRUCTURE ONLY**.
///
/// **CANONICAL DI PATH:** Riverpod Providers (ref.watch/ref.read)
/// - Use apiClientProvider for ApiClient
/// - Use loggerServiceProvider for ILoggerService
///
/// **ALLOWED USE CASES for ApiDI:**
/// 1. App initialization in main.dart to register ApiClient
/// 2. DI wrapper helpers that bridge to Riverpod providers
///
/// **DEPRECATED:**
/// - Direct ApiDI.apiClient access in feature code
/// - Use ref.watch(apiClientProvider) instead
///
/// Call `ApiDI.init()` during app initialization to register
/// all API-related dependencies.
class ApiDI {
  static final _sl = GetIt.instance;

  /// Initialize API dependencies
  ///
  /// Must be called after:
  /// - Firebase is initialized (AuthInterceptor uses FirebaseAuth directly)
  /// - ILoggerService is registered
  static void init({ApiEnvironment? environment}) {
    // Set environment if provided
    if (environment != null) {
      ApiConfig.setEnvironment(environment);
    }

    // Register ApiClient as lazy singleton
    // 🔧 FIX: No longer requires IAuthenticationService - AuthInterceptor uses FirebaseAuth directly
    if (!_sl.isRegistered<ApiClient>()) {
      _sl.registerLazySingleton<ApiClient>(
        () => ApiClient(logger: ServiceLocator.getService<ILoggerService>()),
      );
    }
  }

  /// Reset API dependencies (for testing)
  static void reset() {
    if (_sl.isRegistered<ApiClient>()) {
      _sl.unregister<ApiClient>();
    }
  }

  /// Get ApiClient instance
  ///
  /// R4.1: DEPRECATED for feature code - use apiClientProvider instead
  /// Kept for bootstrap compatibility
  static ApiClient get apiClient => _sl<ApiClient>();

  /// Get ILoggerService instance
  ///
  /// R4.1: DEPRECATED for feature code - use loggerServiceProvider instead
  /// Kept for bootstrap compatibility
  static ILoggerService get logger => _sl<ILoggerService>();
}
