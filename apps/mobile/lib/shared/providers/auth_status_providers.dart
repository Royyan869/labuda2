/// Auth status providers
///
/// Non-authority auth projections used by routing and UI guards.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'authenticated_account_provider.dart';

/// Provider to check if user is authenticated.
///
/// Returns true only for [AuthStateAuthenticated].
final isAuthenticatedProvider = Provider<bool>((ref) {
  final authState = ref.watch(authControllerProvider);
  return authState is AuthStateAuthenticated;
});

/// Provider to get current user ID for account-scoped keys.
///
/// This is keyed from [authenticatedUserProvider] so it fails closed during
/// loading, syncing, degraded, and error states.
final currentUserIdProvider = Provider<String>((ref) {
  return ref.watch(authenticatedUserProvider)?.id ?? '';
});

/// Provider to check if current user's email is verified.
///
/// Source of truth: [AuthStateAuthenticated.emailVerified].
final isEmailVerifiedProvider = Provider<bool>((ref) {
  final authState = ref.watch(authControllerProvider);
  if (authState is AuthStateAuthenticated) {
    return authState.emailVerified;
  }
  return false;
});

/// Provider to check if user is authenticated but email not verified.
final isUnverifiedProvider = Provider<bool>((ref) {
  final isAuthenticated = ref.watch(isAuthenticatedProvider);
  final isEmailVerified = ref.watch(isEmailVerifiedProvider);
  return isAuthenticated && !isEmailVerified;
});

/// Provider to check if backend sync is in progress.
final isSyncingWithBackendProvider = Provider<bool>((ref) {
  final authState = ref.watch(authControllerProvider);
  return authState is AuthStateFirebaseAuthenticated ||
      authState is AuthStateSyncingWithBackend;
});

/// Provider to get auth error if any.
final authErrorProvider = Provider<String?>((ref) {
  final authState = ref.watch(authControllerProvider);
  if (authState is AuthStateError) {
    return authState.message;
  }
  return null;
});

/// Provider to check if current user is admin.
final isAdminProvider = Provider<bool>((ref) {
  final user = ref.watch(authenticatedUserProvider);
  return user?.isAdmin ?? false;
});
