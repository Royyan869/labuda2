/// Payment Repository Implementation
///
/// API-based implementation of PaymentRepository interface.
///
/// PHASE 1F: Payment domain closure - using unified PaymentStatus from core
library;

import 'package:hishumi/core/common/result.dart';
import 'package:hishumi/core/core.dart' as core show ILoggerService;

import '../../domain/entities/payment.dart';
import '../../domain/entities/payment_intent.dart';
import '../../domain/repositories/payment_repository.dart';
import '../mappers/payment_mapper.dart';
import '../remote/payment_remote_datasource.dart';

/// Payment repository implementation
class PaymentRepositoryImpl implements PaymentRepository {
  final PaymentRemoteDatasource _datasource;
  final core.ILoggerService _logger;

  PaymentRepositoryImpl({
    required PaymentRemoteDatasource datasource,
    required core.ILoggerService logger,
  }) : _datasource = datasource,
       _logger = logger;

  @override
  Future<Result<PaymentIntent>> createPayment(
    CreatePaymentRequest request,
  ) async {
    try {
      // Validate request
      final validationError = request.validate();
      if (validationError != null) {
        return Result.error(validationError);
      }

      final dto = PaymentMapper.toCreatePaymentDto(request);
      final result = await _datasource.createPayment(dto);

      if (result.isSuccess && result.data != null) {
        final intent = PaymentMapper.toPaymentIntentEntity(result.data!);
        return Result.success(intent);
      }

      return _forwardFailure<PaymentIntent>(result);
    } catch (e, stackTrace) {
      _logger.error(
        'Error creating payment',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<Payment>> getPayment(String paymentId) async {
    try {
      if (paymentId.isEmpty) {
        return Result.error('Payment ID is required');
      }

      final result = await _datasource.getPayment(paymentId);

      if (result.isSuccess && result.data != null) {
        final payment = PaymentMapper.toPaymentEntity(result.data!);
        return Result.success(payment);
      }

      return _forwardFailure<Payment>(result);
    } catch (e, stackTrace) {
      _logger.error(
        'Error getting payment',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<List<PaymentMethodOption>>> getPaymentMethodOptions(
    String orderId,
  ) async {
    try {
      if (orderId.isEmpty) {
        return Result.error('Order ID is required');
      }

      final result = await _datasource.getPaymentMethods(orderId);

      if (result.isSuccess && result.data != null) {
        final options = result.data!.methods
            .map(
              (m) => PaymentMethodOption(
                methodCode: m.methodCode,
                displayName: m.displayName,
                buyerPaymentFeeAmount: m.buyerPaymentFeeAmount,
                totalPayableAmount: m.totalPayableAmount,
              ),
            )
            .toList();
        return Result.success(options);
      }

      return _forwardFailure<List<PaymentMethodOption>>(result);
    } catch (e, stackTrace) {
      _logger.error(
        'Error getting payment method options',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<PreOrderPaymentPricing>> getPreOrderPaymentPricing(
    String pricingToken, {
    bool useCoins = false,
  }) async {
    try {
      if (pricingToken.isEmpty) {
        return Result.error('Pricing token is required');
      }

      final result = await _datasource.getPreOrderPaymentMethods(
        pricingToken,
        useCoins: useCoins,
      );

      if (result.isSuccess && result.data != null) {
        return Result.success(result.data!.toEntity());
      }

      return _forwardFailure<PreOrderPaymentPricing>(result);
    } catch (e, stackTrace) {
      _logger.error(
        'Error getting pre-order payment pricing',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return Result.error(e.toString());
    }
  }

  /// Forward the API layer's failure verbatim.
  ///
  /// The backend `code` is the authority for *what kind* of failure this is.
  /// The previous error mapper re-derived a kind by grepping the error text for
  /// 'network' / 'not found' / 'expired' / 'invalid', then fabricated a typed
  /// payload for it — a second classification that could only fire by accident
  /// and discarded the real code. Transport and parse failures carry no code,
  /// and their message is then the only truth there is.
  Result<T> _forwardFailure<T>(Result<Object?> source) => Result.error(
        source.error ?? 'Unknown error',
        code: source.errorCode,
        statusCode: source.statusCode,
        details: source.errorDetails,
      );
}
