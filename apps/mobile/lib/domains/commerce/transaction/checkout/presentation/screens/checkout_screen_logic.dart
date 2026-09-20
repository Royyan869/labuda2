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

  // CONCURRENCY GUARD: never run two preview requests at once, but never drop
  // an input change either. A request that arrives while one is in flight is
  // QUEUED and re-issued with the latest inputs when the current one completes
  // — the latest inputs always win.
  if (state._isFetchingPreview) {
    state._previewRefreshQueued = true;
    return;
  }

  // ForSale availability check applies only to the for-sale path.
  // For auction checkout (auctionId != null), the backend validates auction state.
  if (state.widget.auctionId == null) {
    // Await the canonical detail future: a transient cache miss must not kill
    // the preview pipeline silently (that produced an endless spinner).
    ForSale? forSale;
    try {
      forSale = await state.ref.read(
        forSaleDetailProvider(state.widget.forSaleId).future,
      );
    } catch (_) {
      forSale = null;
    }
    if (!state.mounted) return;

    if (forSale == null) {
      // No product authority → pricing cannot be computed. Fail closed into a
      // retryable readiness state instead of a permanent loading state.
      state._updateState(() {
        state._previewError = 'Produk tidak ditemukan';
      });
      if (isManualRefresh) {
        AppSnackBar.showError(state.context, 'Produk tidak ditemukan');
      }
      return;
    }

    if (!forSale.isAvailable) {
      state._updateState(() {
        state._previewError = 'Produk tidak tersedia';
      });
      if (isManualRefresh) {
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

  // Canonical preview inputs. This is the SAME builder that produces the
  // request signature, so "what was requested" and "what is current" can never
  // drift apart. Only fields that actually determine backend pricing are
  // carried: notes and coin intent are ORDER-CREATION inputs (they never reach
  // POST /pricing/preview), so they must not invalidate a preview.
  final previewParams = state._buildPreviewParams();

  // R1 — CANONICAL PREVIEW RESOLUTION.
  //
  // Two defects lived here and both had the same symptom (checkout stuck on
  // "Memuat harga", `_previewResult` never populated, "Buat Pesanan"
  // permanently disabled):
  //
  // 1. The provider family KEY IS BUILT FROM THE CURRENT INPUTS, so a request
  //    starts in AsyncLoading. Reading it without awaiting returned AsyncLoading
  //    forever and `hasValue` stayed false.
  // 2. `provider.future` must never be awaited on a provider that Riverpod may
  //    auto-retry: the retry loop keeps the future pending, so a real backend
  //    failure was indistinguishable from a slow request. The preview boundary
  //    therefore disables auto-retry (see `orderPreviewProvider`), which makes
  //    the awaited future settle on both outcomes — data OR a real error.
  //
  // The request is also issued via `refresh`: a preview is a backend price
  // SNAPSHOT, not a cached answer, so every fetch must reach the canonical
  // boundary. (Re-reading a cached result would keep an expired token alive and
  // would reset the expiry countdown without a new token behind it.)
  final requestSignature = state._buildCheckoutSignature();

  try {
    final previewResult = await state.ref.refresh(
      orderPreviewProvider(previewParams).future,
    );
    if (!state.mounted) return;

    // R1.1 — STALE / OUT-OF-ORDER PROTECTION.
    //
    // If the preview inputs changed while this request was in flight, this
    // result no longer describes what the buyer is looking at. It MUST be
    // discarded (never applied, never submitted) and a refresh must be
    // queued so the latest inputs win — no silently dropped refresh.
    if (requestSignature != state._buildCheckoutSignature()) {
      state._previewRefreshQueued = true;
      return;
    }

    state._updateState(() {
      state._previewResult = previewResult;
      state._previewSignature = requestSignature;
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
  } catch (e) {
    // TRUTHFUL FAILURE: the readiness projection turns this into an
    // actionable retry state instead of a permanent spinner.
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
    // CONCURRENCY GUARD: always release the in-flight lock, then honor a
    // queued refresh so no input change is silently dropped.
    if (state.mounted) {
      state._updateState(() {
        state._isFetchingPreview = false;
      });
    }
    final shouldRefreshQueued = state._previewRefreshQueued;
    state._previewRefreshQueued = false;
    if (shouldRefreshQueued &&
        state.mounted &&
        state._previewSignature != state._buildCheckoutSignature()) {
      // The queued refresh came from a real pricing-input change, so re-issue
      // the request with the latest inputs. The latest result wins.
      await _checkoutFetchPreview(state);
    }
  }
}

Future<void> _checkoutHandleCreateOrder(_CheckoutScreenState state) async {
  final productId = state.widget.productId;

  // IMMEDIATE SUBMIT LOCK: Prevent double-tap synchronously
  if (state._isSubmitting) return;
  state._isSubmitting = true;

  try {
    // ForSale validation applies only to the for-sale path.
    // For auction checkout (auctionId != null), backend validates auction state
    // and seller authority — Guard 6 still rejects inactive sellers.
    if (state.widget.auctionId == null) {
      ForSale? forSale;
      try {
        forSale = await state.ref.read(
          forSaleDetailProvider(state.widget.forSaleId).future,
        );
      } catch (_) {
        forSale = null;
      }
      if (!state.mounted) return;

      if (forSale == null) {
        state._showOrderError(
          'Produk tidak ditemukan',
          suggestion: 'Silakan kembali dan pilih produk lain',
        );
        return;
      }

      if (!forSale.isAvailable) {
        state._showOrderError(
          'Produk tidak tersedia',
          suggestion: 'Produk mungkin telah terjual atau dihapus oleh penjual',
        );
        return;
      }

      // SELLER TRUST GATE: Block checkout when seller subscription expired.
      // Backend Guard 6 also rejects, but this gives a specific user-facing message
      // instead of a generic "order creation failed" error.
      if (forSale.sellerTrustLifecycle != ContentLifecycle.active) {
        state._showOrderError(
          'Penjual tidak aktif',
          suggestion:
              'Penjual ini tidak memiliki langganan aktif. '
              'Transaksi tidak dapat dilanjutkan.',
        );
        return;
      }
    }

    // STRICT VALIDATION: Validate forSaleId first
    if (state.widget.forSaleId.isEmpty) {
      state._showOrderError(
        'ForSale tidak valid',
        suggestion: 'Silakan kembali dan pilih produk lain',
      );
      return;
    }

    // READINESS GATE — the single authority for "may this order be created?".
    //
    // This replaces the ad-hoc product/address/shipping/preview/token checks
    // with one truthful projection: prerequisites, preview presence, preview
    // IDENTITY (current inputs), token expiry and token usability. A pricing
    // token from a preview that no longer matches the current inputs can never
    // be submitted, and a permanently loading preview can never masquerade as
    // "almost ready".
    final readiness = state._readiness;
    switch (readiness) {
      case CheckoutReadiness.ready:
        break;
      case CheckoutReadiness.missingProduct:
        state._showOrderError(
          'ID produk tidak tersedia',
          suggestion:
              'Checkout ini membutuhkan product authority dari backend. '
              'Silakan buka kembali dari sumber yang menyediakan ID produk.',
        );
        return;
      case CheckoutReadiness.missingAddress:
        AppSnackBar.showError(
          state.context,
          'Pilih alamat pengiriman terlebih dahulu',
        );
        return;
      case CheckoutReadiness.missingShipping:
        AppSnackBar.showError(
          state.context,
          'Pilih opsi pengiriman terlebih dahulu',
        );
        return;
      case CheckoutReadiness.loading:
        state._showOrderError(
          'Harga belum dimuat',
          suggestion: 'Mohon tunggu harga dimuat dari server',
        );
        return;
      case CheckoutReadiness.error:
      case CheckoutReadiness.stale:
        state._showOrderError(readiness.title, suggestion: readiness.message);
        return;
      case CheckoutReadiness.expired:
        state._showTokenExpiredDialog();
        return;
    }

    final notifier = state.ref.read(checkoutNotifierProvider.notifier);

    final request = CheckoutRequest(
      productId: productId,
      forSaleId: state.widget.forSaleId,
      quantity: 1,
      useCoins: state._useCoins ? true : null,
      notes: state._notesController.text.trim().isEmpty
          ? null
          : state._notesController.text.trim(),
      addressId: state._selectedAddressId!,
      // Guaranteed by readiness == ready (current + unexpired + usable token).
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

    // Payment URLs are presented exclusively inside Labuda's internal WebView.
    // External-browser payment navigation is obsolete and must not be reintroduced.
    final paymentUrl = paymentIntent.paymentUrl;
    if (paymentUrl != null && paymentUrl.isNotEmpty && state.mounted) {
      await state.context.push(
        '/payment-webview?url=${Uri.encodeComponent(paymentUrl)}&orderId=${Uri.encodeComponent(orderResponse.orderId)}',
      );
    }

    // Navigate to payment result screen to poll for status (backend-authoritative)
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
