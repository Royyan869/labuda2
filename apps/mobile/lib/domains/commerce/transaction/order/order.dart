export 'domain/domain.dart';

// Data Layer - Providers (Riverpod)
export 'data/order_providers.dart';

// Data Layer - NOT EXPORTED (Implementation Detail)
// ❌ NOT exported: data/data.dart, data/dto/, data/mappers/, data/remote/
// These are internal to the module and accessed via repository interface only.
export 'presentation/providers/order_state.dart';
export 'presentation/providers/order_notifier.dart';
export 'presentation/providers/order_providers.dart';

// Screens - restored from flutter_old for router compatibility
export 'presentation/screens/order_list_screen.dart';
export 'presentation/screens/order_detail_screen.dart';

// Presentation widgets - complete widgets for order detail screens
export 'presentation/widgets/order_widgets.dart';
export 'presentation/widgets/refund_request_dialog.dart';
export 'presentation/widgets/auto_release_countdown_widget.dart';

// Dynamic Action Buttons - Decision V2 Contract
// Backend-driven action rendering (NO hardcoded logic)
export 'presentation/widgets/dynamic_action_buttons.dart';

// Export API response types from local order module
export 'data/models/api/order_api_models.dart';
