import 'dart:async';
import 'package:flutter/widgets.dart';
import 'package:labuda/core/src/interfaces/services/i_presence_service.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';

/// Observer untuk app lifecycle yang mengelola presence tracking
/// Wrap di root app untuk auto track online status
class AppLifecycleObserver extends StatefulWidget {
  final Widget child;
  final IPresenceService presenceService;
  final ILoggerService logger;
  final String? Function() getCurrentUserId;

  const AppLifecycleObserver({
    super.key,
    required this.child,
    required this.presenceService,
    required this.logger,
    required this.getCurrentUserId,
  });

  @override
  State<AppLifecycleObserver> createState() => _AppLifecycleObserverState();
}

class _AppLifecycleObserverState extends State<AppLifecycleObserver>
    with WidgetsBindingObserver {
  Timer? _presenceTimer;
  String? _currentUserId;

  /// Interval untuk refresh presence (dalam detik)
  /// Harus kurang dari presenceTimeoutMinutes di AppPresenceService
  static const int _presenceRefreshInterval = 60; // 1 minute

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _initPresence();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _stopPresenceTimer();
    _setOffline();
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    widget.logger.info('App lifecycle state changed: $state');

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

  void _initPresence() {
    // Delay untuk menunggu auth state ready
    Future.delayed(const Duration(seconds: 1), () {
      _currentUserId = widget.getCurrentUserId();
      if (_currentUserId != null) {
        _startPresenceTracking();
      }
    });
  }

  void _handleAppResumed() {
    _currentUserId = widget.getCurrentUserId();
    if (_currentUserId != null) {
      _startPresenceTracking();
    }
  }

  void _handleAppInactive() {
    _stopPresenceTimer();
    _setOffline();
  }

  void _startPresenceTracking() {
    if (_currentUserId == null) return;

    widget.logger.info('Starting presence tracking for: $_currentUserId');

    // Set online immediately
    widget.presenceService.startTracking(_currentUserId!);

    // Start periodic refresh timer
    _stopPresenceTimer();
    _presenceTimer = Timer.periodic(
      const Duration(seconds: _presenceRefreshInterval),
      (_) => _refreshPresence(),
    );
  }

  void _refreshPresence() {
    final userId = widget.getCurrentUserId();
    if (userId != null && userId == _currentUserId) {
      widget.presenceService.updatePresence(userId: userId, isOnline: true);
    } else if (userId != _currentUserId) {
      // User changed (login/logout)
      _handleUserChanged(userId);
    }
  }

  void _handleUserChanged(String? newUserId) {
    // Set old user offline
    if (_currentUserId != null) {
      widget.presenceService.stopTracking(_currentUserId!);
    }

    _currentUserId = newUserId;

    // Start tracking new user
    if (newUserId != null) {
      _startPresenceTracking();
    } else {
      _stopPresenceTimer();
    }
  }

  void _setOffline() {
    if (_currentUserId != null) {
      widget.presenceService.stopTracking(_currentUserId!);
    }
  }

  void _stopPresenceTimer() {
    _presenceTimer?.cancel();
    _presenceTimer = null;
  }

  /// Call this when user logs in
  void onUserLogin(String userId) {
    widget.logger.info('User logged in: $userId');
    _currentUserId = userId;
    _startPresenceTracking();
  }

  /// Call this when user logs out
  void onUserLogout() {
    widget.logger.info('User logged out');
    _stopPresenceTimer();
    if (_currentUserId != null) {
      widget.presenceService.stopTracking(_currentUserId!);
      _currentUserId = null;
    }
  }

  @override
  Widget build(BuildContext context) {
    return widget.child;
  }
}
