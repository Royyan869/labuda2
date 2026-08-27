// Export BannerAction for external usage

// Dart
import 'dart:async';
import 'dart:collection';
import 'package:labuda/domains/system/notification/presentation/widgets/in_app_notification_banner.dart';

// Flutter
import 'package:flutter/material.dart';
export 'package:labuda/domains/system/notification/presentation/widgets/in_app_notification_banner.dart'
    show BannerAction;

/// In-App Banner Service
///
/// Manages in-app notification banners with:
/// - Auto-dismiss after 4 seconds
/// - Manual dismiss (swipe up, tap X)
/// - Banner queueing (if multiple notifications arrive)
/// - Max queue size limit (prevent overflow)
///
/// Size: < 250 lines (per GUIDELINES)
class InAppBannerService {
  OverlayEntry? _currentOverlay;
  Timer? _autoHideTimer;
  final Queue<_BannerData> _bannerQueue = Queue();
  bool _isShowing = false;

  /// Max queue size to prevent memory overflow
  /// If queue exceeds this, oldest items will be dropped
  static const int maxQueueSize = 10;

  /// Show in-app notification banner
  ///
  /// Banner will auto-dismiss after 4 seconds unless manually dismissed.
  /// If queue is full, oldest notification will be dropped.
  ///
  /// Either [context] or [overlay] must be provided.
  /// If [overlay] is provided, it will be used directly (preferred for FCM).
  /// If only [context] is provided, Overlay.of(context) will be used.
  void show({
    BuildContext? context,
    OverlayState? overlay,
    required String title,
    required String body,
    String? avatarUrl,
    VoidCallback? onTap,
    List<BannerAction>? actions,
    Duration autoHideDuration = const Duration(seconds: 4),
  }) {
    // Validate: either context or overlay must be provided
    assert(
      context != null || overlay != null,
      'Either context or overlay must be provided',
    );
    // If banner already showing, queue this one
    if (_isShowing) {
      // Check queue size limit
      if (_bannerQueue.length >= maxQueueSize) {
        // Drop oldest notification to prevent overflow
        _bannerQueue.removeFirst();
      }

      _bannerQueue.add(
        _BannerData(
          context: context,
          overlay: overlay,
          title: title,
          body: body,
          avatarUrl: avatarUrl,
          onTap: onTap,
          actions: actions,
          autoHideDuration: autoHideDuration,
        ),
      );
      return;
    }

    _showBannerInternal(
      context: context,
      overlay: overlay,
      title: title,
      body: body,
      avatarUrl: avatarUrl,
      onTap: onTap,
      actions: actions,
      autoHideDuration: autoHideDuration,
    );
  }

  /// Internal method to show banner
  void _showBannerInternal({
    BuildContext? context,
    OverlayState? overlay,
    required String title,
    required String body,
    String? avatarUrl,
    VoidCallback? onTap,
    List<BannerAction>? actions,
    required Duration autoHideDuration,
  }) {
    _isShowing = true;

    // Create overlay entry with banner widget
    _currentOverlay = OverlayEntry(
      builder: (overlayContext) {
        return InAppNotificationBanner(
          title: title,
          body: body,
          avatarUrl: avatarUrl,
          onTap: onTap != null
              ? () {
                  onTap();
                  dismiss();
                }
              : null,
          actions: actions,
          onDismiss: dismiss,
        );
      },
    );

    // Insert to overlay
    try {
      // Use provided overlay directly, or get from context
      final targetOverlay = overlay ?? Overlay.of(context!);
      targetOverlay.insert(_currentOverlay!);
    } catch (e) {
      _isShowing = false;
      return;
    }

    // Setup auto-hide timer
    _autoHideTimer = Timer(autoHideDuration, dismiss);
  }

  /// Dismiss current banner
  ///
  /// If there are queued banners, show next one.
  void dismiss() {
    if (!_isShowing) return;

    // Cancel auto-hide timer
    _autoHideTimer?.cancel();
    _autoHideTimer = null;

    // Remove overlay
    _currentOverlay?.remove();
    _currentOverlay = null;
    _isShowing = false;

    // Show next banner in queue if any
    if (_bannerQueue.isNotEmpty) {
      final next = _bannerQueue.removeFirst();
      // Small delay before showing next banner
      Future.delayed(const Duration(milliseconds: 300), () {
        _showBannerInternal(
          context: next.context,
          overlay: next.overlay,
          title: next.title,
          body: next.body,
          avatarUrl: next.avatarUrl,
          onTap: next.onTap,
          actions: next.actions,
          autoHideDuration: next.autoHideDuration,
        );
      });
    }
  }

  /// Clear all queued banners
  void clearQueue() {
    _bannerQueue.clear();
  }

  /// Check if banner is currently showing
  bool get isShowing => _isShowing;

  /// Get queue length
  int get queueLength => _bannerQueue.length;

  /// Dispose service (cleanup)
  void dispose() {
    _autoHideTimer?.cancel();
    _currentOverlay?.remove();
    _bannerQueue.clear();
    _isShowing = false;
  }
}

/// Internal class to hold banner data for queueing
class _BannerData {
  final BuildContext? context;
  final OverlayState? overlay;
  final String title;
  final String body;
  final String? avatarUrl;
  final VoidCallback? onTap;
  final List<BannerAction>? actions;
  final Duration autoHideDuration;

  _BannerData({
    this.context,
    this.overlay,
    required this.title,
    required this.body,
    this.avatarUrl,
    this.onTap,
    this.actions,
    required this.autoHideDuration,
  });
}
