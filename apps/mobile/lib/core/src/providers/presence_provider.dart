import 'dart:async';
import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/src/interfaces/services/i_presence_service.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/core/src/interfaces/services/i_local_storage_service.dart';
import 'package:labuda/core/providers/core_providers.dart'
    show
        presenceServiceProvider,
        loggerServiceProvider,
        localStorageServiceProvider;
// W14-B2: Import for role refresh on app resume
import 'package:labuda/domains/user/identity/authentication/authentication.dart';

/// Presence state containing isTracking and isEnabled
class PresenceState {
  final bool isTracking;
  final bool isEnabled;

  const PresenceState({this.isTracking = false, this.isEnabled = true});

  PresenceState copyWith({bool? isTracking, bool? isEnabled}) {
    return PresenceState(
      isTracking: isTracking ?? this.isTracking,
      isEnabled: isEnabled ?? this.isEnabled,
    );
  }
}

/// Global presence management provider
/// Handles app lifecycle and user presence tracking
class PresenceManager extends Notifier<PresenceState>
    with WidgetsBindingObserver {
  Timer? _presenceTimer;
  String? _currentUserId;
  IPresenceService? _presenceService;
  ILoggerService? _logger;
  ILocalStorageService? _localStorage;

  static const String _showOnlineStatusKey = 'show_online_status';

  /// Interval untuk refresh presence (dalam detik)
  static const int _presenceRefreshInterval = 60; // 1 minute

  @override
  PresenceState build() {
    // Initialize when provider is first read
    _init();

    // Cleanup when provider is disposed
    ref.onDispose(_cleanup);

    return const PresenceState();
  }

  void _init() {
    try {
      _presenceService = ref.read(presenceServiceProvider);
      _logger = ref.read(loggerServiceProvider);
      _localStorage = ref.read(localStorageServiceProvider);
      _loadEnabledSetting();
      WidgetsBinding.instance.addObserver(this);
    } catch (e) {
      // Services not ready yet, will retry on setUser
    }
  }

  void _loadEnabledSetting() async {
    try {
      final result = await _localStorage?.getBool(_showOnlineStatusKey);
      if (result != null && result.isSuccess && result.data != null) {
        state = state.copyWith(isEnabled: result.data);
      }
    } catch (e) {
      // Use default (enabled)
    }
  }

  void _cleanup() {
    WidgetsBinding.instance.removeObserver(this);
    _stopPresenceTimer();
    _setOffline();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    _logger?.info('App lifecycle state changed: $state');

    switch (state) {
      case AppLifecycleState.resumed:
        _handleAppResumed();
        break;
      case AppLifecycleState.inactive:
      case AppLifecycleState.paused:
      case AppLifecycleState.detached:
      case AppLifecycleState.hidden:
        _handleAppInactive();
        break;
    }
  }

  /// Toggle online status visibility on/off
  Future<void> setEnabled(bool enabled) async {
    _logger?.info('PresenceManager: setEnabled called with: $enabled');

    // Save to local storage
    await _localStorage?.setBool(_showOnlineStatusKey, enabled);

    state = state.copyWith(isEnabled: enabled);

    if (enabled && _currentUserId != null) {
      // Start tracking if enabled
      _startPresenceTracking();
    } else if (!enabled) {
      // Stop tracking and set offline if disabled
      _stopPresenceTimer();
      _setOffline();
    }
  }

  /// Get current enabled state
  bool get isEnabled => state.isEnabled;

  /// Call when user logs in
  void setUser(String? userId) {
    _logger?.info('PresenceManager: setUser called with: $userId');

    // Retry init if services weren't ready before
    if (_presenceService == null) {
      _init();
    }

    // Handle user change
    if (_currentUserId != null && _currentUserId != userId) {
      // Set old user offline
      _presenceService?.stopTracking(_currentUserId!);
    }

    _currentUserId = userId;

    if (userId != null && state.isEnabled) {
      _startPresenceTracking();
    } else {
      _stopPresenceTimer();
    }
  }

  /// Call when user logs out
  void clearUser() {
    _logger?.info('PresenceManager: clearUser called');
    _stopPresenceTimer();
    if (_currentUserId != null) {
      _presenceService?.stopTracking(_currentUserId!);
      _currentUserId = null;
    }
    state = state.copyWith(isTracking: false);
  }

  void _handleAppResumed() {
    if (_currentUserId != null && state.isEnabled) {
      _startPresenceTracking();
    }

    // W14-B2: Trigger role refresh on app resume
    // This ensures role changes made while app was in background are reflected
    _triggerRoleRefresh();
  }

  /// W14-B2: Trigger user data refresh to pick up role changes
  void _triggerRoleRefresh() {
    try {
      // Get the container - we need to access authControllerProvider
      // Use a delayed call to avoid doing work during the lifecycle transition
      Future.delayed(const Duration(milliseconds: 500), () {
        try {
          final container = ProviderScope.containerOf(
            WidgetsBinding.instance.rootElement!,
          );
          final authController = container.read(
            authControllerProvider.notifier,
          );
          authController.refreshUserData();
        } catch (e) {
          // Silently fail - container might not be available yet
          _logger?.debug('Role refresh skipped on resume: $e');
        }
      });
    } catch (e) {
      // Silently fail - not critical
      _logger?.debug('Role refresh setup failed: $e');
    }
  }

  void _handleAppInactive() {
    _stopPresenceTimer();
    _setOffline();
  }

  void _startPresenceTracking() {
    if (_currentUserId == null ||
        _presenceService == null ||
        !state.isEnabled) {
      return;
    }

    _logger?.info('Starting presence tracking for: $_currentUserId');

    // Set online immediately
    _presenceService!.startTracking(_currentUserId!);
    state = state.copyWith(isTracking: true);

    // Start periodic refresh timer
    _stopPresenceTimer();
    _presenceTimer = Timer.periodic(
      const Duration(seconds: _presenceRefreshInterval),
      (_) => _refreshPresence(),
    );
  }

  void _refreshPresence() {
    if (_currentUserId != null && _presenceService != null && state.isEnabled) {
      _presenceService!.updatePresence(userId: _currentUserId!, isOnline: true);
    }
  }

  void _setOffline() {
    if (_currentUserId != null && _presenceService != null) {
      _presenceService!.stopTracking(_currentUserId!);
      state = state.copyWith(isTracking: false);
    }
  }

  void _stopPresenceTimer() {
    _presenceTimer?.cancel();
    _presenceTimer = null;
  }
}

/// Provider for presence management
final presenceManagerProvider =
    NotifierProvider<PresenceManager, PresenceState>(() {
      return PresenceManager();
    });

/// Provider to watch single user's online status
final userOnlineStatusProvider = StreamProvider.family<bool, String>((
  ref,
  userId,
) {
  try {
    final presenceService = ref.watch(presenceServiceProvider);
    return presenceService.watchUserPresence(userId);
  } catch (e) {
    return Stream.value(false);
  }
});

/// Provider to watch multiple users' online status
final usersOnlineStatusProvider =
    StreamProvider.family<Map<String, bool>, List<String>>((ref, userIds) {
      try {
        final presenceService = ref.watch(presenceServiceProvider);
        return presenceService.watchUsersPresence(userIds);
      } catch (e) {
        return Stream.value({});
      }
    });

/// Provider that auto-syncs auth state with presence tracking
/// Use this in app.dart builder to ensure presence is always synced
final presenceAuthSyncProvider = Provider<void>((ref) {
  // This provider should be watched from a widget that has access to authControllerProvider
  // The actual sync is done via PresenceAuthSync widget
  return;
});
