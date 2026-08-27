/// Canonical identity helpers for auction media.
///
/// Auction media should stay visually stable across polling refreshes even
/// when the resolved read URL rotates or the object is re-signed.
library;

import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
const Set<String> _transientQueryKeys = {
  'x-amz-algorithm',
  'x-amz-credential',
  'x-amz-date',
  'x-amz-expires',
  'x-amz-signature',
  'x-amz-security-token',
  'x-amz-signedheaders',
  'x-amz-token',
  'expires',
  'signature',
  'policy',
  'token',
};

/// Normalize a media reference into a stable visual/cache identity.
///
/// Rules:
/// - preserve the path/object key
/// - ignore transient signed-query parameters
/// - keep non-transient query parameters, but in a canonical order
/// - keep relative keys stable without inventing host authority
String normalizeAuctionMediaReference(String reference) {
  final trimmed = reference.trim();
  if (trimmed.isEmpty) return '';

  final uri = Uri.tryParse(trimmed);
  if (uri == null) {
    return _normalizeRawReference(trimmed);
  }

  final normalizedPath = _normalizePath(
    uri.path.isNotEmpty ? uri.path : trimmed,
  );
  final query = _stableQuerySuffix(uri);

  if (normalizedPath.isNotEmpty) {
    return query.isEmpty ? normalizedPath : '$normalizedPath?$query';
  }

  if (uri.hasScheme || uri.hasAuthority) {
    return query.isEmpty ? trimmed : '$trimmed?$query';
  }

  return query.isEmpty ? trimmed : '$trimmed?$query';
}

/// Stable media identity used for widget state and cache coherence.
///
/// The identity is keyed by auction authority plus logical slot position.
String auctionMediaLogicalKey({
  required String auctionId,
  required String mediaReference,
  required int position,
}) {
  return '$auctionId|$position|${normalizeAuctionMediaReference(mediaReference)}';
}

/// Stable fingerprint for an auction snapshot.
///
/// This is intentionally focused on meaningful public surface data so polling
/// can suppress duplicate emissions without swallowing real changes.
String auctionSnapshotFingerprint(Auction auction) {
  final buffer = StringBuffer()
    ..write('auction:')
    ..write(auction.id)
    ..write('|seller:')
    ..write(auction.sellerId)
    ..write('|title:')
    ..write(auction.title)
    ..write('|description:')
    ..write(auction.description)
    ..write('|sellerUsername:')
    ..write(auction.sellerUsername ?? '')
    ..write('|sellerFarmName:')
    ..write(auction.sellerFarmName ?? '')
    ..write('|sellerAvatar:')
    ..write(auction.sellerAvatar ?? '')
    ..write('|sellerUserLifecycle:')
    ..write(auction.sellerUserLifecycle.name)
    ..write('|sellerTrustLifecycle:')
    ..write(auction.sellerTrustLifecycle.name)
    ..write('|sellerTier:')
    ..write(auction.sellerTier ?? '')
    ..write('|openingBid:')
    ..write(auction.openingBid)
    ..write('|currentBid:')
    ..write(auction.currentBid)
    ..write('|bidIncrement:')
    ..write(auction.bidIncrement)
    ..write('|buyNowPrice:')
    ..write(auction.buyNowPrice ?? '')
    ..write('|condition:')
    ..write(auction.condition?.name ?? '')
    ..write('|startTime:')
    ..write(auction.startTime.toIso8601String())
    ..write('|endTime:')
    ..write(auction.endTime.toIso8601String())
    ..write('|startedAt:')
    ..write(auction.startedAt?.toIso8601String() ?? '')
    ..write('|endedAt:')
    ..write(auction.endedAt?.toIso8601String() ?? '')
    ..write('|settlementDeadline:')
    ..write(auction.settlementDeadline?.toIso8601String() ?? '')
    ..write('|status:')
    ..write(auction.status.name)
    ..write('|winnerId:')
    ..write(auction.winnerId ?? '')
    ..write('|winnerUsername:')
    ..write(auction.winnerUsername ?? '')
    ..write('|winningBid:')
    ..write(auction.winningBid ?? '')
    ..write('|totalBidders:')
    ..write(auction.totalBidders)
    ..write('|totalWatchers:')
    ..write(auction.totalWatchers)
    ..write('|totalViews:')
    ..write(auction.totalViews)
    ..write('|productId:')
    ..write(auction.productId ?? '')
    ..write('|farmAddressId:')
    ..write(auction.farmAddressId ?? '')
    ..write('|location:')
    ..write(auction.location?.cityId ?? '')
    ..write(',')
    ..write(auction.location?.provinceId ?? '')
    ..write('|media:');

  for (final media in auction.media) {
    buffer
      ..write(media.id)
      ..write('~')
      ..write(media.type.name)
      ..write('~');
  }

  return buffer.toString();
}

/// Stable list fingerprint for polling snapshots.
String auctionListFingerprint(List<Auction> auctions) {
  return auctions.map(auctionSnapshotFingerprint).join('||');
}

String _normalizeRawReference(String value) {
  final path = _normalizePath(value);
  return path.isEmpty ? value.trim() : path;
}

String _normalizePath(String value) {
  var out = value.trim();
  if (out.isEmpty) return '';
  if (out.contains('://')) {
    final uri = Uri.tryParse(out);
    if (uri != null) {
      out = uri.path;
    }
  }
  out = out.replaceAll('\\', '/');
  out = out.replaceFirst(RegExp(r'^/+'), '');
  out = out.replaceAll(RegExp(r'/+'), '/');
  return out;
}

String _stableQuerySuffix(Uri uri) {
  final entries = <MapEntry<String, String>>[];
  uri.queryParametersAll.forEach((rawKey, values) {
    final key = rawKey.trim();
    if (key.isEmpty) return;
    if (_transientQueryKeys.contains(key.toLowerCase())) return;

    final cleanedValues =
        values
            .map((value) => value.trim())
            .where((value) => value.isNotEmpty)
            .toList()
          ..sort();

    for (final value in cleanedValues) {
      entries.add(MapEntry(key, value));
    }
  });

  if (entries.isEmpty) return '';

  entries.sort((a, b) {
    final keyCompare = a.key.compareTo(b.key);
    if (keyCompare != 0) return keyCompare;
    return a.value.compareTo(b.value);
  });

  final buffer = StringBuffer();
  for (var i = 0; i < entries.length; i++) {
    final entry = entries[i];
    if (i > 0) buffer.write('&');
    buffer
      ..write(Uri.encodeQueryComponent(entry.key))
      ..write('=')
      ..write(Uri.encodeQueryComponent(entry.value));
  }
  return buffer.toString();
}
