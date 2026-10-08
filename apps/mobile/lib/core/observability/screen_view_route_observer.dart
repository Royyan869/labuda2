import 'dart:async';

import 'package:flutter/widgets.dart';

import 'package:labuda/core/src/interfaces/services/i_analytics_repository.dart';
import 'screen_names.dart';

/// Route observer that emits canonical screen views through the single
/// analytics sink.
///
/// Screen identity is resolved by [AnalyticsScreen] from the route's canonical
/// name — NEVER from a dynamic path segment. Resource identifiers (user,
/// product, auction, order ids) never reach the analytics backend as a screen
/// name.
///
/// This is the ONE screen-tracking authority. Do not add per-screen manual
/// screen tracking alongside it.
class ScreenViewRouteObserver extends RouteObserver<PageRoute<dynamic>> {
  final IAnalyticsRepository _analytics;

  ScreenViewRouteObserver(this._analytics);

  @override
  void didPush(Route<dynamic> route, Route<dynamic>? previousRoute) {
    super.didPush(route, previousRoute);
    _trackScreenView(route);
  }

  @override
  void didPop(Route<dynamic> route, Route<dynamic>? previousRoute) {
    super.didPop(route, previousRoute);
    if (previousRoute != null) {
      _trackScreenView(previousRoute);
    }
  }

  @override
  void didReplace({Route<dynamic>? newRoute, Route<dynamic>? oldRoute}) {
    super.didReplace(newRoute: newRoute, oldRoute: oldRoute);
    if (newRoute != null) {
      _trackScreenView(newRoute);
    }
  }

  void _trackScreenView(Route<dynamic> route) {
    final screenName = AnalyticsScreen.resolve(route.settings.name);
    if (screenName == AnalyticsScreen.unknown) {
      return;
    }

    unawaited(_analytics.logScreenView(screenName: screenName));
  }
}
