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

    state._forSale = forSale;
    if (state._selectedQuantity > forSale.stock) {
      state._selectedQuantity = forSale.stock > 0 ? forSale.stock : 1;
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
      // Reset auto-refresh flag since we have a fresh token
      state._hasAutoRefreshed = false;
      // **STOCK WARNING UX FIX 2:** Mark stock warning as shown after first successful preview
      state._hasShownStockWarning = true;
    });

    // Start countdown timer
    state._startExpiryCountdown();

    // Canonical pre-order payment pricing for the fresh token: the buyer must
    // be able to choose a method and see the final total BEFORE "Buat Pesanan".
    await _checkoutLoadPreOrderPaymentMethods(state);

    if (isManualRefresh && state.mounted) {
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

/// Loads the canonical PRE-ORDER payment pricing for the currently applied
/// pricing token. Backend is the sole fee authority; the client only stores the
/// options and the buyer's selection.
///
/// INVARIANT: this only runs for a CURRENT preview (matching inputs, unexpired
/// token). Any input change clears it, so a stale token's method list can never
/// be shown or submitted.
Future<void> _checkoutLoadPreOrderPaymentMethods(
  _CheckoutScreenState state,
) async {
  // BID-WIN: no pre-order method selection exists (Owner canonical — the
  // winner binds a method at Order Detail via the first payment). Nothing to
  // load, no fee to fold in.
  if (state._isBidWin) {
    if (state.mounted) {
      state._updateState(() {
        state._preOrderPricing = null;
        state._selectedPaymentMethodCode = null;
        state._paymentMethodsError = null;
        state._isLoadingPaymentMethods = false;
      });
    }
    return;
  }

  final token = state._previewResult?.pricingToken;

  if (token == null || token.isEmpty || !state._hasFreshPreview) {
    if (state.mounted) {
      state._updateState(() {
        state._preOrderPricing = null;
        state._selectedPaymentMethodCode = null;
        state._paymentMethodsError = null;
        state._isLoadingPaymentMethods = false;
      });
    }
    return;
  }

  state._updateState(() {
    state._isLoadingPaymentMethods = true;
    state._paymentMethodsError = null;
  });

  try {
    final repo = state.ref.read(paymentRepositoryProvider);
    final result = await repo.getPreOrderPaymentPricing(
      token,
      useCoins: state._useCoins,
    );
    if (!state.mounted) return;

    result.fold(
      (error) {
        state._updateState(() {
          state._isLoadingPaymentMethods = false;
          state._preOrderPricing = null;
          state._selectedPaymentMethodCode = null;
          state._paymentMethodsError = error;
        });
      },
      (pricing) {
        // Keep the buyer's selection when the method is still offered;
        // otherwise default to the first method so a final total is always
        // visible once methods are available.
        final stillValid = pricing.methods.any(
          (m) => m.methodCode == state._selectedPaymentMethodCode,
        );
        final selected = stillValid
            ? state._selectedPaymentMethodCode
            : (pricing.methods.isNotEmpty
                  ? pricing.methods.first.methodCode
                  : null);
        state._updateState(() {
          state._isLoadingPaymentMethods = false;
          state._paymentMethodsError = null;
          state._preOrderPricing = pricing;
          state._selectedPaymentMethodCode = selected;
        });
      },
    );
  } catch (e) {
    if (!state.mounted) return;
    state._updateState(() {
      state._isLoadingPaymentMethods = false;
      state._paymentMethodsError = e.toString();
    });
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
      case CheckoutReadiness.paymentMethodsError:
        state._showOrderError(readiness.title, suggestion: readiness.message);
        return;
      case CheckoutReadiness.expired:
        state._showTokenExpiredDialog();
        return;
      case CheckoutReadiness.loadingPaymentMethods:
        AppSnackBar.showError(
          state.context,
          'Metode pembayaran masih dimuat. Mohon tunggu sebentar.',
        );
        return;
      case CheckoutReadiness.missingPaymentMethod:
        AppSnackBar.showError(
          state.context,
          'Pilih metode pembayaran terlebih dahulu',
        );
        return;
    }

    final notifier = state.ref.read(checkoutNotifierProvider.notifier);

    // SUBMISSION SNAPSHOT: capture the buyer's financial intent ONCE, before
    // the order is built. Method-binding checkouts guarantee a method is
    // selected (readiness). Bid-win submits NO method — the backend rejects
    // one on bid-win creation; Order Detail binds the first payment's method.
    final submittedUseCoins = state._useCoins;
    final submittedPaymentMethodCode = state._isBidWin
        ? null
        : state._selectedPaymentMethodCode!;

    final request = CheckoutRequest(
      productId: productId,
      forSaleId: state.widget.forSaleId,
      quantity: state._selectedQuantity,
      useCoins: submittedUseCoins ? true : null,
      notes: state._notesController.text.trim().isEmpty
          ? null
          : state._notesController.text.trim(),
      addressId: state._selectedAddressId!,
      // Guaranteed by readiness == ready (current + unexpired + usable token).
      pricingToken: state._previewResult!.pricingToken!,
      // Guaranteed by readiness == ready: a method is selected and its final
      // amount is known. The backend binds this method to the order.
      paymentMethodCode: submittedPaymentMethodCode,
      // Pass commerce context through to order creation
      auctionId: state.widget.auctionId,
      negotiationId: state.widget.negotiationId,
      shippingQuoteId: state.widget.shippingQuoteId,
      shippingOptionId: state.widget.shippingQuoteId == null
          ? state._selectedShippingOptionId
          : null,
    );

    // -----------------------------------------------------------
    // CREATE ORDER (POST /orders → returns the created Order)
    // -----------------------------------------------------------
    final orderResponse = await notifier.createOrder(request);

    if (orderResponse == null || !state.mounted) return;

    // ORDER CREATED. Order creation and payment initiation are SEPARATE
    // lifecycles: Checkout owns ONLY the order and NEVER auto-initiates a
    // payment. The created order is handed to the canonical Order Detail
    // surface, which shows the order status and owns the "Bayar Sekarang"
    // action (the backend decision action) plus its own recovery.
    //
    // `pushReplacement` swaps Checkout for Order Detail so Back returns to the
    // origin (forSale / chat), never to a spent Checkout.
    state.context.pushReplacement(
      RoutePaths.orderDetailPath(orderResponse.orderId),
    );
  } finally {
    // Always reset submit lock, even on error
    if (state.mounted) {
      state._isSubmitting = false;
    }
  }
}
