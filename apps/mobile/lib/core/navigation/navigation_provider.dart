import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';

/// Provider untuk NavigationHandler
///
/// CANONICAL navigation provider — SINGLE declaration for
/// `navigationHandlerProvider` in the app.
///
/// Dependency injection pattern untuk navigation service.
/// AppRouter drives the active Riverpod-managed GoRouter via the global
/// navigatorKey (see core/src/router/app_router.dart).
final navigationHandlerProvider = Provider<NavigationHandler>((ref) {
  return AppRouter();
});

/// Extension untuk mendapatkan NavigationHandler dari Widget
extension NavigationExtension on WidgetRef {
  NavigationHandler get navigation => read(navigationHandlerProvider);
}

/// Helper untuk binding NavigationHandler di dalam subtree app.
///
/// [NavigationScope] provides the canonical [navigationHandlerProvider] to the
/// whole router subtree (see app.dart) so every screen resolves the same
/// AppRouter-backed handler regardless of the surrounding ProviderScope.
class NavigationScope extends ConsumerWidget {
  final Widget child;

  const NavigationScope({super.key, required this.child});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return ProviderScope(
      overrides: [navigationHandlerProvider.overrideWithValue(AppRouter())],
      child: child,
    );
  }
}
