/// Home Module - Clean Architecture
///
/// 🟢 TIPE A - SUPPORT/INFRA MODULE (FREEZE & ISOLATE)
/// Feed data fetched via FeedApiDatasource from /api/v1/feed endpoint.
library;

// ============================================================================
// DOMAIN LAYER (Entities + Repository Interface)
// ============================================================================
export 'domain/domain.dart';

// ============================================================================
// DATA LAYER - Internal use only, NOT exported
// ============================================================================
// ❌ NOT exported: data/home_repository_impl.dart
// Repository impl diakses via provider di presentation layer

// ============================================================================
// PRESENTATION LAYER (Notifiers + State + Providers + Screens)
// ============================================================================
export 'presentation/providers/feed/feed_state.dart';
export 'presentation/providers/feed/feed_notifier.dart';
export 'presentation/providers/tab/tab_switch_state.dart';
export 'presentation/providers/tab/tab_switch_notifier.dart';
export 'presentation/providers/home_providers.dart';
export 'presentation/screens/home_screen.dart';
export 'presentation/screens/main_screen.dart';

// Main Navigation Components
export 'presentation/models/main_tab.dart';
export 'presentation/widgets/main_app_bar.dart';
export 'presentation/widgets/main_bottom_navigation.dart';
export 'presentation/widgets/main_drawer/main_drawer.dart';
export 'presentation/widgets/main_drawer/drawer_header.dart';
export 'presentation/widgets/main_drawer/drawer_item.dart';
export 'presentation/widgets/main_drawer/drawer_footer.dart';
export 'presentation/handlers/main_screen_navigation_handler.dart';
export 'navigation/home_navigation.dart';
