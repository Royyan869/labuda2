import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/transaction/shipping/data/dto/shipping_dto.dart';

/// Shipping Remote Datasource
/// API-based datasource menggunakan ApiClient
class ShippingRemoteDatasource {
  final ApiClient _apiClient;

  ShippingRemoteDatasource(this._apiClient);

  // =====================================
  // Shipping Option Methods
  // =====================================

  /// Create a shipping option as ONE package (identity + destinations).
  /// Bare options without destinations are rejected by the backend.
  Future<ShippingSetupDto> createShippingSetup(
    Map<String, dynamic> data,
  ) async {
    final response = await _apiClient.post(
      '/seller/shipping/options',
      data: data,
    );
    final envelope = _decodeEnvelope(response, 'create shipping option');
    return _decodeShippingSetupEnvelope(envelope).shippingSetup;
  }

  /// Get shipping option by ID
  Future<ShippingSetupDto> getShippingSetup(String optionId) async {
    final response = await _apiClient.get('/seller/shipping/options/$optionId');
    final envelope = _decodeEnvelope(response, 'get shipping option');
    return _decodeShippingSetupEnvelope(envelope).shippingSetup;
  }

  /// Update a shipping option as ONE package. When [data] carries
  /// `destinations`, the backend replaces the full destination set in one
  /// transaction. Editing is allowed at any time (orders keep their snapshot).
  Future<ShippingSetupDto> updateShippingSetup(
    String optionId,
    Map<String, dynamic> data,
  ) async {
    final response = await _apiClient.put(
      '/seller/shipping/options/$optionId',
      data: data,
    );
    final envelope = _decodeEnvelope(response, 'update shipping option');
    return _decodeShippingSetupEnvelope(envelope).shippingSetup;
  }

  /// Canonical retire/restore path (PATCH .../active). Linked options must be
  /// deactivated, never hard-deleted.
  Future<void> toggleShippingSetup(String optionId, bool isActive) async {
    await _apiClient.patch(
      '/seller/shipping/options/$optionId/active',
      data: {'is_active': isActive},
    );
  }

  /// Delete shipping option. The backend refuses (409) while the option is
  /// linked to any listing — the seller must deactivate instead.
  Future<void> deleteShippingSetup(String optionId) async {
    await _apiClient.delete('/seller/shipping/options/$optionId');
  }

  /// List my shipping options
  Future<List<ShippingSetupDto>> listMyShippingSetups({
    bool includeInactive = true,
  }) async {
    final response = await _apiClient.get(
      '/seller/shipping/options',
      queryParameters: {'include_inactive': includeInactive},
    );
    final envelope = _decodeEnvelope(response, 'list shipping options');
    return _decodeShippingSetupsEnvelope(envelope).shippingSetups;
  }

  /// List my active shipping options only
  Future<List<ShippingSetupDto>> listMyActiveShippingSetups() async {
    return listMyShippingSetups(includeInactive: false);
  }

  // =====================================
  // Product-Shipping Link Methods
  // =====================================

  /// Set the shipping options that apply to a given product.
  ///
  /// Overwrite semantics: the backend deletes existing rows in
  /// `product_shipping_options` and inserts a fresh set in a single tx
  /// (see backend [ProductShippingService.SetProductShippingSetups]).
  /// An empty list clears all linked options.
  ///
  /// Backend rejects if any of the option IDs do not belong to the calling
  /// seller, or if the forSale already has active orders.
  Future<void> setProductShippingSetups(
    String productId,
    List<String> shippingSetupIds,
  ) async {
    await _apiClient.put(
      '/products/$productId/shipping',
      data: {'shipping_option_ids': shippingSetupIds},
    );
  }

  // =====================================
  // Delivery Check Methods
  // =====================================

  /// Check delivery availability
  Future<CheckDeliveryResponseDto> checkDeliveryAvailability(
    Map<String, dynamic> data,
  ) async {
    final response = await _apiClient.post('/shipping/check', data: data);
    final envelope = _decodeEnvelope(response, 'check delivery');
    return CheckDeliveryResponseDto.fromJson(envelope);
  }

  // =====================================
  // Shipping Proof Methods
  // =====================================

  /// Upload shipping proof for an order
  Future<ShippingProofDto> uploadShippingProof(
    String orderId,
    Map<String, dynamic> data,
  ) async {
    final response = await _apiClient.post(
      '/orders/$orderId/shipping-proof',
      data: data,
    );
    final envelope = _decodeEnvelope(response, 'upload shipping proof');
    return ShippingProofDto.fromJson(_expectMap(envelope['shipping_proof']));
  }

  /// Get shipping proof for an order
  Future<ShippingProofDto> getShippingProof(String orderId) async {
    final response = await _apiClient.get('/orders/$orderId/shipping-proof');
    final envelope = _decodeEnvelope(response, 'get shipping proof');
    return ShippingProofDto.fromJson(_expectMap(envelope['shipping_proof']));
  }

  /// Update shipping proof for an order
  Future<ShippingProofDto> updateShippingProof(
    String orderId,
    Map<String, dynamic> data,
  ) async {
    final response = await _apiClient.put(
      '/orders/$orderId/shipping-proof',
      data: data,
    );
    final envelope = _decodeEnvelope(response, 'update shipping proof');
    return ShippingProofDto.fromJson(_expectMap(envelope['shipping_proof']));
  }
}

Map<String, dynamic> _decodeEnvelope(dynamic response, String context) {
  final data = response.data;
  if (data is! Map<String, dynamic>) {
    throw FormatException(
      'Expected $context response envelope to be a JSON object, got ${data.runtimeType}',
    );
  }

  if (data['success'] != true) {
    throw FormatException('Expected successful $context response envelope');
  }

  final inner = data['data'];
  if (inner is! Map<String, dynamic>) {
    throw FormatException(
      'Expected $context response data to be a JSON object, got ${inner.runtimeType}',
    );
  }

  return inner;
}

ShippingSetupEnvelopeDto _decodeShippingSetupEnvelope(
  Map<String, dynamic> envelope,
) {
  final optionJson = _expectMap(envelope['shipping_option']);
  final option = ShippingSetupDto.fromJson(optionJson);

  final coveragesRaw = envelope['coverages'];
  final coverages = coveragesRaw is List
      ? coveragesRaw
            .map((entry) => ShippingCoverageDto.fromJson(_expectMap(entry)))
            .toList(growable: false)
      : option.coverages;

  return ShippingSetupEnvelopeDto(
    shippingSetup: ShippingSetupDto(
      id: option.id,
      name: option.name,
      type: option.type,
      isActive: option.isActive,
      internalPurpose: option.internalPurpose,
      coverages: coverages,
      createdAt: option.createdAt,
      updatedAt: option.updatedAt,
    ),
  );
}

SellerShippingSetupsEnvelopeDto _decodeShippingSetupsEnvelope(
  Map<String, dynamic> envelope,
) {
  return SellerShippingSetupsEnvelopeDto.fromJson(envelope);
}

Map<String, dynamic> _expectMap(dynamic value) {
  if (value is! Map<String, dynamic>) {
    throw FormatException('Expected JSON object, got ${value.runtimeType}');
  }
  return value;
}

class ShippingSetupEnvelopeDto {
  final ShippingSetupDto shippingSetup;

  const ShippingSetupEnvelopeDto({required this.shippingSetup});
}
