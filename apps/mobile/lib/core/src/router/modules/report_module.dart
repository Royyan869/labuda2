import 'package:go_router/go_router.dart';
import 'package:labuda/core/src/router/route_paths.dart';
import 'package:labuda/domains/system/report/report.dart';

import 'base_module.dart';

/// Report Module - Report screen
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
        return ReportScreen(targetType: targetType, targetId: targetId);
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
