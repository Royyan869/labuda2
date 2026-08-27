/// Support Module - Clean Architecture
///
/// 🟡 TIPE B - BUSINESS MODULE (NON-CRITICAL) - DUAL SOURCE
/// Firestore & API Go coexist, switching via repository router.
///
/// Structure:
/// - domain/     : Entities, repositories interfaces (pure Dart)
/// - data/        : Datasources (Firestore + future API), repository impls
/// - application/ : Notifiers (business logic orchestration)
/// - presentation/: UI screens and widgets
///
/// Usage:
/// Override providers in main app:
/// ```dart
/// ProviderScope(
///   overrides: SupportDI.overrides(),
///   child: MyApp(),
/// )
/// ```
library;

// ============================================================================
// DOMAIN LAYER (Entities + Repository Interface)
// ============================================================================
export 'domain/domain.dart';

// ============================================================================
// DATA LAYER - Internal use only, NOT exported
// ============================================================================
// ❌ NOT exported: datasources, repository implementations
// (Firestore is current source, API Go will be added via dual-source router)

// ============================================================================
// APPLICATION LAYER (Notifiers + State)
// ============================================================================
export 'presentation/providers/support_state.dart';
export 'presentation/providers/support_notifier.dart';

// ============================================================================
// PRESENTATION LAYER (Providers + Screens + Widgets)
// ============================================================================
export 'presentation/presentation.dart';
