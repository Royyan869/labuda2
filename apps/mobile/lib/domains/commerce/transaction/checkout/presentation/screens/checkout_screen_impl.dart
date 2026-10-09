/// Checkout Screen
///
/// Real Transaction Flow - Supports Direct Buy, Negotiation, and Auction
///
/// Flow:
/// 1. User views forSale details OR negotiates price OR wins auction
/// 2. User clicks "Beli Sekarang" (Buy Now) or "Lanjut Beli" (Continue Purchase) or "Klaim Kemenangan" (Claim Victory)
/// 3. Navigate to this CheckoutScreen with forSaleId + productId + optional commerce context
/// 4. Call preview API to get backend-calculated pricing
/// 5. Review order details with preview pricing
/// 6. Click "Buat Pesanan" (Create Order) or "Amankan Kemenangan" (Secure Victory)
/// 7. Create order via API with product_id+source_type+source_id (+ negotiation_id or auction_id)
/// 8. Navigate to the canonical Order Detail surface.
///
/// Checkout owns ORDER CREATION ONLY. It never initiates a payment: the created
/// order (`pending_payment`) is payable from Order Detail's canonical
/// "Bayar Sekarang" action, which is also the recovery surface.
///
/// **IMPORTANT:** Pricing is sourced from backend preview API, NOT from forSale.price
///
/// **ONE PURCHASE FUNNEL:**
/// - Direct forSale purchase: provide sale surface ID + productId
/// - Negotiation purchase: provide sale surface ID + productId + negotiationId
/// - Auction BUY-NOW: provide sale surface ID + productId + auctionId (auction active)
/// - Auction BID-WIN: provide sale surface ID + productId + auctionId + bidWin
///
/// **AUCTION BID-WIN (winner checkout):**
/// - Green winner banner + "Amankan Kemenangan" CTA
/// - NO payment-method selection here (Owner canonical): the order is created
///   without a method; the winner picks one at Order Detail's PaymentMethodPicker
///   and the first POST /payments binds it. The checkout shows the escrow base
///   honestly and never fakes a fee-inclusive final total.
///
/// **TOKEN EXPIRY:**
/// - Pricing tokens have limited lifetime (typically 10 minutes)
/// - Screen shows countdown to expiry
/// - User can manually refresh pricing when needed
///
/// **DISCOUNT HONESTY:**
/// - Promo discounts apply to forSale and auction (buy-now + bid-win); negotiation excluded
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
import 'package:labuda/domains/chat/chat/presentation/models/pending_commerce_attachment.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_providers.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/order/presentation/providers/order_providers.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/finance/transaction/payment/presentation/presentation.dart'
    show
        PaymentMethodOption,
        PaymentMethodPickerSheet,
        paymentRepositoryProvider,
        PreOrderPaymentMethodOption,
        PreOrderPaymentPricing;
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/address_selection_summary.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/entities/shipping.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart'
    show shippingRepositoryProvider;

part '../widgets/checkout_order_summary_section.dart';
part '../widgets/checkout_notes_section.dart';
part '../widgets/checkout_action_bar.dart';
part '../widgets/checkout_coin_section.dart';
part '../widgets/checkout_discount_section.dart';
part '../widgets/checkout_warning_banners.dart';
part '../widgets/checkout_shipping_section.dart';
part 'checkout_screen_logic.dart';

/// Composer draft carried by the ONE Chat entry point that already knows the
/// buyer is opening Chat because normal shipping is unavailable for this
/// destination: the Checkout uncovered-shipping shortcut. It asks about
/// shipping and is PRE-FILLED into the textarea (never auto-sent). No other
/// Chat entry supplies a draft.
const kCheckoutUncoveredShippingDraft =
    'Halo, apakah bisa dibantu ongkir untuk produk ini ke alamat saya?';

class _CheckoutQuantitySelector extends StatelessWidget {
  final int quantity;
  final int maximum;
  final VoidCallback onMinus;
  final VoidCallback onPlus;

  const _CheckoutQuantitySelector({
    required this.quantity,
    required this.maximum,
    required this.onMinus,
    required this.onPlus,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.end,
      children: [
        IconButton(
          onPressed: quantity > 1 ? onMinus : null,
          icon: const Icon(Icons.remove),
          tooltip: 'Kurangi jumlah',
        ),
        Text('$quantity'),
        IconButton(
          onPressed: quantity < maximum ? onPlus : null,
          icon: const Icon(Icons.add),
          tooltip: 'Tambah jumlah',
        ),
      ],
    );
  }
}

/// Checkout Screen
///
/// Checkout screen for transaction flow
/// Supports direct buy, negotiation, and auction commerce contexts
///
/// **CANONICAL PRICING FLOW:**
/// All pricing comes from backend preview API with pricing token.
/// Private agreement pricing is validated by backend - frontend cannot inject price.
class CheckoutScreen extends ConsumerStatefulWidget {
  final String? productId;
  final String forSaleId;

  /// Chat commerce context
  final String? negotiationId;

  /// Auction checkout context - buy-now or bid-win
  final String? auctionId;

  /// True when this is an auction BID-WIN checkout (the winner completing the
  /// settlement window). Bid-win orders are created WITHOUT a payment method
  /// — the winner chooses one at Order Detail and the first payment binds it.
  final bool bidWin;

  /// **SHIPPING QUOTE FIX:** Shipping quote ID from seller's manual quote
  /// When provided, the checkout will use the seller's quoted shipping price
  final String? shippingQuoteId;

  /// Conversation (chat room) the checkout was initiated from. Required when
  /// shippingQuoteId is set — the quote is scoped to the conversation that
  /// produced it and the backend rejects checkout from any other conversation.
  final String? chatId;

  const CheckoutScreen({
    super.key,
    this.productId,
    required this.forSaleId,
    this.negotiationId,
    this.auctionId,
    this.bidWin = false,
    this.shippingQuoteId,
    this.chatId,
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

  // Shipping option state Ã¢â‚¬â€ for standard checkout (not negotiation)
  String? _selectedShippingOptionId;
  List<DeliveryOption> _deliveryOptions = [];
  bool _isLoadingDeliveryOptions = false;

  // Stock warning state - tracks if user has been warned about limited stock
  bool _hasShownStockWarning = false;
  int _selectedQuantity = 1;
  ForSale? _forSale;

  // Auction context - any auction-sourced checkout (buy-now or bid-win).
  bool get _isAuctionContext =>
      widget.auctionId != null && widget.auctionId!.isNotEmpty;

  // Auction BID-WIN context - the winner completing the settlement window.
  // Winner messaging + unbound-order creation apply here only; buy-now is a
  // regular purchase of an active auction.
  bool get _isBidWin => _isAuctionContext && widget.bidWin;

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

  // CANONICAL PRE-ORDER PAYMENT STATE (Phase 2).
  //
  // One selection authority for the whole screen: the pre-order pricing (with
  // every method's backend-computed fee + FINAL payable) and the selected
  // method code. The canonical trigger/picker are only consumers/editors of
  // this state.
  PreOrderPaymentPricing? _preOrderPricing;
  String? _selectedPaymentMethodCode;
  bool _isLoadingPaymentMethods = false;
  String? _paymentMethodsError;

  Timer? _expiryCountdownTimer;

  @override
  void initState() {
    super.initState();
    // Fetch coin balance on screen load
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(coinProvider.notifier).getBalance();
    });
  }

  void _updateState(VoidCallback callback) => setState(callback);

  void _changeQuantity(int delta) {
    final stock = _forSale?.stock ?? 1;
    final next = (_selectedQuantity + delta).clamp(1, stock > 0 ? stock : 1);
    if (next == _selectedQuantity) return;
    setState(() => _selectedQuantity = next);
    _schedulePreview();
  }

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
      quantity: _selectedQuantity,
      addressId: _selectedAddressId,
      discountCode: _appliedDiscount?.code,
      negotiationId: widget.negotiationId,
      auctionId: widget.auctionId,
      // Backend GeneratePreviewRequest requires both fields (binding:"required").
      sourceType: isAuction ? 'auction' : 'for_sale',
      sourceId: isAuction ? widget.auctionId! : widget.forSaleId,
      shippingQuoteId: widget.shippingQuoteId,
      // Conversation scope carried from the chat that produced the quote.
      chatId: widget.chatId,
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
      params.chatId ?? '',
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
      isLoadingPaymentMethods: _isLoadingPaymentMethods,
      hasPaymentMethodsError: _paymentMethodsError != null,
      hasSelectedPaymentMethod: _selectedPaymentMethodCode != null,
      // Bid-win never binds a method at creation (chosen at Order Detail),
      // so the payment-method states never gate its readiness.
      requiresPaymentMethodSelection: !_isBidWin,
    ),
  );

  @override
  Widget build(BuildContext context) {
    // Theme is read ONCE from the canonical colour scheme. Checkout keeps no
    // dark/light branch of its own: the app-wide AppBarTheme/ColorScheme decides
    // every surface, so checkout can never drift from the rest of the app.
    final colorScheme = Theme.of(context).colorScheme;
    final checkoutState = ref.watch(checkoutNotifierProvider);
    // ForSale detail drives quantity/stock/trust facts on for-sale checkouts
    // only; auction contexts resolve their sale-surface data from the auction
    // authority (see the order summary section).
    if (!_isAuctionContext) {
      ref.watch(forSaleDetailProvider(widget.forSaleId));
    }

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
        // Backend-rejection handler (defense-in-depth): the backend stays
        // the single authority for EMAIL_VERIFICATION_REQUIRED. Buy-now and
        // direct checkout funnel through this same notifier.
        if (next.errorCode == api_codes.emailVerificationRequired) {
          ref.read(checkoutNotifierProvider.notifier).clearError();
          AppSnackBar.showError(
            context,
            'Verifikasi email kamu diperlukan sebelum melakukan checkout.',
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
            // Coin balance affects the redeemable K, which changes the cash
            // base the payment fee is computed on — reload the pre-order
            // pricing. It does not change the product/shipping preview.
            _loadPreOrderPaymentMethods();
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
            icon: const Icon(Icons.arrow_back, semanticLabel: 'Kembali'),
            onPressed: () => Navigator.of(context).pop(),
          ),
        ),
        body: SingleChildScrollView(
          padding: const EdgeInsets.all(AppMetrics.p16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // AUCTION BID-WIN CONTEXT: winner-specific messaging — this is
              // the settlement-window completion of a won auction, framed as
              // "securing your victory".
              if (_isBidWin) _AuctionWinnerBanner(),

              // NEGOTIATION UX FIX: Show warning when checking out from negotiation
              // Negotiation acceptance does NOT reserve the product - checkout is required
              if (_isNegotiationCheckout) _NegotiationWarningBanner(),

              // **STOCK WARNING UX FIX 2:** Show stock warning after preview succeeds
              if (_previewResult != null && !_hasShownStockWarning)
                Padding(
                  padding: const EdgeInsets.only(bottom: AppMetrics.p16),
                  child: _StockWarningBanner(),
                ),

              if ((_forSale?.stock ?? 0) > 1) ...[
                _CheckoutQuantitySelector(
                  quantity: _selectedQuantity,
                  maximum: _forSale!.stock,
                  onMinus: () => _changeQuantity(-1),
                  onPlus: () => _changeQuantity(1),
                ),
                const SizedBox(height: 24),
              ],

              // Order Summary Section
              // A preview is rendered ONLY while it is current: a result that no
              // longer matches the inputs stays out of the money model.
              _OrderSummarySection(
                forSaleId: widget.forSaleId,
                previewResult: displayPreview,
                readiness: readiness,
                onRefreshPricing: () => _fetchPreview(isManualRefresh: true),
                isAuctionCheckout: _isAuctionContext,
                preOrderPricing:
                    _hasFreshPreview && !_isBidWin ? _preOrderPricing : null,
                selectedMethodCode: _selectedPaymentMethodCode,
              ),

              const SizedBox(height: 24),

              // **CV3:** Shipping Clarity Banner - Explains seller-managed shipping model
              // This sets proper expectations before user fills out address
              _ShippingClarityBanner(),

              const SizedBox(height: 16),

              // Shipping Address Summary — canonical ONE-address summary
              // (caller-selected or primary); the full list only lives in
              // the AddressPickerSheet.
              AddressSelectionSummary(
                selectedAddressId: _selectedAddressId,
                onAddressSelected: _onAddressSelected,
              ),

              const SizedBox(height: 24),

              // Shipping Option Picker Ã¢â‚¬â€ only for standard checkout (not negotiation)
              if (widget.shippingQuoteId == null)
                _ShippingSetupPickerSection(
                  deliveryOptions: _deliveryOptions,
                  selectedOptionId: _selectedShippingOptionId,
                  isLoading: _isLoadingDeliveryOptions,
                  hasAddress: _selectedAddressId != null,
                  onSelected: _onShippingOptionSelected,
                  // Single chat channel: the picker CTA and the uncovered-area
                  // dialog both go through `_openChatWithSeller`.
                  onContactSeller: _openChatWithSeller,
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
                  // Coin intent changes the cash base, so the backend-computed
                  // fee and final total must be recomputed. No local arithmetic.
                  _loadPreOrderPaymentMethods();
                },
              ),

              // DISCOUNT HONESTY: Discount Input Section
              // - Shows discount code input field
              // - Validates via backend, displays result honestly
              // - The applied discount is NOT rendered as its own summary row:
              //   the backend folds it into the canonical money model
              //   (subtotal / total_before_coins_amount) returned by the preview
              // - Hidden for negotiation only
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

              // PAYMENT METHOD — creation-time binding checkouts only
              // (for_sale + auction buy-now). Bid-win orders are created
              // WITHOUT a method (Owner canonical): the winner picks one at
              // Order Detail and the first payment binds it, so an honest
              // note replaces the picker — checkout never fakes a
              // fee-inclusive final total.
              if (displayPreview != null && !_isBidWin)
                PaymentMethodTrigger(
                  label: 'Metode Pembayaran',
                  selectedMethodCode: _selectedPaymentMethodCode,
                  selectedMethodDisplayName:
                      _selectedPaymentOption?.displayName,
                  isLoading: _isLoadingPaymentMethods,
                  hasMethods: _hasPreOrderPaymentMethods,
                  errorMessage: _paymentMethodsError,
                  onTap: _pickPaymentMethod,
                  onRetry: _loadPreOrderPaymentMethods,
                ),
              if (displayPreview != null && !_isBidWin)
                const SizedBox(height: 24),
              if (displayPreview != null && _isBidWin)
                _BidWinPaymentMethodNote(),
              if (displayPreview != null && _isBidWin)
                const SizedBox(height: 24),

              // Notes Section — carried to POST /orders (order input), never to
              // the pricing preview, so editing notes must not invalidate the
              // current preview.
              _NotesSection(notesController: _notesController),
            ],
          ),
        ),
        bottomNavigationBar: _CheckoutBottomBar(
          isCreatingOrder: checkoutState.isCreatingOrder,
          isSubmitting: _isSubmitting,
          isReady: isPricingAvailable,
          disabledReason: readiness.message,
          finalPayableAmount: _selectedPaymentOption?.finalPayableAmount,
          onCreateOrder: _handleCreateOrder,
          isBidWin: _isBidWin,
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
    _previewDebounceTimer = Timer(AppMotion.slow, () {
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

  Future<void> _loadPreOrderPaymentMethods() =>
      _checkoutLoadPreOrderPaymentMethods(this);

  /// The selected method's option, from the canonical pre-order pricing.
  ///
  /// Gated on a CURRENT preview: a method list (and its final total) computed
  /// for a different/stale input set must never be displayed or submitted.
  PreOrderPaymentMethodOption? get _selectedPaymentOption =>
      _hasFreshPreview && !_isLoadingPaymentMethods
      ? _preOrderPricing?.optionFor(_selectedPaymentMethodCode)
      : null;

  /// Whether the loaded pre-order pricing offers at least one method.
  bool get _hasPreOrderPaymentMethods =>
      (_preOrderPricing?.methods ?? const []).isNotEmpty;

  /// Opens the canonical payment-method picker for the pre-order methods and
  /// stores the buyer's choice. Selection state stays owned by this screen —
  /// the picker only returns a `methodCode` (or null on dismissal).
  Future<void> _pickPaymentMethod() async {
    final pricing = _preOrderPricing;
    if (pricing == null || pricing.methods.isEmpty) return;
    final code = await PaymentMethodPickerSheet.show(
      context,
      selectedMethodCode: _selectedPaymentMethodCode,
      methods: [
        for (final m in pricing.methods)
          PaymentMethodOption(
            methodCode: m.methodCode,
            displayName: m.displayName,
            buyerPaymentFeeAmount: m.buyerPaymentFeeAmount,
            totalPayableAmount: m.finalPayableAmount,
          ),
      ],
    );
    if (!mounted || code == null) return;
    setState(() => _selectedPaymentMethodCode = code);
  }

  /// Starts the countdown timer for token expiry
  ///
  /// Also handles auto-refresh when token is near expiry to prevent
  /// disruption during checkout flow.
  void _startExpiryCountdown() {
    _expiryCountdownTimer?.cancel();
    // Token expiry authority is the SERVER `expires_at`, never a client
    // created_at + fixed-duration guess. A 30s tick is enough to drive
    // auto-regeneration and the (optional) countdown without rebuilding the
    // whole screen every second.
    _expiryCountdownTimer = Timer.periodic(const Duration(seconds: 30), (
      timer,
    ) {
      if (_previewResult?.expiresAt == null) {
        timer.cancel();
        return;
      }

      if (mounted) {
        setState(() {
          // Refresh countdown/expiry projection.
        });

        // AUTO REGENERATION: silently re-preview when the token is near expiry.
        // Only once per near-expiry cycle, and never during submission.
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

  /// Checks if the current pricing token is expired (server `expires_at`).
  bool _isTokenExpired() {
    final expiresAt = _previewResult?.expiresAt;
    if (expiresAt == null) return true;
    return DateTime.now().isAfter(expiresAt);
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

  /// Gets remaining time until token expiry (server `expires_at`).
  Duration? _getTokenRemainingTime() {
    final expiresAt = _previewResult?.expiresAt;
    if (expiresAt == null) return null;
    if (DateTime.now().isAfter(expiresAt)) return Duration.zero;
    return expiresAt.difference(DateTime.now());
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
          size: AppIconSize.display,
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
          color: context.statusColors.warning,
          size: AppIconSize.display,
        ),
        title: Text(title),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(message),
            const SizedBox(height: 16),
            Container(
              padding: const EdgeInsets.all(AppMetrics.p12),
              decoration: BoxDecoration(
                color: Theme.of(context).colorScheme.surfaceContainerHighest,
                borderRadius: BorderRadius.circular(AppShape.r8),
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.info_outline,
                    size: AppIconSize.inlineGlyph,
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      CheckoutHonestyMessages.firstComeFirstServedExplanation,
                      style: context.typeRoles.bodyDense.copyWith(
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
          color: context.statusColors.warning,
          size: AppIconSize.display,
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
  /// room, carries this forSale as the canonical pending product attachment
  /// (sent only through the composer send icon) so the seller knows what the
  /// buyer is asking about, enforces the guest boundary and the self-chat
  /// guard, and navigates to the canonical `/chat/<room-id>` route.
  ///
  /// Checkout MUST NOT resolve chat rooms or build chat routes itself: it owns
  /// no chat authority. The only checkout-owned concern is the failure copy on
  /// THIS surface, so the CTA never becomes a lying affordance.
  Future<void> _openChatWithSeller() async {
    final forSale = ref
        .read(forSaleDetailProvider(widget.forSaleId))
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
      attachment: PendingCommerceAttachment.forSale(
        forSaleId: forSale.forSaleId,
        title: forSale.title,
        imageUrl: forSale.media.isNotEmpty
            ? (forSale.media.first.thumbnailUrl ??
                  forSale.media.first.originalUrl)
            : null,
        price: forSale.price.toInt(),
      ),
      sellerId: sellerId,
      // This entry point alone knows WHY the buyer is opening Chat (normal
      // shipping is unavailable for this destination), so it is the only one
      // allowed to pre-fill the shipping question. It is a draft, not a send.
      draftMessage: kCheckoutUncoveredShippingDraft,
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
          color: context.statusColors.warning,
          size: AppIconSize.display,
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
            child: const Text('Refresh Harga'),
          ),
        ],
      ),
    );
  }
}
