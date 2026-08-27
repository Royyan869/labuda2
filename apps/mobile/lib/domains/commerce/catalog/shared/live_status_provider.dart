/// Live Status Provider
///
/// ═══════════════════════════════════════════════════════════════════════════════
/// SOCIAL FIX 2 — LIVE STATUS PROVIDER
/// ═══════════════════════════════════════════════════════════════════════════════
/// Make UI always reflect real backend state
///
/// ═══════════════════════════════════════════════════════════════════════════════
/// TWO SOURCES OF TRUTH:
/// ═══════════════════════════════════════════════════════════════════════════════
/// 1. SNAPSHOT (ShareReference.preview):
///    - Immutable data set when share was created
///    - Used for immediate UI display, offline scenarios
///    - Can be STALE - object may have changed
///
/// 2. LIVE STATUS (Backend API):
///    - Fetched from backend via canonical ID
///    - Always reflects current state
///    - Used for business decisions, honest status indicators
///
/// ═══════════════════════════════════════════════════════════════════════════════
/// UI UPDATE RULE:
/// ═══════════════════════════════════════════════════════════════════════════════
/// IF mismatch between snapshot and live:
///   snapshot: available
///   live: sold
/// → UI MUST show: "SOLD"
///
/// Live status ALWAYS wins over snapshot.
/// ═══════════════════════════════════════════════════════════════════════════════
///
/// ═══════════════════════════════════════════════════════════════════════════════
/// USAGE:
/// ═══════════════════════════════════════════════════════════════════════════════
/// ```dart
/// final liveStatus = ref.watch(liveStatusProvider(shareReference));
/// liveStatus.when(
///   data: (status) => _buildWithStatus(status),
///   loading: () => _buildWithSnapshotOnly(),
///   error: (_, __) => _buildWithSnapshotOnly(),
/// );
/// ```
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_status.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_providers.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';
import 'package:labuda/shared/object/object_preview.dart' as obj;

// =============================================================================
// LIVE STATUS PROVIDER - Main Entry Point
// =============================================================================

/// Live status for a ShareReference
///
/// Merges snapshot data (from ShareReference.preview) with live status
/// from backend. Live status always takes priority over snapshot.
///
/// This is the SINGLE SOURCE OF TRUTH for UI status display.
sealed class LiveStatus {
  const LiveStatus();

  /// Whether the status was determined from live data (true) or snapshot (false)
  abstract final bool isFromLive;

  /// Whether the target is currently available
  abstract final bool isAvailable;

  /// Display label for UI badges
  abstract final String label;

  /// Optional message for additional context
  abstract final String? message;

  /// Whether to show a warning badge on UI
  bool get shouldShowWarning => !isAvailable;

  /// Whether the snapshot and live status mismatch
  bool get hasMismatch;
}

/// Live status for listing references
class ListingLiveStatus extends LiveStatus {
  /// Availability status
  final ListingAvailabilityStatus availability;

  @override
  final bool isFromLive;

  @override
  final bool isAvailable;

  @override
  final String label;

  @override
  final String? message;

  @override
  final bool hasMismatch;

  const ListingLiveStatus({
    required this.availability,
    required this.isFromLive,
    required this.isAvailable,
    required this.label,
    this.message,
    required this.hasMismatch,
  });

  /// Create from snapshot data only (preview)
  factory ListingLiveStatus.fromSnapshot(obj.ObjectPreview snapshot) {
    final availability = snapshot.isDeleted
        ? ListingAvailabilityStatus.deleted
        : snapshot.isSold
        ? ListingAvailabilityStatus.sold
        : ListingAvailabilityStatus.available;

    return ListingLiveStatus(
      availability: availability,
      isFromLive: false,
      isAvailable:
          snapshot.isAvailable && !snapshot.isSold && !snapshot.isDeleted,
      label: availability.label,
      message: snapshot.isDeleted
          ? 'Listing ini telah dihapus'
          : snapshot.isSold
          ? 'Barang ini sudah terjual'
          : null,
      hasMismatch: false,
    );
  }

  /// Create from live listing data
  factory ListingLiveStatus.fromLive(
    ForSale listing,
    obj.ObjectPreview snapshot,
  ) {
    final availability = ListingAvailabilityStatus.fromListing(listing);
    final snapshotStatus = snapshot.isDeleted
        ? ListingAvailabilityStatus.deleted
        : snapshot.isSold
        ? ListingAvailabilityStatus.sold
        : ListingAvailabilityStatus.available;

    return ListingLiveStatus(
      availability: availability,
      isFromLive: true,
      isAvailable: listing.isAvailable,
      label: availability.label,
      message: availability.message,
      hasMismatch: availability != snapshotStatus,
    );
  }

  /// Unknown status (error/loading)
  factory ListingLiveStatus.unknown() {
    return const ListingLiveStatus(
      availability: ListingAvailabilityStatus.unknown,
      isFromLive: false,
      isAvailable: true, // Assume available for unknown
      label: 'Memuat...',
      message: null,
      hasMismatch: false,
    );
  }

  @override
  String toString() =>
      'ListingLiveStatus(availability: $availability, isFromLive: $isFromLive, isAvailable: $isAvailable)';
}

/// Live status for auction references
class AuctionLiveStatus extends LiveStatus {
  /// Auction status
  final AuctionDisplayStatus status;

  @override
  final bool isFromLive;

  @override
  final bool isAvailable;

  @override
  final String label;

  @override
  final String? message;

  @override
  final bool hasMismatch;

  /// Time remaining for active auctions (in seconds)
  final int? timeRemainingSeconds;

  /// Whether user can bid on this auction
  final bool isBiddable;

  const AuctionLiveStatus({
    required this.status,
    required this.isFromLive,
    required this.isAvailable,
    required this.label,
    this.message,
    required this.hasMismatch,
    this.timeRemainingSeconds,
    this.isBiddable = false,
  });

  /// Create from snapshot data only (preview)
  factory AuctionLiveStatus.fromSnapshot(obj.ObjectPreview snapshot) {
    final status = snapshot.isDeleted
        ? AuctionDisplayStatus.deleted
        : snapshot.isClosed
        ? AuctionDisplayStatus.ended
        : AuctionDisplayStatus.active;

    return AuctionLiveStatus(
      status: status,
      isFromLive: false,
      isAvailable:
          snapshot.isAvailable && !snapshot.isClosed && !snapshot.isDeleted,
      label: status.label,
      message: snapshot.isDeleted
          ? 'Lelang ini telah dihapus'
          : snapshot.isClosed
          ? 'Lelang telah berakhir'
          : null,
      hasMismatch: false,
      isBiddable: snapshot.isAvailable && !snapshot.isClosed,
    );
  }

  /// Create from live auction data
  factory AuctionLiveStatus.fromLive(
    Auction auction,
    obj.ObjectPreview snapshot,
  ) {
    final status = AuctionDisplayStatus.fromAuction(auction);
    final snapshotStatus = snapshot.isDeleted
        ? AuctionDisplayStatus.deleted
        : snapshot.isClosed
        ? AuctionDisplayStatus.ended
        : AuctionDisplayStatus.active;

    // Use backend decision contract for biddability if available
    final isBiddable =
        auction.decision?.allowedActions.contains('bid') ?? status.isActive;

    // Extract time remaining from backend decision hints
    final timeRemaining = auction.decision?.display?.timeRemainingSeconds;

    return AuctionLiveStatus(
      status: status,
      isFromLive: true,
      isAvailable: status.isActive || status.isScheduled,
      label: status.label,
      message: status.message,
      hasMismatch: status != snapshotStatus,
      timeRemainingSeconds: timeRemaining,
      isBiddable: isBiddable,
    );
  }

  /// Unknown status (error/loading)
  factory AuctionLiveStatus.unknown() {
    return const AuctionLiveStatus(
      status: AuctionDisplayStatus.unknown,
      isFromLive: false,
      isAvailable: true, // Assume available for unknown
      label: 'Memuat...',
      message: null,
      hasMismatch: false,
      isBiddable: false,
    );
  }

  @override
  String toString() =>
      'AuctionLiveStatus(status: $status, isFromLive: $isFromLive, isAvailable: $isAvailable)';
}

/// Profile live status (no live status needed)
class ProfileLiveStatus extends LiveStatus {
  @override
  final bool isFromLive;

  @override
  final bool isAvailable;

  @override
  final String label;

  @override
  final String? message;

  @override
  bool get shouldShowWarning => false;

  @override
  bool get hasMismatch => false;

  const ProfileLiveStatus({
    this.isFromLive = false,
    this.isAvailable = true,
    this.label = 'Profil',
    this.message,
  });

  factory ProfileLiveStatus.fromSnapshot(obj.ObjectPreview snapshot) {
    return ProfileLiveStatus(
      isAvailable: !snapshot.isDeleted,
      label: 'Profil',
      message: snapshot.isDeleted ? 'Pengguna tidak ditemukan' : null,
    );
  }
}

/// Content live status (no live status needed)
class ContentLiveStatus extends LiveStatus {
  @override
  final bool isFromLive;

  @override
  final bool isAvailable;

  @override
  final String label;

  @override
  final String? message;

  @override
  bool get shouldShowWarning => !isAvailable;

  @override
  bool get hasMismatch => false;

  const ContentLiveStatus({
    this.isFromLive = false,
    this.isAvailable = true,
    this.label = 'Konten',
    this.message,
  });

  factory ContentLiveStatus.fromSnapshot(obj.ObjectPreview snapshot) {
    return ContentLiveStatus(
      isAvailable: !snapshot.isDeleted,
      label: 'Konten',
      message: snapshot.isDeleted ? 'Konten tidak ditemukan' : null,
    );
  }
}

// =============================================================================
// LISTING STATUS ENUMS
// =============================================================================

/// Listing availability status for display
enum ListingAvailabilityStatus {
  available,
  sold,
  withdrawn,
  noStock,
  deleted,
  unknown;

  String get label {
    switch (this) {
      case ListingAvailabilityStatus.available:
        return 'Tersedia';
      case ListingAvailabilityStatus.sold:
        return 'Terjual';
      case ListingAvailabilityStatus.withdrawn:
        return 'Ditarik';
      case ListingAvailabilityStatus.noStock:
        return 'Habis';
      case ListingAvailabilityStatus.deleted:
        return 'Tidak Tersedia';
      case ListingAvailabilityStatus.unknown:
        return 'Memuat...';
    }
  }

  String? get message {
    switch (this) {
      case ListingAvailabilityStatus.available:
        return null;
      case ListingAvailabilityStatus.sold:
        return 'Barang ini sudah terjual';
      case ListingAvailabilityStatus.withdrawn:
        return 'Listing telah ditarik oleh penjual';
      case ListingAvailabilityStatus.noStock:
        return 'Stok barang telah habis';
      case ListingAvailabilityStatus.deleted:
        return 'Listing ini tidak tersedia';
      case ListingAvailabilityStatus.unknown:
        return null;
    }
  }

  /// Create from Listing entity
  static ListingAvailabilityStatus fromListing(ForSale listing) {
    if (listing.status == ForSaleStatus.sold) {
      return ListingAvailabilityStatus.sold;
    }
    if (listing.status == ForSaleStatus.withdrawn) {
      return ListingAvailabilityStatus.withdrawn;
    }
    if (listing.stock <= 0) {
      return ListingAvailabilityStatus.noStock;
    }
    if (listing.isAvailable) {
      return ListingAvailabilityStatus.available;
    }
    return ListingAvailabilityStatus.unknown;
  }
}

// =============================================================================
// AUCTION STATUS ENUMS
// =============================================================================

/// Auction display status for UI
enum AuctionDisplayStatus {
  active,
  ended,
  scheduled,
  waitingSettlement,
  cancelled,
  deleted,
  unknown;

  String get label {
    switch (this) {
      case AuctionDisplayStatus.active:
        return 'Sedang Berlangsung';
      case AuctionDisplayStatus.ended:
        return 'Berakhir';
      case AuctionDisplayStatus.scheduled:
        return 'Terjadwal';
      case AuctionDisplayStatus.waitingSettlement:
        return 'Menunggu Pembayaran';
      case AuctionDisplayStatus.cancelled:
        return 'Dibatalkan';
      case AuctionDisplayStatus.deleted:
        return 'Tidak Tersedia';
      case AuctionDisplayStatus.unknown:
        return 'Memuat...';
    }
  }

  String? get message {
    switch (this) {
      case AuctionDisplayStatus.active:
        return null;
      case AuctionDisplayStatus.ended:
        return 'Lelang telah berakhir';
      case AuctionDisplayStatus.scheduled:
        return 'Lelang akan dimulai segera';
      case AuctionDisplayStatus.waitingSettlement:
        return 'Menunggu pembayaran dari pemenang';
      case AuctionDisplayStatus.cancelled:
        return 'Lelang telah dibatalkan';
      case AuctionDisplayStatus.deleted:
        return 'Lelang tidak ditemukan';
      case AuctionDisplayStatus.unknown:
        return null;
    }
  }

  bool get isActive => this == AuctionDisplayStatus.active;
  bool get isScheduled => this == AuctionDisplayStatus.scheduled;
  bool get isTerminal =>
      this == AuctionDisplayStatus.ended ||
      this == AuctionDisplayStatus.cancelled ||
      this == AuctionDisplayStatus.deleted;

  /// Create from Auction entity
  static AuctionDisplayStatus fromAuction(Auction auction) {
    switch (auction.status) {
      case AuctionStatus.active:
        return AuctionDisplayStatus.active;
      case AuctionStatus.ended:
        return AuctionDisplayStatus.ended;
      case AuctionStatus.scheduled:
      case AuctionStatus.draft:
        return AuctionDisplayStatus.scheduled;
      case AuctionStatus.waitingSettlement:
        return AuctionDisplayStatus.waitingSettlement;
      case AuctionStatus.cancelled:
        return AuctionDisplayStatus.cancelled;
      case AuctionStatus.expiredBNR:
        return AuctionDisplayStatus.ended;
    }
  }
}

// =============================================================================
// LIVE STATUS PROVIDER - Riverpod Provider
// =============================================================================

/// Provider for live status of a ShareReference
///
/// Returns merged status combining snapshot and live data.
/// Auto-disposes to avoid keeping stale data in memory.
///
/// USAGE:
/// ```dart
/// final liveStatus = ref.watch(liveStatusProvider(shareReference));
/// liveStatus.when(
///   data: (status) => StatusBadge(status: status),
///   loading: () => CircularProgressIndicator(),
///   error: (_, __) => StatusBadge.fromSnapshot(shareReference.preview),
/// );
/// ```
final liveStatusProvider = FutureProvider.autoDispose
    .family<LiveStatus, ShareReference>((ref, shareReference) async {
      switch (shareReference.targetType) {
        case ShareTargetType.forSale:
          return _getListingLiveStatus(ref, shareReference);

        case ShareTargetType.auction:
          return _getAuctionLiveStatus(ref, shareReference);

        case ShareTargetType.profile:
          return ProfileLiveStatus.fromSnapshot(shareReference.preview);

        case ShareTargetType.content:
          return ContentLiveStatus.fromSnapshot(shareReference.preview);
      }
    });

/// Get live status for listing
Future<ListingLiveStatus> _getListingLiveStatus(
  Ref ref,
  ShareReference shareReference,
) async {
  try {
    final listingAsync = await ref.read(
      forSaleDetailProvider(shareReference.targetId).future,
    );

    if (listingAsync == null) {
      // Listing not found - treat as deleted
      return ListingLiveStatus(
        availability: ListingAvailabilityStatus.deleted,
        isFromLive: true,
        isAvailable: false,
        label: 'Tidak Tersedia',
        message: 'Listing tidak ditemukan',
        hasMismatch: shareReference.preview.isAvailable,
      );
    }

    return ListingLiveStatus.fromLive(listingAsync, shareReference.preview);
  } catch (_) {
    // On error, return snapshot-only status
    return ListingLiveStatus.fromSnapshot(shareReference.preview);
  }
}

/// Get live status for auction
Future<AuctionLiveStatus> _getAuctionLiveStatus(
  Ref ref,
  ShareReference shareReference,
) async {
  try {
    final auctionAsync = await ref.read(
      auctionDetailProvider(shareReference.targetId).future,
    );

    if (auctionAsync == null) {
      // Auction not found - treat as deleted
      return AuctionLiveStatus(
        status: AuctionDisplayStatus.deleted,
        isFromLive: true,
        isAvailable: false,
        label: 'Tidak Tersedia',
        message: 'Lelang tidak ditemukan',
        hasMismatch: shareReference.preview.isAvailable,
        isBiddable: false,
      );
    }

    return AuctionLiveStatus.fromLive(auctionAsync, shareReference.preview);
  } catch (_) {
    // On error, return snapshot-only status
    return AuctionLiveStatus.fromSnapshot(shareReference.preview);
  }
}

// =============================================================================
// CONVENIENCE PROVIDERS
// =============================================================================

/// Quick status check for listing - returns just the availability status
///
/// Useful for simple UI that only needs to know if listing is available.
/// For full status details, use liveStatusProvider instead.
final listingAvailabilityProvider = FutureProvider.autoDispose
    .family<ListingAvailabilityStatus, String>((ref, listingId) async {
      try {
        final listing = await ref.read(forSaleDetailProvider(listingId).future);
        if (listing == null) {
          return ListingAvailabilityStatus.deleted;
        }
        return ListingAvailabilityStatus.fromListing(listing);
      } catch (_) {
        return ListingAvailabilityStatus.unknown;
      }
    });

/// Quick status check for auction - returns just the display status
///
/// Useful for simple UI that only needs to know auction status.
/// For full status details, use liveStatusProvider instead.
final auctionStatusProvider = FutureProvider.autoDispose
    .family<AuctionDisplayStatus, String>((ref, auctionId) async {
      try {
        final auction = await ref.read(auctionDetailProvider(auctionId).future);
        if (auction == null) {
          return AuctionDisplayStatus.deleted;
        }
        return AuctionDisplayStatus.fromAuction(auction);
      } catch (_) {
        return AuctionDisplayStatus.unknown;
      }
    });

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

/// Get badge color based on live status
///
/// Returns a color string for UI theming based on the status.
String getStatusBadgeColor(LiveStatus status) {
  return switch (status) {
    ListingLiveStatus(availability: ListingAvailabilityStatus.available) =>
      'green',
    ListingLiveStatus(availability: ListingAvailabilityStatus.sold) => 'red',
    ListingLiveStatus(availability: ListingAvailabilityStatus.withdrawn) =>
      'orange',
    ListingLiveStatus(availability: ListingAvailabilityStatus.noStock) =>
      'orange',
    ListingLiveStatus(availability: ListingAvailabilityStatus.deleted) =>
      'gray',
    AuctionLiveStatus(status: AuctionDisplayStatus.active) => 'blue',
    AuctionLiveStatus(status: AuctionDisplayStatus.scheduled) => 'purple',
    AuctionLiveStatus(status: AuctionDisplayStatus.waitingSettlement) =>
      'orange',
    AuctionLiveStatus(status: AuctionDisplayStatus.ended) => 'gray',
    AuctionLiveStatus(status: AuctionDisplayStatus.cancelled) => 'red',
    AuctionLiveStatus(status: AuctionDisplayStatus.deleted) => 'gray',
    _ => 'gray',
  };
}

/// Check if live status should be refreshed
///
/// Returns true if the status is old enough to warrant a refresh.
/// This helps balance API calls with data freshness.
bool shouldRefreshStatus(
  DateTime? lastChecked, {
  Duration maxAge = const Duration(minutes: 1),
}) {
  if (lastChecked == null) return true;
  return DateTime.now().difference(lastChecked) > maxAge;
}

/// Create a fallback status from snapshot when live data is unavailable
LiveStatus createFallbackStatus(ShareReference shareReference) {
  return switch (shareReference.targetType) {
    ShareTargetType.forSale => ListingLiveStatus.fromSnapshot(
      shareReference.preview,
    ),
    ShareTargetType.auction => AuctionLiveStatus.fromSnapshot(
      shareReference.preview,
    ),
    ShareTargetType.profile => ProfileLiveStatus.fromSnapshot(
      shareReference.preview,
    ),
    ShareTargetType.content => ContentLiveStatus.fromSnapshot(
      shareReference.preview,
    ),
  };
}
