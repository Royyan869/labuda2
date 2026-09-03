import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/entities/shipping.dart';

/// Shipping Repository Interface
/// Handles CRUD operations for seller shipping options
abstract class ShippingRepository {
  // =====================================
  // Shipping Option CRUD
  // =====================================

  /// Get all shipping options for the authenticated seller.
  Future<Result<List<ShippingSetup>>> listMyShippingSetups();

  /// Get active shipping options for the authenticated seller.
  Future<Result<List<ShippingSetup>>> listMyActiveShippingSetups();

  /// Get shipping option by ID
  Future<Result<ShippingSetup>> getShippingSetupById(String optionId);

  /// Create new shipping option
  Future<Result<ShippingSetup>> createShippingSetup(
    CreateShippingSetupRequest request,
  );

  /// Update shipping option
  Future<Result<ShippingSetup>> updateShippingSetup(
    String optionId,
    UpdateShippingSetupRequest request,
  );

  /// Update a shipping option together with its coverages in a single
  /// atomic request.
  Future<Result<ShippingSetup>> updateShippingSetupFull(
    String optionId,
    UpdateShippingSetupFullRequest request,
  );

  /// Delete shipping option
  Future<Result<void>> deleteShippingSetup(String optionId);

  /// Toggle active status
  Future<Result<void>> toggleActiveStatus(String optionId, bool isActive);

  // =====================================
  // Coverage Management
  // =====================================

  /// Add coverage to a shipping option
  Future<Result<ShippingCoverage>> addCoverage(
    String optionId,
    AddCoverageRequest request,
  );

  /// Update coverage
  Future<Result<ShippingCoverage>> updateCoverage(
    String coverageId,
    UpdateCoverageRequest request,
  );

  /// Delete coverage
  Future<Result<void>> deleteCoverage(String coverageId);

  // =====================================
  // Product-Shipping Link
  // =====================================

  /// Set the shipping-option subset that applies to a single product.
  ///
  /// Overwrite semantics: the backend replaces the current
  /// `product_shipping_options` rows for [productId] with [shippingSetupIds].
  /// Empty list is allowed and clears all links (but a publish gate will
  /// then reject the listing for SHIPPING_NOT_CONFIGURED on next activation).
  Future<Result<void>> setProductShippingSetups(
    String productId,
    List<String> shippingSetupIds,
  );

  // =====================================
  // Delivery Check
  // =====================================

  /// Check if seller can deliver to specific location
  /// Returns list of available delivery options with rates
  Future<Result<List<DeliveryOption>>> checkDeliveryAvailability(
    CheckDeliveryRequest request,
  );
}

/// Repository untuk Shipping Proof (terpisah karena domain berbeda - order related)
abstract class ShippingProofRepository {
  /// Upload shipping proof for an order
  Future<Result<ShippingProof>> uploadShippingProof(
    String orderId,
    CreateShippingProofRequest request,
  );

  /// Get shipping proof for an order
  Future<Result<ShippingProof>> getShippingProof(String orderId);

  /// Update shipping proof for an order
  Future<Result<ShippingProof>> updateShippingProof(
    String orderId,
    UpdateShippingProofRequest request,
  );
}

// =====================================
// Shipping Proof Entities
// =====================================

/// Shipping Proof entity
///
/// SHIPPING CONFIRMATION TRUTH:
/// - shippingReference: The shipping reference (resi, phone/WA, or other)
/// - referenceType: Type hint ("tracking" | "phone" | "other")
/// - shippingNote: Optional note from seller
class ShippingProof {
  final String id;
  final String orderId;
  final String sellerId;
  final List<String> photos;
  final List<String> videos;

  // Shipping confirmation fields (canonical)
  final String? shippingReference;
  final String? referenceType; // "tracking" | "phone" | "other"
  final String? shippingNote;

  // Additional fields
  final String? courierPhone;
  final String? formattedCourierPhone;

  final DateTime createdAt;
  final DateTime updatedAt;

  const ShippingProof({
    required this.id,
    required this.orderId,
    required this.sellerId,
    required this.photos,
    required this.videos,
    this.shippingReference,
    this.referenceType,
    this.shippingNote,
    this.courierPhone,
    this.formattedCourierPhone,
    required this.createdAt,
    required this.updatedAt,
  });

  /// Get the effective shipping reference
  String? get effectiveReference => shippingReference;

  /// Get the effective reference type (default to 'tracking' if not set)
  String get effectiveReferenceType => referenceType ?? 'tracking';

  /// Get label for shipping reference based on type
  String get referenceLabel {
    switch (effectiveReferenceType) {
      case 'phone':
        return 'No. HP / WA';
      case 'other':
        return 'Referensi Lain';
      default:
        return 'Nomor Resi';
    }
  }

  ShippingProof copyWith({
    String? id,
    String? orderId,
    String? sellerId,
    List<String>? photos,
    List<String>? videos,
    String? shippingReference,
    String? referenceType,
    String? shippingNote,
    String? courierPhone,
    String? formattedCourierPhone,
    DateTime? createdAt,
    DateTime? updatedAt,
  }) {
    return ShippingProof(
      id: id ?? this.id,
      orderId: orderId ?? this.orderId,
      sellerId: sellerId ?? this.sellerId,
      photos: photos ?? this.photos,
      videos: videos ?? this.videos,
      shippingReference: shippingReference ?? this.shippingReference,
      referenceType: referenceType ?? this.referenceType,
      shippingNote: shippingNote ?? this.shippingNote,
      courierPhone: courierPhone ?? this.courierPhone,
      formattedCourierPhone:
          formattedCourierPhone ?? this.formattedCourierPhone,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
    );
  }

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is ShippingProof &&
        other.id == id &&
        other.orderId == orderId &&
        other.sellerId == sellerId;
  }

  @override
  int get hashCode => Object.hash(id, orderId, sellerId);
}

/// Request untuk create shipping proof
class CreateShippingProofRequest {
  final List<String>? photos;
  final List<String>? videos;
  final String? shippingReference;
  final String? referenceType;
  final String? shippingNote;
  final String? courierPhone;

  const CreateShippingProofRequest({
    this.photos,
    this.videos,
    this.shippingReference,
    this.referenceType,
    this.shippingNote,
    this.courierPhone,
  });

  Map<String, dynamic> toJson() {
    return {
      if (photos != null) 'photos': photos,
      if (videos != null) 'videos': videos,
      if (shippingReference != null) 'shipping_reference': shippingReference,
      if (referenceType != null) 'reference_type': referenceType,
      if (shippingNote != null) 'shipping_note': shippingNote,
      if (courierPhone != null) 'courier_phone': courierPhone,
    };
  }
}

/// Request untuk update shipping proof
class UpdateShippingProofRequest {
  final List<String>? photos;
  final List<String>? videos;
  final String? shippingReference;
  final String? referenceType;
  final String? shippingNote;
  final String? courierPhone;

  const UpdateShippingProofRequest({
    this.photos,
    this.videos,
    this.shippingReference,
    this.referenceType,
    this.shippingNote,
    this.courierPhone,
  });

  Map<String, dynamic> toJson() {
    return {
      if (photos != null) 'photos': photos,
      if (videos != null) 'videos': videos,
      if (shippingReference != null) 'shipping_reference': shippingReference,
      if (referenceType != null) 'reference_type': referenceType,
      if (shippingNote != null) 'shipping_note': shippingNote,
      if (courierPhone != null) 'courier_phone': courierPhone,
    };
  }
}
