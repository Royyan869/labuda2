/// Seller Module - Clean Architecture Implementation
///
/// Refactored seller module following clean architecture principles:
/// - Domain: Pure Dart entities and repository interfaces
/// - Data: DTOs, Mappers, and Datasources (Firebase/API isolated)
/// - Presentation: Dumb UI widgets and screens
library;

// ============================================
// DOMAIN
// ============================================
export 'domain/entities/seller_dashboard.dart';
export 'domain/entities/seller_analytics.dart';
export 'domain/entities/seller_earnings.dart';
export 'domain/entities/seller_activity.dart';
export 'domain/entities/seller_subscription.dart';

// ============================================
// SELLER UPGRADE CONFIG (for use by other modules)
// ============================================
export 'package:labuda/core/config/seller_upgrade_config.dart';
export 'package:labuda/core/config/seller_upgrade_config_entity.dart';
export 'package:labuda/core/config/seller_upgrade_config_service.dart';
export 'package:labuda/core/config/seller_upgrade_config_provider.dart';

// ============================================
// DOMAIN REPOSITORIES
// ============================================
export 'domain/repositories/seller_repository.dart';

// ============================================
// DOMAIN BARREL
// ============================================
export 'domain/domain.dart';

// ============================================
// DATA LAYER - Providers (Riverpod)
// ============================================
export 'data/seller_providers.dart';

// ============================================
// APPLICATION
// ============================================
export 'presentation/providers/providers.dart';

// ============================================
// DI HELPER
// ============================================
export 'seller_di.dart';

// ============================================
// DATA - NOT EXPORTED (Implementation Detail)
// ============================================
// ❌ NOT exported: data/data.dart
// Data layer is internal and accessed via repository interface only.

// ============================================
// PRESENTATION
// ============================================
export 'presentation/presentation.dart';
