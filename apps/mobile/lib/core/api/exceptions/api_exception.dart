// API Exception types for handling backend errors
// Maps HTTP status codes and API error responses to typed exceptions

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
    super.code = 'NETWORK_ERROR',
    super.statusCode,
    super.details,
  });
}

/// Request timeout
class TimeoutException extends ApiException {
  const TimeoutException({
    super.message = 'Request timed out',
    super.code = 'TIMEOUT',
    super.statusCode,
    super.details,
  });
}

/// Request was cancelled
class CancelledException extends ApiException {
  const CancelledException({
    super.message = 'Request was cancelled',
    super.code = 'CANCELLED',
    super.statusCode,
    super.details,
  });
}

/// Unknown/unexpected error
class UnknownApiException extends ApiException {
  const UnknownApiException({
    required super.message,
    super.code = 'UNKNOWN_ERROR',
    super.statusCode,
    super.details,
  });
}

/// Factory for creating exceptions from HTTP status codes
class ApiExceptionFactory {
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
