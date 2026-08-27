/// Generic result class for handling success and error states
class Result<T> {
  final T? _data;
  final String? _error;
  final String? _errorCode;
  final int? _statusCode;
  final Map<String, dynamic>? _errorDetails;
  final bool _isSuccess;

  const Result._(
    this._data,
    this._error,
    this._isSuccess, [
    this._errorCode,
    this._statusCode,
    this._errorDetails,
  ]);

  /// Creates a successful result with data
  factory Result.success(T data) => Result._(data, null, true);

  /// Creates a failure result with error message
  ///
  /// [code] preserves the backend's machine-readable error code (e.g.
  /// `EMAIL_VERIFICATION_REQUIRED`, `EMPTY_DATA`). [statusCode] preserves
  /// the HTTP status code when available so call sites can distinguish
  /// transport failures (5xx) from schema/envelope violations (200 with
  /// broken payload).
  factory Result.error(
    String error, {
    String? code,
    int? statusCode,
    Map<String, dynamic>? details,
  }) => Result._(null, error, false, code, statusCode, details);

  /// Returns true if the result is successful
  bool get isSuccess => _isSuccess;

  /// Returns true if the result is an error
  bool get isError => !_isSuccess;

  /// Alias for isError for backward compatibility
  bool get isFailure => !_isSuccess;

  /// Returns the data if successful, null otherwise
  T? get data => _data;

  /// Returns the error message if failed, null otherwise
  String? get error => _error;

  /// Returns the machine-readable error code if available (e.g.
  /// `EMAIL_VERIFICATION_REQUIRED`, `ACCOUNT_DELETED`).
  /// Null for transport-level errors or success results.
  String? get errorCode => _errorCode;

  /// Returns the HTTP status code if available. Null on success or when
  /// the failure originated outside of an HTTP response (e.g. local parse
  /// of a cached payload).
  int? get statusCode => _statusCode;

  /// Returns structured error details from the API response (e.g.
  /// `{"permanent_ban": true, "active_strikes": 4}`). Null on success or
  /// when no details were provided.
  Map<String, dynamic>? get errorDetails => _errorDetails;

  /// Transforms the data if successful, otherwise returns error result
  ///
  /// PASS 2A: the error branch must forward `_errorCode`/`_statusCode`/
  /// `_errorDetails` — dropping them here silently strips the backend's
  /// structured error code from every `Result.map()` call in the app,
  /// forcing callers back onto free-text message matching.
  Result<U> map<U>(U Function(T data) transform) {
    if (isSuccess) {
      try {
        return Result.success(transform(_data as T));
      } catch (e) {
        return Result.error('Transform error: ${e.toString()}');
      }
    }
    return Result.error(
      _error ?? 'Unknown error',
      code: _errorCode,
      statusCode: _statusCode,
      details: _errorDetails,
    );
  }

  /// Executes a function based on the result state
  U fold<U>(U Function(String error) onError, U Function(T data) onSuccess) {
    if (isSuccess) {
      return onSuccess(_data as T);
    }
    return onError(_error ?? 'Unknown error');
  }

  @override
  String toString() {
    if (isSuccess) {
      return 'Result.success($_data)';
    }
    return 'Result.error($_error)';
  }
}
