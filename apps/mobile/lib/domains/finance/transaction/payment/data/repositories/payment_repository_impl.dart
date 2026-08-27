/// Payment Repository Implementation
///
/// API-based implementation of PaymentRepository interface.
///
/// PHASE 1F: Payment domain closure - using unified PaymentStatus from core
library;

import 'package:labuda/core/core.dart'
    as core
    show PaymentChannel, ILoggerService;

import '../../domain/entities/payment.dart';
import '../../domain/entities/payment_intent.dart';
import '../../domain/entities/payment_method.dart';
import '../../domain/failures/payment_failure.dart';
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
  Future<RepositoryResult<PaymentIntent>> createPayment(
    CreatePaymentRequest request,
  ) async {
    try {
      // Validate request
      final validationError = request.validate();
      if (validationError != null) {
        return RepositoryResult.failure(ValidationFailure(validationError));
      }

      final dto = PaymentMapper.toCreatePaymentDto(request);
      final result = await _datasource.createPayment(dto);

      if (result.isSuccess && result.data != null) {
        final intent = PaymentMapper.toPaymentIntentEntity(result.data!);
        return RepositoryResult.success(intent);
      }

      return RepositoryResult.failure(_mapApiError(result.error));
    } catch (e, stackTrace) {
      _logger.error(
        'Error creating payment',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return RepositoryResult.failure(UnknownFailure(e.toString()));
    }
  }

  @override
  Future<RepositoryResult<Payment>> getPayment(String paymentId) async {
    try {
      if (paymentId.isEmpty) {
        return RepositoryResult.failure(
          ValidationFailure('Payment ID is required'),
        );
      }

      final result = await _datasource.getPayment(paymentId);

      if (result.isSuccess && result.data != null) {
        final payment = PaymentMapper.toPaymentEntity(result.data!);
        return RepositoryResult.success(payment);
      }

      return RepositoryResult.failure(_mapApiError(result.error));
    } catch (e, stackTrace) {
      _logger.error(
        'Error getting payment',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return RepositoryResult.failure(UnknownFailure(e.toString()));
    }
  }

  @override
  Future<RepositoryResult<List<PaymentMethodOption>>> getPaymentMethodOptions(
    String orderId,
  ) async {
    try {
      if (orderId.isEmpty) {
        return RepositoryResult.failure(
          ValidationFailure('Order ID is required'),
        );
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
        return RepositoryResult.success(options);
      }

      return RepositoryResult.failure(_mapApiError(result.error));
    } catch (e, stackTrace) {
      _logger.error(
        'Error getting payment method options',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return RepositoryResult.failure(UnknownFailure(e.toString()));
    }
  }

  @override
  List<PaymentMethod> getAvailablePaymentMethods() {
    return PaymentMapper.getAllPaymentMethods();
  }

  @override
  double calculateFee(core.PaymentChannel channel, double amount) {
    // Legacy stub: checkout fee authority lives on the backend now.
    return 0.0;
  }

  @override
  double calculateTotal(core.PaymentChannel channel, double amount) {
    // Legacy stub: checkout total is provided by backend snapshots.
    return amount;
  }

  /// Map API error to PaymentFailure
  PaymentFailure _mapApiError(dynamic error) {
    if (error == null) {
      return const UnknownFailure('Unknown error');
    }

    final errorStr = error.toString();

    // Check for common error patterns
    if (errorStr.contains('network') || errorStr.contains('connection')) {
      return NetworkFailure(errorStr);
    }

    if (errorStr.contains('not found')) {
      return PaymentNotFoundFailure('payment');
    }

    if (errorStr.contains('expired')) {
      return PaymentExpiredFailure(DateTime.now());
    }

    if (errorStr.contains('invalid') || errorStr.contains('validation')) {
      return ValidationFailure(errorStr);
    }

    return UnknownFailure(errorStr);
  }
}
