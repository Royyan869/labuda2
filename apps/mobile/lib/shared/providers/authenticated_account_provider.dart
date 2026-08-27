/// Authenticated account provider
///
/// Hydrated authority only. No presentation continuity, no fallback snapshot,
/// and no synthesis from Firebase principal.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';

/// Hydrated account authority.
///
/// Returns the fully hydrated account only when [AuthStateAuthenticated]
/// is active; all other auth states resolve to null.
final authenticatedUserProvider = Provider<AuthUser?>((ref) {
  final authState = ref.watch(authControllerProvider);
  return switch (authState) {
    AuthStateAuthenticated(:final user) => user,
    _ => null,
  };
});
