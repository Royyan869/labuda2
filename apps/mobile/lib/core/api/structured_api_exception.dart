/// Exception that carries structured API error information through
/// throw/catch chains, preventing error code loss in fold→throw→catch paths.
///
/// When a datasource uses `Result.fold((error) => throw ..., ...)`, wrapping
/// the error in this exception preserves the machine-readable code so the
/// repository layer can propagate it via `RepositoryResult.error(..., code:)`.
class StructuredApiException implements Exception {
  final String message;
  final String? code;
  final Map<String, dynamic>? details;

  const StructuredApiException({
    required this.message,
    this.code,
    this.details,
  });

  @override
  String toString() => message;
}
