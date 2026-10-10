// CTA FLOW CONTRACT — NegotiationProposalCard (owner truth, 2026-09-30).
//
// Canonical rules under proof:
//  1. CTA ([Terima | Counter]) lives ONLY on the OPPONENT's latest proposal
//     of an active session — never on my own bubble;
//  2. superseded rounds carry NO action row (disabled buttons on old rounds
//     are a KILLED design);
//  3. Option A: Tolak is SELLER-ONLY — the buyer exits by countering or by
//     letting the active session auto-expire;
//  4. my own latest proposal renders the waiting line instead of buttons.
//
// `isFromCurrentUser` is transport truth passed from the message row; turn
// truth comes from the backend session (server now enforces alternation).

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/common/result.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/commerce/negotiation/negotiation/domain/entities/negotiation.dart';
import 'package:hishumi/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_notifier.dart';
import 'package:hishumi/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_providers.dart';
import 'package:hishumi/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_state.dart';
import 'package:hishumi/domains/commerce/negotiation/negotiation/presentation/widgets/negotiation_proposal_card.dart';
import 'package:hishumi/shared/attachment/entities/attachment.dart';
import 'package:hishumi/shared/providers/auth_status_providers.dart';

/// Serves a fixed session state without touching the network layer.
class _StubNegotiationNotifier extends NegotiationNotifier {
  _StubNegotiationNotifier(this.initial, this.session);
  final NegotiationState initial;
  final Negotiation session;
  int acceptCalls = 0;

  @override
  NegotiationState build() => initial;

  /// Commit accept without the network layer (build() never inits the
  /// late repository here), mirroring the production state update:
  /// currentNegotiation = accepted session.
  @override
  Future<Result<Negotiation>> acceptOffer({
    required String chatRoomId,
    required String sessionId,
  }) async {
    acceptCalls++;
    final accepted = session.copyWith(status: NegotiationStatus.accepted);
    state = state.copyWith(currentNegotiation: accepted);
    return Result.success(accepted);
  }
}

Negotiation _session({required int round, required bool viewerCanAct}) {
  return Negotiation(
    id: 'session-1',
    chatId: 'room-1',
    fixedPriceSaleId: 'for-sale-1',
    forSaleName: '',
    originalPrice: 300000,
    buyerId: 'buyer-1',
    buyerName: '',
    sellerId: 'seller-1',
    status: NegotiationStatus.active,
    currentOfferPrice: 250000,
    viewerCanAct: viewerCanAct,
    round: round,
    createdAt: DateTime.utc(2026, 9, 30),
    updatedAt: DateTime.utc(2026, 9, 30),
  );
}

NegotiationProposalAttachment _proposal(int sequence) {
  return NegotiationProposalAttachment(
    sessionId: 'session-1',
    proposalSequence: sequence,
    price: 250000,
  );
}

Future<_StubNegotiationNotifier> _pumpCard(
  WidgetTester tester, {
  required String viewerId,
  required Negotiation session,
  required int sequence,
  required bool isFromCurrentUser,
  VoidCallback? onDealBuy,
}) async {
  // The card reads the live session from the notifier — serve it.
  final stub = _StubNegotiationNotifier(
    NegotiationState(currentNegotiation: session),
    session,
  );
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        currentUserIdProvider.overrideWithValue(viewerId),
        negotiationNotifierProvider.overrideWith(() => stub),
      ],
      child: MaterialApp(
        home: Scaffold(
          body: NegotiationProposalCard(
            attachment: _proposal(sequence),
            isFromCurrentUser: isFromCurrentUser,
            onDealBuy: onDealBuy,
          ),
        ),
      ),
    ),
  );
  await tester.pump();
  return stub;
}

void main() {
  testWidgets('buyer Terima on opponent-latest: commit + onDealBuy (navigate ke checkout), no Tolak', (
    tester,
  ) async {
    var dealBuyCalls = 0;
    final stub = await _pumpCard(
      tester,
      viewerId: 'buyer-1',
      session: _session(round: 2, viewerCanAct: true),
      sequence: 2,
      isFromCurrentUser: false,
      onDealBuy: () => dealBuyCalls++,
    );

    expect(find.text('Terima'), findsOneWidget);
    expect(find.text('Counter'), findsOneWidget);
    expect(find.text('Tolak'), findsNothing); // Option A: no reject for buyer
    expect(find.textContaining('Menunggu respons'), findsNothing);

    // Owner flow: buyer klik Terima -> accept commit -> navigate ke checkout.
    await tester.tap(find.text('Terima'));
    await tester.pump(); // flush async accept commit
    await tester.pump(const Duration(milliseconds: 50));
    expect(stub.acceptCalls, 1);
    expect(dealBuyCalls, 1,
        reason: 'buyer Terima MUST navigate to checkout (openForSaleCheckout) right after commit');

    await tester.pumpAndSettle(); // snackbar lifecycle settles clean
    expect(dealBuyCalls, 1);
    expect(tester.takeException(), isNull);
  });

  testWidgets('own latest proposal renders the waiting line, never a button row', (
    tester,
  ) async {
    await _pumpCard(
      tester,
      viewerId: 'buyer-1',
      session: _session(round: 2, viewerCanAct: true),
      sequence: 2,
      isFromCurrentUser: true,
    );

    expect(find.text('Terima'), findsNothing);
    expect(find.text('Counter'), findsNothing);
    expect(find.text('Tolak'), findsNothing);
    expect(find.textContaining('Menunggu respons'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('superseded round carries no action row (killed disabled-button design)', (
    tester,
  ) async {
    await _pumpCard(
      tester,
      viewerId: 'buyer-1',
      session: _session(round: 2, viewerCanAct: true),
      sequence: 1,
      isFromCurrentUser: false,
    );

    expect(find.text('Terima'), findsNothing);
    expect(find.text('Counter'), findsNothing);
    expect(find.text('Tolak'), findsNothing);
    // The round itself still renders as history.
    expect(find.textContaining('Rp 250.000'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('seller: Terima + Counter + Tolak on the buyer\'s latest proposal', (
    tester,
  ) async {
    await _pumpCard(
      tester,
      viewerId: 'seller-1',
      session: _session(round: 3, viewerCanAct: true),
      sequence: 3,
      isFromCurrentUser: false,
    );

    expect(find.text('Terima'), findsOneWidget);
    expect(find.text('Counter'), findsOneWidget);
    expect(find.text('Tolak'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
