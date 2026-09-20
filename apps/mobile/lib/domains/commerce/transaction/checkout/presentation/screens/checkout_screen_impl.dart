/// Checkout Screen
///
/// Real Transaction Flow - Supports Direct Buy, Seller Quote, Negotiation, and Auction
///
/// Flow:
/// 1. User views forSale details OR negotiates price OR wins auction
/// 2. User clicks "Beli Sekarang" (Buy Now) or "Lanjut Beli" (Continue Purchase) or "Klaim Kemenangan" (Claim Victory)
/// 3. Navigate to this CheckoutScreen with forSaleId + productId + optional commerce context
/// 4. Call preview API to get backend-calculated pricing
/// 5. Review order details with preview pricing
/// 6. Click "Buat Pesanan" (Create Order) or "Amankan Kemenangan" (Secure Victory)
/// 7. Create order via API with product_id+source_type+source_id (+ negotiation_id or auction_id)
/// 8. Present payment URL inside Labuda's internal WebView (PaymentWebviewScreen)
///    External-browser payment navigation is obsolete and must not be reintroduced.
///
/// **IMPORTANT:** Pricing is sourced from backend preview API, NOT from forSale.price
///
/// **CHAT COMMERCE SUPPORT:**
/// - Direct forSale purchase: provide sale surface ID + productId
/// - Negotiation purchase: provide sale surface ID + productId + negotiationId
/// - Auction purchase: provide sale surface ID + productId + auctionId (winner or buy now)
///
/// **AUCTION WINNER FLOW (AW1/AW2):**
/// - When auctionId is present, winner gets special treatment:
///   - Green banner: "Selamat! Anda Memenangkan Lelang Ã°Å¸Å½â€°"
///   - Button text: "Amankan Kemenangan" instead of "Buat Pesanan"
///   - Messages framed as "claiming victory" not "making purchase"
///   - Error messages use winner-specific language
///
/// **CV2:** returnToChat enables seamless navigation back to chat after checkout
///
/// **TOKEN EXPIRY:**
/// - Pricing tokens have limited lifetime (typically 10 minutes)
/// - Screen shows countdown to expiry
/// - User can manually refresh pricing when needed
///
/// **DISCOUNT HONESTY:**
/// - Promo discounts ONLY apply to direct forSale purchase (not seller quote, negotiation, auction)
/// - Discount codes are validated by backend, frontend only displays result
/// - Applied discount shows clear description and amount from backend
/// - No fake pricing or misleading savings - all numbers come from backend preview
library;

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/api/api_error_codes.dart' as api_codes;
import 'package:labuda/domains/commerce/transaction/checkout/checkout.dart';
import 'package:labuda/domains/commerce/transaction/checkout/presentation/models/checkout_readiness.dart';
import 'package:labuda/domains/commerce/transaction/checkout/presentation/utils/checkout_honesty_messages.dart';
import 'package:labuda/domains/finance/wallet/coins/coins.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/widgets/discount_input_field.dart';
import 'package:labuda/domains/chat/chat/presentation/utils/commerce_chat_navigation.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/order/presentation/providers/order_providers.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/widgets/blocked_action_gate.dart';
import 'package:labuda/domains/finance/transaction/payment/presentation/presentation.dart'
    show
        InitiatePaymentRequest,
        paymentInitiationProvider,
        paymentRepositoryProvider,
        PaymentMethodPickerSheet,
        PaymentMethodOption;
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/domains/user/profile/presentation/providers/address_providers.dart';
import 'package:labuda/domains/user/profile/presentation/providers/notifiers/address_notifier.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/entities/shipping.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart'
    show shippingRepositoryProvider;

part '../widgets/checkout_order_summary_section.dart';
part '../widgets/checkout_address_section.dart';
part '../widgets/checkout_notes_section.dart';
part '../widgets/checkout_action_bar.dart';
part '../widgets/checkout_coin_section.dart';
part '../widgets/checkout_discount_section.dart';
part '../widgets/checkout_warning_banners.dart';
part '../widgets/checkout_shipping_section.dart';
part 'checkout_screen_logic.dart';

/// Checkout Screen
///
/// Checkout screen for transaction flow
/// Supports direct buy, seller quote, negotiation, and auction commerce contexts
///
/// **CANONICAL PRICING FLOW:**
/// All pricing comes from backend preview API with pricing token.
/// Private agreement pricing is validated by backend - frontend cannot inject price.
///
/// **CV2:** returnToChat enables seamless navigation back to chat after successful checkout
class CheckoutScreen extends ConsumerStatefulWidget {
  final String? productId;
  final String forSaleId;

  /// Chat commerce context
  final String? negotiationId;

  /// Auction checkout context - for winning bid or buy now
  final String? auctionId;

  /// **SHIPPING QUOTE FIX:** Shipping quote ID from seller's manual quote
  /// When provided, the checkout will use the seller's quoted shipping price
  final String? shippingQuoteId;

  /// **CV2:** Chat ID to return to after successful order completion
  /// When set, the "Back to Chat" navigation will be available
  final String? returnToChat;

  const CheckoutScreen({
    super.key,
    this.productId,
    required this.forSaleId,
    this.negotiationId,
    this.auctionId,
    this.shippingQuoteId,
    this.returnToChat,
  });

  @override
  ConsumerState<CheckoutScreen> createState() => _CheckoutScreenState();
}

class _CheckoutScreenState extends ConsumerState<CheckoutScreen> {
  // Form controllers
  final _notesController = TextEditingController();

  // State
  bool _useCoins = false;
  int _coinBalance = 0;

  // Saved address selection Ã¢â‚¬â€ backend expects address_id (UUID)
  String? _selectedAddressId;
  AddressEntity? _selectedAddress;

  // Shipping option state Ã¢â‚¬â€ for standard checkout (not seller quote)
  String? _selectedShippingOptionId;
  List<DeliveryOption> _deliveryOptions = [];
  bool _isLoadingDeliveryOptions = false;

  // Stock warning state - tracks if user has been warned about limited stock
  bool _hasShownStockWarning = false;

  // Auction winner context - used to display winner-specific messaging
  bool get _isAuctionWinner =>
      widget.auctionId != null && widget.auctionId!.isNotEmpty;

  // Negotiation checkout context - used to display negotiation-specific messaging
  bool get _isNegotiationCheckout =>
      widget.negotiationId != null && widget.negotiationId!.isNotEmpty;

  // Preview state - stores the backend-calculated pricing
  PreviewOrderResult? _previewResult;

  // R1.1: IDENTITY OF THE APPLIED PREVIEW.
  //
  // Signature of the preview inputs that produced `_previewResult`. A preview
  // is only current while this equals the signature of the CURRENT inputs, so
  // a preview computed for other inputs can never be rendered as current nor
  // submitted as the pricing token for an order.
  String? _previewSignature;

  // R1.1: a preview input changed while a request was in flight. The refresh is
  // queued (never silently dropped) and re-issued when the in-flight request
  // completes, so the latest inputs always win.
  bool _previewRefreshQueued = false;

  // DISCOUNT HONESTY: Applied discount state from backend validation
  // - Frontend only stores what backend returns
  // - Used for display purposes only
  // - NOT used for any calculations (all pricing from preview API)
  Discount? _appliedDiscount;

  // CONCURRENCY GUARD: Preview fetch state to prevent overlapping calls
  bool _isFetchingPreview = false;
  String? _previewError;

  // Auto-refresh guard to prevent multiple refresh calls during near-expiry
  bool _hasAutoRefreshed = false;

  // Debouncer for preview API calls
  Timer? _previewDebounceTimer;

  // Immediate submit lock - set synchronously to prevent double-tap
  bool _isSubmitting = false;

  // Token expiry tracking
  DateTime? _previewTokenCreatedAt;
  Timer? _expiryCountdownTimer;
  static const Duration _tokenValidityDuration = Duration(minutes: 10);

  @override
  void initState() {
    super.initState();
    // Fetch coin balance on screen load
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(coinProvider.notifier).getBalance();
    });
  }

  void _updateState(VoidCallback callback) => setState(callback);

  // ==========================================================================
  // PREVIEW INPUTS + READINESS PROJECTION (R1 / R1.1 / R1.2)
  // ==========================================================================

  /// Whether this checkout is driven by a seller shipping quote instead of a
  /// buyer-selected shipping option.
  bool get _isShippingQuoteCheckout =>
      widget.shippingQuoteId != null && widget.shippingQuoteId!.isNotEmpty;

  /// The CURRENT preview inputs.
  ///
  /// This is the single builder used both to issue the preview request and to
  /// compute the request signature, so "what we asked for" and "what is
  /// current" can never drift.
  ///
  /// ONLY fields that determine backend pricing are included. Notes and coin
  /// intent are order-creation inputs — the preview endpoint cannot consume
  /// them — so they must not invalidate a preview.
  PreviewOrderParams _buildPreviewParams() {
    final isAuction = widget.auctionId != null && widget.auctionId!.isNotEmpty;
    return PreviewOrderParams(
      productId: widget.productId,
      quantity: 1,
      addressId: _selectedAddressId,
      discountCode: _appliedDiscount?.code,
      negotiationId: widget.negotiationId,
      auctionId: widget.auctionId,
      // Backend GeneratePreviewRequest requires both fields (binding:"required").
      sourceType: isAuction ? 'auction' : 'for_sale',
      sourceId: isAuction ? widget.auctionId! : widget.forSaleId,
      shippingQuoteId: widget.shippingQuoteId,
      // Order-owned PreviewOrderParams field stays `shippingSetupId`; Checkout
      // only supplies its own `_selectedShippingOptionId` value into it.
      shippingSetupId: _isShippingQuoteCheckout
          ? null
          : _selectedShippingOptionId,
    );
  }

  /// Identity of the CURRENT preview inputs.
  ///
  /// It mirrors exactly the fields that reach POST /pricing/preview (see
  /// OrderRepository.previewOrder), so a result whose signature differs from
  /// the current one describes a DIFFERENT buyer intent and can never be
  /// applied or submitted.
  String _buildCheckoutSignature() {
    final params = _buildPreviewParams();
    return <String>[
      params.productId ?? '',
      params.sourceType ?? '',
      params.sourceId ?? '',
      '${params.quantity}',
      params.addressId ?? '',
      params.shippingSetupId ?? '',
      params.shippingQuoteId ?? '',
      params.negotiationId ?? '',
      params.discountCode ?? '',
    ].join('|');
  }

  /// Whether an applied preview exists AND still describes the current inputs.
  bool get _hasFreshPreview =>
      _previewResult != null &&
      _previewSignature != null &&
      _previewSignature == _buildCheckoutSignature();

  /// The single truthful readiness of the pricing step.
  ///
  /// `forSale.price` is never an input: a local price can never make checkout
  /// ready. The action bar and the create-order guard both read THIS value, so
  /// the button and the guard can never disagree.
  CheckoutReadiness get _readiness => evaluateCheckoutReadiness(
    CheckoutReadinessInputs(
      hasProductId: widget.productId != null && widget.productId!.isNotEmpty,
      hasAddress: _selectedAddressId != null && _selectedAddressId!.isNotEmpty,
      requiresShippingSelection: !_isShippingQuoteCheckout,
      hasShippingSelection:
          _selectedShippingOptionId != null &&
          _selectedShippingOptionId!.isNotEmpty,
      hasPreview: _previewResult != null,
      isPreviewCurrent: _hasFreshPreview,
      isPreviewExpired: _previewResult != null && _isTokenExpired(),
      hasPricingToken: (_previewResult?.pricingToken ?? '').isNotEmpty,
      isLoadingPreview: _isFetchingPreview,
      hasPreviewError: _previewError != null,
    ),
  );

  @override
  Widget build(BuildContext context) {
    // Theme is read ONCE from the canonical colour scheme. Checkout keeps no
    // dark/light branch of its own: the app-wide AppBarTheme/ColorScheme decides
    // every surface, so checkout can never drift from the rest of the app.
    final colorScheme = Theme.of(context).colorScheme;
    final checkoutState = ref.watch(checkoutNotifierProvider);
    ref.watch(forSaleDetailProvider(widget.forSaleId));

    // READINESS: the only authority for "may the buyer press Buat Pesanan".
    // Pricing availability is read from the backend preview, never from
    // forSale.price.
    final readiness = _readiness;
    final isPricingAvailable = readiness.isReady;
    final displayPreview = _hasFreshPreview ? _previewResult : null;

    // Listen to response state
    ref.listen<CheckoutState>(checkoutNotifierProvider, (previous, next) {
      if (next.error != null && mounted) {
        final errorMessage = next.error!;
        // Inline gate: backend rejected order creation because the user's
        // email is not verified. Buy-now and direct checkout funnel through
        // this same notifier, so the gate covers both call sites.
        if (next.errorCode == api_codes.emailVerificationRequired) {
          ref.read(checkoutNotifierProvider.notifier).clearError();
          showBlockedActionGate(
            context,
            actionDescription: 'melakukan checkout',
          );
          return;
        }
        // Commerce restriction family — canonical backend rejection.
        // Dispatch is by error CODE inside the canonical presenter:
        // COMMERCE_RESTRICTED → restriction snackbar,
        // MARKET_AUTHORITY_REQUIRED → canonical seller renewal. This screen
        // only clears its own notifier error first (same ordering as the
        // email-verification gate above) so it is never re-presented.
        if (CommerceRestrictionPresenter.isRestrictionPresented(
          next.errorCode,
        )) {
          ref.read(checkoutNotifierProvider.notifier).clearError();
        }
        if (CommerceRestrictionPresenter.handle(
          context,
          errorCode: next.errorCode,
          actionDescription: 'melakukan checkout',
        )) {
          return;
        }
        // Check if this is a CheckoutException with specific handling
        if (errorMessage.contains('PRICING_TOKEN_EXPIRED') ||
            errorMessage.contains('Waktu harga telah habis')) {
          _showTokenExpiredDialog();
        } else if (errorMessage.contains('PRICING_TOKEN_INVALID') ||
            errorMessage.contains('Token harga tidak valid')) {
          _showOrderError(
            CheckoutHonestyMessages.tokenInvalidTitle,
            suggestion: CheckoutHonestyMessages.tokenInvalidMessage,
          );
        } else if (errorMessage.contains('FOR_SALE_UNAVAILABLE') ||
            errorMessage.contains('OUT_OF_STOCK') ||
            errorMessage.contains('forSale not available') ||
            errorMessage.contains('insufficient stock')) {
          _showAvailabilityError(errorMessage);
        } else if (errorMessage.contains('NEGOTIATION_UNAVAILABLE') ||
            errorMessage.contains('negotiation') &&
                errorMessage.contains('available')) {
          _showOrderError(
            CheckoutHonestyMessages.negotiationUnavailableTitle,
            suggestion: CheckoutHonestyMessages.negotiationUnavailableMessage,
          );
        } else if (errorMessage.contains('QUOTE_UNAVAILABLE') ||
            errorMessage.contains('seller quote') &&
                errorMessage.contains('available')) {
          _showOrderError(
            CheckoutHonestyMessages.quoteUnavailableTitle,
            suggestion: CheckoutHonestyMessages.quoteUnavailableMessage,
          );
        } else if (errorMessage.contains('AUCTION_UNAVAILABLE') ||
            errorMessage.contains('auction') &&
                errorMessage.contains('available')) {
          _showOrderError(
            CheckoutHonestyMessages.auctionUnavailableTitle,
            suggestion: CheckoutHonestyMessages.auctionUnavailableMessage,
          );
        } else if (next.errorCode == 'SHIPPING_INVALID' ||
            errorMessage.contains('SHIPPING_INVALID') ||
            errorMessage.contains('Alamat pengiriman tidak valid')) {
          _showOrderError(
            CheckoutHonestyMessages.shippingAddressInvalidTitle,
            suggestion: CheckoutHonestyMessages.shippingAddressInvalidMessage,
          );
        } else if (next.errorCode == 'NO_SHIPPING_OPTIONS') {
          // Phase 0 honesty: seller has not configured any shipping options for
          // this forSale. Render canonical message + working Hubungi Penjual CTA.
          _showShippingUnavailableError(code: 'NO_SHIPPING_OPTIONS');
        } else if (next.errorCode == 'SHIPPING_OPTION_UNAVAILABLE' ||
            // Legacy substring fallback while older clients/servers are in flight.
            errorMessage.contains('SHIPPING_UNAVAILABLE') ||
            errorMessage.contains('pengiriman tidak tersedia') ||
            errorMessage.contains('shipping tidak available')) {
          _showShippingUnavailableError(code: 'SHIPPING_OPTION_UNAVAILABLE');
        } else {
          // Generic error - show snackbar for other errors
          AppSnackBar.showError(context, errorMessage);
        }
        ref.read(checkoutNotifierProvider.notifier).clearError();
      }
    });

    // Listen to coin state changes
    ref.listen<CoinState>(coinProvider, (previous, next) {
      next.maybeWhen(
        balanceLoaded: (balance, coinBalanceEntity) {
          if (mounted) {
            setState(() {
              _coinBalance = balance;
            });
            // Refetch preview when coin balance changes
            _schedulePreview();
          }
        },
        orElse: () {},
      );
    });

    // NAVIGATION GUARD: Prevent accidental back during order submission
    return PopScope(
      canPop: !_isSubmitting && !checkoutState.isCreatingOrder,
      child: Scaffold(
        // Lowest surface tone: the app palette equals this to the current
        // light/dark checkout background without any local branch.
        backgroundColor: colorScheme.surfaceContainerLowest,
        appBar: AppBar(
          title: const Text('Checkout'),
          leading: IconButton(
            icon: const Icon(Icons.arrow_back),
            onPressed: () => Navigator.of(context).pop(),
          ),
          // Colours come from the canonical AppBarTheme (AppTheme), not from a
          // checkout-local brightness branch.
          elevation: 0,
          surfaceTintColor: Colors.transparent,
          scrolledUnderElevation: 0,
        ),
        body: SingleChildScrollView(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // AUCTION WINNER CONTEXT: Show winner-specific messaging for auction checkout
              // This frames the checkout as "securing your victory" rather than generic purchase
              if (_isAuctionWinner) _AuctionWinnerBanner(),

              // NEGOTIATION UX FIX: Show warning when checking out from negotiation
              // Negotiation acceptance does NOT reserve the product - checkout is required
              if (_isNegotiationCheckout) _NegotiationWarningBanner(),

              // **STOCK WARNING UX FIX 2:** Show stock warning after preview succeeds
              if (_previewResult != null && !_hasShownStockWarning)
                Padding(
                  padding: const EdgeInsets.only(bottom: 16),
                  child: _StockWarningBanner(),
                ),

              // Order Summary Section
              // A preview is rendered ONLY while it is current: a result that no
              // longer matches the inputs stays out of the money model.
              _OrderSummarySection(
                forSaleId: widget.forSaleId,
                previewResult: displayPreview,
                readiness: readiness,
                remainingTime: _getTokenRemainingTime(),
                onRefreshPricing: () => _fetchPreview(isManualRefresh: true),
                isAuctionCheckout: _isAuctionWinner,
              ),

              const SizedBox(height: 24),

              // **CV3:** Shipping Clarity Banner - Explains seller-managed shipping model
              // This sets proper expectations before user fills out address
              _ShippingClarityBanner(),

              const SizedBox(height: 16),

              // Shipping Address Section Ã¢â‚¬â€ saved address picker
              _SavedAddressPickerSection(
                selectedAddressId: _selectedAddressId,
                onAddressSelected: _onAddressSelected,
              ),

              const SizedBox(height: 24),

              // Shipping Option Picker Ã¢â‚¬â€ only for standard checkout (not seller quote)
              if (widget.shippingQuoteId == null)
                _ShippingSetupPickerSection(
                  deliveryOptions: _deliveryOptions,
                  selectedOptionId: _selectedShippingOptionId,
                  isLoading: _isLoadingDeliveryOptions,
                  hasAddress: _selectedAddressId != null,                   onSelected: _onShippingOptionSelected,
                ),
              if (widget.shippingQuoteId == null) const SizedBox(height: 24),

              // Coin Toggle Section
              // Coin intent is an ORDER-CREATION input, not a pricing input:
              // POST /pricing/preview cannot consume it, so toggling coins must
              // not invalidate the current preview.
              _CoinToggleSection(
                useCoins: _useCoins,
                coinBalance: _coinBalance,
                onToggle: (value) {
                  setState(() => _useCoins = value);
                },
              ),

              // DISCOUNT HONESTY: Discount Input Section
              // - Shows discount code input field
              // - Validates via backend, displays result honestly
              // - The applied discount is NOT rendered as its own summary row:
              //   the backend folds it into the canonical money model
              //   (subtotal / total_before_coins_amount) returned by the preview
              // - Hidden for seller quote / negotiation only
              if (_supportsDiscounts())
                _DiscountSection(
                  sellerId: displayPreview?.sellerId ?? '',
                  subtotal: displayPreview?.subtotal ?? 0,
                  contextType: widget.auctionId == null
                      ? 'for_sale'
                      : 'auction',
                  onDiscountApplied: _handleDiscountApplied,
                  appliedDiscount: _appliedDiscount,
                ),

              // Notes Section — carried to POST /orders (order input), never to
              // the pricing preview, so editing notes must not invalidate the
              // current preview.
              _NotesSection(notesController: _notesController),

              const SizedBox(height: 100), // Space for bottom bar
            ],
          ),
        ),
        bottomNavigationBar: _CheckoutBottomBar(
          isCreatingOrder: checkoutState.isCreatingOrder,
          isSubmitting: _isSubmitting,
          isReady: isPricingAvailable,
          disabledReason: readiness.message,
          previewResult: displayPreview,
          onCreateOrder: _handleCreateOrder,
          isAuctionWinner: _isAuctionWinner,
        ),
      ),
    );
  }

  /// Schedules a preview refresh (debounced) for a REAL pricing-input change.
  ///
  /// Prerequisites are gated here: a preview can only be requested once the
  /// request could actually produce canonical pricing.
  void _schedulePreview() {
    // Cancel previous timer
    _previewDebounceTimer?.cancel();

    // Only preview if address is selected
    if (_selectedAddressId == null || _selectedAddressId!.isEmpty) {
      return;
    }

    // For standard checkout, also require shipping option selection
    if (widget.shippingQuoteId == null &&
        (_selectedShippingOptionId == null ||
            _selectedShippingOptionId!.isEmpty)) {
      return;
    }

    // Schedule preview after debounce
    _previewDebounceTimer = Timer(const Duration(milliseconds: 500), () {
      _fetchPreview();
    });
  }

  /// Trigger preview when address selection changes
  void _onAddressSelected(AddressEntity address) {
    setState(() {
      _selectedAddress = address;
      _selectedAddressId = address.id;
      _selectedShippingOptionId = null;
      _deliveryOptions = [];
    });
    if (widget.shippingQuoteId == null) {
      _loadDeliveryOptions();
    } else {
      _schedulePreview();
    }
  }

  Future<void> _loadDeliveryOptions() async {
    if (_selectedAddress == null) return;

    // Resolve the product ID for the delivery availability check.
    //
    // Auction path: productId is already in widget.productId (passed explicitly
    // from the auction detail buy-now CTA). Never use forSaleDetailProvider
    // with auction.id — that lookup returns null, blocking delivery options.
    //
    // FPS path: productId is extracted from the forSale detail (existing behaviour).
    final String? productId;
    if (widget.auctionId != null && widget.auctionId!.isNotEmpty) {
      productId = widget.productId;
    } else {
      // The product detail may still be in flight on first open. Awaiting the
      // canonical future keeps the shipping/preview pipeline from dying on a
      // transient miss (previously: silent early return, no retry, no preview).
      ForSale? forSale;
      try {
        forSale = await ref.read(
          forSaleDetailProvider(widget.forSaleId).future,
        );
      } catch (_) {
        forSale = null;
      }
      if (!mounted) return;
      if (forSale == null) return;
      productId = forSale.productId;
    }

    setState(() {
      _isLoadingDeliveryOptions = true;
    });
    try {
      final repo = ref.read(shippingRepositoryProvider);
      if (productId == null || productId.isEmpty) {
        if (!mounted) return;
        setState(() {
          _isLoadingDeliveryOptions = false;
          _deliveryOptions = [];
        });
        AppSnackBar.showError(
          context,
          'Product ID belum tersedia untuk memuat opsi pengiriman.',
        );
        return;
      }
      final result = await repo.checkDeliveryAvailability(
        CheckDeliveryRequest(
          productId: productId,
          // 2-digit / 4-digit BPS codes (address `Province.id` / `City.id`).
          provinceId: _selectedAddress!.province.id,
          cityId: _selectedAddress!.city.id,
        ),
      );
      if (!mounted) return;
      setState(() {
        _deliveryOptions = result.data ?? [];
        _isLoadingDeliveryOptions = false;
        if (_deliveryOptions.length == 1) {
          _selectedShippingOptionId = _deliveryOptions.first.shippingSetupId;
          _schedulePreview();
        }
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _isLoadingDeliveryOptions = false;
      });
    }
  }

  void _onShippingOptionSelected(String shippingOptionId) {
    setState(() {
      _selectedShippingOptionId = shippingOptionId;
    });
    _schedulePreview();
  }

  Future<void> _fetchPreview({bool isManualRefresh = false}) =>
      _checkoutFetchPreview(this, isManualRefresh: isManualRefresh);

  /// Starts the countdown timer for token expiry
  ///
  /// Also handles auto-refresh when token is near expiry to prevent
  /// disruption during checkout flow.
  void _startExpiryCountdown() {
    _expiryCountdownTimer?.cancel();
    _expiryCountdownTimer = Timer.periodic(const Duration(seconds: 1), (timer) {
      if (_previewTokenCreatedAt == null) {
        timer.cancel();
        return;
      }

      final expiryTime = _previewTokenCreatedAt!.add(_tokenValidityDuration);
      if (DateTime.now().isAfter(expiryTime)) {
        timer.cancel();
        if (mounted) {
          setState(() {
            // Token expired - will be reflected in UI
          });
        }
      } else if (mounted) {
        setState(() {
          // Update countdown UI
        });

        // AUTO REFRESH: Silent refresh when token is near expiry
        // Only refresh if not currently submitting to avoid disruption
        // Only trigger once per near-expiry cycle to avoid spam
        if (isTokenNearExpiry &&
            !_isSubmitting &&
            !_isFetchingPreview &&
            !_hasAutoRefreshed) {
          _hasAutoRefreshed = true;
          _fetchPreview(isManualRefresh: false);
        }
      }
    });
  }

  /// Checks if the current pricing token is expired
  bool _isTokenExpired() {
    if (_previewTokenCreatedAt == null) return true;
    return DateTime.now().isAfter(
      _previewTokenCreatedAt!.add(_tokenValidityDuration),
    );
  }

  /// Checks if the pricing token is near expiry (less than 2 minutes remaining)
  ///
  /// When true, auto-refresh will be triggered to prevent token expiry
  /// during checkout flow.
  bool get isTokenNearExpiry {
    final remaining = _getTokenRemainingTime();
    if (remaining == null) return false;
    return remaining.inSeconds < 120;
  }

  /// Checks if promo discounts are supported for this checkout
  ///
  /// SOURCE GUARD: Promo discounts apply to forSale and auction checkout.
  /// Negotiation (private agreement) remains excluded.
  bool _supportsDiscounts() {
    return widget.negotiationId == null;
  }

  /// DISCOUNT HONESTY: Handler for discount application from DiscountInputField
  ///
  /// HONESTY:
  /// - This callback only stores the backend-validated code
  /// - The preview API is re-requested so the backend returns authoritative
  ///   pricing with the discount already folded into the money model
  /// - Frontend NEVER calculates discount amounts locally (the validated amount
  ///   is not stored: the canonical discount is the backend's `discount_amount`,
  ///   which is already reflected in the preview subtotal/total)
  void _handleDiscountApplied(Discount? discount, double discountAmount) {
    setState(() {
      _appliedDiscount = discount;
    });

    // The discount code is a real pricing input: a new preview is required.
    _schedulePreview();
  }

  /// Gets remaining time until token expiry
  Duration? _getTokenRemainingTime() {
    if (_previewTokenCreatedAt == null) return null;
    final expiryTime = _previewTokenCreatedAt!.add(_tokenValidityDuration);
    if (DateTime.now().isAfter(expiryTime)) return Duration.zero;
    return expiryTime.difference(DateTime.now());
  }

  @override
  void dispose() {
    _previewDebounceTimer?.cancel();
    _expiryCountdownTimer?.cancel();
    _notesController.dispose();
    super.dispose();
  }

  Future<void> _handleCreateOrder() => _checkoutHandleCreateOrder(this);

  /// Shows an error dialog with detailed error message and suggestion
  void _showOrderError(String title, {String? suggestion}) {
    if (!mounted) return;
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        icon: Icon(
          Icons.error_outline,
          color: Theme.of(context).colorScheme.error,
          size: 48,
        ),
        title: Text(title),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (suggestion != null) ...[
              Text(suggestion),
              const SizedBox(height: 16),
            ],
            Text(
              'Apakah Anda ingin mencoba lagi?',
              style: Theme.of(context).textTheme.bodyMedium,
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('Kembali'),
          ),
          ElevatedButton(
            onPressed: () {
              Navigator.of(context).pop();
              _fetchPreview(isManualRefresh: true);
            },
            style: ElevatedButton.styleFrom(
              backgroundColor: Theme.of(context).colorScheme.primary,
              foregroundColor: Theme.of(context).colorScheme.onPrimary,
            ),
            child: const Text('Refresh Harga'),
          ),
        ],
      ),
    );
  }

  /// Shows an availability error dialog with UX honesty about first-come-first-served
  void _showAvailabilityError(String errorMessage) {
    if (!mounted) return;

    // Determine if it's out of stock or forSale unavailable
    final isOutOfStock =
        errorMessage.contains('OUT_OF_STOCK') ||
        errorMessage.contains('insufficient stock') ||
        errorMessage.contains('habis');

    final title = isOutOfStock
        ? CheckoutHonestyMessages.outOfStockTitle
        : CheckoutHonestyMessages.forSaleUnavailableTitle;

    final message = isOutOfStock
        ? CheckoutHonestyMessages.outOfStockMessage
        : CheckoutHonestyMessages.forSaleUnavailableMessage;

    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        icon: Icon(
          Icons.inventory_2_outlined,
          color: AppColors.statusWarning,
          size: 48,
        ),
        title: Text(title),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(message),
            const SizedBox(height: 16),
            Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: Theme.of(context).colorScheme.surfaceContainerHighest,
                borderRadius: BorderRadius.circular(8),
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.info_outline,
                    size: 16,
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      CheckoutHonestyMessages.firstComeFirstServedExplanation,
                      style: TextStyle(
                        fontSize: 12,
                        color: Theme.of(context).colorScheme.onSurfaceVariant,
                        fontStyle: FontStyle.italic,
                      ),
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 16),
            Text(
              CheckoutHonestyMessages.suggestionBrowseProducts,
              style: Theme.of(context).textTheme.bodyMedium,
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('Tutup'),
          ),
          ElevatedButton(
            onPressed: () {
              Navigator.of(context).pop();
              Navigator.of(context).pop(); // Go back to forSale
            },
            style: ElevatedButton.styleFrom(
              backgroundColor: Theme.of(context).colorScheme.primary,
              foregroundColor: Theme.of(context).colorScheme.onPrimary,
            ),
            child: const Text('Kembali ke ForSale'),
          ),
        ],
      ),
    );
  }

  /// Phase 0: shows the uncovered-area / seller-not-configured dialog with a
  /// working "Hubungi Penjual" CTA. Title + body vary by [code]:
  ///   - SHIPPING_OPTION_UNAVAILABLE ? buyer address outside coverage.
  ///   - NO_SHIPPING_OPTIONS         ? seller never linked any options.
  void _showShippingUnavailableError({
    String code = 'SHIPPING_OPTION_UNAVAILABLE',
  }) {
    if (!mounted) return;

    final isNoOptions = code == 'NO_SHIPPING_OPTIONS';
    final title = isNoOptions
        ? 'Pengiriman Belum Diatur Penjual'
        : 'Di Luar Area Pengiriman';
    final body = isNoOptions
        ? 'Penjual belum mengatur pengiriman untuk forSale ini. '
              'Hubungi penjual untuk meminta opsi pengiriman.'
        : 'Produk ini di luar area pengiriman. '
              'Jika Anda berminat, hubungi seller.';

    showDialog(
      context: context,
      builder: (dialogCtx) => AlertDialog(
        icon: Icon(
          Icons.local_shipping_outlined,
          color: AppColors.statusWarning,
          size: 48,
        ),
        title: Text(title),
        content: Text(body),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogCtx).pop(),
            child: const Text('Tutup'),
          ),
          ElevatedButton(
            onPressed: () {
              Navigator.of(dialogCtx).pop();
              _openChatWithSeller();
            },
            style: ElevatedButton.styleFrom(
              backgroundColor: Theme.of(context).colorScheme.primary,
              foregroundColor: Theme.of(context).colorScheme.onPrimary,
            ),
            child: const Text('Hubungi Penjual'),
          ),
        ],
      ),
    );
  }

  /// Opens the canonical pre-order commerce chat with this forSale's seller.
  /// Used by the uncovered-area / seller-not-configured dialog CTA.
  ///
  /// Checkout is PRE-ORDER at this point, so `openOrderCommerceChat` (which
  /// links an order into the room) does not apply. The canonical pre-order
  /// authority is the Chat domain's `openCommerceChat`: it resolves/creates the
  /// room, carries this forSale as a pending product reference so the seller
  /// knows what the buyer is asking about, enforces the guest boundary and the
  /// self-chat guard, and navigates to the canonical `/chat/<room-id>` route.
  ///
  /// Checkout MUST NOT resolve chat rooms or build chat routes itself: it owns
  /// no chat authority. The only checkout-owned concern is the failure copy on
  /// THIS surface, so the CTA never becomes a lying affordance.
  Future<void> _openChatWithSeller() async {
    final forSale = ref         .read(forSaleDetailProvider(widget.forSaleId))
        .asData
        ?.value;
    final sellerId = forSale?.sellerId ?? _previewResult?.sellerId;

    if (forSale == null || sellerId == null || sellerId.isEmpty) {
      if (mounted) {
        AppSnackBar.showError(context, 'Tidak dapat membuka chat penjual.');
      }
      return;
    }

    await openCommerceChat(
      context: context,
      ref: ref,
      reference: ShareReference.forSale(
        forSaleId: forSale.forSaleId,
        title: forSale.title,
        imageUrl: forSale.media.isNotEmpty
            ? (forSale.media.first.thumbnailUrl ??
                  forSale.media.first.originalUrl)
            : null,
        isAvailable: forSale.isAvailable,
        isSold: forSale.stock == 0,
      ),
      sellerId: sellerId,
      onFailure: (error) {
        if (!mounted) return;
        AppSnackBar.showError(
          context,
          error.toLowerCase().contains('blocked')
              ? 'Tidak dapat mengirim pesan. Pengguna ini telah memblokir Anda.'
              : 'Gagal membuka chat. Coba lagi.',
        );
      },
    );
  }

  /// Shows dialog when pricing token has expired
  void _showTokenExpiredDialog() {
    if (!mounted) return;
    showDialog(
      context: context,
      barrierDismissible: false,
      builder: (context) => AlertDialog(
        icon: Icon(
          Icons.timer_outlined,
          color: AppColors.statusWarning,
          size: 48,
        ),
        title: const Text('Waktu Harga Habis'),
        content: const Text(
          'Harga yang ditampilkan sudah kadaluarsa. Silakan refresh untuk mendapatkan harga terbaru sebelum melanjutkan pembayaran.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('Batal'),
          ),
          ElevatedButton(
            onPressed: () {
              Navigator.of(context).pop();
              _fetchPreview(isManualRefresh: true);
            },
            style: ElevatedButton.styleFrom(
              backgroundColor: Theme.of(context).colorScheme.primary,
              foregroundColor: Theme.of(context).colorScheme.onPrimary,
            ),
            child: const Text('Refresh Harga'),
          ),
        ],
      ),
    );
  }
}
