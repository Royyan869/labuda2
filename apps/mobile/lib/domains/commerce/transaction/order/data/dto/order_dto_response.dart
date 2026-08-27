import 'package:labuda/domains/commerce/transaction/order/data/models/api/order_api_models.dart'
    show
        OrderApiResponse,
        OrderListApiResponse,
        OrderStatsApiResponse,
        CheckDeliveryApiResponse,
        ShippingProofApiResponse,
        ShippingAddressApiResponse,
        ProductSummaryApiResponse,
        CheckDeliveryApiRequest,
        CreateShippingProofApiRequest;

/// Type aliases to map old module DTOs to new naming convention
/// This allows order_refactor to use existing DTOs without rewriting them
typedef OrderDto = OrderApiResponse;
typedef OrderListDto = OrderListApiResponse;
typedef OrderStatsDto = OrderStatsApiResponse;
// RefundDto & RefundListDto now defined in refund_dto.dart (not typedefs anymore)
// The new RefundDto has full backend field mapping including calculation fields
typedef CheckDeliveryDto = CheckDeliveryApiResponse;
typedef ShippingProofDto = ShippingProofApiResponse;
typedef ShippingAddressResponseDto = ShippingAddressApiResponse;
typedef ProductSummaryDto = ProductSummaryApiResponse;
typedef CheckDeliveryRequestDto = CheckDeliveryApiRequest;
typedef CreateShippingProofDto = CreateShippingProofApiRequest;
