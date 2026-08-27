/// Polling Status Indicator
///
/// Non-blocking UI indicator for polling status degradation.
/// Shows visual feedback when polling is experiencing issues.
library;

import 'package:flutter/material.dart';

/// Polling status for UI indication
enum PollingStatus {
  /// Polling is healthy
  healthy,

  /// Polling is degraded (some errors, but still working)
  degraded,

  /// Polling is failed (no connection, or multiple errors)
  failed,
}

/// Polling status data
class PollingStatusData {
  /// Current status
  final PollingStatus status;

  /// Domain being polled
  final String domain;

  /// Consecutive error count
  final int consecutiveErrors;

  /// Whether the device is offline
  final bool isOffline;

  /// Last error message (if any)
  final String? lastError;

  const PollingStatusData({
    required this.status,
    required this.domain,
    this.consecutiveErrors = 0,
    this.isOffline = false,
    this.lastError,
  });

  /// Get display message based on status
  String get message {
    if (isOffline) {
      return 'Anda offline - status mungkin tertunda';
    }
    switch (status) {
      case PollingStatus.healthy:
        return '';
      case PollingStatus.degraded:
        return 'Koneksi bermasalah - status mungkin tertunda';
      case PollingStatus.failed:
        return 'Gagal memuat data terbaru';
    }
  }

  /// Get icon data based on status
  IconData? get icon {
    switch (status) {
      case PollingStatus.healthy:
        return null;
      case PollingStatus.degraded:
        return Icons.warning_amber_rounded;
      case PollingStatus.failed:
        return Icons.error_outline;
    }
  }

  /// Get background color based on status
  Color? get backgroundColor {
    switch (status) {
      case PollingStatus.healthy:
        return null;
      case PollingStatus.degraded:
        return const Color(0xFFFFF8E1); // Light yellow
      case PollingStatus.failed:
        return const Color(0xFFFFEBEE); // Light red
    }
  }

  /// Get text color based on status
  Color? get textColor {
    switch (status) {
      case PollingStatus.healthy:
        return null;
      case PollingStatus.degraded:
        return const Color(0xFF8D6E63); // Brown
      case PollingStatus.failed:
        return const Color(0xFFD32F2F); // Red
    }
  }

  /// Create from polling monitor status map
  factory PollingStatusData.fromMonitorStatus(Map<String, dynamic> statusMap) {
    final consecutiveErrors = statusMap['consecutive_errors'] as int? ?? 0;
    final isDegraded = statusMap['is_degraded'] as bool? ?? false;

    PollingStatus status;
    if (consecutiveErrors == 0) {
      status = PollingStatus.healthy;
    } else if (isDegraded) {
      status = PollingStatus.failed;
    } else {
      status = PollingStatus.degraded;
    }

    return PollingStatusData(
      status: status,
      domain: statusMap['domain'] as String? ?? 'unknown',
      consecutiveErrors: consecutiveErrors,
      lastError: statusMap['last_error'] as String?,
    );
  }

  /// Create offline status
  factory PollingStatusData.offline({String? domain}) {
    return const PollingStatusData(
      status: PollingStatus.failed,
      domain: 'offline',
      isOffline: true,
    );
  }

  /// Create healthy status
  factory PollingStatusData.healthy({String? domain}) {
    return PollingStatusData(
      status: PollingStatus.healthy,
      domain: domain ?? 'unknown',
    );
  }
}

/// Non-blocking polling status indicator widget
///
/// Shows a small banner when polling status is degraded.
/// Can be placed anywhere in the UI where polling status is relevant.
class PollingStatusIndicator extends StatelessWidget {
  /// Current polling status data
  final PollingStatusData status;

  /// Whether to show the indicator (default: true)
  final bool show;

  /// Callback when indicator is tapped (optional)
  final VoidCallback? onTap;

  const PollingStatusIndicator({
    super.key,
    required this.status,
    this.show = true,
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    // Don't show if status is healthy or show is false
    if (!show || status.status == PollingStatus.healthy) {
      return const SizedBox.shrink();
    }

    final message = status.message;
    if (message.isEmpty) {
      return const SizedBox.shrink();
    }

    return Material(
      color: status.backgroundColor,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
          child: Row(
            children: [
              if (status.icon != null) ...[
                Icon(status.icon, size: 16, color: status.textColor),
                const SizedBox(width: 8),
              ],
              Expanded(
                child: Text(
                  message,
                  style: TextStyle(color: status.textColor, fontSize: 12),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Compact badge version for use in app bars or small spaces
class PollingStatusBadge extends StatelessWidget {
  /// Current polling status data
  final PollingStatusData status;

  /// Whether to show the badge (default: true)
  final bool show;

  const PollingStatusBadge({super.key, required this.status, this.show = true});

  @override
  Widget build(BuildContext context) {
    // Don't show if status is healthy or show is false
    if (!show || status.status == PollingStatus.healthy) {
      return const SizedBox.shrink();
    }

    Color color;
    IconData icon;

    switch (status.status) {
      case PollingStatus.healthy:
        return const SizedBox.shrink();
      case PollingStatus.degraded:
        color = Colors.orange;
        icon = Icons.warning_amber_rounded;
        break;
      case PollingStatus.failed:
        color = Colors.red;
        icon = Icons.error_outline;
        break;
    }

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        border: Border.all(color: color, width: 1),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 10, color: color),
          if (status.isOffline) ...[
            const SizedBox(width: 4),
            Text(
              'Offline',
              style: TextStyle(
                color: color,
                fontSize: 10,
                fontWeight: FontWeight.w500,
              ),
            ),
          ],
        ],
      ),
    );
  }
}
