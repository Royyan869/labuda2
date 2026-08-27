/// Polling Monitor
///
/// Utility for monitoring polling operations with structured logging,
/// backoff on error, and metric collection.
library;

import 'dart:async';
import 'dart:math';

import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';

// ==============================================================================
// TASK C: CORRELATION FIELDS
// ==============================================================================

/// Device correlation context for log enrichment
/// These fields help identify patterns across devices, versions, and networks
class PollingCorrelationContext {
  /// Unique device identifier (UUID)
  final String? deviceId;

  /// Application version (e.g., "1.2.3+42")
  final String? appVersion;

  /// Network type (wifi, mobile, ethernet, none, other)
  final String? networkType;

  /// OS version (e.g., "iOS 17.2", "Android 14")
  final String? osVersion;

  const PollingCorrelationContext({
    this.deviceId,
    this.appVersion,
    this.networkType,
    this.osVersion,
  });

  /// Convert to map for logging
  Map<String, dynamic> toMap() {
    return {
      if (deviceId != null) 'device_id': deviceId,
      if (appVersion != null) 'app_version': appVersion,
      if (networkType != null) 'network_type': networkType,
      if (osVersion != null) 'os_version': osVersion,
    };
  }

  /// Empty context (when correlation data not available)
  static const empty = PollingCorrelationContext();

  /// Global correlation context (set once at app startup)
  static PollingCorrelationContext _globalContext =
      const PollingCorrelationContext();

  /// Set the global correlation context
  static void setGlobalContext(PollingCorrelationContext context) {
    _globalContext = context;
  }

  /// Get the global correlation context
  static PollingCorrelationContext get globalContext => _globalContext;
}

/// Polling domain for categorization
enum PollingDomain {
  /// Subscription status polling
  subscription('subscription'),

  /// Auction data polling
  auction('auction'),

  /// Coin/balance polling
  coins('coins');

  final String value;
  const PollingDomain(this.value);
}

/// Polling event types for structured logging
enum PollingEventType {
  /// Polling started
  start('poll_start'),

  /// Polling succeeded
  success('poll_success'),

  /// Polling failed
  error('poll_error'),

  /// Backoff applied
  backoff('poll_backoff'),

  /// Backoff reset after success
  backoffReset('poll_backoff_reset');

  final String value;
  const PollingEventType(this.value);
}

/// Polling metrics
class PollingMetrics {
  /// Domain being polled
  final PollingDomain domain;

  /// Operation identifier (e.g., auction ID, seller ID)
  final String? operationId;

  /// Latency in milliseconds
  final int? latencyMs;

  /// Error message (if failed)
  final String? error;

  /// Consecutive error count
  final int consecutiveErrors;

  /// Current backoff interval in seconds
  final int? backoffIntervalSeconds;

  /// Correlation context (device info, app version, etc.)
  final PollingCorrelationContext? correlation;

  const PollingMetrics({
    required this.domain,
    this.operationId,
    this.latencyMs,
    this.error,
    this.consecutiveErrors = 0,
    this.backoffIntervalSeconds,
    this.correlation,
  });

  /// Convert to map for logging (includes correlation fields)
  Map<String, dynamic> toMap() {
    final baseMap = {
      'event': 'polling',
      'domain': domain.value,
      if (operationId != null) 'operation_id': operationId,
      if (latencyMs != null) 'latency_ms': latencyMs,
      if (error != null) 'error': error,
      'consecutive_errors': consecutiveErrors,
      if (backoffIntervalSeconds != null)
        'backoff_interval_s': backoffIntervalSeconds,
    };

    // Add correlation fields if available
    if (correlation != null) {
      return {...baseMap, ...correlation!.toMap()};
    }

    // Otherwise try to use global context
    final globalCorr = PollingCorrelationContext.globalContext;
    if (globalCorr != PollingCorrelationContext.empty) {
      return {...baseMap, ...globalCorr.toMap()};
    }

    return baseMap;
  }
}

/// Configuration for polling backoff behavior
class PollingBackoffConfig {
  /// Base polling interval in seconds
  final int baseIntervalSeconds;

  /// Maximum backoff interval in seconds
  final int maxBackoffSeconds;

  /// Backoff increments in seconds: [15, 30, 90]
  final List<int> backoffSteps;

  const PollingBackoffConfig({
    this.baseIntervalSeconds = 30,
    this.maxBackoffSeconds = 90,
    this.backoffSteps = const [15, 30, 90],
  });

  /// Default config for subscription polling
  static const subscription = PollingBackoffConfig(
    baseIntervalSeconds: 30,
    maxBackoffSeconds: 90,
    backoffSteps: [15, 30, 90],
  );

  /// Default config for auction polling
  static const auction = PollingBackoffConfig(
    baseIntervalSeconds: 10,
    maxBackoffSeconds: 60,
    backoffSteps: [10, 20, 30, 60],
  );
}

/// State for tracking backoff
class PollingBackoffState {
  /// Consecutive error count
  int consecutiveErrors = 0;

  /// Current backoff interval (null = use base interval)
  int? currentBackoffSeconds;

  /// Last error message
  String? lastError;

  /// Timestamp of last successful poll
  DateTime? lastSuccessAt;

  /// Timestamp of last error
  DateTime? lastErrorAt;

  /// Reset backoff state after success
  void reset() {
    consecutiveErrors = 0;
    currentBackoffSeconds = null;
    lastError = null;
    lastErrorAt = null;
  }

  /// Increment error count and update backoff
  void incrementError(String error, PollingBackoffConfig config) {
    consecutiveErrors++;
    lastError = error;
    lastErrorAt = DateTime.now();

    // Calculate backoff based on consecutive errors
    final stepIndex = (consecutiveErrors - 1).clamp(
      0,
      config.backoffSteps.length - 1,
    );
    currentBackoffSeconds = config.backoffSteps[stepIndex];
  }

  /// Get current interval in seconds
  int getCurrentInterval(PollingBackoffConfig config) {
    return currentBackoffSeconds ?? config.baseIntervalSeconds;
  }
}

/// Monitor for polling operations
class PollingMonitor {
  final ILoggerService _logger;
  final PollingDomain _domain;
  final String? _operationId;
  final PollingBackoffConfig _config;

  final _backoffState = PollingBackoffState();
  final _random = Random();

  /// Create a new polling monitor
  PollingMonitor({
    required ILoggerService logger,
    required PollingDomain domain,
    String? operationId,
    PollingBackoffConfig config = PollingBackoffConfig.subscription,
  }) : _logger = logger,
       _domain = domain,
       _operationId = operationId,
       _config = config;

  /// Get the current backoff state
  PollingBackoffState get backoffState => _backoffState;

  /// Log poll start
  void logStart() {
    _ignoreFuture(
      _logger.info(
        'Polling started',
        extra: PollingMetrics(
          domain: _domain,
          operationId: _operationId,
          consecutiveErrors: _backoffState.consecutiveErrors,
          backoffIntervalSeconds: _backoffState.currentBackoffSeconds,
        ).toMap(),
      ),
    );
  }

  /// Log poll success with latency
  void logSuccess({int? latencyMs}) {
    _ignoreFuture(
      _logger.info(
        'Polling succeeded',
        extra: PollingMetrics(
          domain: _domain,
          operationId: _operationId,
          latencyMs: latencyMs,
          consecutiveErrors: _backoffState.consecutiveErrors,
          backoffIntervalSeconds: _backoffState.currentBackoffSeconds,
        ).toMap(),
      ),
    );

    // Reset backoff on success
    if (_backoffState.consecutiveErrors > 0) {
      _backoffState.reset();
      _ignoreFuture(
        _logger.info(
          'Backoff reset after success',
          extra: {
            'event': PollingEventType.backoffReset.value,
            'domain': _domain.value,
            if (_operationId != null) 'operation_id': _operationId,
          },
        ),
      );
    }

    _backoffState.lastSuccessAt = DateTime.now();
  }

  /// Log poll error
  void logError(String error, {int? latencyMs}) {
    _backoffState.incrementError(error, _config);

    _ignoreFuture(
      _logger.warning(
        'Polling failed',
        extra: PollingMetrics(
          domain: _domain,
          operationId: _operationId,
          latencyMs: latencyMs,
          error: error,
          consecutiveErrors: _backoffState.consecutiveErrors,
          backoffIntervalSeconds: _backoffState.currentBackoffSeconds,
        ).toMap(),
      ),
    );

    _ignoreFuture(
      _logger.warning(
        'Backoff applied',
        extra: {
          'event': PollingEventType.backoff.value,
          'domain': _domain.value,
          if (_operationId != null) 'operation_id': _operationId,
          'consecutive_errors': _backoffState.consecutiveErrors,
          'backoff_interval_s': _backoffState.currentBackoffSeconds,
        },
      ),
    );
  }

  /// Get current polling interval with jitter
  Duration getCurrentInterval() {
    final baseSeconds = _backoffState.getCurrentInterval(_config);

    // Add jitter: ±20% to avoid thundering herd
    final jitter = (baseSeconds * 0.2).toInt();
    final jittered = baseSeconds - jitter + _random.nextInt(2 * jitter + 1);

    // Clamp to valid range
    final clamped = jittered.clamp(
      _config.baseIntervalSeconds,
      _config.maxBackoffSeconds,
    );

    return Duration(seconds: clamped);
  }

  /// Wrap a polling operation with monitoring
  Future<T> monitor<T>(Future<T> Function() operation) async {
    final startTime = DateTime.now();
    logStart();

    try {
      final result = await operation();
      final latencyMs = DateTime.now().difference(startTime).inMilliseconds;
      logSuccess(latencyMs: latencyMs);
      return result;
    } catch (e) {
      final latencyMs = DateTime.now().difference(startTime).inMilliseconds;
      logError(e.toString(), latencyMs: latencyMs);
      rethrow;
    }
  }

  /// Wrap a polling operation that returns Result with monitoring
  Future<T> monitorResult<T>(Future<T> Function() operation) async {
    final startTime = DateTime.now();
    logStart();

    try {
      final result = await operation();
      final latencyMs = DateTime.now().difference(startTime).inMilliseconds;
      logSuccess(latencyMs: latencyMs);
      return result;
    } catch (e) {
      final latencyMs = DateTime.now().difference(startTime).inMilliseconds;
      logError(e.toString(), latencyMs: latencyMs);
      rethrow;
    }
  }

  /// Check if polling is in degraded state (many consecutive errors)
  bool get isDegraded => _backoffState.consecutiveErrors >= 3;

  /// Get status summary for UI
  Map<String, dynamic> getStatusSummary() {
    return {
      'domain': _domain.value,
      if (_operationId != null) 'operation_id': _operationId,
      'consecutive_errors': _backoffState.consecutiveErrors,
      'is_degraded': isDegraded,
      'current_interval_s': _backoffState.getCurrentInterval(_config),
      'last_success_at': _backoffState.lastSuccessAt?.toIso8601String(),
      'last_error_at': _backoffState.lastErrorAt?.toIso8601String(),
      'last_error': _backoffState.lastError,
    };
  }
}

/// Helper to ignore futures (fire-and-forget)
void _ignoreFuture(Future<void> future) {}
