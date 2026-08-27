import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';

/// Provider untuk NavigationHandler
///
/// Dependency injection pattern untuk navigation service
final navigationHandlerProvider = Provider<NavigationHandler>((ref) {
  // Return singleton AppRouter instance
  // AppRouter is now a wrapper that delegates to RouterNavigationImpl
  return AppRouter();
});

/// Extension untuk mendapatkan NavigationHandler dari Widget
extension NavigationExtension on WidgetRef {
  NavigationHandler get navigation => read(navigationHandlerProvider);
}

/// Helper untuk setup NavigationHandler tanpa context dependency
///
/// UPDATED: Router is now managed by Riverpod, no need to fetch from ServiceLocator
/// NavigationScope simply provides the AppRouter singleton which is already available
class NavigationScope extends ConsumerWidget {
  final Widget child;

  const NavigationScope({super.key, required this.child});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // NavigationHandler is provided via navigationHandlerProvider
    // which returns the AppRouter singleton
    // No need to fetch from ServiceLocator anymore
    return ProviderScope(
      overrides: [navigationHandlerProvider.overrideWithValue(AppRouter())],
      child: child,
    );
  }
}
