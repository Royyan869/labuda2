/// Notification Module - Public API
///
/// Exports untuk digunakan oleh modul lain.
/// ONLY export entities, screens, widgets, dan providers.
/// DO NOT export models, datasources, atau implementation details.
library;

// ============================================================================
// Domain Entities (Public)
// ============================================================================
export 'domain/entities/notification_entity.dart';
export 'domain/entities/notification_preference_entity.dart';

// ============================================================================
// Domain Repository Interface (Public - untuk DI registration)
// ============================================================================
export 'domain/repositories/i_notification_repository.dart';

// ============================================================================
// Domain Use Cases (Public - untuk DI registration)
// ============================================================================
export 'domain/use_cases/delete_all_notifications_use_case.dart';
export 'domain/use_cases/delete_notification_use_case.dart';
export 'domain/use_cases/delete_read_notifications_use_case.dart';
export 'domain/use_cases/get_notifications_use_case.dart';
export 'domain/use_cases/get_preferences_use_case.dart';
export 'domain/use_cases/get_unread_count_use_case.dart';
export 'domain/use_cases/mark_all_as_read_use_case.dart';
export 'domain/use_cases/mark_as_read_use_case.dart';
export 'domain/use_cases/update_preferences_use_case.dart';

// ============================================================================
// Presentation - Screens (Public)
// ============================================================================
export 'presentation/screens/notification_list_screen.dart';
export 'presentation/screens/notification_settings_screen.dart';

// ============================================================================
// Presentation - Widgets (Public)
// ============================================================================
export 'presentation/widgets/notification_badge_widget.dart';
export 'presentation/widgets/notification_empty_state_widget.dart';
export 'presentation/widgets/notification_item_widget.dart';
export 'presentation/widgets/preference_toggle_widget.dart';
export 'presentation/widgets/notification_initializer.dart';
export 'presentation/widgets/notification_list_app_bar.dart';
export 'presentation/widgets/notification_list_content.dart';
export 'presentation/widgets/notification_dismissible_item.dart';
export 'presentation/widgets/notification_settings_section.dart';
export 'presentation/widgets/in_app_notification_banner.dart';

// ============================================================================
// Presentation - Providers (Public)
// ============================================================================
export 'presentation/providers/navigation_provider.dart';
export 'presentation/providers/notification_list_provider.dart';
export 'presentation/providers/notification_settings_provider.dart';
export 'presentation/providers/unread_count_provider.dart';

// ============================================================================
// Handlers & Helpers (Public - untuk notification routing)
// ============================================================================
export 'presentation/helpers/notification_dialog_helper.dart';

// ============================================================================
// Data Layer - Providers (Riverpod)
// ============================================================================
export 'data/notification_providers.dart';

// ============================================================================
// Dependency Injection (Public)
// ============================================================================
// FCM & Local Notification services (still using GetIt for now)
export 'di/notification_di.dart';

// ============================================================================
// NOTES:
// ============================================================================
// 1. Data layer TIDAK di-export (models, datasources, repositories_impl, mappers)
// 2. Services (FCM, local notification) hanya untuk internal DI
// 3. INotificationTrigger interface ada di lib/core/interfaces/
// 4. NotificationType enum ada di lib/core/interfaces/i_notification_trigger.dart
