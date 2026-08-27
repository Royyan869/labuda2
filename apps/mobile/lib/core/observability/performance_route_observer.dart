import 'package:flutter/material.dart';
import 'performance_monitor.dart';
import 'firebase_performance_impl.dart';

/// NavigatorObserver that automatically tracks screen rendering performance
/// Measures time from route push to route completion
///
/// **Location:** lib/core/observability/performance_route_observer.dart
/// **Replaces:** features/performance/lib/presentation/observers/performance_route_observer.dart
class CorePerformanceRouteObserver extends RouteObserver<PageRoute<dynamic>> {
  final PerformanceMonitor _performanceMonitor;
  final Map<Route<dynamic>, DateTime> _routeStartTimes = {};

  CorePerformanceRouteObserver(this._performanceMonitor);

  /// Factory for default instance with Firebase implementation
  factory CorePerformanceRouteObserver.defaultImpl() {
    return CorePerformanceRouteObserver(FirebasePerformanceImpl.instance());
  }

  @override
  void didPush(Route<dynamic> route, Route<dynamic>? previousRoute) {
    super.didPush(route, previousRoute);
    if (route is PageRoute) {
      _startScreenTrace(route);
    }
  }

  @override
  void didReplace({Route<dynamic>? newRoute, Route<dynamic>? oldRoute}) {
    super.didReplace(newRoute: newRoute, oldRoute: oldRoute);
    if (oldRoute != null) {
      _stopScreenTrace(oldRoute);
    }
    if (newRoute is PageRoute) {
      _startScreenTrace(newRoute);
    }
  }

  @override
  void didPop(Route<dynamic> route, Route<dynamic>? previousRoute) {
    super.didPop(route, previousRoute);
    _stopScreenTrace(route);
  }

  /// Starts a performance trace for a screen
  void _startScreenTrace(Route<dynamic> route) {
    final screenName = _getScreenName(route);
    if (screenName == null) return;

    _routeStartTimes[route] = DateTime.now();

    final traceName = 'screen_$screenName';
    _performanceMonitor.startTrace(traceName);
    _performanceMonitor.setTraceAttribute(traceName, 'screen_name', screenName);
  }

  /// Stops a performance trace for a screen
  void _stopScreenTrace(Route<dynamic> route) {
    final screenName = _getScreenName(route);
    if (screenName == null) return;

    final startTime = _routeStartTimes.remove(route);
    if (startTime != null) {
      final duration = DateTime.now().difference(startTime);
      final traceName = 'screen_$screenName';

      _performanceMonitor.setTraceMetric(
        traceName,
        'duration_ms',
        duration.inMilliseconds,
      );
      _performanceMonitor.stopTrace(traceName);
    }
  }

  /// Extracts screen name from route
  String? _getScreenName(Route<dynamic> route) {
    if (route.settings.name != null && route.settings.name!.isNotEmpty) {
      // Remove leading slash and convert to snake_case
      return route.settings.name!
          .replaceFirst('/', '')
          .replaceAll('/', '_')
          .toLowerCase();
    }
    return null;
  }
}
