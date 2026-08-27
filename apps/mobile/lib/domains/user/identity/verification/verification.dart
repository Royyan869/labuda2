// Seller Verification Module (Simplified V2)
//
// Clean implementation aligned with backend API contract.
// Minimal trust verification for seller withdrawal gating.

// =============================================================================
// DATA LAYER (V2 Implementation)
// =============================================================================

export 'data/remote/verification_v2_datasource.dart'
    show
        VerificationStatusDto,
        SubmitKYCRequest,
        SubmitKYCResponse,
        VerificationV2Datasource;
export 'data/repositories/seller_verification_repository_v2.dart'
    show SellerVerificationRepositoryV2, SellerVerificationData;

// Canonical 8-state lifecycle enum — entity is the authority.
export 'domain/entities/seller_verification_status.dart'
    show SellerVerificationStatus, SellerVerificationStatusExtension;

// =============================================================================
// PRESENTATION LAYER (V2 Provider)
// =============================================================================

export 'presentation/providers/seller_verification_v2_provider.dart'
    show
        SellerVerificationV2State,
        SellerVerificationV2Notifier,
        sellerVerificationV2NotifierProvider,
        sellerVerificationV2StateProvider,
        isSellerVerifiedV2Provider;

// Legacy redirect-only VerificationScreen removed. The canonical
// seller-verification UI lives in
// `domains/user/preference/seller/presentation/screens/seller_verification_screen.dart`
// and is wired to `/verification` and `/verification/seller` by the router.
