/// Auth status providers
///
/// Non-authority auth projections used by routing and UI guards.
library;

import 'package:firebase_auth/firebase_auth.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/user/identity/authentication/domain/entities/firebase_principal.dart';
import 'authenticated_account_provider.dart';

/// SINGLE AUTHORITY: does the CURRENT Firebase identity actually hold a
/// password credential?
///
/// Credential availability is a FIREBASE identity fact — the linked-provider
/// list (`User.providerData`) is the only truthful source:
///   - email/password account        → ['password']            → true
///   - dual (Google + email/password) → ['google.com','password'] → true
///   - Google-only account            → ['google.com']          → false
/// `AuthUser.provider` from the backend is the REGISTRATION-ORIGIN provider
/// (a single enum value) and must never be used for this decision — it
/// cannot represent a dual-credential account.
///
/// The decision reuses the canonical [FirebasePrincipal.providerIds] capture
/// so the provider-id mapping lives in exactly one place.
///
/// FAIL-CLOSED: no current Firebase user (e.g., a HiShumi-only session after
/// a mid-session Firebase sign-out, or an unreadable identity) is NOT proof
/// that a password exists — the Change Password action stays unavailable.
/// The read is a stateless lookup of the live identity; there is no cache
/// that could leak a previous user's credential status across logout or an
/// account switch.
final hasPasswordCredentialProvider = Provider<bool>((ref) {
  final user = FirebaseAuth.instance.currentUser;
  if (user == null) {
    return false;
  }
  return FirebasePrincipal.fromFirebaseUser(
    user,
  ).providerIds.contains('password');
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

/// Provider to check if backend sync is in progress.
final isSyncingWithBackendProvider = Provider<bool>((ref) {
  final authState = ref.watch(authControllerProvider);
  return authState is AuthStateFirebaseAuthenticated ||
      authState is AuthStateSyncingWithBackend;
});
