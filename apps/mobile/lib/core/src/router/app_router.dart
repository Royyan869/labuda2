import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
// R4.2: Import LoggerService directly instead of mega-barrel
import 'package:labuda/shared/services/logger_service.dart' show LoggerService;
// W14-B2: Import for authenticatedUserProvider and isSyncingWithBackendProvider
import 'package:labuda/shared/providers/authenticated_account_provider.dart'
    show authenticatedUserProvider;
import 'package:labuda/shared/providers/auth_status_providers.dart'
    show isSyncingWithBackendProvider;
import 'router_modules_manager.dart';
import 'router_error_page.dart';

/// Global navigator key for overlay access (used by FCM banner)
final GlobalKey<NavigatorState> navigatorKey = GlobalKey<NavigatorState>();

/// Cached modules manager - initialized once at app startup
/// This ensures modules are only initialized once and routes are consistent
RouterModulesManager? _cachedModulesManager;

/// Initialize router modules - must be called before using goRouterProvider
///
/// This should be called during app initialization (e.g., in main.dart)
/// to ensure all modules are properly initialized before router is created.
Future<void> initializeRouterModules() async {
  if (_cachedModulesManager != null) {
    LoggerService.instance.info('Router modules already initialized');
    return;
  }

  final logger = LoggerService.instance;
  logger.info('Initializing router modules...');

  _cachedModulesManager = RouterModulesManager();
  await _cachedModulesManager!.initializeModules();

  logger.info('Router modules initialized successfully');
}

/// Provider untuk GoRouter - SINGLE CANONICAL OWNER of App Entry Routing
///
/// **ROUTING DECISION OWNERSHIP:**
/// - This provider owns ALL route redirect logic
/// - No other module should make route decisions based on auth state
/// - Router watches AuthController state and reacts via redirect callback
///
/// **FINAL AUTH FLOW (LOCKED - DO NOT CHANGE):**
/// if not authenticated → /welcome
/// if profile not complete (username only) → /auth/complete-profile
/// else → /home
///
/// ARSITEKTUR:
/// 1. authControllerProvider state berubah
/// 2. goRouterProvider rebuild (karena ref.watch)
/// 3. GoRouter redirect dipanggil ulang
/// 4. User diarahkan sesuai AppAuthStatus
///
/// Tidak ada manual ProviderContainer.
/// Tidak ada RouterRefreshNotifier.
/// Tidak ada refreshListenable.
final goRouterProvider = Provider<GoRouter>((ref) {
  // Watch auth state - router rebuilds when auth state changes
  final authState = ref.watch(authControllerProvider);
  final authController = ref.read(authControllerProvider.notifier);

  // Watch current user for role-based guards — currently only the seller
  // route guard (_sellerRouteGuardCore below). PASS 2C: the "W7-B: Admin
  // route guard" this comment used to reference does not exist — mobile
  // has no admin routes/screens at all (admin is web-only, in apps/admin).
  // AdminGuard/isAdminProvider (permission_guard.dart,
  // authenticated_account_provider.dart + auth_status_providers.dart) are unused scaffolding kept in case mobile
  // ever grows admin screens; they are not wired into this router.
  final authenticatedUser = ref.watch(authenticatedUserProvider);

  // W14-B2: Watch backend sync status to prevent deep-link bypass during initialization
  final isSyncingWithBackend = ref.watch(isSyncingWithBackendProvider);

  // Use cached modules manager, or create a new one if not initialized yet
  // This should not happen in normal flow since initializeRouterModules() should be called first
  final modulesManager = _cachedModulesManager ?? RouterModulesManager();

  return GoRouter(
    navigatorKey: navigatorKey,
    initialLocation: RoutePaths.splash,
    routes: modulesManager.buildRoutes(),
    observers: [ref.watch(screenViewRouteObserverProvider)],
    errorBuilder: (context, state) => RouterErrorPage(state: state),
    redirect: (context, state) {
      final location = state.uri.path;
      final authStatus = authController.appAuthStatus;
      LoggerService.instance.warning(
        '[ROUTER] redirect: loc=$location syncing=$isSyncingWithBackend status=$authStatus',
      );

      // W14-B2: BLOCKING REDIRECT - Prevent deep-link bypass during auth initialization
      // When backend sync is in progress, block all routes except splash
      // This prevents role guards from being evaluated before user data is loaded
      if (isSyncingWithBackend && location != '/splash') {
        return '/splash';
      }

      // W14-B2: Seller route guard - check before auth redirect
      final sellerGuardResult = _handleSellerRouteGuard(
        authenticatedUser,
        state,
      );
      if (sellerGuardResult != null) {
        return sellerGuardResult;
      }

      // Check for specific states first, then fall back to AppAuthStatus
      return _handleAuthenticationRedirect(
        authState,
        authController.appAuthStatus,
        state,
      );
    },
  );
});

/// W14-B2: Seller route guard - router-level protection for seller routes
///
/// Gates /seller/* routes, `/create/for-sale`, and seller verification entry routes using
/// backend-derived seller authority, NOT users.role.
///
/// Policy:
/// - /seller/upgrade is ALWAYS accessible (onboarding entry point for non-sellers)
/// - All other /seller/* routes require hasMarketAuthority (active subscription)
/// - /create/for-sale requires the same market authority as other market actions
/// - /verification and /verification/seller are seller-scoped verification surfaces
///   and follow the same authority rule
/// - Users without seller authority are redirected to /seller/upgrade
/// - Market feature gates are evaluated here for router-level protection
///   and rechecked at the screen/mutation boundary
///
/// Returns:
/// - null if user can access the route
/// - redirect path if user should be redirected
@visibleForTesting
String? handleSellerRouteGuardForTest(
  AuthUser? authenticatedUser,
  String location,
) => _sellerRouteGuardCore(authenticatedUser, location);

/// Test seam for [_handleAuthenticationRedirect].
/// Exposes the pure redirect function so unit tests can verify routing rules
/// without spinning up a full GoRouter / Riverpod stack.
@visibleForTesting
String? handleAuthRedirectForTest(
  AuthState authState,
  AppAuthStatus authStatus,
  String location,
) {
  // Construct a minimal GoRouterState-like object — we only need the location.
  // Since GoRouterState is not easily instantiable in tests, we call the private
  // helper directly through a thin wrapper that avoids the GoRouterState dependency.
  return _authRedirectForLocation(authState, authStatus, location);
}

@visibleForTesting
String? normalizeProfileIngressForTest(String location) =>
    _normalizeProfileIngress(Uri.parse(location));

/// Pure, location-based redirect logic — the SINGLE real implementation.
/// [_handleAuthenticationRedirect] (the live GoRouter `redirect:` callback)
/// delegates straight to this function; it takes a plain [String] location
/// instead of [GoRouterState] purely so it can also be exercised directly
/// by tests via [handleAuthRedirectForTest] without spinning up a full
/// GoRouter / Riverpod stack.
String? _authRedirectForLocation(
  AuthState authState,
  AppAuthStatus authStatus,
  String location,
) {
  const splashRoute = '/splash';
  const completeProfileRoute = '/auth/complete-profile';

  final normalizedProfileRoute = _normalizeProfileIngress(Uri.parse(location));
  if (normalizedProfileRoute != null) {
    return normalizedProfileRoute;
  }

  if (authState is AuthStateAccountRestricted) {
    const restrictedRoute = RoutePaths.accountRestricted;
    return location == restrictedRoute ? null : restrictedRoute;
  }

  if (authState is AuthStateRequiresProfileCompletion) {
    return location == completeProfileRoute ? null : completeProfileRoute;
  }

  switch (authStatus) {
    case AppAuthStatus.initializing:
      return location == splashRoute ? null : splashRoute;

    case AppAuthStatus.unauthenticated:
      if (location.startsWith('/auth')) return null;
      if (location == '/welcome') return null;
      const publicBrowsePrefixes = [
        '/for-sale',
        '/auction',
        '/search',
        '/content',
        '/user',
      ];
      if (publicBrowsePrefixes.any(
        (p) => location == p || location.startsWith('$p/'),
      )) {
        return null;
      }
      return '/welcome';

    case AppAuthStatus.accountRestricted:
      return location == RoutePaths.accountRestricted
          ? null
          : RoutePaths.accountRestricted;

    case AppAuthStatus.authenticated:
      if ((location.startsWith('/auth') && location != completeProfileRoute) ||
          location == '/welcome' ||
          location == splashRoute) {
        return '/home';
      }
      return null;

    case AppAuthStatus.degraded:
      // PASS 2B: previously bounced /splash -> /welcome here, showing an
      // ordinary welcome screen with zero explanation while the real
      // problem (backend unreachable, or rejecting sync) was invisible.
      // Stay wherever the user is instead — SplashScreen now renders a
      // dedicated backend-unavailable/backend-failure UI (with retry)
      // when parked on splash in this state; any other route is left
      // untouched exactly as before. This also matches this function's
      // own documented contract above ("degraded -> NO redirect").
      return null;
  }
}

const _reservedProfileSegments = {'edit', 'personal-info', 'addresses'};

String? _normalizeProfileIngress(Uri uri) {
  if (uri.pathSegments.length != 2 || uri.pathSegments.first != 'profile') {
    return null;
  }

  final targetUserId = uri.pathSegments[1];
  if (targetUserId.isEmpty || _reservedProfileSegments.contains(targetUserId)) {
    return null;
  }

  return Uri(
    path: '/user/$targetUserId',
    queryParameters: uri.queryParameters.isEmpty ? null : uri.queryParameters,
    fragment: uri.fragment.isEmpty ? null : uri.fragment,
  ).toString();
}

String? _handleSellerRouteGuard(
  AuthUser? authenticatedUser,
  GoRouterState state,
) => _sellerRouteGuardCore(authenticatedUser, state.uri.path);

String? _sellerRouteGuardCore(AuthUser? authenticatedUser, String location) {
  final isCreateForSaleRoute = location == RoutePaths.createForSale;
  // Only check seller-scoped routes.
  final isSellerRoute = location.startsWith('/seller');
  final isSellerVerificationRoute =
      location == '/verification' ||
      location.startsWith('/verification/seller');
  if (!isCreateForSaleRoute && !isSellerRoute && !isSellerVerificationRoute) {
    return null;
  }

  if (isCreateForSaleRoute) {
    if (authenticatedUser?.hasSellerProfile == true) {
      if (authenticatedUser?.hasMarketAuthority == true) {
        return null;
      }
      LoggerService.instance.warning(
        'User with expired market authority attempted create-for-sale route: '
        '$location',
      );
      return RoutePaths.sellerUpgrade;
    }

    LoggerService.instance.warning(
      'User without seller profile attempted create-for-sale route: $location',
    );
    return RoutePaths.sellerUpgrade;
  }

  // TIER 0: /seller/upgrade — always accessible.
  // Onboarding entry point for non-sellers; renewal path for expired sellers.
  if (isSellerRoute && location.startsWith('/seller/upgrade')) {
    return null;
  }

  // TIER 1: WORKSPACE / OBLIGATION routes.
  // Gate: hasSellerProfile (seller profile existence).
  // Rationale: subscription expiry blocks NEW market actions, not existing
  // obligations. An expired seller must be able to view orders, fulfill
  // shipments, access earnings visibility, configure payout bank accounts,
  // and check/submit verification. These surfaces survive subscription expiry.
  // Doctrine: workspace access = hasSellerProfile; market actions = hasMarketAuthority.
  const workspaceRoutes = [
    '/seller/dashboard',
    '/seller/orders',
    '/seller/earnings',
    '/seller/bank-accounts',
  ];
  final isWorkspaceRoute =
      workspaceRoutes.any((p) => location == p || location.startsWith('$p/')) ||
      isSellerVerificationRoute;

  if (isWorkspaceRoute) {
    if (authenticatedUser?.hasSellerProfile == true) {
      return null;
    }
    LoggerService.instance.warning(
      'User without seller profile attempted workspace route: $location',
    );
    return RoutePaths.sellerUpgrade;
  }

  // TIER 2: MARKET ACTION routes (shipping setup, promotions, any unlisted /seller/* route).
  // Gate: hasMarketAuthority (active subscription).
  // Expired sellers cannot create/modify market config until they renew.
  if (authenticatedUser?.hasMarketAuthority == true) {
    return null;
  }

  LoggerService.instance.warning(
    'User without market authority attempted seller market route: $location',
  );
  return RoutePaths.sellerUpgrade;
}

/// Pure authentication redirect function - FINAL AUTH FLOW OWNER
///
/// **SINGLE SOURCE OF TRUTH for app entry routing:**
/// This function is the ONLY place where route decisions are made based on auth state.
/// All other parts of the app should use AuthController state for UI decisions,
/// but NEVER make independent routing choices.
///
/// **IMPORTANT: Fungsi ini PURE dan DETERMINISTIC**
/// - Membaca AuthState untuk specific states (profile completion)
/// - Fallback ke AppAuthStatus untuk simplified logic
/// - Tidak ada side effect
/// - Redirect berdasarkan status final yang disederhanakan
///
/// **FINAL FLOW TRUTH (LOCKED):**
/// Routing Rules (PRIORITY ORDER ENFORCED):
/// 1️⃣ AuthStateRequiresProfileCompletion → /auth/complete-profile
/// 2️⃣ AppAuthStatus.authenticated → /home
/// 3️⃣ AppAuthStatus.initializing → /splash
/// 4️⃣ AppAuthStatus.unauthenticated → /welcome
/// 5️⃣ AppAuthStatus.degraded → NO redirect (stay on current route)
String? _handleAuthenticationRedirect(
  AuthState authState,
  AppAuthStatus authStatus,
  GoRouterState state,
) {
  // PASS 2B: this used to duplicate the entire switch statement that lives
  // in [_authRedirectForLocation] (originally "extracted" into a pure
  // function for testability, but the live call site was never switched
  // over to call it — leaving two copies of the same routing rules that
  // could silently drift). Delegating removes the duplication and
  // guarantees tests exercising the pure seam actually describe what the
  // real router does.
  return _authRedirectForLocation(authState, authStatus, state.uri.path);
}

/// AppRouter sebagai delegation ke RouterNavigationImpl
///
/// Class ini tetap ada untuk backward compatibility dengan kode
/// yang menggunakan AppRouter singleton. Namun sekarang ini
/// hanya wrapper untuk RouterNavigationImpl yang mendapatkan
/// router dari goRouterProvider.
class AppRouter implements NavigationHandler {
  static final AppRouter _instance = AppRouter._internal();
  factory AppRouter() => _instance;
  AppRouter._internal();

  final ILoggerService _logger = LoggerService.instance;

  /// Get current GoRouter instance from global navigatorKey
  ///
  /// This allows programmatic navigation without BuildContext by using
  /// the global navigatorKey that's registered with goRouterProvider.
  GoRouter? get _currentRouter {
    final context = navigatorKey.currentContext;
    if (context == null) {
      _logger.warning('NavigatorKey context is null - router not ready yet');
      return null;
    }
    return GoRouter.of(context);
  }

  /// Initialize router - now no-op since router is managed by Riverpod
  ///
  /// Method ini tetap ada untuk backward compatibility.
  /// Modules initialization sekarang dilakukan melalui initializeRouterModules()
  Future<void> initialize() async {
    _logger.info(
      'AppRouter.initialize() called - now no-op, router managed by Riverpod',
    );
  }

  /// Dispose - now no-op since router is managed by Riverpod
  void dispose() {
    _logger.info(
      'AppRouter.dispose() called - now no-op, router managed by Riverpod',
    );
  }

  // NavigationHandler interface implementations - using GoRouter directly
  @override
  void navigateToHome() => _currentRouter?.go('/home');

  @override
  void navigateBack() {
    final router = _currentRouter;
    if (router != null && router.canPop()) {
      router.pop();
    } else {
      _logger.warning(
        'Cannot navigate back - no previous route or router not ready',
      );
    }
  }

  @override
  void navigateToProfile() => _currentRouter?.push('/profile');

  @override
  void navigateToLogin() => _currentRouter?.push('/auth/sign-in');

  @override
  void navigateToSignIn() => navigateToLogin();

  @override
  void navigateToRegister() => _currentRouter?.push('/auth/sign-up');

  @override
  void navigateToSignUp() => navigateToRegister();

  @override
  void navigateToForgotPassword() =>
      _currentRouter?.push('/auth/forgot-password');

  @override
  void navigateToWelcome() => _currentRouter?.go('/welcome');

  @override
  void navigateToEditProfile() => _currentRouter?.go('/profile');

  @override
  void navigateToUserProfile(String userId) =>
      _currentRouter?.push('/user/$userId');

  void navigateToAuctionDetail(String auctionId) =>
      _currentRouter?.push('/auction/$auctionId');

  @override
  void navigateToCreateAuction() => _currentRouter?.push('/create/auction');

  @override
  void navigateToChat() => _currentRouter?.push('/chat');

  @override
  void navigateToChatConversation(String conversationId) =>
      _currentRouter?.push('/chat/$conversationId');

  @override
  void navigateToSettings() => _currentRouter?.push('/settings');

  @override
  void navigateToNotificationSettings() =>
      _currentRouter?.push('/settings/notifications');

  @override
  void navigateToPrivacySettings() => _currentRouter?.push('/settings');

  @override
  void navigateToSearch() => _currentRouter?.push('/search');

  @override
  void navigateToSearchResults(String query, {String? type}) {
    var route = '/search/results?q=${Uri.encodeComponent(query)}';
    if (type != null) {
      route += '&type=${Uri.encodeComponent(type)}';
    }
    _currentRouter?.push(route);
  }

  // Report methods - not part of NavigationHandler interface
  void navigateToReport(String targetId, String targetType) =>
      _currentRouter?.push('/report/$targetType/$targetId');

  // Additional helper method used by AppNavigationHandler
  void navigateTo(String route, {Map<String, String>? parameters}) {
    try {
      if (parameters != null && parameters.isNotEmpty) {
        _currentRouter?.goNamed(route, pathParameters: parameters);
      } else {
        _currentRouter?.goNamed(route);
      }
      _logger.info('Navigated to: $route');
    } catch (e) {
      _logger.error('Navigation error to $route: $e');
    }
  }

  // Missing NavigationHandler implementations
  @override
  void navigateToForSaleDetail(String forSaleId) => _currentRouter?.push(
    RoutePaths.forSaleDetail.replaceFirst(
      ':forSaleId',
      forSaleId,
    ),
  );

  @override
  void navigateToCreateForSale() =>
      _currentRouter?.push(RoutePaths.createForSale);

  @override
  void navigateToAuction(String auctionId) =>
      _currentRouter?.push('/auction/$auctionId');

  @override
  void navigateToNotifications() => _currentRouter?.push('/notifications');

  @override
  void navigateToKycVerification({String? userId}) {
    _logger.warning(
      'navigateToKycVerification${userId == null ? '' : '($userId)'} is deprecated; routing to seller verification',
    );
    _currentRouter?.push('/verification/seller');
  }

  @override
  void navigateToBusinessDocuments() =>
      _currentRouter?.push('/verification/seller');

  @override
  void navigateToSellerVerification() =>
      _currentRouter?.push('/verification/seller');

  @override
  void navigateToCheckout() {
    _logger.warning(
      'navigateToCheckout requires a listing context; no bare route emitted',
    );
  }

  @override
  void navigateToSavedItems() => _currentRouter?.push('/saved-items');

  @override
  void navigateToOrders() => _currentRouter?.push('/orders');

  @override
  void navigateToOrderDetail(String orderId) =>
      _currentRouter?.push('/orders/$orderId');

  @override
  void navigateToOrderHistory() => _currentRouter?.push('/orders');

  @override
  void navigateToPayment(dynamic paymentRequest) {
    _logger.warning(
      'navigateToPayment called - payment navigation should be handled by payment module',
    );
    _logger.warning(
      'navigateToPayment is deprecated here; payment flow must start from checkout',
    );
  }

  @override
  void navigateToSellerEarnings() =>
      _currentRouter?.push(RoutePaths.sellerEarnings);

  @override
  void navigateToSellerForSales() =>
      _currentRouter?.push(RoutePaths.sellerForSales);

  @override
  void navigateToSellerRefundList() => _currentRouter?.push('/seller/orders');

  @override
  void navigateToSellerUpgrade() =>
      _currentRouter?.push(RoutePaths.sellerUpgrade);

  @override
  void navigateToExternalProductDetail(String productId) =>
      _currentRouter?.push('/seller/promotions/external-products/$productId');

  @override
  void navigateToCoinBalance() => _currentRouter?.push('/coins');

  @override
  void navigateToCoinHistory() => _currentRouter?.push('/coins/history');

  @override
  void navigateToSellerDashboard() =>
      _currentRouter?.push(RoutePaths.sellerDashboard);

  @override
  void navigateToBlockedUsers() => _currentRouter?.push('/settings');

  @override
  void navigateToContentDetail(String contentId) =>
      _currentRouter?.push('/content/$contentId');

  @override
  void navigateToCreateContent() =>
      _currentRouter?.push(RoutePaths.createContent);

  @override
  void navigateToAddressPayment() => _currentRouter?.go('/settings');

  @override
  void navigateToSecurity() => _currentRouter?.push('/settings');

  @override
  void showBottomSheet<T>(Widget Function(BuildContext) builder) =>
      _logger.warning('showBottomSheet called but requires BuildContext');

  @override
  void showModalDialog<T>(Widget Function(BuildContext) builder) =>
      _logger.warning('showModalDialog called but requires BuildContext');

  @override
  void showSnackBar(String message, {bool isError = false}) => _logger.warning(
    'showSnackBar called but requires BuildContext: $message',
  );
}
