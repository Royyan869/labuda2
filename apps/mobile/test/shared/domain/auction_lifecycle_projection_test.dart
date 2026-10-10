// AUCTION LIFE-CYCLE PROJECTION — mobile read-through contract.
//
// Commerce projects the canonical public auction PHASE plus the minimal
// outcome discriminator `has_winner`. Mobile MUST copy both verbatim and must
// never derive lifecycle from timestamps, status, winner, or current time.

import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/shared/domain/entities/resource_projection.dart';

Map<String, dynamic> _auctionJson({
  required String lifecycle,
  bool includeHasWinner = true,
  bool hasWinner = false,
}) {
  final auction = <String, dynamic>{
    'title': 'Lelang Koi',
    'media': const [],
    'current_bid': 1425000,
    'end_at': '2026-12-10T12:34:56Z',
    'lifecycle': lifecycle,
    if (includeHasWinner) 'has_winner': hasWinner,
    'seller': {
      'user': {'id': 'seller-1', 'username': 'petani', 'lifecycle': 'active'},
      'lifecycle': 'active',
    },
  };
  return {
    'state': 'LIVE',
    'resource_type': 'auction',
    'resource_id': 'auc-1',
    'canonical_url': '/auction/auc-1',
    'viewer_capabilities': {
      'can_view': true,
      'can_interact': true,
      'can_manage': false,
      'blocked_by_tombstone': false,
    },
    'auction': auction,
  };
}

AuctionLivePayload _payload(Map<String, dynamic> json) =>
    ResourceProjection.fromJson(json).payload! as AuctionLivePayload;

void main() {
  test('phase and has_winner are copied verbatim (waiting_settlement + winner)', () {
    final payload = _payload(
      _auctionJson(lifecycle: 'waiting_settlement', hasWinner: true),
    );
    expect(payload.lifecycle, 'waiting_settlement');
    expect(payload.hasWinner, isTrue);
  });

  test('ended + no winner is copied verbatim', () {
    final payload = _payload(
      _auctionJson(lifecycle: 'ended', hasWinner: false),
    );
    expect(payload.lifecycle, 'ended');
    expect(payload.hasWinner, isFalse);
  });

  test('every canonical public phase is carried unchanged', () {
    for (final phase in const [
      'scheduled',
      'active',
      'waiting_settlement',
      'ended',
      'cancelled',
    ]) {
      expect(_payload(_auctionJson(lifecycle: phase)).lifecycle, phase);
    }
  });

  test('absent has_winner defaults to false (no client derivation)', () {
    final payload = _payload(
      _auctionJson(lifecycle: 'active', includeHasWinner: false),
    );
    expect(payload.lifecycle, 'active');
    expect(payload.hasWinner, isFalse);
  });

  test('round-trips with has_winner present', () {
    final json = _auctionJson(lifecycle: 'ended', hasWinner: true);
    expect(ResourceProjection.fromJson(json).toJson(), json);
  });
}
