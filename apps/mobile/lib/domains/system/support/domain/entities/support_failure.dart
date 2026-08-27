library;

/// Domain Entities for Support Failure Handling
/// Pure Dart - bebas dari Firebase, Flutter, dan external dependencies

import 'package:equatable/equatable.dart';

// ============================================
// FAILURE TYPES
// ============================================

/// Base Failure class untuk Support module
abstract class SupportFailure extends Equatable {
  final String message;
  final dynamic originalError;

  const SupportFailure({required this.message, this.originalError});

  @override
  List<Object?> get props => [message, originalError];
}

/// General failure untuk unexpected errors
class SupportFailureUnknown extends SupportFailure {
  const SupportFailureUnknown({
    super.message = 'An unknown error occurred',
    super.originalError,
  });
}

/// Network failure
class SupportFailureNetwork extends SupportFailure {
  const SupportFailureNetwork({
    super.message = 'Network error occurred',
    super.originalError,
  });
}

/// Permission failure - user tidak punya akses
class SupportFailurePermission extends SupportFailure {
  const SupportFailurePermission({
    super.message = 'You do not have permission to perform this action',
    super.originalError,
  });
}

/// Not found failure - ticket tidak ditemukan
class SupportFailureNotFound extends SupportFailure {
  const SupportFailureNotFound({
    super.message = 'Support ticket not found',
    super.originalError,
  });
}

/// Validation failure - input tidak valid
class SupportFailureValidation extends SupportFailure {
  final List<String> errors;

  const SupportFailureValidation({
    super.message = 'Validation failed',
    this.errors = const [],
    super.originalError,
  });

  @override
  List<Object?> get props => [message, errors, originalError];
}

/// Ticket already assigned failure
class SupportFailureAlreadyAssigned extends SupportFailure {
  const SupportFailureAlreadyAssigned({
    super.message = 'Ticket is already assigned to another admin',
    super.originalError,
  });
}

/// Ticket already resolved/closed failure
class SupportFailureAlreadyResolved extends SupportFailure {
  const SupportFailureAlreadyResolved({
    super.message = 'Ticket is already resolved or closed',
    super.originalError,
  });
}

/// Cannot reopen ticket failure
class SupportFailureCannotReopen extends SupportFailure {
  const SupportFailureCannotReopen({
    super.message = 'Cannot reopen this ticket',
    super.originalError,
  });
}

/// Result type untuk support operations
/// Menggunakan Either-like pattern (Left = Failure, Right = Success)
class SupportResult<T> {
  final T? data;
  final SupportFailure? failure;

  const SupportResult._({this.data, this.failure});

  /// Create success result
  factory SupportResult.success(T data) {
    return SupportResult._(data: data);
  }

  /// Create failure result
  factory SupportResult.failure(SupportFailure failure) {
    return SupportResult._(failure: failure);
  }

  /// Check if result is success
  bool get isSuccess => failure == null;

  /// Check if result is failure
  bool get isFailure => failure != null;

  /// Get data or throw
  T get dataOrThrow {
    if (failure != null) {
      throw Exception(failure!.message);
    }
    return data as T;
  }

  /// Fold pattern - transform result berdasarkan success/failure
  R fold<R>({
    required R Function(T data) onSuccess,
    required R Function(SupportFailure failure) onFailure,
  }) {
    if (isSuccess) {
      return onSuccess(data as T);
    }
    return onFailure(failure!);
  }

  /// Map success data
  SupportResult<R> map<R>(R Function(T data) mapper) {
    if (isSuccess) {
      try {
        return SupportResult.success(mapper(data as T));
      } catch (e) {
        return SupportResult.failure(
          SupportFailureUnknown(message: e.toString(), originalError: e),
        );
      }
    }
    return SupportResult.failure(failure!);
  }

  /// Async map success data
  Future<SupportResult<R>> mapAsync<R>(
    Future<R> Function(T data) mapper,
  ) async {
    if (isSuccess) {
      try {
        final result = await mapper(data as T);
        return SupportResult.success(result);
      } catch (e) {
        return SupportResult.failure(
          SupportFailureUnknown(message: e.toString(), originalError: e),
        );
      }
    }
    return SupportResult.failure(failure!);
  }
}

/// Extension untuk `Future<SupportResult>`
extension SupportResultFutureExtension<T> on Future<SupportResult<T>> {
  /// Map success data asynchronously
  Future<SupportResult<R>> map<R>(R Function(T data) mapper) async {
    final result = await this;
    return result.map(mapper);
  }

  /// Chain async operations
  Future<SupportResult<R>> andThen<R>(
    Future<SupportResult<R>> Function(T data) mapper,
  ) async {
    final result = await this;
    if (result.isFailure) {
      return SupportResult.failure(result.failure!);
    }
    return await mapper(result.data as T);
  }
}

/// Extension untuk SupportResult dengan nullable data
extension SupportResultNullableExtension<T> on SupportResult<T?> {
  /// Get data or null
  T? get dataOrNull => isSuccess ? data : null;
}
