// =============================================================================
// ARCHITECTURE GUARDRAIL (R5) - FEATURE MODULE BARREL FILE
// =============================================================================
//
// **CANONICAL PATTERN for Feature Barrel Files:**
// - Export domain entities (public API)
// - Export domain repository interfaces (i_*.dart)
// - Export domain use cases
// - Export presentation/providers (application layer)
// - Export presentation/screens and widgets
//
// **DO NOT EXPORT:**
// - data/models/* (internal implementation)
// - data/datasources/* (internal implementation)
// - data/repositories/*_impl.dart (internal implementation)
// - data/mappers/* (internal implementation)
// - data/services/* (unless explicitly shared)
//
// **USAGE:**
// ```dart
// import 'package:labuda/domains/user/profile/profile_feature.dart';
// // Now you have access to: ProfileEntity, IProfileRepository, etc.
// ```
//
// **IMPORT FROM OTHER FEATURES:**
// Use direct imports to specific files, NOT re-exports:
// ```dart
// import 'package:labuda/domains/user/identity/verification/verification.dart';  // ✅
// import 'package:labuda/domains/user/preference/seller/seller.dart';              // ✅
// ```
// =============================================================================

// Profile Feature - Barrel File
//
// Following RESTRUCTURE.md guidelines for modular architecture
//
// Exports:
// - Domain: entities, repository interfaces, use cases
// - Application: notifiers, state, validators
// - Presentation: providers, screens, widgets
//
// Usage in other features:
// ```dart
// import 'package:labuda/domains/user/profile/profile_feature.dart';
// ```

// ========================================
// DOMAIN LAYER
// ========================================
export 'domain/domain.dart';

// Data Layer - Providers (Riverpod)
export 'data/profile_providers.dart';

// Domain use cases
export 'domain/use_cases/get_profile_use_case.dart';
export 'domain/use_cases/update_profile_use_case.dart';

// ========================================
// APPLICATION LAYER
// ========================================
// Notifier exports are now in presentation/providers
export 'presentation/providers/notifiers/profile_notifier.dart';
export 'presentation/providers/notifiers/address_notifier.dart';
export 'presentation/providers/state/profile_state.dart';
export 'presentation/providers/state/address_state.dart';

// ========================================
// PRESENTATION LAYER
// ========================================
export 'presentation/presentation.dart';

// ========================================
// LEGACY PROVIDERS (backward compatibility)
// ========================================
// These will be migrated to presentation/providers eventually
// MIGRATION: Bootstrap pattern removed - now using data layer providers directly
export 'presentation/providers/profile_about_provider.dart';
export 'presentation/providers/profile_core_provider.dart'
    hide profileProvider, currentUserProfileProvider, ProfileActions;
export 'presentation/providers/profile_search_provider.dart';
export 'presentation/providers/profile_stream_provider.dart';
export 'presentation/providers/phone_verification_provider.dart';
export 'presentation/providers/address_list_provider.dart'
    hide primaryAddressProvider, addressCountProvider;
export 'presentation/providers/bank_account_provider.dart';
export 'presentation/providers/user_data_provider.dart';
export 'presentation/providers/blocked_users_provider.dart';

// Services exposed for other modules
export 'data/services/phone_verification_service.dart'
    show PhoneVerificationService;

// ========================================
// SCREENS (direct exports until barrel is created)
// ========================================
export 'presentation/screens/profile_screen.dart';
export 'presentation/screens/unified_edit_profile_screen.dart';
export 'presentation/screens/settings_screen.dart';
export 'presentation/screens/address_list_screen.dart';
export 'presentation/screens/security_screen.dart';
export 'presentation/screens/bank_account_screen.dart';
export 'presentation/screens/profile_qr_screen.dart';
export 'presentation/screens/ktp_camera_screen.dart';
export 'presentation/screens/selfie_camera_screen.dart';
export 'presentation/screens/privacy_policy_screen.dart';
export 'presentation/screens/terms_of_service_screen.dart';
export 'presentation/screens/blocked_users_screen.dart';
export 'presentation/screens/personal_information_screen.dart';

// ========================================
// WIDGETS (direct exports until barrel is created)
// ========================================
export 'presentation/widgets/profile_feed_tab.dart';
export 'presentation/widgets/profile_reviews_tab.dart';
export 'presentation/widgets/address_card_widget.dart';
export 'presentation/widgets/address_empty_state_widget.dart';
export 'presentation/widgets/address_form_dialog.dart';
export 'presentation/widgets/add_edit_address_dialog.dart';
export 'presentation/widgets/bank_account_card_widget.dart';
export 'presentation/widgets/bank_account_empty_state_widget.dart';
export 'presentation/widgets/personal_information_section.dart';
export 'presentation/widgets/ktp_upload_section.dart';
export 'presentation/widgets/ktp_preview_section.dart';
// REMOVED: achievement_badge.dart - NO backend support, deleted in PROFILE PURGE
export 'presentation/widgets/profile_avatar.dart';
export 'presentation/widgets/profile_cover.dart';
export 'presentation/widgets/profile_info.dart';
export 'presentation/widgets/profile_actions.dart';

// ========================================
// RE-EXPORTS FROM OTHER MODULES - REMOVED
// ========================================
// MIGRATION: Re-exports removed per CLIENT_MIGRATION_STANDARD.md
// - Verification: Import directly from domains/user/identity/verification/verification.dart
// - Seller: Import directly from domains/user/preference/seller/seller.dart
// This prevents tight coupling between features.

// ========================================
// NOTES
// ========================================
// ❌ NOT exported (data layer - internal only):
// - data/models/*
// - data/datasources/*
// - data/mappers/*
// - data/repositories/*_impl.dart
// - data/services/*
//
// ✅ Exported (public API):
// - domain/entities/*
// - domain/repositories/i_*.dart (interfaces only)
// - domain/use_cases/*
// - application/*
// - presentation/*

// =============================================================================
// ARCHITECTURE GUARDRAIL (R5) - DO NOT VIOLATE LAYER BOUNDARIES
// =============================================================================
//
// **DILARANG (FORBIDDEN):**
// - Importing data layer internals from other features
// - Directly accessing datasources or repositories_impl
// - Circumventing the provider pattern for service access
//
// **Use the barrel file import to access public API:**
// ```dart
// import 'package:labuda/domains/user/profile/profile_feature.dart';
// final profile = ref.watch(profileProvider);
// ```
// =============================================================================
