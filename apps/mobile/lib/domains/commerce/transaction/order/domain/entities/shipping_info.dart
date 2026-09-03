import 'package:equatable/equatable.dart';
import 'shipping_types.dart';

/// Shipping Information
///
/// SHIPPING CONFIRMATION TRUTH:
/// - trackingNumber: Generic reference (resi, phone/WA, or other)
/// - referenceType: Type hint for UI labeling ("tracking" | "phone" | "other")
/// - shippingNote: Seller's shipping note for buyer context
class ShippingInfo extends Equatable {
  final String recipientName;
  final String phone;
  final String address;
  final String? provinceId;
  final String? provinceName;
  final String? cityId;
  final String? cityName;
  final String? districtId;
  final String? districtName;
  final String? villageId;
  final String? villageName;
  final String? postalCode;
  final double? latitude;
  final double? longitude;
  final ShippingMethod method;
  final String? courierName;
  final String? trackingNumber;

  /// SHIPPING CONFIRMATION TRUTH: Reference type for honest UI labeling
  /// - "tracking": Courier tracking number (resi)
  /// - "phone": Phone/WhatsApp number for manual coordination
  /// - "other": Other reference (e.g., travel driver name, etc.)
  final String? referenceType;

  /// SHIPPING CONFIRMATION TRUTH: Seller's shipping note
  /// Provides buyer context like "berangkat malam ini", "dititip ke sopir travel"
  final String? shippingNote;

  final double shippingCost;
  final String? notes;
  final String? shippingSetupId;

  const ShippingInfo({
    required this.recipientName,
    required this.phone,
    required this.address,
    this.provinceId,
    this.provinceName,
    this.cityId,
    this.cityName,
    this.districtId,
    this.districtName,
    this.villageId,
    this.villageName,
    this.postalCode,
    this.latitude,
    this.longitude,
    required this.method,
    this.courierName,
    this.trackingNumber,
    this.referenceType,
    this.shippingNote,
    required this.shippingCost,
    this.notes,
    this.shippingSetupId,
  });

  bool get hasCoordinates => latitude != null && longitude != null;

  /// Get honest label for shipping reference based on reference type
  String getReferenceLabel() {
    switch (referenceType) {
      case 'phone':
        return 'No. HP / WA Pengiriman';
      case 'other':
        return 'Referensi Pengiriman';
      case 'tracking':
      default:
        return 'Nomor Resi';
    }
  }

  @override
  List<Object?> get props => [
    recipientName,
    phone,
    address,
    provinceId,
    provinceName,
    cityId,
    cityName,
    districtId,
    districtName,
    villageId,
    villageName,
    postalCode,
    latitude,
    longitude,
    method,
    courierName,
    trackingNumber,
    referenceType,
    shippingNote,
    shippingCost,
    notes,
    shippingSetupId,
  ];
}
