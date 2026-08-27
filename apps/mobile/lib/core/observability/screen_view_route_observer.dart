import 'dart:async';

import 'package:flutter/widgets.dart';

import 'package:labuda/core/src/interfaces/services/i_analytics_repository.dart';

/// Route observer that emits `screen_view` through the canonical Stack A sink.
///
/// This stays in core observability so it can be mounted directly in the live
/// GoRouter path without touching the legacy analytics stack.
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
    final routeIdentifier = route.settings.name;
    if (routeIdentifier == null || routeIdentifier.isEmpty) {
      return;
    }

    unawaited(
      _analytics.logEvent(
        'screen_view',
        parameters: {
          'screen_name': _screenName(routeIdentifier),
          'screen_path': routeIdentifier,
          'screen_class': route.runtimeType.toString(),
        },
      ),
    );
  }

  String _screenName(String routeIdentifier) {
    final uri = Uri.tryParse(routeIdentifier);
    if (uri != null && uri.pathSegments.isNotEmpty) {
      return uri.pathSegments.last;
    }

    final segments = routeIdentifier
        .split('/')
        .where((segment) => segment.isNotEmpty)
        .toList();
    if (segments.isNotEmpty) {
      return segments.last;
    }

    return routeIdentifier;
  }
}
