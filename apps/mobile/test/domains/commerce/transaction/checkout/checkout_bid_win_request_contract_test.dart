// Auction bid-win checkout request contract — Owner canonical.
//
// An auction bid-win order is created WITHOUT a payment method: the winner
// picks the method at Order Detail and the first POST /payments binds it.
// These contracts pin the client half of that rule:
//
//   - CheckoutRequest.paymentMethodCode is nullable;
//   - the wire omits payment_method_code when null (no default, no fallback);
//   - the bid-win submission path passes NO method;
//   - the pre-order method loader is a no-op for bid-win;
//   - readiness never gates bid-win on a method.
//
// The backend rejects a method on bid-win creation — a regression here would
// surface as a hard 400 from POST /orders.
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/commerce/transaction/checkout/domain/entities/checkout_request.dart';

const _requestEntity =
    'lib/domains/commerce/transaction/checkout/domain/entities/checkout_request.dart';
const _repositoryImpl =
    'lib/domains/commerce/transaction/checkout/data/repositories/checkout_repository_impl.dart';
const _screenLogic =
    'lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_logic.dart';
const _screenImpl =
    'lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_impl.dart';
const _readiness =
    'lib/domains/commerce/transaction/checkout/presentation/models/checkout_readiness.dart';

String _src(String path) =>
    File(path).readAsStringSync().replaceAll('\r\n', '\n');

void main() {
  group('Bid-win CheckoutRequest — unbound-order contract', () {
    test('paymentMethodCode is nullable (bid-win submits none)', () {
      final request = CheckoutRequest(
        productId: 'product-1',
        forSaleId: 'auction-1',
        addressId: 'address-1',
        pricingToken: 'token-1',
        paymentMethodCode: null,
        auctionId: 'auction-1',
      );
      expect(request.paymentMethodCode, isNull);
      expect(
        () => CheckoutRequest(
          productId: 'product-1',
          forSaleId: 'auction-1',
          addressId: 'address-1',
          pricingToken: 'token-1',
        ),
        returnsNormally,
        reason: 'the constructor must not require a payment method',
      );
    });

    test('the wire omits payment_method_code when no method is bound', () {
      final repo = _src(_repositoryImpl);
      expect(
        repo,
        contains(
          "if (request.paymentMethodCode != null)\n"
          "            'payment_method_code': request.paymentMethodCode,",
        ),
        reason:
            'payment_method_code must be conditional on the wire — a null '
            'method (bid-win) is omitted, never sent as a default',
      );
    });

    test('the bid-win submission path passes no method', () {
      final logic = _src(_screenLogic);
      expect(
        logic,
        contains('state._isBidWin\n        ? null\n        : state._selectedPaymentMethodCode!'),
        reason:
            'bid-win must submit null paymentMethodCode; method-binding '
            'checkouts keep the force-unwrapped selection',
      );
      expect(
        logic,
        contains('if (state._isBidWin)'),
        reason: 'the pre-order method loader must no-op for bid-win',
      );
    });

    test('readiness exposes the method-binding discriminator', () {
      expect(_src(_readiness), contains('requiresPaymentMethodSelection'));
      expect(_src(_screenImpl), contains('requiresPaymentMethodSelection: !_isBidWin'));
    });

    test('bid-win hides the pre-order payment-method picker', () {
      final impl = _src(_screenImpl);
      expect(impl, contains('if (displayPreview != null && !_isBidWin)'));
      expect(impl, contains('_BidWinPaymentMethodNote'));
    });
  });
}
