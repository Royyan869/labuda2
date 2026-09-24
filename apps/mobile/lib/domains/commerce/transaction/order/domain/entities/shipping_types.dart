/// Shipping Method — canonical transport vocabulary. Mirrors the backend
/// snapshot values (train, bus, travel, plane, custom, manual). The legacy
/// 'courier' and 'selfPickup' designs are killed and must not be revived.
enum ShippingMethod { bus, travel, train, plane, custom }

extension ShippingMethodExtension on ShippingMethod {
  String get label {
    switch (this) {
      case ShippingMethod.bus:
        return 'Bus Kargo';
      case ShippingMethod.travel:
        return 'Travel';
      case ShippingMethod.train:
        return 'Kereta';
      case ShippingMethod.plane:
        return 'Pesawat Cargo';
      case ShippingMethod.custom:
        return 'Custom';
    }
  }

  /// Maps the backend wire snapshot (shipping_transport_type) to the
  /// canonical enum. Unknown/empty values fall back to custom.
  static ShippingMethod fromWireType(String? wireType) {
    switch (wireType) {
      case 'bus':
        return ShippingMethod.bus;
      case 'travel':
        return ShippingMethod.travel;
      case 'train':
        return ShippingMethod.train;
      case 'plane':
        return ShippingMethod.plane;
      default:
        return ShippingMethod.custom;
    }
  }
}
