/// Repository Result
///
/// Generic result type for repository operations.
library;

class RepositoryResult<T> {
  final T? data;
  final String? error;

  /// Machine-readable code for [error], when the failure came from a known
  /// API contract (e.g. `EMAIL_VERIFICATION_REQUIRED`). Null when the error
  /// is transport-level or untagged.
  final String? errorCode;

  /// Structured details from the API error response (e.g.
  /// `{"permanent_ban": true, "active_strikes": 4}`). Null when no details
  /// were provided or the error is transport-level.
  final Map<String, dynamic>? errorDetails;

  RepositoryResult({this.data, this.error, this.errorCode, this.errorDetails});

  bool get isSuccess => data != null;
  bool get isError => error != null;
  bool get isFailure => error != null;

  factory RepositoryResult.success(T data) => RepositoryResult(data: data);
  factory RepositoryResult.failure(
    String error, {
    String? code,
    Map<String, dynamic>? details,
  }) => RepositoryResult(error: error, errorCode: code, errorDetails: details);

  // Alias for failure to match code using .error()
  factory RepositoryResult.error(
    String error, {
    String? code,
    Map<String, dynamic>? details,
  }) => RepositoryResult(error: error, errorCode: code, errorDetails: details);

  // Map method for transformations
  RepositoryResult<R> map<R>(R Function(T data) transform) {
    if (isSuccess && data != null) {
      try {
        return RepositoryResult.success(transform(data as T));
      } catch (e) {
        return RepositoryResult.error(e.toString());
      }
    }
    return RepositoryResult.error(error ?? 'Unknown error');
  }

  // Fold method for handling both cases
  R fold<R>(R Function(T data) onSuccess, R Function(String error) onError) {
    if (isSuccess && data != null) {
      return onSuccess(data as T);
    }
    return onError(error ?? 'Unknown error');
  }
}
