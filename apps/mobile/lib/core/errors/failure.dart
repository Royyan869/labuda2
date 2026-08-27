abstract class Failure {
  final String message;
  final String? code;
  final dynamic details;

  const Failure({required this.message, this.code, this.details});

  @override
  String toString() => 'Failure(message: $message, code: $code)';

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is Failure && other.message == message && other.code == code;
  }

  @override
  int get hashCode => message.hashCode ^ code.hashCode;
}

class NetworkFailure extends Failure {
  const NetworkFailure({required super.message, super.code, super.details});
}

class ServerFailure extends Failure {
  const ServerFailure({required super.message, super.code, super.details});
}

class CacheFailure extends Failure {
  const CacheFailure({required super.message, super.code, super.details});
}

class ValidationFailure extends Failure {
  const ValidationFailure({required super.message, super.code, super.details});
}

class AuthenticationFailure extends Failure {
  const AuthenticationFailure({
    required super.message,
    super.code,
    super.details,
  });
}

class AuthorizationFailure extends Failure {
  const AuthorizationFailure({
    required super.message,
    super.code,
    super.details,
  });
}

class UnknownFailure extends Failure {
  const UnknownFailure({required super.message, super.code, super.details});
}

/// Factory untuk common failure types
extension FailureFactory on Failure {
  static Failure unexpected(String message) =>
      UnknownFailure(message: message, code: 'UNEXPECTED_ERROR');

  static Failure network(String message) =>
      NetworkFailure(message: message, code: 'NETWORK_ERROR');

  static Failure validation(String message) =>
      ValidationFailure(message: message, code: 'VALIDATION_ERROR');
}
