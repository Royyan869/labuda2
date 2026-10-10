// API Exception types for handling backend errors
// Maps HTTP status codes and API error responses to typed exceptions
//
// This file owns the mobile API failure vocabulary. In particular, the
// `ApiExceptionFactory.fromTransport` table is the SINGLE place that turns a
// `DioExceptionType` into a canonical error code — the code that eventually
// lands in `Result.errorCode`. Consumers branch on that code
// (`api_error_codes.dart`), never on the human message.

import 'dart:io';

import 'package:dio/dio.dart';
import 'package:hishumi/core/api/api_error_codes.dart';

/// Base class for all API exceptions
abstract class ApiException implements Exception {
  final String message;
  final String? code;
  final int? statusCode;
  final dynamic details;

  const ApiException({
    required this.message,
    this.code,
    this.statusCode,
    this.details,
  });

  @override
  String toString() =>
      '$runtimeType(message: $message, code: $code, statusCode: $statusCode)';
}

/// 400 Bad Request - Invalid request data
class BadRequestException extends ApiException {
  const BadRequestException({
    required super.message,
    super.code = 'BAD_REQUEST',
    super.statusCode = 400,
    super.details,
  });
}

/// 401 Unauthorized - Missing or invalid authentication
class UnauthorizedException extends ApiException {
  const UnauthorizedException({
    required super.message,
    super.code = 'UNAUTHORIZED',
    super.statusCode = 401,
    super.details,
  });
}

/// 403 Forbidden - Authenticated but not allowed
class ForbiddenException extends ApiException {
  const ForbiddenException({
    required super.message,
    super.code = 'FORBIDDEN',
    super.statusCode = 403,
    super.details,
  });
}

/// 404 Not Found - Resource doesn't exist
class NotFoundException extends ApiException {
  const NotFoundException({
    required super.message,
    super.code = 'NOT_FOUND',
    super.statusCode = 404,
    super.details,
  });
}

/// 409 Conflict - Resource conflict (duplicate, etc)
class ConflictException extends ApiException {
  const ConflictException({
    required super.message,
    super.code = 'CONFLICT',
    super.statusCode = 409,
    super.details,
  });
}

/// 422 Unprocessable Entity - Validation errors
class ValidationException extends ApiException {
  final Map<String, List<String>>? fieldErrors;

  const ValidationException({
    required super.message,
    this.fieldErrors,
    super.code = 'VALIDATION_ERROR',
    super.statusCode = 422,
    super.details,
  });

  @override
  String toString() =>
      'ValidationException(message: $message, fieldErrors: $fieldErrors)';
}

/// 429 Too Many Requests - Rate limited
class RateLimitException extends ApiException {
  final int? retryAfterSeconds;

  const RateLimitException({
    required super.message,
    this.retryAfterSeconds,
    super.code = 'RATE_LIMITED',
    super.statusCode = 429,
    super.details,
  });
}

/// 500 Internal Server Error
class ServerException extends ApiException {
  const ServerException({
    required super.message,
    super.code = 'SERVER_ERROR',
    super.statusCode = 500,
    super.details,
  });
}

/// 503 Service Unavailable - Maintenance, etc
class ServiceUnavailableException extends ApiException {
  const ServiceUnavailableException({
    required super.message,
    super.code = 'SERVICE_UNAVAILABLE',
    super.statusCode = 503,
    super.details,
  });
}

/// Network-related errors (no internet, timeout, etc)
class NetworkException extends ApiException {
  const NetworkException({
    required super.message,
    super.code = networkError,
    super.statusCode,
    super.details,
  });
}

/// Request timeout
class TimeoutException extends ApiException {
  const TimeoutException({
    super.message = 'Request timed out',
    super.code = requestTimeout,
    super.statusCode,
    super.details,
  });
}

/// Request was cancelled
class CancelledException extends ApiException {
  const CancelledException({
    super.message = 'Request was cancelled',
    super.code = requestCancelled,
    super.statusCode,
    super.details,
  });
}

/// Unknown/unexpected error
class UnknownApiException extends ApiException {
  const UnknownApiException({
    required super.message,
    super.code = unknownError,
    super.statusCode,
    super.details,
  });
}

/// Factory for creating exceptions from Dio failures and HTTP status codes
class ApiExceptionFactory {
  /// Classify a TRANSPORT failure — a request that never produced a usable
  /// HTTP envelope — into a typed [ApiException] carrying the canonical
  /// transport code from `api_error_codes.dart`.
  ///
  /// Returns `null` when the failure is not a transport failure, i.e. when
  /// [DioException.type] is `badResponse`: that case DOES carry an HTTP
  /// envelope and is classified by [fromStatusCode]. Returning null instead of
  /// guessing keeps the two families apart — callers hold the HTTP branch.
  ///
  /// Both the Dio error interceptor (which wraps the result into
  /// `DioException.error`) and [ApiClient.extractException] (the fallback for
  /// an un-intercepted failure) classify through THIS method, so every
  /// transport failure reaches `Result.errorCode` with the same code.
  static ApiException? fromTransport(DioException exception) {
    switch (exception.type) {
      case DioExceptionType.connectionTimeout:
      case DioExceptionType.sendTimeout:
      case DioExceptionType.receiveTimeout:
      case DioExceptionType.transformTimeout:
        return const TimeoutException(
          message: 'Connection timed out. Please try again.',
          code: requestTimeout,
        );

      case DioExceptionType.cancel:
        return const CancelledException(code: requestCancelled);

      case DioExceptionType.connectionError:
        // A connectionError means the socket/handshake to the configured
        // backend host failed (refused, unreachable, wrong IP, backend down).
        // It is distinct from the device having no network at all — reporting
        // it as "no internet" misleads a user whose WiFi/data is fine but
        // whose backend is unreachable.
        return const NetworkException(
          message:
              'Cannot reach HiShumi server. Check that the backend is running '
              'and the device is on the same network.',
          code: backendUnreachable,
        );

      case DioExceptionType.badCertificate:
        return const NetworkException(
          message: 'SSL certificate error. Please try again later.',
          code: sslError,
        );

      case DioExceptionType.badResponse:
        // Not a transport failure — see the `null` contract above.
        return null;

      case DioExceptionType.unknown:
        if (exception.error is SocketException) {
          return const NetworkException(
            message: 'Network error. Please check your connection.',
            code: networkError,
          );
        }
        return UnknownApiException(
          message: exception.message ?? 'An unexpected error occurred',
          code: unknownError,
          details: exception.error,
        );
    }
  }

  /// Factory for creating exceptions from HTTP status codes
  static ApiException fromStatusCode(
    int statusCode,
    String message, {
    String? code,
    dynamic details,
    Map<String, List<String>>? fieldErrors,
    int? retryAfterSeconds,
  }) {
    switch (statusCode) {
      case 400:
        return BadRequestException(
          message: message,
          code: code,
          details: details,
        );
      case 401:
        return UnauthorizedException(
          message: message,
          code: code,
          details: details,
        );
      case 403:
        return ForbiddenException(
          message: message,
          code: code,
          details: details,
        );
      case 404:
        return NotFoundException(
          message: message,
          code: code,
          details: details,
        );
      case 409:
        return ConflictException(
          message: message,
          code: code,
          details: details,
        );
      case 422:
        return ValidationException(
          message: message,
          code: code,
          details: details,
          fieldErrors: fieldErrors,
        );
      case 429:
        return RateLimitException(
          message: message,
          code: code,
          details: details,
          retryAfterSeconds: retryAfterSeconds,
        );
      case 500:
        return ServerException(message: message, code: code, details: details);
      case 503:
        return ServiceUnavailableException(
          message: message,
          code: code,
          details: details,
        );
      default:
        if (statusCode >= 500) {
          return ServerException(
            message: message,
            code: code,
            statusCode: statusCode,
            details: details,
          );
        }
        return UnknownApiException(
          message: message,
          code: code,
          statusCode: statusCode,
          details: details,
        );
    }
  }
}
