import 'dart:async';

import 'package:firebase_analytics/firebase_analytics.dart';
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/material.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:timeago/timeago.dart' as timeago;

import 'app.dart';
import 'core/core.dart' hide sl;
import 'core/messaging/fcm_service_impl.dart';
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
import 'domains/user/identity/authentication/data/services/firebase_authentication_service.dart';
import 'features/explore/explore.dart';
import 'features/home/home.dart';
import 'firebase_options.dart';
import 'shared/services/logger_service.dart' show LoggerService;
import 'shared/services/validation_service.dart';
import 'shared/services/local_storage_service.dart';

/// All service instances constructed during bootstrap.
/// Passed directly to ProviderScope — zero GetIt reads after construction.
class _AppBootstrap {
  final ApiClient apiClient;
  final ILoggerService logger;
  final IAuthenticationService authService;
  final ILocalStorageService localStorage;
  final IValidationService validation;
  final NavigationHandler navigationHandler;
  final INavigationRegistry navigationRegistry;
  final IPresenceService presenceService;
  final WebSocketService webSocketService;
  final FcmService fcmService;
  final LocalNotificationService localNotificationService;
  final IAnalyticsRepository analyticsRepository;
  final INotificationTrigger notificationTrigger;

  _AppBootstrap({
    required this.apiClient,
    required this.logger,
    required this.authService,
    required this.localStorage,
    required this.validation,
    required this.navigationHandler,
    required this.navigationRegistry,
    required this.presenceService,
    required this.webSocketService,
    required this.fcmService,
    required this.localNotificationService,
    required this.analyticsRepository,
    required this.notificationTrigger,
  });
}

void main() {
  FlutterError.onError = (FlutterErrorDetails details) {
    FlutterError.presentError(details);
    debugPrint('FlutterError: ${details.exception}');
  };

  runZonedGuarded(
    () async {
      WidgetsFlutterBinding.ensureInitialized();

      final b = await _initServices();

      runApp(
        ProviderScope(
          overrides: [
            core_providers.apiClientProvider.overrideWithValue(b.apiClient),
            core_providers.loggerServiceProvider.overrideWithValue(b.logger),
            core_providers.authServiceProvider.overrideWithValue(b.authService),
            core_providers.localStorageServiceProvider.overrideWithValue(
              b.localStorage,
            ),
            core_providers.validationServiceProvider.overrideWithValue(
              b.validation,
            ),
            core_providers.navigationHandlerProvider.overrideWithValue(
              b.navigationHandler,
            ),
            core_providers.navigationRegistryProvider.overrideWithValue(
              b.navigationRegistry,
            ),
            core_providers.presenceServiceProvider.overrideWithValue(
              b.presenceService,
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
      debugPrint('Stack: $stack');
    },
  );
}

/// Constructs all service instances directly — no GetIt reads.
/// Returns a bootstrap record for ProviderScope wiring.
Future<_AppBootstrap> _initServices() async {
  EnvConfig.init();
  timeago.setLocaleMessages('id', timeago.IdMessages());

  final logger = LoggerService.instance;
  logger.info('[BOOTSTRAP] _initServices() start');

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

  // ── Core services ──────────────────────────────────────────────────────────
  final validation = ValidationService();
  logger.info('[BOOTSTRAP] LocalStorage.initialize() start');
  final localStorage = LocalStorageService();
  await localStorage.initialize();
  logger.info('[BOOTSTRAP] LocalStorage.initialize() done ✓');
  final navigationRegistry = NavigationRegistryImpl();
  final authService = FirebaseAuthenticationService();
  final apiClient = ApiClient(logger: logger);

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
  final analyticsService = FirebaseAnalyticsService(FirebaseAnalytics.instance);
  final analyticsRepository = FirebaseAnalyticsRepositoryImpl(analyticsService);

  // ── WebSocket + Presence ───────────────────────────────────────────────────
  final webSocketService = WebSocketService(baseUrl: ApiConfig.wsUrl);
  final presenceService = AppPresenceServiceApi(
    apiClient: apiClient,
    webSocketService: webSocketService,
    logger: logger,
  );

  // ── Navigation ─────────────────────────────────────────────────────────────
  final navigationHandler = AppRouter();
  logger.info('[BOOTSTRAP] initializeRouterModules() start');
  await initializeRouterModules();
  logger.info('[BOOTSTRAP] initializeRouterModules() done ✓');
  _registerNavigationTabs(navigationRegistry);

  FeatureFlags.printConfigSummary();
  logger.info('[BOOTSTRAP] _initServices() complete — calling runApp()');

  return _AppBootstrap(
    apiClient: apiClient,
    logger: logger,
    authService: authService,
    localStorage: localStorage,
    validation: validation,
    navigationHandler: navigationHandler,
    navigationRegistry: navigationRegistry,
    presenceService: presenceService,
    webSocketService: webSocketService,
    fcmService: fcmService,
    localNotificationService: localNotificationService,
    analyticsRepository: analyticsRepository,
    notificationTrigger: notificationTrigger,
  );
}

void _registerNavigationTabs(INavigationRegistry registry) {
  registerHomeTab(registry);
  registerExploreTab(registry);
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
