import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  group('Checkout completion proof', () {
    test(
      'pricing readiness keeps local projection non-authoritative before backend preview',
      () {
        final screenSource = File(
          'lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_impl.dart',
        ).readAsStringSync();
        final logicSource = File(
          'lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_logic.dart',
        ).readAsStringSync();
        final summarySource = File(
          'lib/domains/commerce/transaction/checkout/presentation/widgets/checkout_order_summary_section.dart',
        ).readAsStringSync();

        expect(screenSource, contains('Memuat harga dari server...'));
        expect(logicSource, contains('!state._hasFreshPreview'));
        expect(
          summarySource,
          contains('previewResult: hasFreshPricing ? previewResult : null'),
        );
        expect(summarySource, contains('Harga lokal sementara'));
        expect(
          summarySource,
          contains(
            'subtotal = hasPricing ? previewResult!.subtotal : listing.price;',
          ),
        );
        expect(
          summarySource,
          contains(
            'shippingCost = hasPricing ? previewResult!.shippingCost : 0.0;',
          ),
        );
        expect(summarySource, contains('total = hasPricing'));
      },
    );

    test('pricing failure renders retry copy instead of permanent loading', () {
      final summarySource = File(
        'lib/domains/commerce/transaction/checkout/presentation/widgets/checkout_order_summary_section.dart',
      ).readAsStringSync();

      expect(summarySource, contains('error != null && !hasPricing'));
      expect(summarySource, contains('Gagal Memuat Harga'));
      expect(summarySource, contains('Refresh'));
      expect(summarySource, contains('helperMessage'));
    });

    test(
      'stale preview signatures are discarded and queued refresh is preserved',
      () {
        final logicSource = File(
          'lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_logic.dart',
        ).readAsStringSync();

        expect(
          logicSource,
          contains('requestSignature != state._buildCheckoutSignature()'),
        );
        expect(logicSource, contains('state._previewRefreshQueued = true;'));
        expect(
          logicSource,
          contains('state._previewSignature = requestSignature;'),
        );
        expect(logicSource, contains('state._schedulePreview();'));
      },
    );

    test(
      'shipping coverage keeps addressless, unconfigured, out-of-coverage and covered states separate',
      () {
        final screenSource = File(
          'lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_impl.dart',
        ).readAsStringSync();
        final shippingSource = File(
          'lib/domains/commerce/transaction/checkout/presentation/widgets/checkout_shipping_section.dart',
        ).readAsStringSync();

        expect(
          screenSource,
          contains('Pilih alamat pengiriman terlebih dahulu'),
        );
        expect(
          screenSource,
          contains('Pilih opsi pengiriman untuk memuat harga dari server.'),
        );
        expect(screenSource, contains('Memuat opsi pengiriman...'));
        expect(
          screenSource,
          contains('Penjual belum mengatur pengiriman untuk listing ini.'),
        );
        expect(screenSource, contains('onContactSeller: _openChatWithSeller,'));
        expect(
          screenSource,
          isNot(contains('Di luar coverage pengiriman yang tersedia.')),
        );
        expect(
          screenSource,
          isNot(contains('Tidak ada opsi pengiriman tersedia')),
        );
        expect(screenSource, isNot(contains('_CheckoutDeliveryAvailability')));
        expect(
          screenSource,
          isNot(contains('shippingRemoteDatasourceProvider')),
        );
        expect(
          shippingSource,
          contains('deliveryAvailability?.isNoShippingConfiguration == true'),
        );
        expect(
          shippingSource,
          contains('deliveryAvailability?.isOutOfCoverage == true'),
        );
        expect(shippingSource, contains('actionLabel: \'Hubungi Penjual\''));
        expect(shippingSource, contains('onAction: onContactSeller'));
        expect(
          shippingSource,
          contains('Pilih alamat pengiriman terlebih dahulu'),
        );
        expect(shippingSource, contains('Penjual belum mengatur pengiriman'));
        expect(shippingSource, contains('Di luar coverage pengiriman'));
      },
    );

    test(
      'canonical chat and quote resume logic stay on supported checkout sources',
      () {
        final screenSource = File(
          'lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_impl.dart',
        ).readAsStringSync();
        final logicSource = File(
          'lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_logic.dart',
        ).readAsStringSync();
        final auctionBranchStart = screenSource.indexOf(
          'if (widget.auctionId != null && widget.auctionId!.isNotEmpty) {',
        );
        final auctionBranch = auctionBranchStart >= 0
            ? (() {
                final auctionBranchEnd = screenSource.indexOf(
                  '} else {',
                  auctionBranchStart,
                );
                return auctionBranchEnd > auctionBranchStart
                    ? screenSource.substring(
                        auctionBranchStart,
                        auctionBranchEnd,
                      )
                    : screenSource;
              })()
            : screenSource;

        expect(screenSource, contains('ShareReference.fixedPriceSale('));
        expect(screenSource, contains('ShareReference.auction('));
        expect(screenSource, contains('context: shareReference'));
        expect(screenSource, contains('chatListProvider'));
        expect(screenSource, contains('getOrCreateChat('));
        expect(screenSource, isNot(contains('chatRepositoryProvider')));
        expect(
          screenSource,
          isNot(contains('shippingRemoteDatasourceProvider')),
        );
        expect(screenSource, isNot(contains('/chat?userId')));
        expect(auctionBranch, contains('ShareReference.auction('));
        expect(
          auctionBranch,
          isNot(contains('ShareReference.fixedPriceSale(')),
        );
        expect(logicSource, contains("sourceType = 'auction';"));
        expect(logicSource, contains("sourceType = 'fixed_price_sale';"));
        expect(
          logicSource,
          contains('shippingQuoteId: state.widget.shippingQuoteId,'),
        );
        expect(
          logicSource,
          contains('shippingOptionId: state.widget.shippingQuoteId == null'),
        );
        expect(logicSource, contains('? state._selectedShippingOptionId'));
        expect(logicSource, contains(': null,'));
        expect(screenSource, contains('didPopNext()'));
        expect(screenSource, contains('_schedulePreview();'));
      },
    );

    test(
      'dark mode relies on semantic theme tokens in the changed checkout files',
      () {
        final screenSource = File(
          'lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_impl.dart',
        ).readAsStringSync();
        final summarySource = File(
          'lib/domains/commerce/transaction/checkout/presentation/widgets/checkout_order_summary_section.dart',
        ).readAsStringSync();
        final actionBarSource = File(
          'lib/domains/commerce/transaction/checkout/presentation/widgets/checkout_action_bar.dart',
        ).readAsStringSync();

        expect(screenSource, contains('colorScheme.surface'));
        expect(summarySource, contains('colorScheme.onSurfaceVariant'));
        expect(
          actionBarSource,
          contains('colorScheme.surfaceContainerHighest'),
        );
        expect(
          actionBarSource,
          contains('Theme.of(context).brightness == Brightness.dark'),
        );
        expect(actionBarSource, contains('? 0.32'));
        expect(actionBarSource, contains(': 0.05'));
        expect(
          actionBarSource,
          contains('Theme.of(context).colorScheme.surface'),
        );
      },
    );
  });
}
