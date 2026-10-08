/// Polling Monitor
///
/// Utility for monitoring polling operations with structured logging,
/// backoff on error, and metric collection.
library;

import 'dart:async';
import 'dart:math';

import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';

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

  const PollingMetrics({
    required this.domain,
    this.operationId,
    this.latencyMs,
    this.error,
    this.consecutiveErrors = 0,
    this.backoffIntervalSeconds,
  });

  /// Convert to map for logging
  Map<String, dynamic> toMap() {
    return {
      'event': 'polling',
      'domain': domain.value,
      if (operationId != null) 'operation_id': operationId,
      if (latencyMs != null) 'latency_ms': latencyMs,
      if (error != null) 'error': error,
      'consecutive_errors': consecutiveErrors,
      if (backoffIntervalSeconds != null)
        'backoff_interval_s': backoffIntervalSeconds,
    };
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
}

/// Helper to ignore futures (fire-and-forget)
void _ignoreFuture(Future<void> future) {}
