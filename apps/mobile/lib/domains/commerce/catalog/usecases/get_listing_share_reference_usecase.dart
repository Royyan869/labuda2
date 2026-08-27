import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/repositories/for_sale_repository.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';

/// Get Fixed-Price-Sale Share Reference Use Case
///
/// **DOMAIN:** Commerce → Catalog
/// **RESPONSIBILITY:** Business logic for fetching fixed-price-sale references
/// **BOUNDARY:** Enforces availability rules before returning reference
///
/// **RULES:**
/// - Only active fixed-price sales can be attached
/// - Positive stock required (if applicable)
/// - Business validation happens HERE (not in UI or Provider)
class GetFixedPriceSaleShareReferenceUseCase {
  final ForSaleRepository _listingRepository;

  const GetFixedPriceSaleShareReferenceUseCase(this._listingRepository);

  /// Execute the use case
  ///
  /// Returns [Result.success] with [ShareReference] if fixed-price sale is available
  /// Returns [Result.error] if fixed-price sale is unavailable or validation fails
  Future<Result<ShareReference>> execute(String fixedPriceSaleId) async {
    try {
      final result = await _listingRepository.getForSaleById(
        fixedPriceSaleId,
      );

      if (result.isSuccess && result.data != null) {
        final listing = result.data!;

        // **BUSINESS LOGIC HERE - NOT IN UI**
        // AVAILABILITY ENFORCEMENT: Only allow attaching active fixed-price sales
        if (!listing.status.isAvailableForCommerce) {
      return Result.error(
        'Produk dijual ini tidak tersedia. Status: ${listing.status.displayName}',
      );
    }

    // Stock validation (business rule)
    if (listing.stock <= 0) {
      return Result.error('Produk dijual ini sudah habis stoknya.');
    }

    // Create ShareReference for chat attachment
    final shareReference = ShareReference.forSale(
      forSaleId: listing.forSaleId,
      title: listing.title,
      imageUrl: listing.media.isNotEmpty
          ? listing.media.first.originalUrl
          : null,
      isAvailable: listing.status.isAvailableForCommerce,
      isSold: listing.status == ForSaleStatus.sold,
          isClosed: !listing.status.isAvailableForCommerce,
          isDeleted: false,
        );

        return Result.success(shareReference);
      } else {
        return Result.error(result.error ?? 'Fixed-price sale not found');
      }
    } catch (e) {
      return Result.error('Failed to get fixed-price sale: $e');
    }
  }
}
