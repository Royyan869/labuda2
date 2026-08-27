part of 'checkout_screen_impl.dart';

Future<void> _checkoutFetchPreview(
  _CheckoutScreenState state, {
  bool isManualRefresh = false,
}) async {
  final productId = state.widget.productId;

  // Validate address is selected
  if (state._selectedAddressId == null || state._selectedAddressId!.isEmpty) {
    if (isManualRefresh && state.mounted) {
      AppSnackBar.showError(
        state.context,
        'Pilih alamat pengiriman terlebih dahulu',
      );
    }
    return;
  }

  if (productId == null || productId.isEmpty) {
    if (state.mounted) {
      state._updateState(() {
        state._previewError = 'ID produk tidak tersedia untuk checkout ini';
      });
    }
    if (isManualRefresh && state.mounted) {
      AppSnackBar.showError(
        state.context,
        'ID produk tidak tersedia untuk checkout ini',
      );
    }
    return;
  }

  // CONCURRENCY GUARD: Prevent overlapping preview fetches
  if (state._isFetchingPreview) {
    // A fetch is already in progress, skip this one
    // The debounce timer will naturally limit rapid calls
    return;
  }

  // Listing availability check applies only to fixed-price-sale path.
  // For auction checkout (auctionId != null), the backend validates auction state.
  if (state.widget.auctionId == null) {
    final listingAsync = state.ref.read(
      forSaleDetailProvider(state.widget.fixedPriceSaleId),
    );
    final listing = listingAsync.value;

    if (listing == null) {
      if (isManualRefresh && state.mounted) {
        AppSnackBar.showError(state.context, 'Produk tidak ditemukan');
      }
      return;
    }

    if (!listing.isAvailable) {
      if (isManualRefresh && state.mounted) {
        AppSnackBar.showError(state.context, 'Produk tidak tersedia');
      }
      return;
    }
  }

  // CONCURRENCY GUARD: Set fetching flag
  state._isFetchingPreview = true;
  // Clear previous error when starting new fetch
  if (state.mounted) {
    state._updateState(() {
      state._previewError = null;
    });
  }

  try {
    // Derive source_type and source_id from commerce context.
    // Backend GeneratePreviewRequest requires both fields (binding:"required").
    final String sourceType;
    final String sourceId;
    if (state.widget.auctionId != null && state.widget.auctionId!.isNotEmpty) {
      sourceType = 'auction';
      sourceId = state.widget.auctionId!;
    } else {
      sourceType = 'fixed_price_sale';
      sourceId = state.widget.fixedPriceSaleId;
    }

    // Create preview params with address_id for backend-side resolution
    final previewParams = PreviewOrderParams(
      productId: productId,
      quantity: 1,
      addressId: state._selectedAddressId,
      discountCode: state._appliedDiscount?.code,
      useCoins: state._useCoins,
      notes: state._notesController.text.trim().isEmpty
          ? null
          : state._notesController.text.trim(),
      negotiationId: state.widget.negotiationId,
      auctionId: state.widget.auctionId,
      sourceType: sourceType,
      sourceId: sourceId,
      shippingQuoteId: state.widget.shippingQuoteId,
      shippingOptionId: state.widget.shippingQuoteId == null
          ? state._selectedShippingOptionId
          : null,
    );

    // Call preview API via provider
    final previewAsync = state.ref.read(orderPreviewProvider(previewParams));

    // Update state with preview result
    if (previewAsync.hasValue && previewAsync.value != null) {
      if (state.mounted) {
        state._updateState(() {
          state._previewResult = previewAsync.value;
          state._previewTokenCreatedAt = DateTime.now();
          // Reset auto-refresh flag since we have a fresh token
          state._hasAutoRefreshed = false;
          // **STOCK WARNING UX FIX 2:** Mark stock warning as shown after first successful preview
          state._hasShownStockWarning = true;
        });

        // Start countdown timer
        state._startExpiryCountdown();

        if (isManualRefresh) {
          AppSnackBar.showSuccess(state.context, 'Harga berhasil diperbarui');
        }
      }
    } else if (previewAsync.hasError) {
      // SAFETY: Explicit error handling - store error state
      final error = previewAsync.error;
      if (state.mounted) {
        state._updateState(() {
          state._previewError = error?.toString() ?? 'Gagal memuat harga';
        });
        // Show error only on manual refresh
        if (isManualRefresh) {
          AppSnackBar.showError(
            state.context,
            'Gagal memperbarui harga. Silakan coba lagi',
          );
        }
      }
    }
  } catch (e) {
    // SAFETY: Explicit error state
    if (state.mounted) {
      state._updateState(() {
        state._previewError = e.toString();
      });
      // Show error only on manual refresh
      if (isManualRefresh) {
        AppSnackBar.showError(
          state.context,
          'Gagal memperbarui harga. Silakan coba lagi',
        );
      }
    }
  } finally {
    // CONCURRENCY GUARD: Always clear fetching flag
    if (state.mounted) {
      state._updateState(() {
        state._isFetchingPreview = false;
      });
    }
  }
}

Future<void> _checkoutHandleCreateOrder(_CheckoutScreenState state) async {
  final productId = state.widget.productId;

  // IMMEDIATE SUBMIT LOCK: Prevent double-tap synchronously
  if (state._isSubmitting) return;
  state._isSubmitting = true;

  try {
    // Listing validation applies only to fixed-price-sale path.
    // For auction checkout (auctionId != null), backend validates auction state
    // and seller authority — Guard 6 still rejects inactive sellers.
    if (state.widget.auctionId == null) {
      final listingAsync = state.ref.read(
        forSaleDetailProvider(state.widget.fixedPriceSaleId),
      );
      final listing = listingAsync.value;

      if (listing == null) {
        state._showOrderError(
          'Produk tidak ditemukan',
          suggestion: 'Silakan kembali dan pilih produk lain',
        );
        return;
      }

      if (!listing.isAvailable) {
        state._showOrderError(
          'Produk tidak tersedia',
          suggestion: 'Produk mungkin telah terjual atau dihapus oleh penjual',
        );
        return;
      }

      // SELLER TRUST GATE: Block checkout when seller subscription expired.
      // Backend Guard 6 also rejects, but this gives a specific user-facing message
      // instead of a generic "order creation failed" error.
      if (listing.sellerTrustLifecycle != ContentLifecycle.active) {
        state._showOrderError(
          'Penjual tidak aktif',
          suggestion:
              'Penjual ini tidak memiliki langganan aktif. '
              'Transaksi tidak dapat dilanjutkan.',
        );
        return;
      }
    }

    // VALIDATION: Check if preview result is available
    // This ensures pricing comes from backend, not from listing.price
    if (state._previewResult == null) {
      state._showOrderError(
        'Harga belum dimuat',
        suggestion: 'Mohon tunggu harga dimuat dari server',
      );
      return;
    }

    // TOKEN EXPIRY CHECK: Check if pricing token has expired
    if (state._isTokenExpired()) {
      state._showTokenExpiredDialog();
      return;
    }

    // STRICT VALIDATION: Check pricingToken exists
    // This ensures order uses the exact pricing snapshot from preview
    if (state._previewResult!.pricingToken == null ||
        state._previewResult!.pricingToken!.isEmpty) {
      state._showOrderError(
        'Token harga tidak valid',
        suggestion: 'Silakan muat ulang harga dengan tombol "Refresh Harga"',
      );
      return;
    }

    if (productId == null || productId.isEmpty) {
      state._showOrderError(
        'ID produk tidak tersedia',
        suggestion:
            'Checkout ini membutuhkan product authority dari backend. '
            'Silakan buka kembali dari sumber yang menyediakan ID produk.',
      );
      return;
    }

    // STRICT VALIDATION: Validate fixedPriceSaleId first
    if (state.widget.fixedPriceSaleId.isEmpty) {
      state._showOrderError(
        'Listing tidak valid',
        suggestion: 'Silakan kembali dan pilih produk lain',
      );
      return;
    }

    final notifier = state.ref.read(checkoutNotifierProvider.notifier);

    // Validate saved address is selected
    if (state._selectedAddressId == null || state._selectedAddressId!.isEmpty) {
      AppSnackBar.showError(
        state.context,
        'Pilih alamat pengiriman terlebih dahulu',
      );
      return;
    }

    // Validate shipping option selected (for standard checkout only)
    if (state.widget.shippingQuoteId == null &&
        (state._selectedShippingOptionId == null ||
            state._selectedShippingOptionId!.isEmpty)) {
      AppSnackBar.showError(
        state.context,
        'Pilih opsi pengiriman terlebih dahulu',
      );
      return;
    }

    final request = CheckoutRequest(
      productId: productId,
      fixedPriceSaleId: state.widget.fixedPriceSaleId,
      quantity: 1,
      useCoins: state._useCoins ? true : null,
      notes: state._notesController.text.trim().isEmpty
          ? null
          : state._notesController.text.trim(),
      addressId: state._selectedAddressId!,
      pricingToken: state._previewResult!.pricingToken!,
      // Pass commerce context through to order creation
      auctionId: state.widget.auctionId,
      negotiationId: state.widget.negotiationId,
      shippingQuoteId: state.widget.shippingQuoteId,
      shippingOptionId: state.widget.shippingQuoteId == null
          ? state._selectedShippingOptionId
          : null,
    );

    // -----------------------------------------------------------
    // STEP 1: CREATE ORDER (POST /orders ? returns Order entity)
    // -----------------------------------------------------------
    final orderResponse = await notifier.createOrder(request);

    if (orderResponse == null || !state.mounted) return;

    // -----------------------------------------------------------
    // STEP 1B: SELECT PAYMENT METHOD (PASS_18V)
    // -----------------------------------------------------------
    // Backend calculates the buyer payment fee per method; the buyer must
    // choose one before a payment can be created.
    final paymentRepo = state.ref.read(paymentRepositoryProvider);
    final methodsResult = await paymentRepo.getPaymentMethodOptions(
      orderResponse.orderId,
    );
    if (!state.mounted) return;
    final methods = methodsResult.fold<List<PaymentMethodOption>>(
      (options) => options,
      (_) => const [],
    );
    if (methods.isEmpty) {
      AppSnackBar.showError(
        state.context,
        'Tidak ada metode pembayaran tersedia. Silakan coba lagi dari halaman pesanan.',
      );
      final paymentResultUri =
          state.widget.returnToChat != null &&
              state.widget.returnToChat!.isNotEmpty
          ? '/payment-result/${orderResponse.orderId}?return_to_chat=${state.widget.returnToChat}'
          : '/payment-result/${orderResponse.orderId}';
      state.context.push(paymentResultUri, extra: orderResponse.orderNumber);
      return;
    }
    final selectedMethodCode = await PaymentMethodPickerSheet.show(
      state.context,
      methods: methods,
    );
    if (!state.mounted || selectedMethodCode == null) return;

    // -----------------------------------------------------------
    // STEP 2: CREATE PAYMENT (POST /payments ? returns payment_url)
    // -----------------------------------------------------------
    // Reuse the existing PaymentInitiationNotifier (same flow as
    // order detail "Pay Now" retry).
    final paymentNotifier = state.ref.read(paymentInitiationProvider.notifier);
    paymentNotifier.reset(); // Clear any stale state

    final paymentRequest = InitiatePaymentRequest(
      orderId: orderResponse.orderId,
      paymentMethodCode: selectedMethodCode,
    );

    final paymentIntent = await paymentNotifier.initiatePayment(paymentRequest);

    if (!state.mounted) return;

    if (paymentIntent == null) {
      // Payment initiation failed — order exists but payment not created.
      // Navigate to payment result screen so user can retry via "Pay Now".
      final paymentResultUri =
          state.widget.returnToChat != null &&
              state.widget.returnToChat!.isNotEmpty
          ? '/payment-result/${orderResponse.orderId}?return_to_chat=${state.widget.returnToChat}'
          : '/payment-result/${orderResponse.orderId}';
      state.context.push(paymentResultUri, extra: orderResponse.orderNumber);
      return;
    }

    // Launch the payment URL from the payment intent
    final paymentUrl = paymentIntent.paymentUrl;
    if (paymentUrl != null && paymentUrl.isNotEmpty) {
      await state._launchPaymentUrl(paymentUrl, orderResponse.orderId);
    }

    // Navigate to payment result screen to poll for status
    if (state.mounted) {
      final paymentResultUri =
          state.widget.returnToChat != null &&
              state.widget.returnToChat!.isNotEmpty
          ? '/payment-result/${orderResponse.orderId}?return_to_chat=${state.widget.returnToChat}'
          : '/payment-result/${orderResponse.orderId}';
      state.context.push(paymentResultUri, extra: orderResponse.orderNumber);
    }
  } finally {
    // Always reset submit lock, even on error
    if (state.mounted) {
      state._isSubmitting = false;
    }
  }
}
