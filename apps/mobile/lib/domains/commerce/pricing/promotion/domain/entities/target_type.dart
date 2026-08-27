/// Target type enum for promotion.
///
/// Defines what can be promoted:
/// - for_sale: A fixed-price sale
/// - auction: An auction
/// - externalProduct: An external product (URL-based)
enum TargetType {
  /// A fixed-price sale can be promoted
  forSale('for_sale'),

  /// An auction can be promoted
  auction('auction'),

  /// An external product (with embedded URL, title, media) can be promoted
  externalProduct('external_product');

  const TargetType(this.value);

  /// The API value for this enum
  final String value;

  /// Parses target type from string
  static TargetType fromString(String value) {
    return TargetType.values.firstWhere(
      (type) => type.value == value,
      orElse: () => TargetType.forSale,
    );
  }
}

/// Extension for TargetType utility methods
extension TargetTypeX on TargetType {
  /// Whether this target type requires a target_id reference
  bool get requiresTargetId =>
      this == TargetType.forSale || this == TargetType.auction;

  /// Whether this target type requires external fields (URL, title, media)
  bool get requiresExternalFields => this == TargetType.externalProduct;
}
