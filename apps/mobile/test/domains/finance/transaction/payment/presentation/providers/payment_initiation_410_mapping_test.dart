/// Payment-initiation 410/GONE mapping proof (repair: HTTP 410 UX).
///
/// Backend CreatePayment rejects a stale pay attempt past the order's
/// payment window with 410 + code `GONE`. The mobile flow must translate
/// that into the Indonesian business message — never the raw English
/// backend text — and must not fake success or retry.
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/exceptions/api_exception.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/domains/finance/transaction/payment/domain/entities/payment.dart';
import 'package:labuda/domains/finance/transaction/payment/domain/entities/payment_intent.dart';
import 'package:labuda/domains/finance/transaction/payment/domain/repositories/payment_repository.dart';
import 'package:labuda/domains/finance/transaction/payment/presentation/providers/payment_initiation_notifier.dart';
import 'package:labuda/domains/finance/transaction/payment/presentation/providers/payment_providers.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

/// Backend copy that must never reach the user verbatim.
const _backendEnglish = 'Payment window has expired for this order';

/// The owner-locked Indonesian business message for an expired window.
const _expectedIndonesian =
    'Batas waktu pembayaran pesanan ini telah berakhir. Silakan buat pesanan baru.';

/// No-op logger: the notifier logs on every path; the stub absorbs all calls.
class _StubLogger implements core.ILoggerService {
  @override
  dynamic noSuchMethod(Invocation invocation) =>
      Future.value(Result<void>.success(null));
}

class _GonePaymentRepository implements PaymentRepository {
  @override
  Future<Result<PaymentIntent>> createPayment(CreatePaymentRequest request) async {
    return Result.error(
      _backendEnglish,
      code: core.gone,
      statusCode: 410,
    );
  }

  @override
  Future<Result<Payment>> getPayment(String paymentId) async {
    throw UnimplementedError('not needed for the 410 mapping proof');
  }

  @override
  Future<Result<List<PaymentMethodOption>>> getPaymentMethodOptions(
    String orderId,
  ) async {
    throw UnimplementedError('not needed for the 410 mapping proof');
  }

  @override
  Future<Result<PreOrderPaymentPricing>> getPreOrderPaymentPricing(
    String pricingToken, {
    bool useCoins = false,
  }) async {
    throw UnimplementedError('not needed for the 410 mapping proof');
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('HTTP 410 / GONE — payment initiation mapping', () {
    test('ApiExceptionFactory preserves the backend GONE code on 410', () {
      // Chain proof: 410 + envelope code must survive classification so the
      // flow mapper can branch on the code (not on English message text).
      final ex = ApiExceptionFactory.fromStatusCode(
        410,
        _backendEnglish,
        code: 'GONE',
      );
      expect(ex.statusCode, 410);
      expect(ex.code, 'GONE');
    });

    test('stale pay attempt (410 GONE) surfaces the Indonesian business message', () async {
      final container = ProviderContainer(
        overrides: [
          paymentRepositoryProvider.overrideWithValue(_GonePaymentRepository()),
          core.loggerServiceProvider.overrideWith((ref) => _StubLogger()),
        ],
      );
      addTearDown(container.dispose);

      final notifier = container.read(paymentInitiationProvider.notifier);
      final intent = await notifier.initiatePayment(
        const InitiatePaymentRequest(
          orderId: '8d60c573-9a53-44fb-99a5-5e8664c28c41',
          paymentMethodCode: 'bank_transfer',
        ),
      );

      // No fake success: initiation failed.
      expect(intent, isNull);

      final state = container.read(paymentInitiationProvider);
      expect(state.error, isNotNull);
      expect(state.error, _expectedIndonesian,
          reason: '410/GONE must map to the owner-locked Indonesian business copy');
      expect(state.error, isNot(contains('Payment window')),
          reason: 'raw English backend message must never reach the user');
      expect(state.isInitiating, isFalse, reason: 'lock must clear on failure');
      expect(state.isInitiated, isFalse, reason: 'no success state may be fabricated');
    });

    test('unknown codes still fall back to the backend message (existing contract)', () async {
      final container = ProviderContainer(
        overrides: [
          paymentRepositoryProvider.overrideWithValue(_UnknownCodePaymentRepository()),
          core.loggerServiceProvider.overrideWith((ref) => _StubLogger()),
        ],
      );
      addTearDown(container.dispose);

      final notifier = container.read(paymentInitiationProvider.notifier);
      await notifier.initiatePayment(
        const InitiatePaymentRequest(
          orderId: 'order-unknown-code',
          paymentMethodCode: 'bank_transfer',
        ),
      );

      final state = container.read(paymentInitiationProvider);
      expect(state.error, 'some backend message');
    });
  });
}

class _UnknownCodePaymentRepository implements PaymentRepository {
  @override
  Future<Result<PaymentIntent>> createPayment(CreatePaymentRequest request) async {
    return Result.error('some backend message', code: 'SOME_OTHER_CODE');
  }

  @override
  Future<Result<Payment>> getPayment(String paymentId) async {
    throw UnimplementedError('not needed');
  }

  @override
  Future<Result<List<PaymentMethodOption>>> getPaymentMethodOptions(
    String orderId,
  ) async {
    throw UnimplementedError('not needed');
  }

  @override
  Future<Result<PreOrderPaymentPricing>> getPreOrderPaymentPricing(
    String pricingToken, {
    bool useCoins = false,
  }) async {
    throw UnimplementedError('not needed');
  }
}
