import 'dart:async';
import 'dart:io';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/core/providers/core_providers.dart';
import 'package:labuda/domains/commerce/transaction/order/data/order_providers.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/domain.dart';
import 'package:labuda/domains/finance/transaction/payment/domain/entities/payment.dart';
import 'package:labuda/domains/finance/transaction/payment/domain/entities/payment_intent.dart';
import 'package:labuda/domains/finance/transaction/payment/domain/entities/payment_method.dart';
import 'package:labuda/domains/finance/transaction/payment/domain/failures/payment_failure.dart';
import 'package:labuda/domains/finance/transaction/payment/domain/repositories/payment_repository.dart'
    as payment_repo;
import 'package:labuda/domains/finance/transaction/payment/presentation/providers/payment_providers.dart';
import 'package:labuda/domains/finance/transaction/payment/presentation/providers/payment_result_notifier.dart';
import 'package:labuda/domains/finance/transaction/payment/presentation/providers/payment_result_state.dart';
import 'package:labuda/shared/services/logger_service.dart';

class _FakeOrderRepository implements OrderRepository {
  _FakeOrderRepository(this._handler);

  final FutureOr<RepositoryResult<Order>> Function(String orderId) _handler;

  @override
  Future<RepositoryResult<Order>> getOrderById(String orderId) async =>
      _handler(orderId);

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakePaymentRepository implements payment_repo.PaymentRepository {
  _FakePaymentRepository(this._handler);

  final FutureOr<payment_repo.RepositoryResult<Payment>> Function(
    String paymentId,
  )
  _handler;

  @override
  Future<payment_repo.RepositoryResult<Payment>> getPayment(
    String paymentId,
  ) async => _handler(paymentId);

  @override
  List<PaymentMethod> getAvailablePaymentMethods() => const <PaymentMethod>[];

  @override
  double calculateFee(core.PaymentChannel channel, double amount) => 0.0;

  @override
  double calculateTotal(core.PaymentChannel channel, double amount) => amount;

  @override
  Future<payment_repo.RepositoryResult<PaymentIntent>> createPayment(
    CreatePaymentRequest request,
  ) async {
    throw UnimplementedError();
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

Order _order({
  required OrderStatus status,
  required PaymentStatus paymentStatus,
  String? paymentId,
}) {
  return Order(
    id: 'order-1',
    buyerId: 'buyer-1',
    sellerId: 'seller-1',
    items: const [],
    status: status,
    paymentMethod: PaymentMethodType.bankTransfer,
    paymentStatus: paymentStatus,
    shippingInfo: const ShippingInfo(
      recipientName: 'Buyer',
      phone: '08123',
      address: 'Address',
      method: ShippingMethod.courier,
      shippingCost: 10000,
    ),
    pricing: const OrderPricing(
      subtotal: 100000,
      shippingCost: 10000,
      discount: 0,
      total: 110000,
    ),
    createdAt: DateTime.utc(2026, 6, 1),
    source: OrderSource.forSale,
    paymentId: paymentId,
  );
}

Payment _payment({
  required String id,
  required PaymentStatus status,
  String? paymentUrl,
  String? referenceId,
}) {
  return Payment(
    id: id,
    paymentNumber: 'PAY-1',
    userId: 'buyer-1',
    grossAmount: 110000,
    coinDiscount: 0,
    coinDiscountAmount: 0,
    netAmount: 110000,
    status: status,
    referenceType: 'order',
    referenceId: referenceId,
    createdAt: DateTime.utc(2026, 6, 1),
    expiredAt: DateTime.utc(2026, 8, 2),
    paymentUrl: paymentUrl,
  );
}

ProviderContainer _containerWithOrder(
  Order order, {
  payment_repo.PaymentRepository? paymentRepository,
}) {
  return ProviderContainer(
    overrides: [
      loggerServiceProvider.overrideWithValue(LoggerService.instance),
      orderRepositoryProvider.overrideWithValue(
        _FakeOrderRepository((_) => RepositoryResult.success(order)),
      ),
      if (paymentRepository != null)
        paymentRepositoryProvider.overrideWithValue(paymentRepository),
    ],
  );
}

void main() {
  group('PaymentResultNotifier authority', () {
    test(
      'order.status == paid succeeds even when paymentStatus is not paid',
      () async {
        final container = _containerWithOrder(
          _order(
            status: OrderStatus.paid,
            paymentStatus: PaymentStatus.pending,
          ),
        );
        addTearDown(container.dispose);

        await container
            .read(paymentResultProvider.notifier)
            .startChecking('order-1');

        final state = container.read(paymentResultProvider);
        expect(state.status, PaymentResultScreenStatus.success);
        expect(state.isPaymentSuccessful, isTrue);
        expect(state.isPaymentFailed, isFalse);
      },
    );

    test(
      'order.status pending does not succeed even when paymentStatus is paid',
      () async {
        final container = _containerWithOrder(
          _order(
            status: OrderStatus.pending,
            paymentStatus: PaymentStatus.paid,
          ),
        );
        addTearDown(container.dispose);

        await container
            .read(paymentResultProvider.notifier)
            .startChecking('order-1');

        final state = container.read(paymentResultProvider);
        expect(state.status, PaymentResultScreenStatus.checking);
        expect(state.isChecking, isTrue);
        expect(state.pollAttempts, 1);
        expect(state.isPaymentSuccessful, isFalse);
        expect(state.isPaymentPending, isTrue);
      },
    );

    test(
      'order.status pending succeeds when payment resource is settled',
      () async {
        final payment = _payment(
          id: 'pay-1',
          status: PaymentStatus.paid,
          paymentUrl: 'https://pay.example.com/snap/1',
          referenceId: 'order-1',
        );
        final container = _containerWithOrder(
          _order(
            status: OrderStatus.pending,
            paymentStatus: PaymentStatus.pending,
            paymentId: 'pay-1',
          ),
          paymentRepository: _FakePaymentRepository(
            (_) => payment_repo.RepositoryResult.success(payment),
          ),
        );
        addTearDown(container.dispose);

        await container
            .read(paymentResultProvider.notifier)
            .startChecking('order-1');

        final state = container.read(paymentResultProvider);
        expect(state.status, PaymentResultScreenStatus.success);
        expect(state.isPaymentSuccessful, isTrue);
        expect(state.order?.status, OrderStatus.pending);
        expect(state.paymentId, 'pay-1');
        expect(state.paymentUrl, 'https://pay.example.com/snap/1');
        expect(state.hasReusablePaymentUrl, isTrue);
      },
    );

    test(
      'order.status pending keeps polling when payment resource is processing',
      () async {
        final payment = _payment(
          id: 'pay-1',
          status: PaymentStatus.processing,
          paymentUrl: 'https://pay.example.com/snap/1',
          referenceId: 'order-1',
        );
        final container = _containerWithOrder(
          _order(
            status: OrderStatus.pending,
            paymentStatus: PaymentStatus.pending,
            paymentId: 'pay-1',
          ),
          paymentRepository: _FakePaymentRepository(
            (_) => payment_repo.RepositoryResult.success(payment),
          ),
        );
        addTearDown(container.dispose);

        await container
            .read(paymentResultProvider.notifier)
            .startChecking('order-1');

        final state = container.read(paymentResultProvider);
        expect(state.status, PaymentResultScreenStatus.checking);
        expect(state.isChecking, isTrue);
        expect(state.paymentUrl, 'https://pay.example.com/snap/1');
        expect(state.hasReusablePaymentUrl, isTrue);
        expect(state.pollAttempts, 1);
      },
    );

    test(
      'payment lookup failure falls back to order polling without crashing',
      () async {
        final container = _containerWithOrder(
          _order(
            status: OrderStatus.pending,
            paymentStatus: PaymentStatus.pending,
            paymentId: 'pay-1',
          ),
          paymentRepository: _FakePaymentRepository(
            (_) => payment_repo.RepositoryResult.failure(
              const UnknownFailure('payment lookup down'),
            ),
          ),
        );
        addTearDown(container.dispose);

        await container
            .read(paymentResultProvider.notifier)
            .startChecking('order-1');

        final state = container.read(paymentResultProvider);
        expect(state.status, PaymentResultScreenStatus.checking);
        expect(state.pollAttempts, 1);
        expect(state.paymentUrl, isNull);
      },
    );

    test(
      'order.status pending fails when payment resource is denied',
      () async {
        final payment = _payment(
          id: 'pay-1',
          status: PaymentStatus.failed,
          paymentUrl: 'https://pay.example.com/snap/1',
          referenceId: 'order-1',
        );
        final container = _containerWithOrder(
          _order(
            status: OrderStatus.pending,
            paymentStatus: PaymentStatus.pending,
            paymentId: 'pay-1',
          ),
          paymentRepository: _FakePaymentRepository(
            (_) => payment_repo.RepositoryResult.success(payment),
          ),
        );
        addTearDown(container.dispose);

        await container
            .read(paymentResultProvider.notifier)
            .startChecking('order-1');

        final state = container.read(paymentResultProvider);
        expect(state.status, PaymentResultScreenStatus.failed);
        expect(state.isPaymentFailed, isTrue);
        expect(state.order?.status, OrderStatus.pending);
      },
    );

    test(
      'order.status pending fails when payment resource is expired',
      () async {
        final payment = _payment(
          id: 'pay-1',
          status: PaymentStatus.expired,
          referenceId: 'order-1',
        );
        final container = _containerWithOrder(
          _order(
            status: OrderStatus.pending,
            paymentStatus: PaymentStatus.pending,
            paymentId: 'pay-1',
          ),
          paymentRepository: _FakePaymentRepository(
            (_) => payment_repo.RepositoryResult.success(payment),
          ),
        );
        addTearDown(container.dispose);

        await container
            .read(paymentResultProvider.notifier)
            .startChecking('order-1');

        final state = container.read(paymentResultProvider);
        expect(state.status, PaymentResultScreenStatus.failed);
        expect(state.isPaymentFailed, isTrue);
      },
    );

    test('cancelled order fails from canonical order.status', () async {
      final container = _containerWithOrder(
        _order(
          status: OrderStatus.cancelled,
          paymentStatus: PaymentStatus.paid,
        ),
      );
      addTearDown(container.dispose);

      await container
          .read(paymentResultProvider.notifier)
          .startChecking('order-1');

      final state = container.read(paymentResultProvider);
      expect(state.status, PaymentResultScreenStatus.failed);
      expect(state.isPaymentFailed, isTrue);
      expect(state.errorMessage, contains('dibatalkan'));
    });

    test('expired order fails from canonical order.status', () async {
      final container = _containerWithOrder(
        _order(status: OrderStatus.expired, paymentStatus: PaymentStatus.paid),
      );
      addTearDown(container.dispose);

      await container
          .read(paymentResultProvider.notifier)
          .startChecking('order-1');

      final state = container.read(paymentResultProvider);
      expect(state.status, PaymentResultScreenStatus.failed);
      expect(state.isPaymentFailed, isTrue);
      expect(state.errorMessage, contains('kadaluarsa'));
    });

    test('network failure keeps existing network error behavior', () async {
      final container = ProviderContainer(
        overrides: [
          loggerServiceProvider.overrideWithValue(LoggerService.instance),
          orderRepositoryProvider.overrideWithValue(
            _FakeOrderRepository((_) => RepositoryResult.error('network down')),
          ),
        ],
      );
      addTearDown(container.dispose);

      await container
          .read(paymentResultProvider.notifier)
          .startChecking('order-1');

      final state = container.read(paymentResultProvider);
      expect(state.status, PaymentResultScreenStatus.networkError);
      expect(state.errorMessage, contains('network down'));
    });
  });

  group('PaymentResultNotifier resume recheck (F4)', () {
    test(
      'recheckOnResume performs a status-only recheck while checking',
      () async {
        final container = _containerWithOrder(
          _order(
            status: OrderStatus.pending,
            paymentStatus: PaymentStatus.pending,
          ),
        );
        addTearDown(container.dispose);

        final notifier = container.read(paymentResultProvider.notifier);
        await notifier.startChecking('order-1');

        final beforeAttempts = container
            .read(paymentResultProvider)
            .pollAttempts;
        await notifier.recheckOnResume('order-1');
        final state = container.read(paymentResultProvider);

        // Still pending/checking - the resume recheck re-asked the backend
        // (attempt count advanced) without regressing to a fresh session.
        expect(state.status, PaymentResultScreenStatus.checking);
        expect(state.pollAttempts, greaterThan(beforeAttempts));
      },
    );

    test('recheckOnResume is a no-op once status is success', () async {
      final container = _containerWithOrder(
        _order(status: OrderStatus.paid, paymentStatus: PaymentStatus.paid),
      );
      addTearDown(container.dispose);

      final notifier = container.read(paymentResultProvider.notifier);
      await notifier.startChecking('order-1');
      expect(
        container.read(paymentResultProvider).status,
        PaymentResultScreenStatus.success,
      );

      // Resuming after success must not regress the screen back to checking.
      await notifier.recheckOnResume('order-1');
      expect(
        container.read(paymentResultProvider).status,
        PaymentResultScreenStatus.success,
      );
    });

    test('recheckOnResume is a no-op once status is failed', () async {
      final container = _containerWithOrder(
        _order(
          status: OrderStatus.cancelled,
          paymentStatus: PaymentStatus.paid,
        ),
      );
      addTearDown(container.dispose);

      final notifier = container.read(paymentResultProvider.notifier);
      await notifier.startChecking('order-1');
      expect(
        container.read(paymentResultProvider).status,
        PaymentResultScreenStatus.failed,
      );

      await notifier.recheckOnResume('order-1');
      expect(
        container.read(paymentResultProvider).status,
        PaymentResultScreenStatus.failed,
      );
    });

    test('recheckOnResume is a no-op after stopChecking (cancelled)', () async {
      final container = _containerWithOrder(
        _order(
          status: OrderStatus.pending,
          paymentStatus: PaymentStatus.pending,
        ),
      );
      addTearDown(container.dispose);

      final notifier = container.read(paymentResultProvider.notifier);
      await notifier.startChecking('order-1');
      notifier.stopChecking();
      expect(container.read(paymentResultProvider).isCancelled, isTrue);

      await notifier.recheckOnResume('order-1');

      // Still cancelled - resume must not resurrect a user-stopped session.
      expect(container.read(paymentResultProvider).isCancelled, isTrue);
    });
  });

  group('PaymentResultState helper alignment', () {
    test('helper getters follow canonical order.status', () {
      final paidState = PaymentResultState(
        order: _order(
          status: OrderStatus.paid,
          paymentStatus: PaymentStatus.pending,
        ),
      );
      final pendingState = PaymentResultState(
        order: _order(
          status: OrderStatus.pending,
          paymentStatus: PaymentStatus.paid,
        ),
      );
      final cancelledState = PaymentResultState(
        order: _order(
          status: OrderStatus.cancelled,
          paymentStatus: PaymentStatus.paid,
        ),
      );

      expect(paidState.isPaymentSuccessful, isTrue);
      expect(pendingState.isPaymentSuccessful, isFalse);
      expect(pendingState.isPaymentPending, isTrue);
      expect(cancelledState.isPaymentFailed, isTrue);
    });

    test(
      'canContinuePayment requires a reusable URL and non-terminal payment status',
      () {
        final payment = _payment(
          id: 'pay-1',
          status: PaymentStatus.pending,
          paymentUrl: 'https://pay.example.com/snap/1',
        );
        final settledPayment = _payment(
          id: 'pay-1',
          status: PaymentStatus.paid,
          paymentUrl: 'https://pay.example.com/snap/1',
        );
        final noUrlPayment = _payment(
          id: 'pay-1',
          status: PaymentStatus.pending,
        );

        final continuable = PaymentResultState(payment: payment);
        final settled = PaymentResultState(payment: settledPayment);
        final noUrl = PaymentResultState(payment: noUrlPayment);
        final none = const PaymentResultState();

        expect(continuable.canContinuePayment, isTrue);
        expect(settled.canContinuePayment, isFalse);
        expect(noUrl.canContinuePayment, isFalse);
        expect(none.canContinuePayment, isFalse);
      },
    );
  });

  group('PaymentResultScreen recovery contract', () {
    test(
      'status-check and continue-payment are separate handlers, gated on canContinuePayment',
      () {
        final src = File(
          'lib/domains/commerce/transaction/checkout/presentation/screens/payment_result_screen_impl.dart',
        ).readAsStringSync();

        // Two distinct handlers - status-only recheck must never open a URL,
        // reopening the URL must be its own explicit action (Phase 2B-1 F2).
        expect(src, contains('_handleStatusCheck'));
        expect(src, contains('_handleContinuePayment'));
        expect(src, contains('_openExistingPaymentUrl'));
        expect(
          src,
          contains('launchUrl(uri, mode: LaunchMode.externalApplication)'),
        );
        // The continue-payment button must be gated on the non-terminal
        // reusable-URL getter, not shown unconditionally.
        expect(src, contains('state.canContinuePayment'));
        // Status-check handler must not itself call the URL launcher.
        final statusCheckStart = src.indexOf('Future<void> _handleStatusCheck');
        final statusCheckEnd = src.indexOf('\n  }', statusCheckStart);
        final statusCheckBody = src.substring(statusCheckStart, statusCheckEnd);
        expect(statusCheckBody, isNot(contains('_openExistingPaymentUrl')));
        expect(statusCheckBody, isNot(contains('launchUrl')));
      },
    );

    test('app resume wires WidgetsBindingObserver to a status-only recheck', () {
      final src = File(
        'lib/domains/commerce/transaction/checkout/presentation/screens/payment_result_screen_impl.dart',
      ).readAsStringSync();

      expect(src, contains('WidgetsBindingObserver'));
      expect(src, contains('didChangeAppLifecycleState'));
      expect(src, contains('AppLifecycleState.resumed'));
      expect(src, contains('recheckOnResume'));
      expect(src, contains('WidgetsBinding.instance.addObserver(this)'));
      expect(src, contains('WidgetsBinding.instance.removeObserver(this)'));

      // The resume callback body must not open the payment URL directly.
      final resumeStart = src.indexOf('void didChangeAppLifecycleState');
      final resumeEnd = src.indexOf('\n  }', resumeStart);
      final resumeBody = src.substring(resumeStart, resumeEnd);
      expect(resumeBody, isNot(contains('_openExistingPaymentUrl')));
      expect(resumeBody, isNot(contains('launchUrl')));
    });
  });
}
