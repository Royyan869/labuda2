import '../../domain/domain.dart';

/// Order State - Sealed class for state management
sealed class OrderState {
  const OrderState();
}

class OrderInitial extends OrderState {
  const OrderInitial();
}

class OrderLoading extends OrderState {
  const OrderLoading();
}

class OrderLoaded extends OrderState {
  final Order order;
  const OrderLoaded(this.order);
}

class OrderListLoading extends OrderState {
  final String userId;
  const OrderListLoading(this.userId);
}

class OrderListLoaded extends OrderState {
  final List<Order> orders;
  final bool hasMore;
  const OrderListLoaded(this.orders, {this.hasMore = false});
}

class OrderPricingLoading extends OrderState {
  const OrderPricingLoading();
}

class OrderPricingLoaded extends OrderState {
  final OrderPricing pricing;
  const OrderPricingLoaded(this.pricing);
}

class OrderSuccess extends OrderState {
  final String message;
  final Order? order;
  const OrderSuccess(this.message, {this.order});
}

class OrderError extends OrderState {
  final String message;
  const OrderError(this.message);
}
