import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/usecases/build_order_timeline_usecase.dart';
import 'package:labuda/domains/commerce/transaction/usecases/get_order_status_usecase.dart';
import 'package:labuda/domains/commerce/transaction/order/data/order_providers.dart';

/// Order UseCase Providers
///
/// Provides usecase instances with repository injection.

/// Get Order Status UseCase Provider
final getOrderStatusUseCaseProvider = Provider<GetOrderStatusUseCase>((ref) {
  final repository = ref.watch(orderRepositoryProvider);
  return GetOrderStatusUseCase(repository);
});

/// Build Order Timeline UseCase Provider
final buildOrderTimelineUseCaseProvider = Provider<BuildOrderTimelineUseCase>((
  ref,
) {
  // This usecase doesn't need repository - it's pure business logic
  return BuildOrderTimelineUseCase();
});
