/// Attachment Truth Resolver
///
/// Unified resolver for checking live status of commerce attachments.
/// This provides honest, real-time status for:
/// - ForSale attachments (available, sold, withdrawn, unavailable)
/// - Auction attachments (live, ended, inactive)
///
/// **BATCH R1 REALIGNMENT - ATTACHMENT CATEGORY CONTRACT:**
///
/// The Attachment system now contains THREE SEMANTIC CATEGORIES:
///
/// 1. TRUE ATTACHMENT (local payload - no live status needed):
///    - LocationAttachment: No live status (local data only)
///
/// 2. OBJECT REFERENCE (cross-domain references - may have live status):
///    - ForSale (via ShareReference): Has live availability status
///    - Auction (via ShareReference): Has live auction status
///    - Content (via ShareReference): No live status needed (social content)
///
/// 3. WORKFLOW PAYLOAD (domain-specific state - NO live status providers):
///    - NegotiationOfferAttachment: Embedded status for display only (may be stale)
///    - NegotiationResultAttachment: Embedded status for display only (may be stale)
///    - ShippingQuoteAttachment: Embedded status for display only (may be stale)
///    - BidAttachment: Embedded bid for display only (may be stale)
///    - **R1.1 HONEST:** Use negotiationId/offerId for all business actions
///    - For business logic, always resolve via backend using canonical IDs
///
/// **REFERENCE TRUTH ALIGNMENT V1 - LIFECYCLE TRUTH CONTRACT:**
///
/// The Attachment system contains TWO sources of truth for referenced objects:
///
/// 1. PREVIEW DATA (immutable snapshot):
///    - Stored in Attachment itself (title, imageUrl, price, etc.)
///    - Set when attachment is created
///    - NEVER changes - immutable snapshot
///    - Used for: immediate UI display, offline scenarios
///    - LIMITATION: Can be stale - object may have changed since attachment was created
///
/// 2. LIVE STATUS (canonical source):
///    - Fetched from backend using canonical ID (forSaleId, auctionId)
///    - Always reflects current state of the object
///    - Used for: business decisions, honest status indicators
///    - SOURCE OF TRUTH: backend API via providers
///
/// **LIFECYCLE BEHAVIOR:**
/// - When object becomes unavailable (sold, ended, deleted):
///   - Preview data STILL shows (immutable snapshot)
///   - Live status shows unavailable state
///   - UI displays both: preview + honest status badge
///   - Tap behavior: can navigate but may show unavailable/error state
///
/// - This is HONEST behavior - we don't hide references to unavailable objects
/// - Users see what was shared, even if it's no longer available
///
/// **TRUTH CONTRACT:**
/// - Cached attachment data (preview) can be stale
/// - This resolver always fetches live status from backend via providers
/// - Use this to display honest status indicators in attachments
/// - NEVER use preview data for business logic decisions
///
/// **USAGE:**
/// ```dart
/// final status = ref.watch(forSaleAttachmentStatusProvider(forSaleId));
/// status.when(
///   data: (availability) => _buildAttachmentWithStatus(availability),
///   loading: () => _buildLoadingIndicator(),
///   error: (_, __) => _buildAttachmentWithUnknownStatus(),
/// );
/// ```
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_providers.dart';
import 'package:labuda/shared/attachment/entities/attachment.dart';

// =============================================================================
// FOR SALE ATTACHMENT STATUS RESOLUTION
// =============================================================================

/// ForSale availability status for attachments
///
/// Maps ForSale entity status to attachment display states.
/// This is the SINGLE SOURCE OF TRUTH for forSale attachment status display.
enum ForSaleAttachmentStatus {
  /// ForSale is available for purchase
  available,

  /// ForSale has been sold (terminal state)
  soldOut,

  /// ForSale was withdrawn by seller (terminal state)
  withdrawn,

  /// ForSale is not available (no stock, draft, etc.)
  unavailable,

  /// Status could not be determined (error/loading)
  unknown,
}

/// ForSale attachment status with optional display metadata
class ForSaleAttachmentStatusData {
  final ForSaleAttachmentStatus status;
  final String? label;
  final String? message;
  final DateTime? checkedAt;

  const ForSaleAttachmentStatusData({
    required this.status,
    this.label,
    this.message,
    this.checkedAt,
  });

  /// Create status from ForSale entity
  factory ForSaleAttachmentStatusData.fromForSale(ForSale? forSale) {
    if (forSale == null) {
      return const ForSaleAttachmentStatusData(
        status: ForSaleAttachmentStatus.unknown,
        label: 'Tidak Diketahui',
        message: 'ForSale tidak ditemukan',
      );
    }

    // Check availability based on ForSale.isAvailable (canonical source)
    if (forSale.isAvailable) {
      return ForSaleAttachmentStatusData(
        status: ForSaleAttachmentStatus.available,
        label: 'Tersedia',
        message: null,
        checkedAt: DateTime.now(),
      );
    }

    // Check terminal states
    if (forSale.status == ForSaleStatus.sold) {
      return ForSaleAttachmentStatusData(
        status: ForSaleAttachmentStatus.soldOut,
        label: 'Terjual',
        message: 'Barang ini sudah terjual',
        checkedAt: DateTime.now(),
      );
    }

    if (forSale.status == ForSaleStatus.withdrawn) {
      return ForSaleAttachmentStatusData(
        status: ForSaleAttachmentStatus.withdrawn,
        label: 'Ditarik',
        message: 'ForSale telah ditarik oleh penjual',
        checkedAt: DateTime.now(),
      );
    }

    // Check stock
    if (forSale.stock <= 0) {
      return ForSaleAttachmentStatusData(
        status: ForSaleAttachmentStatus.soldOut,
        label: 'Habis',
        message: 'Stok barang telah habis',
        checkedAt: DateTime.now(),
      );
    }

    // Check visibility (draft/private)
    if (forSale.visibility == ForSaleVisibility.private) {
      return ForSaleAttachmentStatusData(
        status: ForSaleAttachmentStatus.unavailable,
        label: 'Tidak Tersedia',
        message: 'ForSale ini tidak tersedia saat ini',
        checkedAt: DateTime.now(),
      );
    }

    // Default unavailable
    return ForSaleAttachmentStatusData(
      status: ForSaleAttachmentStatus.unavailable,
      label: 'Tidak Tersedia',
      message: 'Barang ini tidak tersedia saat ini',
      checkedAt: DateTime.now(),
    );
  }

  /// Whether the forSale can still be purchased
  bool get isPurchasable => status == ForSaleAttachmentStatus.available;

  /// Whether to show a warning badge
  bool get shouldShowWarning =>
      status != ForSaleAttachmentStatus.available &&
      status != ForSaleAttachmentStatus.unknown;
}

/// Provider for forSale attachment status
///
/// Fetches live forSale status and returns availability data.
/// Auto-disposes to avoid keeping stale data in memory.
final forSaleAttachmentStatusProvider = FutureProvider.autoDispose
    .family<ForSaleAttachmentStatusData, String>((ref, forSaleId) async {
      final forSaleAsync = await ref.read(
        forSaleDetailProvider(forSaleId).future,
      );

      return forSaleAsync != null
          ? ForSaleAttachmentStatusData.fromForSale(forSaleAsync)
          : const ForSaleAttachmentStatusData(
              status: ForSaleAttachmentStatus.unknown,
              label: 'Tidak Diketahui',
              message: 'ForSale tidak ditemukan',
            );
    });

// =============================================================================
// AUCTION ATTACHMENT STATUS RESOLUTION
// =============================================================================

/// Auction status for attachments
///
/// Maps Auction entity status to attachment display states.
/// This is the SINGLE SOURCE OF TRUTH for auction attachment status display.
enum AuctionAttachmentStatus {
  /// Auction is currently active and accepting bids
  live,

  /// Auction has ended (terminal state)
  ended,

  /// Auction was cancelled (terminal state)
  cancelled,

  /// Auction is scheduled but not yet started
  scheduled,

  /// Status could not be determined (error/loading)
  unknown,
}

/// Auction attachment status with optional display metadata
class AuctionAttachmentStatusData {
  final AuctionAttachmentStatus status;
  final String? label;
  final String? message;
  final DateTime? checkedAt;

  const AuctionAttachmentStatusData({
    required this.status,
    this.label,
    this.message,
    this.checkedAt,
  });

  /// Create status from Auction entity
  factory AuctionAttachmentStatusData.fromAuction(Auction? auction) {
    if (auction == null) {
      return const AuctionAttachmentStatusData(
        status: AuctionAttachmentStatus.unknown,
        label: 'Tidak Diketahui',
        message: 'Lelang tidak ditemukan',
      );
    }

    // Use auction.status (canonical source from backend)
    switch (auction.status) {
      case AuctionStatus.active:
        return AuctionAttachmentStatusData(
          status: AuctionAttachmentStatus.live,
          label: 'Sedang Berlangsung',
          message: null,
          checkedAt: DateTime.now(),
        );

      case AuctionStatus.ended:
        final label = auction.winnerId != null ? 'Selesai' : 'Berakhir';
        final message = auction.winnerId != null
            ? 'Lelang telah selesai dengan pemenang'
            : 'Lelang berakhir tanpa pemenang';
        return AuctionAttachmentStatusData(
          status: AuctionAttachmentStatus.ended,
          label: label,
          message: message,
          checkedAt: DateTime.now(),
        );

      case AuctionStatus.cancelled:
        return AuctionAttachmentStatusData(
          status: AuctionAttachmentStatus.cancelled,
          label: 'Dibatalkan',
          message: 'Lelang telah dibatalkan',
          checkedAt: DateTime.now(),
        );

      case AuctionStatus.scheduled:
        return AuctionAttachmentStatusData(
          status: AuctionAttachmentStatus.scheduled,
          label: 'Terjadwal',
          message: 'Lelang akan dimulai segera',
          checkedAt: DateTime.now(),
        );

      case AuctionStatus.draft:
        return AuctionAttachmentStatusData(
          status: AuctionAttachmentStatus.scheduled,
          label: 'Draft',
          message: 'Lelang belum dipublikasikan',
          checkedAt: DateTime.now(),
        );

      case AuctionStatus.waitingSettlement:
        return AuctionAttachmentStatusData(
          status: AuctionAttachmentStatus.ended,
          label: 'Menunggu Penyelesaian',
          message: 'Lelang telah selesai, menunggu penyelesaian',
          checkedAt: DateTime.now(),
        );

      case AuctionStatus.expiredBNR:
        return AuctionAttachmentStatusData(
          status: AuctionAttachmentStatus.ended,
          label: 'Waktu Habis',
          message: 'Waktu penyelesaian habis',
          checkedAt: DateTime.now(),
        );
    }
  }

  /// Whether bidding is still allowed
  bool get isBiddable => status == AuctionAttachmentStatus.live;

  /// Whether to show a warning badge
  bool get shouldShowWarning =>
      status != AuctionAttachmentStatus.live &&
      status != AuctionAttachmentStatus.scheduled &&
      status != AuctionAttachmentStatus.unknown;
}

/// Provider for auction attachment status
///
/// Fetches live auction status and returns status data.
/// Auto-disposes to avoid keeping stale data in memory.
final auctionAttachmentStatusProvider = FutureProvider.autoDispose
    .family<AuctionAttachmentStatusData, String>((ref, auctionId) async {
      final auctionAsync = await ref.read(
        auctionDetailProvider(auctionId).future,
      );

      return auctionAsync != null
          ? AuctionAttachmentStatusData.fromAuction(auctionAsync)
          : const AuctionAttachmentStatusData(
              status: AuctionAttachmentStatus.unknown,
              label: 'Tidak Diketahui',
              message: 'Lelang tidak ditemukan',
            );
    });

// =============================================================================
// GENERIC ATTACHMENT STATUS RESOLVER
// =============================================================================

/// Get the appropriate status provider for any attachment
///
/// Returns the provider that should be watched to get live status
/// for the given attachment. Caller must pattern match on the type.
Object? attachmentStatusProviderFor(Attachment attachment) {
  switch (attachment) {
    case LocationAttachment():
    case NegotiationOfferAttachment():
    case NegotiationResultAttachment():
    case ShippingQuoteAttachment():
    case BidAttachment():
      // **R1.1 HONEST:** These attachment types have no live status providers
      // - Social attachments (Location) don't need live status
      // - Workflow payloads have embedded status that may be stale
      // - For workflow actions, always resolve via backend using canonical IDs
      return null;
    default:
      return null;
  }
}

/// Check if an attachment type supports live status resolution
bool attachmentSupportsLiveStatus(Attachment attachment) {
  // **R1.1 HONEST:** Only ForSale and Auction have providers (via ShareReference)
  // Workflow payloads claim support but have NO providers implemented
  // This getter returns the attachment's declared capability, not actual implementation
  return attachment.supportsLiveStatus;
}

/// Extract canonical ID from attachment for status checking
String? attachmentCanonicalId(Attachment attachment) {
  return attachment.canonicalId;
}
