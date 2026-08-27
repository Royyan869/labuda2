/// Commerce seller identity formatting helpers.
///
/// Business truth:
/// - line 1: @username
/// - line 2: store/farm name when present
///
/// Presentation-only helper. No DTO or backend contract changes.
library;

class CommerceSellerIdentity {
  final String line1;
  final String? line2;

  const CommerceSellerIdentity({required this.line1, this.line2});

  String get multilineLabel =>
      line2 == null || line2!.isEmpty ? line1 : '$line1\n$line2';
}

CommerceSellerIdentity? buildCommerceSellerIdentity({
  required String? username,
  required String? storeName,
}) {
  final cleanUsername = username?.trim();
  if (cleanUsername == null || cleanUsername.isEmpty) return null;

  final cleanStoreName = storeName?.trim();
  return CommerceSellerIdentity(
    line1: '@$cleanUsername',
    line2: cleanStoreName != null && cleanStoreName.isNotEmpty
        ? cleanStoreName
        : null,
  );
}
