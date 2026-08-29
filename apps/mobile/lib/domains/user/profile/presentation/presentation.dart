// Profile Presentation Layer
// Phase 7: Cutover Complete - All UI in presentation/
//
// This layer contains:
// - Providers (Riverpod providers for easy UI consumption)
// - Screens (profile screens)
// - Widgets (profile widgets)
//
// Usage in UI:
// ```dart
// // TODO: import 'package:labuda/domains/user/profile/presentation/presentation.dart';
//
// // Watch profile state
// final profileState = ref.watch(profileNotifierProvider);
//
// // Or use convenience providers
// final profile = ref.watch(currentUserProfileProvider);
// final isLoading = ref.watch(isProfileLoadingProvider);
// ```

// ========================================
// PRESENTATION PROVIDERS
// ========================================
export 'providers/address_providers.dart';

// ========================================
// SCREENS & WIDGETS
// ========================================
// For now, import screens and widgets directly
// TODO: Create barrel files for screens/ and widgets/
