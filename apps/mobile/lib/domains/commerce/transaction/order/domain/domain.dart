/// Order Domain
///
/// Exports domain entities, repositories, and value objects for order feature.
library;

export 'repositories/order_repository.dart' show OrderRepository;
export 'repositories/refund_repository.dart'
    show RefundRepository, ListOrderRefundHistoryParams;
export 'repositories/repository_result.dart' show RepositoryResult;

// Domain Entities
export 'entities/order.dart';
export 'entities/order_item.dart';
export 'entities/order_pricing.dart';
export 'entities/order_params.dart';
export 'entities/shipping_info.dart';
export 'entities/shipping_types.dart';
export 'entities/order_status.dart';
export 'entities/order_source.dart';
export 'entities/refund_request.dart';
export 'entities/order_page_result.dart';
export 'entities/refund_history_page_result.dart';
export 'entities/order_confirmation.dart';
// ShippingProof is owned by shipping domain (features/shipping/domain/repositories/shipping_repository.dart)

// Re-export core types used by order domain
export 'package:labuda/core/common/types/payment_types.dart'
    show PaymentMethodType, PaymentStatus, PaymentChannel;
