/// Canonical representation of a Labuda session credential.
///
/// This model holds the platform access and refresh tokens issued by the
/// Labuda backend after Firebase exchange or profile completion.
///
/// Firebase ID tokens are NOT part of this model. Firebase credentials are
/// managed separately by the Firebase SDK.
///
/// A valid [LabudaSessionCredential] always has both tokens present.
/// Partial credentials (access only or refresh only) should be represented
/// as `null` at the [LabudaCredentialStore] level, not as incomplete
/// [LabudaSessionCredential] instances.
class LabudaSessionCredential {
  /// Backend-issued Labuda access JWT.
  final String accessToken;

  /// Backend-issued Labuda refresh JWT.
  final String refreshToken;

  const LabudaSessionCredential({
    required this.accessToken,
    required this.refreshToken,
  });

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is LabudaSessionCredential &&
          runtimeType == other.runtimeType &&
          accessToken == other.accessToken &&
          refreshToken == other.refreshToken;

  @override
  int get hashCode => Object.hash(accessToken, refreshToken);

  @override
  String toString() =>
      'LabudaSessionCredential(accessToken: [REDACTED], refreshToken: [REDACTED])';
}
