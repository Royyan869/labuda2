import 'dart:async';

import 'package:firebase_analytics/firebase_analytics.dart';
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/date_symbol_data_local.dart';

import 'app.dart';
import 'core/core.dart' hide sl;
import 'core/messaging/fcm_service_impl.dart';
import 'core/observability/crash_reporting.dart';
import 'core/observability/performance_monitor.dart';
import 'core/providers/core_providers.dart' as core_providers;
import 'domains/system/analytics/data/repositories/firebase_analytics_repository_impl.dart';
import 'domains/system/analytics/data/services/firebase_analytics_service.dart';
import 'domains/system/notification/data/datasources/notification_api_datasource.dart';
import 'domains/system/notification/data/datasources/notification_remote_datasource.dart';
import 'domains/system/notification/data/notification_providers.dart'
    show fcmServiceProvider, localNotificationServiceProvider;
import 'domains/system/notification/services/fcm_service.dart';
import 'domains/system/notification/services/in_app_banner_service.dart';
import 'domains/system/notification/services/local_notification_service.dart';
import 'domains/system/notification/services/notification_trigger_impl.dart';
import 'features/marketplace/marketplace.dart';
import 'features/home/home.dart';
import 'firebase_options.dart';
import 'shared/services/logger_service.dart' show LoggerService;
import 'shared/services/local_storage_service.dart';

/// All service instances constructed during bootstrap.
/// Passed directly to ProviderScope — zero GetIt reads after construction.
class _AppBootstrap {
  final ApiClient apiClient;
  final ILoggerService logger;
  final ILocalStorageService localStorage;
  final INavigationRegistry navigationRegistry;
  final WebSocketService webSocketService;
  final FcmService fcmService;
  final LocalNotificationService localNotificationService;
  final IAnalyticsRepository analyticsRepository;
  final INotificationTrigger notificationTrigger;

  _AppBootstrap({
    required this.apiClient,
    required this.logger,
    required this.localStorage,
    required this.navigationRegistry,
    required this.webSocketService,
    required this.fcmService,
    required this.localNotificationService,
    required this.analyticsRepository,
    required this.notificationTrigger,
  });
}

void main() {
  runZonedGuarded(
    () async {
      WidgetsFlutterBinding.ensureInitialized();

      final b = await _initServices();

      runApp(
        ProviderScope(
          overrides: [
            core_providers.apiClientProvider.overrideWithValue(b.apiClient),
            core_providers.loggerServiceProvider.overrideWithValue(b.logger),
            core_providers.localStorageServiceProvider.overrideWithValue(
              b.localStorage,
            ),
            core_providers.navigationRegistryProvider.overrideWithValue(
              b.navigationRegistry,
            ),
            core_providers.webSocketServiceProvider.overrideWithValue(
              b.webSocketService,
            ),
            fcmServiceProvider.overrideWithValue(b.fcmService),
            localNotificationServiceProvider.overrideWithValue(
              b.localNotificationService,
            ),
            core_providers.coreAnalyticsRepositoryProvider.overrideWithValue(
              b.analyticsRepository,
            ),
            core_providers.coreNotificationTriggerProvider.overrideWithValue(
              b.notificationTrigger,
            ),
          ],
          child: const LabudaApp(),
        ),
      );
    },
    (error, stack) {
      debugPrint('Uncaught async error: ${_redactSensitiveError(error)}');
      // Uncaught asynchronous errors are genuine crashes.
      CrashReporting.recordFatal(error, stack);
    },
  );
}

/// Canonical application-startup date-formatting initialization.
///
/// Loads the `id_ID` date symbols required by AppFormatters. This is the ONE
/// application-level owner of that dependency: every AppFormatters date path
/// throws LocaleDataException without it, so it runs inside `_initServices()`
/// before `runApp()`. Named as a bootstrap step (like `initializeRouterModules`)
/// so the startup contract is directly testable without booting Firebase.
Future<void> initializeAppDateFormatting() async {
  await initializeDateFormatting('id_ID');
}

/// Constructs all service instances directly — no GetIt reads.
/// Returns a bootstrap record for ProviderScope wiring.
Future<_AppBootstrap> _initServices() async {
  EnvConfig.init();

  final logger = LoggerService.instance;
  logger.info('[BOOTSTRAP] _initServices() start');

  // Date symbols for AppFormatters (`id_ID`) — canonical owner is
  // initializeAppDateFormatting() above; no other production site may
  // initialize date formatting.
  logger.info('[BOOTSTRAP] initializeAppDateFormatting() start');
  await initializeAppDateFormatting();
  logger.info('[BOOTSTRAP] initializeAppDateFormatting() done ✓');
  // REAL DEVICE DEV CONNECTIVITY (PASS 1 — minimal fail-fast):
  // Explicit --dart-define is the canonical authority for LAN. No .env fallback,
  // no chained fallback, no auto-detect of LAN IP. Log the effective authority
  // so a physical device without adb reverse is immediately obvious.
  if (kDebugMode) {
    final baseUrl = ApiConfig.baseUrl;
    final wsUrl = ApiConfig.wsUrl;
    if (ApiConfig.hasOverrideBaseUrl) {
      logger.info('[CONFIG] API override active: $baseUrl');
    } else {
      logger.info(
        '[CONFIG] Using dev default $baseUrl '
        '(physical devices require: adb reverse tcp:8080 tcp:8080)',
      );
    }
    if (ApiConfig.hasOverrideWsUrl) {
      logger.info('[CONFIG] WS override active: $wsUrl');
    } else {
      logger.info('[CONFIG] WS dev default: $wsUrl');
    }
  }

  try {
    logger.info('[BOOTSTRAP] Firebase.initializeApp() start');
    await Firebase.initializeApp(
      options: DefaultFirebaseOptions.currentPlatform,
    );
    logger.info('[BOOTSTRAP] Firebase.initializeApp() done ✓');
  } catch (e) {
    if (e.toString().contains('duplicate-app')) {
      logger.info('[BOOTSTRAP] Firebase already initialized');
    } else {
      logger.error('[BOOTSTRAP] Firebase initialization FAILED: $e');
      // No rethrow — app continues without Firebase (stream will never emit)
    }
  }

  // ── Observability activation ───────────────────────────────────────────────
  // Crash/error monitoring and performance monitoring are separate concerns
  // from product analytics; each has its own canonical authority.
  CrashReporting.initialize();
  PerformanceMonitoring.initialize();
  logger.info('[BOOTSTRAP] CrashReporting + PerformanceMonitoring initialized');

  // ── Core services ──────────────────────────────────────────────────────────
  logger.info('[BOOTSTRAP] LocalStorage.initialize() start');
  final localStorage = LocalStorageService();
  await localStorage.initialize();
  logger.info('[BOOTSTRAP] LocalStorage.initialize() done ✓');
  final navigationRegistry = NavigationRegistryImpl();
  final apiClient = ApiClient(logger: logger, localStorage: localStorage);

  // ── Notification stack ─────────────────────────────────────────────────────
  final notificationPlugin = FlutterLocalNotificationsPlugin();
  // Tier 3 (Runtime Honesty): wire the canonical NotificationApiDatasource
  // so FCM saveUserToken / deleteUserToken reach the real backend endpoints
  // (POST/DELETE /notifications/fcm/token) instead of the previous silent
  // no-op stubs. Logger plumbed through for structured failure visibility.
  final notificationApiDatasource = NotificationApiDatasource(apiClient);
  final notificationDatasource = NotificationRemoteDatasource(
    apiDatasource: notificationApiDatasource,
    logger: logger,
  );
  final inAppBanner = InAppBannerService();
  final localNotificationService = LocalNotificationService(
    plugin: notificationPlugin,
  );
  await localNotificationService.initialize();
  final notificationService = FcmServiceImpl.instance();
  final notificationTrigger = NotificationTriggerImpl(
    notificationService: notificationService,
  );
  final fcmService = FcmService(
    messaging: FirebaseMessaging.instance,
    datasource: notificationDatasource,
    localNotificationService: localNotificationService,
    inAppBannerService: inAppBanner,
    logger: logger,
  );

  // ── Analytics ──────────────────────────────────────────────────────────────
  // Product analytics is a decided capability: collection is enabled
  // explicitly so the app never ships with analytics silently disabled.
  final analyticsService = FirebaseAnalyticsService(FirebaseAnalytics.instance);
  await analyticsService.setAnalyticsCollectionEnabled(true);
  final analyticsRepository = FirebaseAnalyticsRepositoryImpl(analyticsService);

  // ── WebSocket ────────────────────────────────────────────────────────────────
  final webSocketService = WebSocketService(baseUrl: ApiConfig.wsUrl);

  // ── Navigation ─────────────────────────────────────────────────────────────
  logger.info('[BOOTSTRAP] initializeRouterModules() start');
  await initializeRouterModules();
  logger.info('[BOOTSTRAP] initializeRouterModules() done ✓');
  _registerNavigationTabs(navigationRegistry);

  FeatureFlags.printConfigSummary();
  logger.info('[BOOTSTRAP] _initServices() complete — calling runApp()');

  return _AppBootstrap(
    apiClient: apiClient,
    logger: logger,
    localStorage: localStorage,
    navigationRegistry: navigationRegistry,
    webSocketService: webSocketService,
    fcmService: fcmService,
    localNotificationService: localNotificationService,
    analyticsRepository: analyticsRepository,
    notificationTrigger: notificationTrigger,
  );
}

void _registerNavigationTabs(INavigationRegistry registry) {
  registerHomeTab(registry);
  registerMarketplaceTab(registry);
}

/// Removes auth tokens from error strings before logging.
/// Targets `?token=` / `&token=` query params that may appear in
/// WebSocket or HTTP URLs inside exception messages.
String _redactSensitiveError(Object error) {
  return error.toString().replaceAll(
    RegExp(r'token=[^\s&]+'),
    'token=<REDACTED>',
  );
}
