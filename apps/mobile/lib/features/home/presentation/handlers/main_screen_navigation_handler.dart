/// Main Screen local navigation orchestration
///
/// CONVERGED (single navigation authority = navigationHandlerProvider):
/// Pure delegation methods were removed — MainScreen/drawer now call the
/// canonical [navigationHandlerProvider] directly (same pattern already used
/// by MainAppBar).
///
/// This class retains ONLY orchestration that is local to the MainScreen UI:
/// - sign out (auth controller side effect)
/// - own-profile navigation gated on authenticated state
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';

/// Main Screen local navigation orchestration (drawer actions and auth gates).
class MainScreenNavigationHandler {
  final WidgetRef ref;
  final BuildContext context;

  MainScreenNavigationHandler({required this.ref, required this.context});

  /// Handle sign out — auth side effect only.
  /// Route outcome after sign-out is decided by the router redirect.
  VoidCallback get handleSignOut => () {
    ref.read(authControllerProvider.notifier).signOut();
  };

  /// Handle profile — own profile is only reachable when authenticated.
  VoidCallback get handleProfile => () {
    final authState = ref.read(authControllerProvider);
    if (authState is AuthStateAuthenticated) {
      ref.read(navigationHandlerProvider).navigateToProfile();
    }
  };
}
