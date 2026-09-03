// =============================================================================
// ARCHITECTURE GUARDRAIL (R5) - THIS IS NOT A DUMPING GROUND
// =============================================================================
//
// **WHAT BELONGS HERE:**
// - Platform-level primitives (models, enums, entities)
// - Truly reusable UI components (buttons, cards, inputs, etc.)
// - Infrastructure services (localStorage, validation, logger, etc.)
//
// **WHAT DOES NOT BELONG HERE:**
// - Feature-specific business logic
// - Domain services that belong to a specific feature
// - Cross-domain shortcuts that bypass ownership
//
// **RULE OF THUMB:** If it has "business logic" or belongs to a specific domain,
// it should live in `features/<domain>/` NOT here.
//
// Before adding exports here, ask:
// 1. Is this truly reusable across ALL features?
// 2. Does this have domain-specific business logic?
// 3. Is there a clear owner feature that should manage this?
//
// If unsure, keep it in the feature module.
// =============================================================================

library;

// ⭐ Unified Attachment System (entities, mappers, type aliases)
export 'attachment/attachment.dart';

// Domain enums exports
// HARD CLEANUP BATCH: RequestStatus removed - use ContentStatus from domains/social/content/domain/entities/content.dart

// Widget exports
export 'widgets/profile_avatar.dart'; // ⭐ Avatar component
export 'widgets/hybrid_avatar.dart'; // ⭐ Hybrid avatar with smart caching
export 'widgets/follow_button.dart'; // ⭐ Follow button component
export 'widgets/app_button.dart'; // ⭐ Modern buttons
export 'widgets/app_text_field.dart'; // ⭐ Modern text fields
export 'widgets/app_dropdown.dart'; // ⭐ Modern dropdown fields
export 'widgets/app_date_picker.dart'; // ⭐ Modern date picker
export 'widgets/password_strength_indicator.dart'; // ⭐ Password validation
export 'widgets/app_logo.dart'; // ⭐ Logo component
export 'widgets/app_back_button.dart'; // ⭐ Navigation
export 'widgets/app_bar_custom.dart'; // ⭐ Custom AppBar
export 'widgets/app_snackbar.dart'; // ⭐ Notifications
export 'widgets/base_card.dart'; // ⭐ Base card component
export 'widgets/loading_indicator.dart'; // ⭐ Loading states
export 'widgets/error_widget.dart'; // ⭐ Error handling
export 'widgets/app_image.dart'; // ⭐ Image component
export 'widgets/keyboard_dismiss_wrapper.dart'; // ⭐ Global keyboard dismiss
// export 'widgets/presence_auth_sync.dart';    // ✅ Removed - migrated to API-based presence
export 'widgets/online_avatar_widget.dart'; // ⭐ Avatar with online status indicator
// export 'widgets/quick_create_bar.dart';     // ✅ Removed - using bottom nav + icon instead
export 'widgets/language_selector.dart'; // ⭐ Language selection
export 'widgets/theme_selector.dart'; // ⭐ Theme selection
export 'widgets/action_buttons.dart'; // ⭐ Action buttons
export 'widgets/avatar_editor_widget.dart'; // ⭐ Avatar editor
export 'widgets/app_modal.dart'; // ⭐ Reusable modal dialogs
export 'widgets/app_bottom_sheet.dart'; // ⭐ Professional bottom sheets
// export 'widgets/upload_post_bottom_sheet.dart'; // ✅ Moved to respective feature modules
// export 'widgets/upload_request_bottom_sheet.dart'; // ✅ Moved to respective feature modules
// export 'widgets/media_picker_modal.dart';     // ⭐ Professional media picker
// export 'widgets/user_selection_modal.dart';   // ⭐ User selection with search
// export 'widgets/creation_modals.dart'; // ✅ Removed - creation module deleted
export 'widgets/wilayah_dropdown.dart'; // ⭐ Wilayah dropdown
export 'widgets/village_dropdown.dart'; // ⭐ Village dropdown
export 'widgets/village_search_dropdown.dart'; // ⭐ Village search dropdown
// export 'widgets/area_search_field.dart';       // ❌ Removed - too complex
// export 'widgets/universal_feed_card.dart';    // ❌ Removed - use modular cards from respective modules
// export 'widgets/home_screen.dart';        // ❌ Removed - home logic belongs to features/home module
// export 'widgets/firebase_test_widget.dart'; // ❌ Removed - Firebase debug widget deleted
export 'widgets/create_content_bottom_sheet.dart'; // ⭐ Create content modal
export 'widgets/link_picker_modal.dart'; // ⭐ Link picker modal for attachments
export 'widgets/attachment_widget.dart'; // ⭐ Universal attachment widget
export 'widgets/repost_attribution_bar.dart'; // ⭐ SHARE CONTRACT V1: Repost attribution widget
export 'widgets/media_carousel_widget.dart'; // ⭐ Media carousel component
export 'widgets/media_viewer_widget.dart'; // ⭐ Media fullscreen viewer
export 'widgets/media_preview.dart'; // ⭐ Media preview for creation forms (moved from creation module)
export 'widgets/time_ago_widget.dart'; // ⭐ Time ago component
export 'widgets/wizard_progress_indicator.dart'; // ⭐ Reusable wizard progress indicator
export 'widgets/interactive_map_picker_bottom_sheet.dart'; // ⭐ Interactive map picker with draggable pin
export 'widgets/clickable_location_widget.dart'; // ⭐ Clickable location widget that opens Google Maps
export 'widgets/coordinate_preview_modal.dart'; // ⭐ Coordinate preview modal with Edit & View Maps options
export 'widgets/permission_guard.dart'; // ⭐ Auth & Role guard widgets
export 'widgets/blocked_user_banner.dart'; // ⭐ Blocked user banner widget
export 'widgets/external_link_interstitial.dart'; // ⭐ External link safety interstitial

// Mention system exports
export 'utils/mention_parser.dart'; // ⭐ Mention parsing utilities (CROSS_DOMAIN_PRIMITIVE - pure text parsing)
export 'widgets/mentions/mention_rich_text.dart'; // ⭐ Display text with clickable mentions (R2.2 MIGRATED: now imports from search domain)
export 'widgets/mentions/mention_text_field.dart'; // ⭐ TextField with mention autocomplete (R2.2 MIGRATED: now imports from search domain)
export 'widgets/mentions/mention_suggestion_overlay.dart'; // ⭐ Mention suggestion dropdown (R2.2 MIGRATED: now imports from search domain)
// ✅ R2.2 REMOVED: mention_search_api_provider.dart - MIGRATED to features/search/presentation/providers/mention_providers.dart
// ✅ R2.2 REMOVED: mention_resolver_api_provider.dart - MIGRATED to features/search/presentation/providers/mention_providers.dart
// Import from search domain instead: import 'package:labuda/features/search/search/search.dart' show mentionUserSearchProvider, mentionResolverProvider;
export 'providers/core_providers.dart'; // ⭐ Core providers (ApiClient, ILoggerService)
export 'widgets/detail_chip_widget.dart'; // ⭐ Detail chip component
export 'widgets/user_header_widget.dart'; // ⭐ Reusable user header component
export 'widgets/custom_rp_icon.dart'; // ⭐ Custom Rupiah icon
export 'widgets/popup_more_options_button.dart'; // ⭐ Popup 3 dots menu
export 'src/widgets/expandable_text_widget.dart'; // ⭐ Expandable text widget like Facebook
export 'src/widgets/expandable_mention_text_widget.dart'; // ⭐ Expandable text with clickable mentions
// export 'ui/src/widgets/action_button_widget.dart'; // ✅ Removed - using PostCardActions in respective modules
export 'ui/src/widgets/text_input_widget.dart'; // ⭐ Reusable text input widget for chat and comments
export 'ui/src/widgets/text_input_actions.dart'; // ⭐ Quick action buttons for text input
export 'ui/src/helpers/media_picker_helper.dart'; // ⭐ Media picker helper for gallery selection
export 'ui/src/screens/custom_camera_screen.dart'; // ⭐ Custom camera screen
export 'src/widgets/upload_progress_widget.dart'; // ⭐ Upload progress component
export 'src/widgets/upload_task_utils.dart'; // ⭐ Upload task utilities
export 'src/widgets/empty_state_widget.dart'; // ⭐ Reusable empty state component
// export 'widgets/common_button.dart';        // ✅ Removed - use AppButton
// export 'widgets/common_text_field.dart';    // ✅ Removed - use AppTextField
// export 'widgets/empty_state_widget.dart';
// export 'widgets/image_picker_widget.dart';
// export 'widgets/bottom_sheet_widget.dart';

// Model exports
export 'models/wilayah_models.dart'; // ⭐ Wilayah models
// export 'models/api_response.dart';
// export 'models/pagination_model.dart';

// Provider exports
export 'providers/wilayah_provider_simple.dart'; // ⭐ Wilayah state management
export 'providers/authenticated_account_provider.dart'; // ⭐ Hydrated account authority
export 'providers/auth_status_providers.dart'; // ⭐ Auth status / guard providers
export 'src/providers/upload_progress_provider.dart'; // ⭐ Upload progress state management

// Helper exports
export 'helpers/user_initials_helper.dart'; // ⭐ Consistent user initials generation

// Utils exports
export 'utils/currency_input_formatter.dart'; // ⭐ Currency input formatter for Rupiah
export 'utils/currency_utils.dart'; // ⭐ Centralized currency formatting utility
export 'utils/app_formatters.dart'; // ⭐ Centralized formatting utility (currency + date)

// ============================================================================
// SHARED INFRASTRUCTURE SERVICES
// ============================================================================
// These are platform-level infrastructure services that are genuinely reusable
// across all features. NOT domain-specific business logic.
export 'services/local_storage_service.dart';
export 'services/validation_service.dart'; // PLATFORM_INFRA: Generic validation primitives
export 'services/logger_service.dart';
export 'services/local_wilayah_service.dart'; // Local Wilayah service (offline-first)
export 'services/firebase_wilayah_service.dart'; // Firebase Wilayah service
export 'services/postal_code_service.dart'; // Postal code lookup service
export 'services/places_autocomplete_service.dart'; // Google Places autocomplete
export 'services/location_service.dart'; // GPS location service with accuracy tracking

// ============================================================================
// REMOVED EXPORTS - Migrated to Domain Owners
// ============================================================================
// ✅ REMOVED: visibility_filter_api_service.dart
//    -> Belongs in features/follow/data/ (follow relationship management)
//    -> No active consumers in app code
//
// ✅ REMOVED: StorePhotoUploadService re-export
//    -> Owner: domains/user/preference/seller/data/services/store_photo_upload_service.dart
//    -> Import from: domains/user/preference/seller/data/data.dart
//
// ✅ REMOVED (R2.1): feed_aggregation_service.dart
//    -> Use FeedApiDatasource in features/home/data/ instead
//
// ✅ REMOVED (R2.3): user_avatar_api_service.dart
//    -> Migrated to domains/user/profile/data/services/avatar_cache_service.dart
//
// ✅ REMOVED (R2.3): user_search_api_service.dart
//    -> Migrated to domains/user/profile/data/services/user_lookup_service.dart
//

