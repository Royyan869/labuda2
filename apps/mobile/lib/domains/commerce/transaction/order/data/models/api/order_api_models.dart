/// Order API Models
///
/// API response types for the Order module.
/// These were extracted from admin_stubs.dart to their proper home in the Order module.
library;

// Export API response types from local order_api_response_dtos.dart
export 'order_api_response_dtos.dart'
    show
        OrderApiResponse,
        OrderListApiResponse,
        OrderStatsApiResponse,
        RefundApiResponse,
        RefundListApiResponse,
        CheckDeliveryApiResponse,
        ShippingProofApiResponse,
        ShippingAddressApiResponse,
        ProductSummaryApiResponse,
        CheckDeliveryApiRequest,
        CreateShippingProofApiRequest,
        OrderFilterParams,
        RefundFilterParams;

// Re-export domain types used by API layer
// OrderStats: use OrderStatsApiResponse instead (defined in order_api_response_dtos.dart)
// RefundStatus: use RefundStatus from domain entities (refund_request.dart)

// NOTE: OrderListDto and RefundListDto are now defined via typedef in order_dto_response.dart
// to avoid conflicts with OrderListApiResponse and RefundListApiResponse
