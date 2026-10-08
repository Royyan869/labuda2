/// Auction Claim Shipping Modal
///
/// Modal for selecting shipping address and delivery option when claiming an auction.
/// Replaces hardcoded dummy values with real user selection.
///
/// Flow:
/// 1. User selects a shipping address
/// 2. Available delivery options are fetched based on the address
/// 3. User selects a delivery option
/// 4. Claim proceeds with real values
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/entities/shipping.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/finance/wallet/coins/coins.dart';

/// Callback type for claim action with selected shipping details.
///
/// Exactly one of [shippingSetupId] (normal shipping option) or
/// [shippingQuoteId] (seller manual shipping quote, conversation-scoped) is set.
typedef ClaimCallback =
    Future<String?> Function({
      required String addressId,
      String? shippingSetupId,
      String? shippingQuoteId,
      String? chatId,
      String? discountCode,
      bool useCoins,
    });

/// Auction Claim Shipping Modal
///
/// Shows address and delivery option selection for auction claim flow.
///
/// When [shippingQuoteId] is provided (buyer reached checkout via the Chat
/// "Gunakan Ongkir" path), the seller's manual shipping quote replaces normal
/// shipping: the delivery-option picker is hidden and the quote + originating
/// [chatId] are forwarded to the claim authority.
class AuctionClaimShippingModal extends ConsumerStatefulWidget {
  final Auction auction;
  final ClaimCallback onClaim;
  final String? shippingQuoteId;
  final String? chatId;

  const AuctionClaimShippingModal({
    super.key,
    required this.auction,
    required this.onClaim,
    this.shippingQuoteId,
    this.chatId,
  });

  /// Open the claim workflow as a FULL SCREEN (substantial workflow; locked UX
  /// decision) and return order_id on success, null on cancel/failure.
  static Future<String?> show({
    required BuildContext context,
    required Auction auction,
    required ClaimCallback onClaim,
    String? shippingQuoteId,
    String? chatId,
  }) {
    return Navigator.of(context).push<String>(
      MaterialPageRoute(
        builder: (_) => AuctionClaimShippingModal(
          auction: auction,
          onClaim: onClaim,
          shippingQuoteId: shippingQuoteId,
          chatId: chatId,
        ),
      ),
    );
  }

  @override
  ConsumerState<AuctionClaimShippingModal> createState() =>
      _AuctionClaimShippingModalState();
}

class _AuctionClaimShippingModalState
    extends ConsumerState<AuctionClaimShippingModal> {
  // State
  AddressEntity? _selectedAddress;
  DeliveryOption? _selectedDeliveryOption;
  final TextEditingController _discountController = TextEditingController();
  bool _isClaiming = false;
  bool _useCoins = false;
  String? _error;

  // Async data
  List<AddressEntity> _addresses = [];
  bool _isLoadingAddresses = true;
  List<DeliveryOption> _deliveryOptions = [];
  bool _isLoadingDeliveryOptions = false;

  @override
  void initState() {
    super.initState();
    _loadAddresses();
  }

  @override
  void dispose() {
    _discountController.dispose();
    super.dispose();
  }

  /// Load user's shipping addresses
  Future<void> _loadAddresses() async {
    setState(() {
      _isLoadingAddresses = true;
      _error = null;
    });

    try {
      // AUTH-2 (CANONICAL AUTHORITY): hydrated current user from the canonical
      // authenticatedUserProvider instead of legacy authServiceProvider.
      final user = ref.read(authenticatedUserProvider);

      if (user == null) {
        setState(() {
          _isLoadingAddresses = false;
          _error = 'Gagal memuat alamat. Silakan coba lagi.';
        });
        return;
      }

      final addressRepository = ref.read(addressRepositoryProvider);
      final addressesResult = await addressRepository.getAddressesByUserId(
        user.id,
      );

      if (addressesResult.isError) {
        setState(() {
          _isLoadingAddresses = false;
          _error = addressesResult.error ?? 'Gagal memuat alamat';
        });
        return;
      }

      final addresses = addressesResult.data ?? [];

      setState(() {
        _addresses = addresses;
        _isLoadingAddresses = false;

        // Auto-select primary address if available
        if (addresses.isNotEmpty) {
          _selectedAddress = addresses.firstWhere(
            (addr) => addr.isPrimary,
            orElse: () => addresses.first,
          );
          // Load delivery options for selected address
          _loadDeliveryOptions();
        }
      });
    } catch (e) {
      setState(() {
        _isLoadingAddresses = false;
        _error = 'Gagal memuat alamat. Coba lagi.';
      });
    }
  }

  /// Load available delivery options for selected address
  Future<void> _loadDeliveryOptions() async {
    // A manual shipping quote replaces normal shipping: no delivery options.
    if (_hasQuote) return;
    if (_selectedAddress == null) return;

    setState(() {
      _isLoadingDeliveryOptions = true;
      _deliveryOptions = [];
      _selectedDeliveryOption = null;
    });

    try {
      final shippingRepository = ref.read(shippingRepositoryProvider);

      final productId = widget.auction.productId;
      if (productId == null || productId.isEmpty) {
        setState(() {
          _isLoadingDeliveryOptions = false;
          _error = 'Product ID belum tersedia untuk memuat opsi pengiriman.';
        });
        return;
      }

      final request = CheckDeliveryRequest(
        productId: productId,
        // 2-digit / 4-digit BPS codes (address `Province.id` / `City.id`).
        provinceId: _selectedAddress!.province.id,
        cityId: _selectedAddress!.city.id,
      );

      final result = await shippingRepository.checkDeliveryAvailability(
        request,
      );

      if (result.isError) {
        setState(() {
          _isLoadingDeliveryOptions = false;
          _error = result.error ?? 'Gagal memuat opsi pengiriman';
        });
        return;
      }

      final options = result.data ?? [];

      setState(() {
        _deliveryOptions = options;
        _isLoadingDeliveryOptions = false;
        _error = null;

        // Auto-select first option if available
        if (options.isNotEmpty) {
          _selectedDeliveryOption = options.first;
        }
      });
    } catch (e) {
      setState(() {
        _isLoadingDeliveryOptions = false;
        _error = 'Gagal memuat opsi pengiriman. Coba lagi.';
      });
    }
  }

  /// True when this claim uses the seller's manual shipping quote.
  bool get _hasQuote =>
      widget.shippingQuoteId != null && widget.shippingQuoteId!.isNotEmpty;

  /// Handle address selection
  void _onAddressSelected(AddressEntity address) {
    if (_selectedAddress?.id == address.id) return;
    setState(() {
      _selectedAddress = address;
      _selectedDeliveryOption = null;
    });
    _loadDeliveryOptions();
  }

  /// Handle delivery option selection
  void _onDeliveryOptionSelected(DeliveryOption option) {
    setState(() {
      _selectedDeliveryOption = option;
    });
  }

  /// Validate and proceed with claim
  Future<void> _handleClaim() async {
    final hasQuote =
        widget.shippingQuoteId != null && widget.shippingQuoteId!.isNotEmpty;

    // Validation
    if (_selectedAddress == null) {
      setState(() {
        _error = 'Pilih alamat pengiriman terlebih dahulu';
      });
      return;
    }

    if (!hasQuote && _selectedDeliveryOption == null) {
      setState(() {
        _error = 'Pilih opsi pengiriman terlebih dahulu';
      });
      return;
    }

    setState(() {
      _isClaiming = true;
      _error = null;
    });

    try {
      final orderId = await widget.onClaim(
        addressId: _selectedAddress!.id,
        shippingSetupId: hasQuote
            ? null
            : _selectedDeliveryOption!.shippingSetupId,
        shippingQuoteId: hasQuote ? widget.shippingQuoteId : null,
        chatId: hasQuote ? widget.chatId : null,
        discountCode: _discountController.text.trim().isEmpty
            ? null
            : _discountController.text.trim(),
        useCoins: _useCoins,
      );

      if (!mounted) return;

      if (orderId != null) {
        // Success - close modal and return order_id
        Navigator.of(context).pop(orderId);
      } else {
        // Error - show error and keep modal open
        setState(() {
          _isClaiming = false;
          _error = 'Gagal mengklaim lelang. Silakan coba lagi.';
        });
      }
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _isClaiming = false;
        _error = 'Terjadi kesalahan: $e';
      });
    }
  }

  /// True once the user has entered/chosen anything worth losing.
  bool get _hasDraft =>
      _selectedAddress != null || _selectedDeliveryOption != null;

  @override
  Widget build(BuildContext context) {
    return PopScope(
      // Dismissible, but a dirty workflow asks before discarding (locked UX
      // decision: no permanently locked surface).
      canPop: !_hasDraft && !_isClaiming,
      onPopInvokedWithResult: (didPop, _) async {
        if (didPop || _isClaiming) return;
        if (!_hasDraft) {
          if (mounted) Navigator.of(context).pop();
          return;
        }
        final discard = await AppDialog.confirm(
          context: context,
          title: 'Batalkan klaim?',
          message: 'Pilihan pengiriman Anda akan hilang.',
          confirmLabel: 'Batalkan',
          cancelLabel: 'Lanjutkan mengisi',
          intent: AppDialogIntent.destructive,
        );
        if (discard && mounted) Navigator.of(context).pop();
      },
      child: Scaffold(
        appBar: AppBar(
          title: const Text('Klaim Lelang'),
          leading: IconButton(
            onPressed: _isClaiming
                ? null
                : () => Navigator.of(context).maybePop(),
            icon: const Icon(Icons.close, semanticLabel: 'Tutup'),
          ),
        ),
        body: SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(
            AppMetrics.p16,
            AppMetrics.p16,
            AppMetrics.p16,
            AppMetrics.p16,
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                _hasQuote
                    ? 'Lengkapi alamat untuk melanjutkan klaim dengan ongkir dari penawaran penjual.'
                    : 'Lengkapi alamat dan pilih opsi pengiriman untuk melanjutkan klaim.',
                style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
              ),
              const SizedBox(height: AppMetrics.p16),

              // Error banner
              if (_error != null) _buildErrorBanner(),

              // Address selection
              _buildAddressSection(),

              const SizedBox(height: 24),

              // Delivery options OR the seller's manual shipping quote.
              if (_hasQuote)
                _buildQuoteNotice()
              else
                _buildDeliveryOptionsSection(),

              const SizedBox(height: 20),

              _buildDiscountField(),

              const SizedBox(height: 20),

              _buildCoinToggle(),

              // Visual spacing only — the Scaffold (body resize) and
              // BottomActionBar own keyboard/system inset for this route.
              const SizedBox(height: AppMetrics.p16),
            ],
          ),
        ),
        // Bottom action bar
        bottomNavigationBar: _buildBottomBar(context),
      ),
    );
  }

  Widget _buildErrorBanner() {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      margin: const EdgeInsets.only(bottom: AppMetrics.p16),
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: scheme.error.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(AppShape.r8),
        border: Border.all(color: scheme.error.withValues(alpha: 0.3)),
      ),
      child: Row(
        children: [
          Icon(
            Icons.error_outline,
            color: scheme.error,
            size: AppIconSize.action,
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              _error!,
              style: context.typeRoles.labelMicro.copyWith(
                color: scheme.onSurface,
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildAddressSection() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Alamat Pengiriman',
          // Section-header role (canonical map: section → titleMedium); it was
          // s16 here while checkout said the same line at s18.
          style: Theme.of(context).textTheme.titleMedium,
        ),
        const SizedBox(height: 12),
        if (_isLoadingAddresses)
          const Center(
            child: Padding(
              padding: EdgeInsets.all(AppMetrics.p24),
              child: CircularProgressIndicator(),
            ),
          )
        else if (_addresses.isEmpty)
          // Layout is the canonical one; the ACTION is this flow's: close the
          // sheet and prompt (until the claim flow can resume after the
          // address book).
          ShippingAddressEmptyState(
            onAdd: () {
              AppSnackBar.showInfo(
                context,
                'Silakan tambahkan alamat terlebih dahulu',
              );
              Navigator.of(context).pop();
            },
          )
        else
          ..._addresses.map(
            (address) => ShippingAddressCard(
              address: address,
              isSelected: _selectedAddress?.id == address.id,
              onTap: () => _onAddressSelected(address),
            ),
          ),
      ],
    );
  }

  Widget _buildDeliveryOptionsSection() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Opsi Pengiriman',
          style: context.typeRoles.titleSection.copyWith(
            fontWeight: FontWeight.bold,
          ),
        ),
        const SizedBox(height: 12),
        if (_isLoadingDeliveryOptions)
          const Center(
            child: Padding(
              padding: EdgeInsets.all(AppMetrics.p24),
              child: CircularProgressIndicator(),
            ),
          )
        else if (_deliveryOptions.isEmpty && _selectedAddress != null)
          _buildNoDeliveryOptionsState()
        else
          ..._deliveryOptions.map((option) => _buildDeliveryOptionCard(option)),
      ],
    );
  }

  Widget _buildDiscountField() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Kode Promo',
          style: context.typeRoles.titleSection.copyWith(
            fontWeight: FontWeight.bold,
          ),
        ),
        const SizedBox(height: 12),
        TextField(
          controller: _discountController,
          textCapitalization: TextCapitalization.characters,
          decoration: const InputDecoration(labelText: 'Kode promo (opsional)'),
        ),
      ],
    );
  }

  Widget _buildCoinToggle() {
    final coinState = ref.watch(coinProvider);
    final coinBalance = coinState.maybeWhen(
      balanceLoaded: (balance, _) => balance,
      orElse: () => 0,
    );
    if (coinBalance <= 0) return const SizedBox.shrink();
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p12,
        vertical: AppMetrics.p12,
      ),
      decoration: BoxDecoration(
        border: Border.all(color: scheme.outlineVariant),
        borderRadius: BorderRadius.circular(AppShape.r8),
      ),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Gunakan Coins',
                  style: context.typeRoles.titleCompact.copyWith(
                    fontWeight: FontWeight.w600,
                  ),
                ),
                Text(
                  'Saldo: $coinBalance coins',
                  style: context.typeRoles.labelMicro.copyWith(
                    color: scheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ),
          Switch(
            value: _useCoins,
            onChanged: _isClaiming
                ? null
                : (val) => setState(() => _useCoins = val),
          ),
        ],
      ),
    );
  }

  Widget _buildNoDeliveryOptionsState() {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: context.statusColors.warning.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(AppShape.r8),
        border: Border.all(
          color: context.statusColors.warning.withValues(alpha: 0.3),
        ),
      ),
      child: Row(
        children: [
          Icon(
            Icons.warning_amber_outlined,
            color: context.statusColors.warning,
            size: AppIconSize.action,
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Tidak ada opsi pengiriman',
                  style: context.typeRoles.bodyDense.copyWith(
                    fontWeight: FontWeight.w600,
                    color: scheme.onSurface,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  'Penjual belum menyediakan opsi pengiriman ke lokasi Anda.',
                  style: context.typeRoles.labelMicro.copyWith(
                    color: scheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildQuoteNotice() {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: context.statusColors.success.withValues(alpha: 0.10),
        borderRadius: BorderRadius.circular(AppShape.r8),
        border: Border.all(
          color: context.statusColors.success.withValues(alpha: 0.3),
        ),
      ),
      child: Row(
        children: [
          Icon(
            Icons.local_shipping_outlined,
            color: context.statusColors.success,
            size: AppIconSize.action,
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              'Ongkir dari penawaran penjual akan digunakan untuk pesanan ini.',
              style: context.typeRoles.bodyDense.copyWith(
                color: scheme.onSurface,
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildDeliveryOptionCard(DeliveryOption option) {
    final isSelected =
        _selectedDeliveryOption?.shippingSetupId == option.shippingSetupId;
    final scheme = Theme.of(context).colorScheme;

    // Legitimate custom visual (card selection), but it is still a single
    // selection control: a screen reader must hear the selected state and that
    // the options are mutually exclusive.
    return Semantics(
      selected: isSelected,
      inMutuallyExclusiveGroup: true,
      button: true,
      child: GestureDetector(
        onTap: () => _onDeliveryOptionSelected(option),
        child: Container(
          margin: const EdgeInsets.only(bottom: AppMetrics.p8),
          padding: const EdgeInsets.all(AppMetrics.p12),
          decoration: BoxDecoration(
            color: isSelected
                ? scheme.primary.withValues(alpha: 0.08)
                : scheme.surface,
            borderRadius: BorderRadius.circular(AppShape.r8),
            border: Border.all(
              color: isSelected ? scheme.primary : scheme.outlineVariant,
              width: isSelected ? 2 : 1,
            ),
          ),
          child: Row(
            children: [
              // Radio indicator
              Container(
                width: AppIconSize.action,
                height: AppIconSize.action,
                decoration: BoxDecoration(
                  shape: BoxShape.circle,
                  border: Border.all(
                    color: isSelected ? scheme.primary : scheme.outline,
                    width: 2,
                  ),
                  color: isSelected ? scheme.primary : Colors.transparent,
                ),
                child: isSelected
                    ? Icon(
                        Icons.check,
                        size: AppIconSize.inlineGlyph,
                        color: scheme.onPrimary,
                      )
                    : null,
              ),
              const SizedBox(width: 12),
              // Option details
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      option.displayName,
                      style: context.typeRoles.titleCompact.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ],
                ),
              ),
              // Price
              Text(
                AppFormatters.formatCurrency(option.rate),
                style: context.typeRoles.titleCompact.copyWith(
                  fontWeight: FontWeight.bold,
                  color: scheme.primary,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildBottomBar(BuildContext context) {
    final canClaim =
        _selectedAddress != null &&
        (_hasQuote || _selectedDeliveryOption != null) &&
        !_isClaiming;

    // Chrome owned by [BottomActionBar]; this method only decides the
    // claim readiness gate. Both actions share equal flex (the old 2:1
    // claim flex was a second button-size contract).
    return BottomActionBar(
      secondary: BottomBarAction(
        label: 'Batal',
        onPressed: _isClaiming ? null : () => Navigator.of(context).maybePop(),
      ),
      primary: BottomBarAction(
        label: 'Klaim & Lanjutkan',
        onPressed: canClaim ? _handleClaim : null,
        isLoading: _isClaiming,
      ),
    );
  }
}
