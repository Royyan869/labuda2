// Pending product attachment convergence contract.
//
// Owner business truth: a pending product attachment in Chat has NO send CTA.
// The composer send icon is the ONE send authority, and the payload is the
// canonical resourceOccurrence (`direct_commerce_insert_chat`). Every entry
// point — For Sale detail, Auction detail and checkout "Hubungi Penjual" —
// converges on the canonical [PendingCommerceAttachment] model carried through
// openCommerceChat.
//
// These negative/positive contracts make the forbidden design fail CI if it is
// ever reintroduced: a `Kirim` CTA on the pending card, a parallel
// `pendingReference` state, or a product-chat send through `objectReference`.

import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

String _read(String path) => File(path).readAsStringSync();

void main() {
  group('Pending product attachment — one canonical model, one send authority', () {
    test('pending chip has no send CTA (no onSend, no Kirim text)', () {
      final chip = _read('lib/shared/widgets/pending_commerce_chip.dart');
      expect(chip, isNot(contains('onSend')));
      expect(chip, isNot(contains("'Kirim'")));
      expect(chip, contains('onRemove'));
    });

    test('chat screen has no pendingReference parallel state', () {
      final screen = _read(
        'lib/domains/chat/chat/presentation/screens/chat_detail_screen.dart',
      );
      expect(screen, isNot(contains('pendingReference')));
      expect(screen, isNot(contains('_sendPendingReference')));
      expect(screen, isNot(contains('_buildPendingReferenceChip')));
      // ONE pending model, seeded by the entry point, sent as occurrence.
      expect(screen, contains('pendingCommerce'));
      expect(screen, contains('PendingCommerceAttachment'));
      expect(screen, contains('toSendRequest()'));
    });

    test('openCommerceChat carries the canonical pending model only', () {
      final nav = _read(
        'lib/domains/chat/chat/presentation/utils/commerce_chat_navigation.dart',
      );
      expect(nav, contains('PendingCommerceAttachment'));
      expect(nav, contains("'pendingCommerce'"));
      expect(nav, isNot(contains('pendingReference')));
      expect(nav, isNot(contains('ShareReference')));
    });

    test('route extra delivers pendingCommerce to the screen', () {
      final module = _read('lib/core/src/router/modules/chat_module.dart');
      expect(module, contains("extra?['pendingCommerce']"));
      expect(module, contains('pendingCommerce: pendingCommerce'));
      expect(module, isNot(contains('pendingReference')));
    });

    test('all three entry points converge on PendingCommerceAttachment', () {
      final forSale = _read(
        'lib/domains/commerce/catalog/for_sale/presentation/screens/'
        'for_sale_detail_screen.dart',
      );
      final auction = _read(
        'lib/domains/commerce/catalog/auction/presentation/screens/'
        'auction_detail_screen.dart',
      );
      final checkout = _read(
        'lib/domains/commerce/transaction/checkout/presentation/screens/'
        'checkout_screen_impl.dart',
      );

      expect(forSale, contains('PendingCommerceAttachment.forSale('));
      expect(auction, contains('PendingCommerceAttachment.auction('));
      expect(checkout, contains('PendingCommerceAttachment.forSale('));
      for (final src in [forSale, auction, checkout]) {
        expect(src, contains('attachment:'));
        expect(src, isNot(contains('ShareReference')));
      }
    });

    test('shipping-question autofill is Checkout-uncovered-shipping ONLY', () {
      // Checkout knows WHY the buyer is opening Chat (normal shipping is
      // unavailable for this destination), so it alone supplies a draft.
      final checkout = _read(
        'lib/domains/commerce/transaction/checkout/presentation/screens/'
        'checkout_screen_impl.dart',
      );
      expect(checkout, contains('kCheckoutUncoveredShippingDraft'));
      expect(checkout, contains('draftMessage: kCheckoutUncoveredShippingDraft'));

      // Every other Chat entry leaves the composer draft empty.
      final forSale = _read(
        'lib/domains/commerce/catalog/for_sale/presentation/screens/'
        'for_sale_detail_screen.dart',
      );
      final auction = _read(
        'lib/domains/commerce/catalog/auction/presentation/screens/'
        'auction_detail_screen.dart',
      );
      for (final src in [forSale, auction]) {
        expect(src, contains('openCommerceChat('));
        expect(
          src,
          isNot(contains('draftMessage')),
          reason: 'only the Checkout uncovered-shipping shortcut autofills',
        );
      }
    });

    test('chat send chain has no objectReference/attachment_json producer', () {
      final repo = _read(
        'lib/domains/chat/chat/data/repositories/chat_repository_impl.dart',
      );
      expect(repo, isNot(contains('objectReference')));
      expect(repo, isNot(contains('attachment_json')));
      expect(repo, isNot(contains('_normalizeReferenceForChat')));
      expect(repo, isNot(contains('_resourceOccurrenceFor')));

      final notifier = _read(
        'lib/domains/chat/chat/presentation/providers/chat_notifier.dart',
      );
      expect(notifier, isNot(contains('objectReference')));
      expect(notifier, isNot(contains('workflowAttachment')));

      final usecase = _read(
        'lib/domains/chat/chat/domain/usecases/send_message_usecase.dart',
      );
      expect(usecase, isNot(contains('objectReference')));
      expect(usecase, isNot(contains('workflowAttachment')));

      final repoContract = _read(
        'lib/domains/chat/chat/domain/repositories/chat_repository.dart',
      );
      expect(repoContract, isNot(contains('objectReference')));
      expect(repoContract, isNot(contains('workflowAttachment')));
    });
  });
}
