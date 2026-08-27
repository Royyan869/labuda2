/// Invariants for the seller quote CTA fix (Pass 1D-F1).
///
/// Verified behavior:
///   1. Seller of this specific listing sees "Kirim Tawaran" button.
///   2. Buyer (different user) does not see "Kirim Tawaran" but sees buyer CTAs.
///   3. Non-listing chat hides all commerce CTAs.
///   4. Listing detail loading → CTA hidden, no crash.
///   5. Listing detail error  → CTA hidden, no crash.
///   6. Send-message path is unchanged.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_notifier.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_state.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/chat_input_area.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_notifier.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_providers.dart';
import 'package:labuda/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_state.dart';
import 'package:labuda/shared/shared.dart';

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

class _FakeNegotiationNotifier extends NegotiationNotifier {
  @override
  NegotiationState build() => const NegotiationState();
}

// ---------------------------------------------------------------------------
// Test constants
// ---------------------------------------------------------------------------

const _chatId = '00000000-0000-0000-0000-000000000001';
const _fixedPriceSaleId = '00000000-0000-0000-0000-000000000002';
const _productId = '00000000-0000-0000-0000-000000000099';
const _sellerId = '00000000-0000-0000-0000-000000000010';
const _buyerId = '00000000-0000-0000-0000-000000000020';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

ForSale _fakeListing({required String sellerId}) => ForSale(
  forSaleId: _fixedPriceSaleId,
  productId: _productId,
  title: 'Koi Test',
  description: 'Test listing',
  price: 500000,
  stock: 1,
  sellerId: sellerId,
  status: ForSaleStatus.active,
  createdAt: DateTime.utc(2026, 6, 1),
  updatedAt: DateTime.utc(2026, 6, 1),
);

Chat _chatWithListingContext() => Chat(
  id: _chatId,
  participantIds: [_sellerId, _buyerId],
  participantNames: {_sellerId: 'Seller', _buyerId: 'Buyer'},
  participantAvatars: const {},
  createdAt: DateTime.utc(2026, 6, 1),
  status: ChatStatus.active,
  context: ShareReference.forSale(
    forSaleId: _fixedPriceSaleId,
    title: 'Koi Test',
  ),
);

Chat _chatWithoutContext() => Chat(
  id: _chatId,
  participantIds: [_sellerId, _buyerId],
  participantNames: {_sellerId: 'Seller', _buyerId: 'Buyer'},
  participantAvatars: const {},
  createdAt: DateTime.utc(2026, 6, 1),
  status: ChatStatus.active,
);

Widget _buildApp({
  required String currentUserId,
  required Chat chat,
  AsyncValue<ForSale?> listingState = const AsyncValue.loading(),
  VoidCallback? onSendQuote,
  Future<void> Function(String, {MessageType type})? onSendMessage,
}) {
  final controller = TextEditingController();
  return ProviderScope(
    overrides: [
      currentUserIdProvider.overrideWith((ref) => currentUserId),
      chatDetailProvider(
        _chatId,
      ).overrideWithValue(ChatDetailState(chat: chat)),
      forSaleDetailProvider(_fixedPriceSaleId).overrideWithValue(listingState),
      negotiationNotifierProvider.overrideWith(_FakeNegotiationNotifier.new),
    ],
    child: MaterialApp(
      home: Scaffold(
        body: ChatInputArea(
          chatId: _chatId,
          messageController: controller,
          onSendMessage:
              onSendMessage ?? ((content, {type = MessageType.text}) async {}),
          onAttachmentTap: () {},
          onSendQuote: onSendQuote,
        ),
      ),
    ),
  );
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

void main() {
  testWidgets('invariant 1 — seller of this listing sees Kirim Tawaran CTA', (
    tester,
  ) async {
    await tester.pumpWidget(
      _buildApp(
        currentUserId: _sellerId,
        chat: _chatWithListingContext(),
        listingState: AsyncValue.data(_fakeListing(sellerId: _sellerId)),
        onSendQuote: () {},
      ),
    );
    await tester.pump();
    expect(find.text('Kirim Tawaran'), findsOneWidget);
  });

  testWidgets(
    'invariant 2 — buyer does not see Kirim Tawaran; sees Beli Sekarang instead',
    (tester) async {
      await tester.pumpWidget(
        _buildApp(
          currentUserId: _buyerId,
          chat: _chatWithListingContext(),
          listingState: AsyncValue.data(_fakeListing(sellerId: _sellerId)),
        ),
      );
      await tester.pump();
      expect(find.text('Kirim Tawaran'), findsNothing);
      expect(find.text('Beli Sekarang'), findsOneWidget);
    },
  );

  testWidgets('invariant 3 — non-listing chat shows no commerce CTAs', (
    tester,
  ) async {
    await tester.pumpWidget(
      _buildApp(currentUserId: _sellerId, chat: _chatWithoutContext()),
    );
    await tester.pump();
    expect(find.text('Kirim Tawaran'), findsNothing);
    expect(find.text('Beli Sekarang'), findsNothing);
  });

  testWidgets(
    'invariant 4 — listing loading state hides CTA and does not crash',
    (tester) async {
      await tester.pumpWidget(
        _buildApp(
          currentUserId: _sellerId,
          chat: _chatWithListingContext(),
          listingState: const AsyncValue.loading(),
          onSendQuote: () {},
        ),
      );
      await tester.pump();
      expect(find.text('Kirim Tawaran'), findsNothing);
      expect(find.byType(TextField), findsOneWidget);
    },
  );

  testWidgets(
    'invariant 5 — listing error state hides CTA and does not crash',
    (tester) async {
      await tester.pumpWidget(
        _buildApp(
          currentUserId: _sellerId,
          chat: _chatWithListingContext(),
          listingState: AsyncValue.error('network error', StackTrace.empty),
          onSendQuote: () {},
        ),
      );
      await tester.pump();
      expect(find.text('Kirim Tawaran'), findsNothing);
      expect(find.byType(TextField), findsOneWidget);
    },
  );

  testWidgets('invariant 6 — send message behavior unchanged after fix', (
    tester,
  ) async {
    String? sent;
    await tester.pumpWidget(
      _buildApp(
        currentUserId: _buyerId,
        chat: _chatWithListingContext(),
        listingState: AsyncValue.data(_fakeListing(sellerId: _sellerId)),
        onSendMessage: (content, {type = MessageType.text}) async {
          sent = content;
        },
      ),
    );
    await tester.enterText(find.byType(TextField), 'halo penjual');
    await tester.pump();
    await tester.tap(find.byIcon(Icons.send));
    await tester.pump();
    expect(sent, 'halo penjual');
  });
}
