import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/commerce/transaction/checkout/data/checkout_providers.dart';
import 'package:labuda/domains/commerce/transaction/checkout/domain/usecases/create_order_usecase.dart';

/// Checkout UseCase Providers
///
/// Provides usecase instances with repository injection.

/// Create Order UseCase Provider
final createOrderUseCaseProvider = Provider<CreateOrderUseCase>((ref) {
  final repository = ref.watch(checkoutRepositoryProvider);
  return CreateOrderUseCase(repository);
});
