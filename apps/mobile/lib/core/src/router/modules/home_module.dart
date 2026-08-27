import 'package:go_router/go_router.dart';
import 'package:labuda/core/src/router/route_paths.dart';
import 'package:labuda/features/home/home.dart';
import 'base_module.dart';

/// Home Module - Uses HomeScreen from home feature
class HomeModule extends BaseModule {
  @override
  String get moduleName => 'HomeModule';

  @override
  List<GoRoute> get routes => [
    GoRoute(
      path: RoutePaths.home,
      name: RouteNames.home,
      builder: (context, state) => const MainScreen(),
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
