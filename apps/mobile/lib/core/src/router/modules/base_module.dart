import 'package:go_router/go_router.dart';

/// Abstract base class untuk module registration
///
/// Mengikuti NAVIGATION_ARCHITECTURE.md:
/// - Modular route registration pattern ✅
/// - Dependency injection per module ✅
/// - Clean separation of concerns ✅
abstract class BaseModule {
  /// Module name untuk logging dan debugging
  String get moduleName;

  /// Routes yang didefinisikan oleh module ini
  List<GoRoute> get routes;

  /// Initialize module dan register dependencies
  Future<void> initialize();

  /// Register routes ke router utama
  void registerRoutes(List<GoRoute> mainRoutes);

  /// Cleanup resources
  void dispose();
}
