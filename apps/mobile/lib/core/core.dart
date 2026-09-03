// Core module public API export
library;

// Domain exports
export 'common/result.dart';
export 'errors/failure.dart';
export 'common/base_entity.dart';
export 'common/paginated_result.dart';
export 'package:labuda/domains/user/identity/authentication/authentication.dart';
export 'common/types/payment_types.dart';

// Configuration exports
export 'config/env_config.dart';
export 'config/feature_flags.dart';

// Navigation exports
export 'src/navigation/i_navigation_registry.dart';
export 'src/navigation/navigation_registry_impl.dart';

// Interface exports - Only core business interfaces, module-specific interfaces di module masing-masing
export 'src/interfaces/services/i_authentication_service.dart';
export 'src/interfaces/services/i_payment_service.dart';
export 'src/interfaces/services/i_notification_service.dart';
export 'src/interfaces/services/i_local_storage_service.dart';
export 'src/interfaces/services/i_logger_service.dart';
export 'src/interfaces/services/i_validation_service.dart';
export 'src/interfaces/services/i_analytics_repository.dart';
export 'src/interfaces/services/i_presence_service.dart';
export 'interfaces/i_notification_trigger.dart';
export 'interfaces/i_order_payment_handler.dart';

// Utilities exports
export 'src/utils/constants/app_constants.dart';
export 'src/utils/extensions/string_extensions.dart';
export 'src/utils/extensions/context_extensions.dart';

// Constants exports
export 'constants/koi_varieties.dart';
export 'src/enums/koi_gender.dart';

// Theme exports
export 'src/theme/app_theme.dart';
export 'src/theme/app_colors.dart';
export 'src/theme/app_typography.dart';
export 'src/theme/theme_provider.dart';

// Localization exports
export 'src/localization/localization_provider.dart';
export 'src/localization/localization_helper.dart';
export 'src/localization/localization_types.dart';
export 'src/localization/l10n_extension.dart';

// Event exports
export 'src/events/event_bus.dart';
export 'src/events/base_event.dart';

// Router exports
export 'src/router/route_paths.dart';
export 'src/router/app_router.dart';

// Service Locator exports
export 'src/services/service_locator.dart';

// S3 Service exports
export 'services/s3_service.dart';

// Navigation exports
export 'navigation/navigation_handler.dart';
export 'navigation/app_navigation_handler.dart';
export 'navigation/navigation_provider.dart';

// Quick Actions Service exports
export 'src/services/quick_actions_service.dart';

// Presence tracking exports
export 'src/services/app_presence_service_api.dart';
export 'src/services/app_lifecycle_observer.dart';
export 'src/providers/presence_provider.dart';

// API Layer exports (Backend Integration)
export 'api/api.dart';
export 'api/models/common_api_models.dart';

// Core Providers (ApiClient, ILoggerService, etc.)
// New Riverpod providers - replacing GetIt/ServiceLocator gradually
export 'providers/core_providers.dart' hide navigationHandlerProvider;
// Legacy bridge - TODO: Remove after migration complete
export '../../shared/providers/core_providers.dart';

// Core Infra exports (Messaging)
export 'messaging/notification_service.dart';
export 'observability/providers.dart';

// Auth Helper exports
// Canonical Role Vocabulary - SINGLE SOURCE OF TRUTH for all Flutter roles
export 'src/auth/app_role.dart';
// Permission Helper - canonical access interpretation layer
export 'src/auth/permission_helper.dart';

// ADMIN DOMAIN REMOVED: Admin functionality moved to apps/admin (React)
// Mobile app now focuses on core user features only
