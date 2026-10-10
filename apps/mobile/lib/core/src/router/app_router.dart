import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/core/core.dart';
// R4.2: Import LoggerService directly instead of mega-barrel
import 'package:hishumi/shared/services/logger_service.dart' show LoggerService;
// W14-B2: Import for authenticatedUserProvider and isSyncingWithBackendProvider
import 'package:hishumi/shared/providers/authenticated_account_provider.dart'
    show authenticatedUserProvider;
import 'package:hishumi/shared/providers/auth_status_providers.dart'
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
/// - Router membaca AuthController state dan bereaksi lewat redirect callback
///
/// **FINAL AUTH FLOW (LOCKED - DO NOT CHANGE):**
/// if not authenticated → /welcome (guest entry default; guests MAY open the
///     canonical Home route /home directly — GUEST HOME exception below)
/// if profile not complete (username only) → /auth/complete-profile
/// else → /home
///
/// ARSITEKTUR — SINGLE STABLE ROUTER (jangan kembali ke pola lama):
/// 1. GoRouter dibuat SEKALI oleh provider ini; provider ini TIDAK
///    `ref.watch` apapun yang jadi bahan keputusan redirect.
/// 2. Bahan keputusan redirect di-`ref.listen` → memberi tahu router lewat
///    [GoRouterRefresh] → GoRouter menjalankan `redirect:` ULANG di tempat
///    (mekanisme yang sama dengan `GoRouter.refresh()`).
/// 3. Redirect menghasilkan lokasi baru → navigasi berpindah TANPA membuat
///    objek router baru, sehingga riwayat navigasi tidak pernah dibuang.
///
/// MENGAPA POLA LAMA DIBONGKAR: provider ini dulu me-`watch`
/// `authControllerProvider`, jadi SETIAP perubahan auth state membangun
/// instance GoRouter BARU dengan `initialLocation = '/splash'`. Router baru
/// selalu mulai dari /splash lalu redirect melempar user ke /home —
/// akibatnya SATU siklus refresh sesi (kembali dari background; screenshot
/// yang sebentar mengalihkan fokus window sehingga app melewati `inactive` →
/// `resumed`) sudah cukup untuk melempar user dari layar manapun kembali ke
/// Home.
///
/// Tidak ada manual ProviderContainer.
final goRouterProvider = Provider<GoRouter>((ref) {
  final authController = ref.read(authControllerProvider.notifier);

  // Redirect re-evaluation bus. Didaftarkan sebagai `refreshListenable`,
  // bukan sebagai dependensi build, supaya router tidak pernah dibuat ulang.
  final refresh = GoRouterRefresh();

  // Bahan keputusan redirect. Didaftarkan eksplisit satu per satu: kalau suatu
  // saat redirect menambah bahan baru, daftarkan di sini juga — jangan
  // berpindah ke `ref.watch` (itu akan menghidupkan kembali bug splash→home).
  ref.listen(authControllerProvider, (_, _) => refresh.bump());
  ref.listen(authenticatedUserProvider, (_, _) => refresh.bump());
  ref.listen(isSyncingWithBackendProvider, (_, _) => refresh.bump());

  // Use cached modules manager, or create a new one if not initialized yet
  // This should not happen in normal flow since initializeRouterModules() should be called first
  final modulesManager = _cachedModulesManager ?? RouterModulesManager();

  return GoRouter(
    navigatorKey: navigatorKey,
    initialLocation: RoutePaths.splash,
    routes: modulesManager.buildRoutes(),
    // read, NOT watch: the observer must never be able to rebuild (and
    // thereby reset) the router.
    observers: [ref.read(screenViewRouteObserverProvider)],
    refreshListenable: refresh,
    errorBuilder: (context, state) => RouterErrorPage(state: state),
    redirect: (context, state) {
      // Dibaca SEGAR setiap evaluasi: lifetime router jauh lebih panjang daripada
      // satu snapshot auth, jadi nilai yang ditangkap saat build akan basi.
      final location = state.uri.path;
      final authState = ref.read(authControllerProvider);
      final authStatus = authController.appAuthStatus;
      final authenticatedUser = ref.read(authenticatedUserProvider);
      final isSyncingWithBackend = ref.read(isSyncingWithBackendProvider);
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
      return _handleAuthenticationRedirect(authState, authStatus, state);
    },
  );
});

/// [Listenable] that asks the single stable GoRouter to re-run its redirect
/// without rebuilding the router itself (the equivalent of
/// `GoRouter.refresh()`).
///
/// Dipakai sebagai `refreshListenable` oleh [goRouterProvider].
class GoRouterRefresh extends ChangeNotifier {
  /// Marks every redirect input as potentially changed.
  void bump() => notifyListeners();
}

/// W14-B2: Seller route guard - router-level protection for seller routes
///
/// Gates /seller/* routes, the market-action create routes
/// (`/create/for-sale`, `/create/auction`), and seller verification entry routes
/// using backend-derived seller authority, NOT users.role.
///
/// Policy:
/// - /seller/upgrade is ALWAYS accessible (REGISTRATION entry point for
///   non-sellers). Existing sellers are gated inside the wizard (`existingSeller`),
///   never redirected away here, because the registration flow itself returns to
///   this location after onboarding.
/// - /seller/renewal is the RENEWAL entry point: requires hasSellerProfile and
///   deliberately does NOT require market authority (its canonical caller is the
///   expired seller). Non-sellers are redirected to /seller/upgrade.
/// - All other /seller/* routes require hasMarketAuthority (active subscription)
/// - /create/for-sale and /create/auction PUBLISH at create (create = publish,
///   owner decision Oct 2026): both require hasSellerProfile AND
///   hasMarketAuthority. There is no draft workspace; market authority is
///   enforced at create by the client guard and by the owning service.
/// - /verification/seller is the seller-scoped verification surface and
///   follows the same authority rule
/// - Users WITHOUT a seller profile are redirected to /seller/upgrade (registration)
/// - Existing sellers missing market authority are redirected to /seller/renewal
///   (payment-only renewal lifecycle — never back into registration) on MARKET
///   ACTION routes (create for-sale / create auction are market actions:
///   create = publish)
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
  const verifyEmailRoute = '/auth/verify-email';

  final normalizedProfileRoute = _normalizeProfileIngress(Uri.parse(location));
  if (normalizedProfileRoute != null) {
    return normalizedProfileRoute;
  }

  if (authState is AuthStateAccountRestricted) {
    const restrictedRoute = RoutePaths.accountRestricted;
    return location == restrictedRoute ? null : restrictedRoute;
  }

  // D2 HARD GATE (design scope v2): an unverified session is parked in the
  // explicit pending-verification state and routed exclusively to the
  // verify-email screen — the same redirect pattern as
  // RequiresProfileCompletion / AccountRestricted. This is a business-flow
  // state (INV-7): splash degraded stays reserved for infra failures.
  if (authState is AuthStatePendingEmailVerification) {
    return location == verifyEmailRoute ? null : verifyEmailRoute;
  }

  // The verify-email surface is EXCLUSIVE to the pending state. Once the
  // state has moved on — e.g. a post-verification USERNAME_TAKEN rejection
  // returns the session to the registration flow (unauthenticated) — nobody
  // may linger on a gate screen whose gate no longer applies.
  if (location == verifyEmailRoute &&
      (authStatus == AppAuthStatus.unauthenticated ||
          authStatus == AppAuthStatus.degraded)) {
    return '/welcome';
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
        // GUEST HOME (Owner canonical truth): guests may open the canonical
        // Home (/home) directly — e.g. the Welcome Screen Home action.
        // /home is the canonical Home destination and must never alias or
        // fall back to a commerce route — i.e. no legacy "forSale as Home"
        // mapping may ever return.
        // /for-sale stays here only as the PREFIX for the public DETAIL route
        // /for-sale/:forSaleId (there is no standalone /for-sale browse route;
        // Marketplace is the sole public browse destination).
        '/home',
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
      if (location.startsWith('/auth') ||
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
  // Canonical market-action create routes (create = publish). For Sale and
  // Auction publish atomically at create, so both share the SAME authority
  // branch below — one authority, exact-match route class, no per-channel guard.
  const createMarketActionRoutes = <String>[
    RoutePaths.createForSale,
    RoutePaths.createAuction,
  ];
  final isCreateMarketActionRoute = createMarketActionRoutes.contains(location);
  // Only check seller-scoped routes.
  final isSellerRoute = location.startsWith('/seller');
  final isSellerVerificationRoute = location.startsWith(
    RoutePaths.sellerVerification,
  );
  if (!isCreateMarketActionRoute &&
      !isSellerRoute &&
      !isSellerVerificationRoute) {
    return null;
  }

  if (isCreateMarketActionRoute) {
    // CREATE = PUBLISH (owner decision, Oct 2026): POST /for-sale and
    // POST /auctions publish atomically, so market authority (active
    // subscription) is required at create — there is no draft workspace to
    // fall back to.
    if (authenticatedUser?.hasSellerProfile != true) {
      LoggerService.instance.warning(
        'User without seller profile attempted market-action create route: $location',
      );
      return RoutePaths.sellerUpgrade;
    }

    if (authenticatedUser?.hasMarketAuthority != true) {
      // Market action tier: payment-only renewal, never back to registration.
      return RoutePaths.sellerRenewal;
    }

    return null;
  }

  // TIER 0: /seller/upgrade — always accessible.
  // REGISTRATION entry point for non-sellers. Existing sellers are not
  // router-redirected off this location: the registration flow pushes the
  // payment WebView and returns here after onboarding creates the profile, so a
  // router-level redirect would eject a seller from their own in-flight flow.
  // The wizard fails closed for existing sellers (`existingSeller` gate) and
  // renewal surfaces never route here.
  if (isSellerRoute && location.startsWith('/seller/upgrade')) {
    return null;
  }

  // TIER 0.5: /seller/renewal — RENEWAL entry point (payment-only lifecycle).
  // Gate: hasSellerProfile. Market authority is deliberately NOT required here:
  // the canonical caller is the EXPIRED seller renewing. Non-sellers are sent
  // to registration instead.
  if (isSellerRoute && location.startsWith('/seller/renewal')) {
    if (authenticatedUser?.hasSellerProfile == true) {
      return null;
    }
    LoggerService.instance.warning(
      'User without seller profile attempted renewal route: $location',
    );
    return RoutePaths.sellerUpgrade;
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
  // Expired sellers cannot create/modify market config until they renew, and
  // renewal is payment-only — so profile holders go to /seller/renewal, never
  // back into the registration wizard.
  if (authenticatedUser?.hasMarketAuthority == true) {
    return null;
  }

  LoggerService.instance.warning(
    'User without market authority attempted seller market route: $location',
  );
  return authenticatedUser?.hasSellerProfile == true
      ? RoutePaths.sellerRenewal
      : RoutePaths.sellerUpgrade;
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
/// 4️⃣ AppAuthStatus.unauthenticated → /welcome (guest default; guest may
///     open canonical Home /home directly — GUEST HOME exception)
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

/// AppRouter — concrete [NavigationHandler] that drives the Riverpod-managed
/// GoRouter (goRouterProvider) through the global [navigatorKey].
///
/// Navigation flows:
/// UI → canonical navigationHandlerProvider → NavigationHandler → AppRouter
/// → GoRouter → route.
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

  // NavigationHandler interface implementations - using GoRouter directly
  @override
  void navigateToHome() => _currentRouter?.go('/home');

  @override
  void navigateToProfile() => _currentRouter?.push('/profile');

  @override
  void navigateToSignIn() => _currentRouter?.push('/auth/sign-in');

  @override
  void navigateToSignUp() => _currentRouter?.push('/auth/sign-up');

  @override
  void navigateToForgotPassword() =>
      _currentRouter?.push('/auth/forgot-password');

  @override
  void navigateToWelcome() => _currentRouter?.go('/welcome');

  @override
  void navigateToUserProfile(String userId) =>
      _currentRouter?.push('/user/$userId');

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
  void navigateToSearch() => _currentRouter?.push('/search');

  @override
  void navigateToSearchResults(String query, {String? type}) {
    var route = '/search/results?q=${Uri.encodeComponent(query)}';
    if (type != null) {
      route += '&type=${Uri.encodeComponent(type)}';
    }
    _currentRouter?.push(route);
  }

  @override
  void navigateToForSaleDetail(String forSaleId) => _currentRouter?.push(
    RoutePaths.forSaleDetail.replaceFirst(':forSaleId', forSaleId),
  );
  @override
  void navigateToAuction(String auctionId) =>
      _currentRouter?.push('/auction/$auctionId');

  @override
  void navigateToNotifications() => _currentRouter?.push('/notifications');

  @override
  void navigateToSellerVerification() =>
      _currentRouter?.push('/verification/seller');

  @override
  void navigateToSavedItems() => _currentRouter?.push('/saved-items');

  @override
  void navigateToMyBids() => _currentRouter?.push(RoutePaths.myBids);

  @override
  void navigateToOrders() => _currentRouter?.push('/orders');

  @override
  void navigateToOrderDetail(String orderId) =>
      _currentRouter?.push('/orders/$orderId');

  @override
  void navigateToSellerEarnings() =>
      _currentRouter?.push(RoutePaths.sellerEarnings);

  @override
  void navigateToSellerUpgrade() =>
      _currentRouter?.push(RoutePaths.sellerUpgrade);

  @override
  void navigateToSellerRenewal() =>
      _currentRouter?.push(RoutePaths.sellerRenewal);

  @override
  void navigateToExternalProductDetail(String productId) =>
      _currentRouter?.push('/seller/promotions/external-products/$productId');

  @override
  void navigateToCoinHistory() => _currentRouter?.push('/coins/history');

  @override
  void navigateToSellerDashboard() =>
      _currentRouter?.push(RoutePaths.sellerDashboard);

  @override
  void navigateToContentDetail(String contentId) =>
      _currentRouter?.push('/content/$contentId');

  @override
  void navigateToCreateContent() =>
      _currentRouter?.push(RoutePaths.createContent);
}
