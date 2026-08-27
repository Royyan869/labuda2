import 'package:go_router/go_router.dart';
import 'package:labuda/core/src/router/route_paths.dart';
import 'package:labuda/domains/user/preference/seller/presentation/screens/seller_verification_screen.dart'
    show SellerVerificationScreen;

import 'base_module.dart';

/// Verification Module - canonical seller verification route.
///
/// Both `RoutePaths.verification` (legacy `/verification`) and
/// `RoutePaths.sellerVerification` (`/verification/seller`) now render the
/// same real seller-verification screen. The legacy redirect-only stub
/// screen has been removed.
class VerificationModule extends BaseModule {
  @override
  String get moduleName => 'VerificationModule';

  @override
  List<GoRoute> get routes => [
    GoRoute(
      path: RoutePaths.verification,
      name: RouteNames.verification,
      builder: (context, state) => const SellerVerificationScreen(),
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
