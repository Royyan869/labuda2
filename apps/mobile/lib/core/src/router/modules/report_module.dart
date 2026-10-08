import 'package:go_router/go_router.dart';
import 'package:labuda/core/src/router/route_paths.dart';
import 'package:labuda/domains/system/report/report.dart';

import 'base_module.dart';

/// Report Module - Report screen
///
/// `/report` is the ONE canonical report destination: the path carries the
/// report target as query parameters (`?type=<targetType>&id=<targetId>` and
/// an optional display-only `title`), so a report link is stable and
/// shareable. Every report entry point in the app pushes this route instead of
/// constructing the report form directly.
class ReportModule extends BaseModule {
  @override
  String get moduleName => 'ReportModule';

  @override
  List<GoRoute> get routes => [
    GoRoute(
      path: RoutePaths.report,
      name: RouteNames.report,
      builder: (context, state) {
        final targetType = state.uri.queryParameters['type'];
        final targetId = state.uri.queryParameters['id'];
        final targetTitle = state.uri.queryParameters['title'];
        return ReportScreen(
          targetType: targetType,
          targetId: targetId,
          targetTitle: targetTitle,
        );
      },
    ),
  ];

  @override
  Future<void> initialize() async {}

  @override
  void registerRoutes(List<GoRoute> mainRoutes) {
    mainRoutes.addAll(routes);
  }

  @override
  void dispose() {}
}
