/// Checkout Screen
///
/// Real Transaction Flow - Supports Direct Buy, Seller Quote, Negotiation, and Auction
///
/// Flow:
/// 1. User views listing details OR negotiates price OR wins auction
/// 2. User clicks "Beli Sekarang" (Buy Now) or "Lanjut Beli" (Continue Purchase) or "Klaim Kemenangan" (Claim Victory)
/// 3. Navigate to this CheckoutScreen with fixedPriceSaleId + productId + optional commerce context
/// 4. Call preview API to get backend-calculated pricing
/// 5. Review order details with preview pricing
/// 6. Click "Buat Pesanan" (Create Order) or "Amankan Kemenangan" (Secure Victory)
/// 7. Create order via API with product_id+source_type+source_id (+ negotiation_id or auction_id)
/// 8. Open payment URL with url_launcher
///
/// **IMPORTANT:** Pricing is sourced from backend preview API, NOT from listing.price
///
/// **CHAT COMMERCE SUPPORT:**
/// - Direct listing purchase: provide sale surface ID + productId
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
/// - Promo discounts ONLY apply to direct listing purchase (not seller quote, negotiation, auction)
/// - Discount codes are validated by backend, frontend only displays result
/// - Applied discount shows clear description and amount from backend
/// - No fake pricing or misleading savings - all numbers come from backend preview
library;

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/transaction/checkout/checkout.dart';
import 'package:labuda/domains/commerce/transaction/checkout/presentation/utils/checkout_honesty_messages.dart';
import 'package:labuda/domains/finance/wallet/coins/coins.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/widgets/discount_input_field.dart';
import 'package:labuda/domains/chat/chat/chat.dart';
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
import 'package:url_launcher/url_launcher.dart';
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
  final String fixedPriceSaleId;

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
    required this.fixedPriceSaleId,
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
  String? _selectedShippingSetupId;
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

  // DISCOUNT HONESTY: Applied discount state from backend validation
  // - Frontend only stores what backend returns
  // - Used for display purposes only
  // - NOT used for any calculations (all pricing from preview API)
  Discount? _appliedDiscount;
  double _appliedDiscountAmount = 0;

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

    // Add listeners to form fields for auto-preview
    _addPreviewListeners();
  }

  void _updateState(VoidCallback callback) => setState(callback);

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final checkoutState = ref.watch(checkoutNotifierProvider);
    ref.watch(forSaleDetailProvider(widget.fixedPriceSaleId));

    // PRICING AVAILABILITY: Check if preview result is available from backend
    // This replaces the old check for listing.price > 0
    final isPricingAvailable = _previewResult != null;

    // Listen to response state
    ref.listen<CheckoutState>(checkoutNotifierProvider, (previous, next) {
      if (next.error != null && mounted) {
        final errorMessage = next.error!;
        // Inline gate: backend rejected order creation because the user's
        // email is not verified. Buy-now and direct checkout funnel through
        // this same notifier, so the gate covers both call sites.
        if (next.errorCode == 'EMAIL_VERIFICATION_REQUIRED') {
          ref.read(checkoutNotifierProvider.notifier).clearError();
          showBlockedActionGate(
            context,
            actionDescription: 'melakukan checkout',
          );
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
        } else if (errorMessage.contains('LISTING_UNAVAILABLE') ||
            errorMessage.contains('OUT_OF_STOCK') ||
            errorMessage.contains('listing not available') ||
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
          // this listing. Render canonical message + working Hubungi Penjual CTA.
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
        backgroundColor: isDark
            ? AppColors.darkGray900
            : AppColors.neutralGray50,
        appBar: AppBar(
          title: const Text('Checkout'),
          leading: IconButton(
            icon: const Icon(Icons.arrow_back),
            onPressed: () => Navigator.of(context).pop(),
          ),
          backgroundColor: isDark
              ? AppColors.darkGray800
              : AppColors.neutralWhite,
          foregroundColor: isDark
              ? AppColors.neutralWhite
              : AppColors.neutralGray900,
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
              _OrderSummarySection(
                fixedPriceSaleId: widget.fixedPriceSaleId,
                previewResult: _previewResult,
                isTokenExpired: _isTokenExpired(),
                remainingTime: _getTokenRemainingTime(),
                isFetchingPreview: _isFetchingPreview,
                previewError: _previewError,
                onRefreshPricing: () => _fetchPreview(isManualRefresh: true),
                supportsDiscounts: _supportsDiscounts(),
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
                  selectedOptionId: _selectedShippingSetupId,
                  isLoading: _isLoadingDeliveryOptions,
                  hasAddress: _selectedAddressId != null,
                  onSelected: _onShippingSetupSelected,
                ),
              if (widget.shippingQuoteId == null) const SizedBox(height: 24),

              // Coin Toggle Section
              _CoinToggleSection(
                useCoins: _useCoins,
                coinBalance: _coinBalance,
                onToggle: (value) {
                  setState(() => _useCoins = value);
                  // Refetch preview when coin toggle changes
                  _schedulePreview();
                },
              ),

              // DISCOUNT HONESTY: Discount Input Section
              // - Shows discount code input field
              // - Validates via backend, displays result honestly
              // - Applied discount shown in pricing breakdown from backend preview
              // - Hidden for seller quote / negotiation only
              if (_supportsDiscounts())
                _DiscountSection(
                  sellerId: _previewResult?.sellerId ?? '',
                  subtotal: _previewResult?.subtotal ?? 0,
                  contextType: widget.auctionId == null ? 'for_sale' : 'auction',
                  onDiscountApplied: _handleDiscountApplied,
                  appliedDiscount: _appliedDiscount,
                  appliedDiscountAmount: _appliedDiscountAmount,
                ),

              // Notes Section
              _NotesSection(
                notesController: _notesController,
                onNotesChanged: _schedulePreview,
              ),

              const SizedBox(height: 100), // Space for bottom bar
            ],
          ),
        ),
        bottomNavigationBar: _CheckoutBottomBar(
          isCreatingOrder: checkoutState.isCreatingOrder,
          isSubmitting: _isSubmitting,
          isPricingAvailable: isPricingAvailable,
          isTokenExpired: _isTokenExpired(),
          previewResult: _previewResult,
          onCreateOrder: _handleCreateOrder,
          isAuctionWinner: _isAuctionWinner,
        ),
      ),
    );
  }

  void _addPreviewListeners() {
    // Notes controller can trigger preview refresh (e.g. discount code)
    _notesController.addListener(_schedulePreview);
  }

  void _schedulePreview() {
    // Cancel previous timer
    _previewDebounceTimer?.cancel();

    // Only preview if address is selected
    if (_selectedAddressId == null || _selectedAddressId!.isEmpty) {
      return;
    }

    // For standard checkout, also require shipping option selection
    if (widget.shippingQuoteId == null &&
        (_selectedShippingSetupId == null ||
            _selectedShippingSetupId!.isEmpty)) {
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
      _selectedShippingSetupId = null;
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
    // from the auction detail buy-now CTA). Never use fixedPriceSaleDetailProvider
    // with auction.id — that lookup returns null, blocking delivery options.
    //
    // FPS path: productId is extracted from the listing detail (existing behaviour).
    final String? productId;
    if (widget.auctionId != null && widget.auctionId!.isNotEmpty) {
      productId = widget.productId;
    } else {
      final listingAsync = ref.read(
        forSaleDetailProvider(widget.fixedPriceSaleId),
      );
      final listing = listingAsync.value;
      if (listing == null) return;
      productId = listing.productId;
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
          provinceId: _selectedAddress!.province.id,
          cityId: _selectedAddress!.city.id,
          cityName: _selectedAddress!.city.name,
        ),
      );
      if (!mounted) return;
      setState(() {
        _deliveryOptions = result.data ?? [];
        _isLoadingDeliveryOptions = false;
        if (_deliveryOptions.length == 1) {
          _selectedShippingSetupId = _deliveryOptions.first.shippingSetupId;
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

  void _onShippingSetupSelected(String shippingSetupId) {
    setState(() {
      _selectedShippingSetupId = shippingSetupId;
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
  /// SOURCE GUARD: Promo discounts apply to listing and auction checkout.
  /// Negotiation (private agreement) remains excluded.
  bool _supportsDiscounts() {
    return widget.negotiationId == null;
  }

  /// DISCOUNT HONESTY: Handler for discount application from DiscountInputField
  ///
  /// Callback contract:
  /// - discount: null if code was cleared/invalid, otherwise the validated Discount entity
  /// - discountAmount: the validated discount amount from backend
  ///
  /// HONESTY:
  /// - This callback only stores state from backend validation
  /// - Preview API is called to get authoritative pricing with discount applied
  /// - Frontend NEVER calculates discount amounts locally
  ///
  /// FLOW:
  /// 1. DiscountInputField validates code via backend
  /// 2. If valid, store discount state for display
  /// 3. Trigger preview refresh to get updated pricing from backend
  /// 4. Backend preview returns final pricing with discount applied
  void _handleDiscountApplied(Discount? discount, double discountAmount) {
    setState(() {
      _appliedDiscount = discount;
      _appliedDiscountAmount = discountAmount;
    });

    // Trigger preview refresh to get updated pricing from backend
    // This ensures the pricing shown is always from backend with discount applied
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
        icon: Icon(Icons.error_outline, color: AppColors.statusError, size: 48),
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
              backgroundColor: AppColors.primaryRed,
              foregroundColor: Colors.white,
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

    // Determine if it's out of stock or listing unavailable
    final isOutOfStock =
        errorMessage.contains('OUT_OF_STOCK') ||
        errorMessage.contains('insufficient stock') ||
        errorMessage.contains('habis');

    final title = isOutOfStock
        ? CheckoutHonestyMessages.outOfStockTitle
        : CheckoutHonestyMessages.listingUnavailableTitle;

    final message = isOutOfStock
        ? CheckoutHonestyMessages.outOfStockMessage
        : CheckoutHonestyMessages.listingUnavailableMessage;

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
                color: AppColors.neutralGray100,
                borderRadius: BorderRadius.circular(8),
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.info_outline,
                    size: 16,
                    color: AppColors.neutralGray600,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      CheckoutHonestyMessages.firstComeFirstServedExplanation,
                      style: TextStyle(
                        fontSize: 12,
                        color: AppColors.neutralGray600,
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
              Navigator.of(context).pop(); // Go back to listing
            },
            style: ElevatedButton.styleFrom(
              backgroundColor: AppColors.primaryRed,
              foregroundColor: Colors.white,
            ),
            child: const Text('Kembali ke Listing'),
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
        ? 'Penjual belum mengatur pengiriman untuk listing ini. '
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
              backgroundColor: AppColors.primaryRed,
              foregroundColor: Colors.white,
            ),
            child: const Text('Hubungi Penjual'),
          ),
        ],
      ),
    );
  }

  /// Phase 0: open (or create) the canonical direct chat room between the
  /// current buyer and this listing's seller, then navigate to that room.
  /// Used by the uncovered-area / seller-not-configured dialog CTA.
  ///
  /// No order exists at this point (buyer is pre-purchase), so we skip the
  /// optional linkOrderToChat step from GetOrCreateCommerceChatUseCase and
  /// just call the repository directly.
  Future<void> _openChatWithSeller() async {
    final listingAsync = ref.read(
      forSaleDetailProvider(widget.fixedPriceSaleId),
    );
    final listing = listingAsync.asData?.value;
    final sellerId = listing?.sellerId ?? _previewResult?.sellerId;
    final authState = ref.read(authControllerProvider);
    final currentUserId = authState is AuthStateAuthenticated
        ? authState.user.id
        : null;

    if (sellerId == null || sellerId.isEmpty || currentUserId == null) {
      if (mounted) {
        AppSnackBar.showError(context, 'Tidak dapat membuka chat penjual.');
      }
      return;
    }
    if (sellerId == currentUserId) {
      if (mounted) {
        AppSnackBar.showError(context, 'Anda adalah penjual listing ini.');
      }
      return;
    }

    final chatRepository = ref.read(chatRepositoryProvider);
    final result = await chatRepository.getOrCreateChat(
      participantIds: [currentUserId, sellerId],
    );
    if (!mounted) return;
    if (result.isError) {
      AppSnackBar.showError(
        context,
        'Gagal membuka chat: ${result.error ?? 'unknown'}',
      );
      return;
    }
    final room = result.data!;
    context.push('/chat/${room.id}');
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
              backgroundColor: AppColors.primaryRed,
              foregroundColor: Colors.white,
            ),
            child: const Text('Refresh Harga'),
          ),
        ],
      ),
    );
  }

  /// Launches the payment URL with improved fallback handling
  Future<void> _launchPaymentUrl(String url, String orderId) async {
    if (url.isEmpty) {
      _showPaymentLaunchError(orderId);
      return;
    }

    try {
      final uri = Uri.parse(url);
      if (await canLaunchUrl(uri)) {
        final launched = await launchUrl(
          uri,
          mode: LaunchMode.externalApplication,
        );

        if (!launched && mounted) {
          // URL could not be launched
          _showPaymentLaunchError(orderId);
        }
      } else {
        _showPaymentLaunchError(orderId);
      }
    } catch (e) {
      if (mounted) {
        _showPaymentLaunchError(orderId);
      }
    }
  }

  /// Shows error dialog when payment URL cannot be launched
  void _showPaymentLaunchError(String orderId) {
    if (!mounted) return;
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        icon: Icon(Icons.link_off, color: AppColors.statusWarning, size: 48),
        title: const Text('Gagal Membuka Pembayaran'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text(
              'Tidak dapat membuka halaman pembayaran. Namun pesanan Anda sudah berhasil dibuat.',
            ),
            const SizedBox(height: 16),
            Text(
              'Order ID: ${orderId.substring(0, 8)}...',
              style: const TextStyle(fontFamily: 'monospace', fontSize: 12),
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
              // Navigate to order list
              context.push('/orders');
            },
            style: ElevatedButton.styleFrom(
              backgroundColor: AppColors.primaryRed,
              foregroundColor: Colors.white,
            ),
            child: const Text('Lihat Pesanan Saya'),
          ),
        ],
      ),
    );
  }
}
