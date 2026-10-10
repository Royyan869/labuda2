/// Canonical representation of a HiShumi session credential.
///
/// This model holds the platform access and refresh tokens issued by the
/// HiShumi backend after Firebase exchange or profile completion.
///
/// Firebase ID tokens are NOT part of this model. Firebase credentials are
/// managed separately by the Firebase SDK.
///
/// A valid [HiShumiSessionCredential] always has both tokens present.
/// Partial credentials (access only or refresh only) should be represented
/// as `null` at the HiShumi credential store level, not as incomplete
/// [HiShumiSessionCredential] instances.
class HiShumiSessionCredential {
  /// Backend-issued HiShumi access JWT.
  final String accessToken;

  /// Backend-issued HiShumi refresh JWT.
  final String refreshToken;

  const HiShumiSessionCredential({
    required this.accessToken,
    required this.refreshToken,
  });

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is HiShumiSessionCredential &&
          runtimeType == other.runtimeType &&
          accessToken == other.accessToken &&
          refreshToken == other.refreshToken;

  @override
  int get hashCode => Object.hash(accessToken, refreshToken);

  @override
  String toString() =>
      'HiShumiSessionCredential(accessToken: [REDACTED], refreshToken: [REDACTED])';
}
