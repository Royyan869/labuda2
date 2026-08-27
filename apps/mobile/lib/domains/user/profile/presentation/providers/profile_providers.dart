// Profile Presentation Providers
// Phase 6: Wrapper providers for easy UI consumption
//
// These providers wrap the Presentation Layer Notifiers and provide
// convenient access patterns for UI components.

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'notifiers/profile_notifier.dart';
import '../../domain/entities/profile_entity.dart';

// Re-export the Notifier providers from Presentation layer
//
// Usage in UI:
// ```dart
// // Watch state
// final profileState = ref.watch(profileNotifierProvider);
//
// // Call methods
// final notifier = ref.read(profileNotifierProvider.notifier);
// notifier.fetchProfile(userId);
// ```

// ========================================
// PROFILE NOTIFIER PROVIDER (re-export)
// ========================================
export 'notifiers/profile_notifier.dart'
    show profileNotifierProvider, ProfileNotifier;

// ========================================
// CONVENIENCE PROVIDERS
// ========================================

/// Provider for current user's profile state
/// Convenience wrapper that auto-fetches on first read
final currentUserProfileProvider = Provider<AsyncValue<ProfileEntity?>>((ref) {
  final profileState = ref.watch(profileNotifierProvider);
  return profileState.profile;
});

/// Provider for profile loading state
final isProfileLoadingProvider = Provider<bool>((ref) {
  final profileState = ref.watch(profileNotifierProvider);
  return profileState.isUpdating;
});

/// Provider for profile error message
final profileErrorProvider = Provider<String?>((ref) {
  final profileState = ref.watch(profileNotifierProvider);
  return profileState.errorMessage;
});
