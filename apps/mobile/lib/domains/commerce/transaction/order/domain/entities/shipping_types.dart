/// Shipping Method
enum ShippingMethod { bus, travel, train, plane, courier, selfPickup, custom }

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
      case ShippingMethod.courier:
        return 'Kurir';
      case ShippingMethod.selfPickup:
        return 'Ambil Sendiri';
      case ShippingMethod.custom:
        return 'Custom';
    }
  }
}
