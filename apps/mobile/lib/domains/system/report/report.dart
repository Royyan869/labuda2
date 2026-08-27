/// Report Refactor Module
///
/// Refactored report module following clean architecture principles.
/// - Domain: Pure entities and repository interfaces
/// - Data: API-based datasources and repository implementations
/// - Application: Riverpod notifiers (no UseCase classes)
/// - Presentation: Providers for UI consumption
library;

// Domain
export 'domain/entities/entities.dart';
export 'domain/repositories/repositories.dart';

// Data Layer - NOT EXPORTED (Implementation Detail)
// ❌ NOT exported: data/data.dart
// Data layer is internal and accessed via repository interface only.

// Application (states and notifiers, excluding providers)
export 'presentation/providers/report/report_state.dart';
export 'presentation/providers/appeal/appeal_state.dart';

// Presentation (main providers)
export 'presentation/presentation.dart';

// Presentation (Screens)
export 'presentation/screens/report_screen.dart' show ReportScreen;

// DI
export 'report_di.dart';
