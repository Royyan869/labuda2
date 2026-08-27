import 'dart:async';
import 'dart:math';

import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';

/// Helper to ignore futures (fire-and-forget)
void _ignoreFuture(Future<void> future) {}

/// Retry configuration for retryable operations
class RetryConfig {
  /// Maximum number of retry attempts
  final int maxAttempts;

  /// Initial delay between retries (in milliseconds)
  final int initialDelayMs;

  /// Maximum delay between retries (in milliseconds)
  final int maxDelayMs;

  /// Backoff multiplier (e.g., 2.0 = exponential backoff)
  final double backoffMultiplier;

  /// Whether to add jitter to delay (randomized delay)
  final bool useJitter;

  /// Function to determine if an error is retryable
  final bool Function(dynamic error)? retryIf;

  const RetryConfig({
    this.maxAttempts = 3,
    this.initialDelayMs = 500,
    this.maxDelayMs = 10000,
    this.backoffMultiplier = 2.0,
    this.useJitter = true,
    this.retryIf,
  });

  /// Default config for network operations
  static const network = RetryConfig(
    maxAttempts: 3,
    initialDelayMs: 500,
    maxDelayMs: 5000,
    backoffMultiplier: 2.0,
  );

  /// Config for critical operations like user sync
  static const critical = RetryConfig(
    maxAttempts: 5,
    initialDelayMs: 1000,
    maxDelayMs: 10000,
    backoffMultiplier: 2.0,
  );
}

/// Helper class for retrying operations with exponential backoff
class RetryHelper {
  const RetryHelper._();

  /// Execute a function with retry logic
  ///
  /// Returns [Result.success] with the value if successful
  /// Returns [Result.error] if all attempts fail
  static Future<Result<T>> execute<T>(
    Future<T> Function() fn, {
    RetryConfig config = RetryConfig.network,
    String? operationName,
    ILoggerService? logger,
  }) async {
    int attempt = 0;
    dynamic lastError;

    while (attempt < config.maxAttempts) {
      attempt++;

      try {
        final result = await fn();

        if (logger != null && operationName != null && attempt > 1) {
          _ignoreFuture(
            logger.info(
              '$operationName succeeded on attempt $attempt',
              extra: {'attempt': attempt, 'maxAttempts': config.maxAttempts},
            ),
          );
        }

        return Result.success(result);
      } catch (e) {
        lastError = e;

        final shouldRetry = config.retryIf?.call(e) ?? _isRetryableError(e);

        if (!shouldRetry || attempt >= config.maxAttempts) {
          // Don't retry if error is not retryable or we've exhausted attempts
          break;
        }

        if (logger != null && operationName != null) {
          _ignoreFuture(
            logger.warning(
              '$operationName failed on attempt $attempt, retrying...',
              extra: {
                'attempt': attempt,
                'maxAttempts': config.maxAttempts,
                'error': e.toString(),
              },
            ),
          );
        }

        // Calculate delay with exponential backoff and jitter
        final delay = _calculateDelay(
          attempt,
          config.initialDelayMs,
          config.maxDelayMs,
          config.backoffMultiplier,
          config.useJitter,
        );

        await Future.delayed(Duration(milliseconds: delay));
      }
    }

    // All attempts failed
    if (logger != null && operationName != null) {
      _ignoreFuture(
        logger.error(
          '$operationName failed after $attempt attempts',
          extra: {'error': lastError.toString()},
        ),
      );
    }

    return Result.error(
      'Failed after $attempt attempts: ${lastError.toString()}',
    );
  }

  /// Execute a Result-returning function with retry logic
  static Future<Result<T>> executeResult<T>(
    Future<Result<T>> Function() fn, {
    RetryConfig config = RetryConfig.network,
    String? operationName,
    ILoggerService? logger,
  }) async {
    int attempt = 0;
    Result<T>? lastResult;

    while (attempt < config.maxAttempts) {
      attempt++;

      final result = await fn();

      if (result.isSuccess) {
        if (logger != null && operationName != null && attempt > 1) {
          _ignoreFuture(
            logger.info(
              '$operationName succeeded on attempt $attempt',
              extra: {'attempt': attempt, 'maxAttempts': config.maxAttempts},
            ),
          );
        }
        return result;
      }

      lastResult = result;
      final error = result.error;

      final shouldRetry =
          config.retryIf?.call(error) ?? _isRetryableError(error);

      if (!shouldRetry || attempt >= config.maxAttempts) {
        break;
      }

      if (logger != null && operationName != null) {
        _ignoreFuture(
          logger.warning(
            '$operationName failed on attempt $attempt, retrying...',
            extra: {
              'attempt': attempt,
              'maxAttempts': config.maxAttempts,
              'error': error,
            },
          ),
        );
      }

      final delay = _calculateDelay(
        attempt,
        config.initialDelayMs,
        config.maxDelayMs,
        config.backoffMultiplier,
        config.useJitter,
      );

      await Future.delayed(Duration(milliseconds: delay));
    }

    if (logger != null && operationName != null) {
      _ignoreFuture(
        logger.error(
          '$operationName failed after $attempt attempts',
          extra: {'error': lastResult?.error},
        ),
      );
    }

    return lastResult ?? Result.error('Failed after $attempt attempts');
  }

  /// Calculate delay with exponential backoff and optional jitter
  static int _calculateDelay(
    int attempt,
    int initialDelayMs,
    int maxDelayMs,
    double backoffMultiplier,
    bool useJitter,
  ) {
    // Exponential backoff
    var delay = (initialDelayMs * pow(backoffMultiplier, attempt - 1)).toInt();

    // Cap at max delay
    delay = delay.clamp(initialDelayMs, maxDelayMs);

    // Add jitter (±25%)
    if (useJitter) {
      final random = Random();
      final jitter = (delay * 0.25).toInt();
      delay = delay - jitter + random.nextInt(2 * jitter + 1);
    }

    return delay.clamp(initialDelayMs, maxDelayMs);
  }

  /// Determine if an error is retryable
  static bool _isRetryableError(dynamic error) {
    if (error == null) return false;

    final errorStr = error.toString().toLowerCase();

    // Network-related errors
    if (errorStr.contains('network') ||
        errorStr.contains('connection') ||
        errorStr.contains('timeout') ||
        errorStr.contains('socket') ||
        errorStr.contains('internet')) {
      return true;
    }

    // HTTP 5xx errors
    if (errorStr.contains('500') ||
        errorStr.contains('502') ||
        errorStr.contains('503') ||
        errorStr.contains('504')) {
      return true;
    }

    // Too many requests
    if (errorStr.contains('429') || errorStr.contains('too many requests')) {
      return true;
    }

    return false;
  }
}

/// Extension to add retry capability to Future
extension RetryFutureExtension<T> on Future<T> {
  /// Retry this future with the given config
  Future<Result<T>> retry({
    RetryConfig config = RetryConfig.network,
    String? operationName,
    ILoggerService? logger,
  }) {
    return RetryHelper.execute(
      () => this,
      config: config,
      operationName: operationName,
      logger: logger,
    );
  }
}
