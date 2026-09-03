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

/// Callback type for claim action with selected shipping details
typedef ClaimCallback =
    Future<String?> Function({
      required String addressId,
      required String shippingSetupId,
      String? discountCode,
      bool useCoins,
    });

/// Auction Claim Shipping Modal
///
/// Shows address and delivery option selection for auction claim flow.
class AuctionClaimShippingModal extends ConsumerStatefulWidget {
  final Auction auction;
  final ClaimCallback onClaim;

  const AuctionClaimShippingModal({
    super.key,
    required this.auction,
    required this.onClaim,
  });

  /// Show the modal and return order_id on success, null on cancel/failure
  static Future<String?> show({
    required BuildContext context,
    required Auction auction,
    required ClaimCallback onClaim,
  }) {
    return showModalBottomSheet<String>(
      context: context,
      isScrollControlled: true,
      isDismissible: false,
      backgroundColor: Colors.transparent,
      builder: (context) =>
          AuctionClaimShippingModal(auction: auction, onClaim: onClaim),
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
      final authService = ref.read(authServiceProvider);
      final userResult = await authService.getCurrentUser();

      if (userResult.isError || userResult.data == null) {
        setState(() {
          _isLoadingAddresses = false;
          _error = 'Gagal memuat alamat. Silakan coba lagi.';
        });
        return;
      }

      final user = userResult.data!;
      final addressRepository = ref.read(addressRepositoryProvider);
      final addressesResult = await addressRepository.getAddressesByPurpose(
        user.id,
        AddressPurpose.shipping,
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
        provinceId: _selectedAddress!.province.id,
        cityId: _selectedAddress!.city.id,
        cityName: _selectedAddress!.city.name,
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
    // Validation
    if (_selectedAddress == null) {
      setState(() {
        _error = 'Pilih alamat pengiriman terlebih dahulu';
      });
      return;
    }

    if (_selectedDeliveryOption == null) {
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
      final shippingSetupId = _selectedDeliveryOption!.shippingSetupId;

      final orderId = await widget.onClaim(
        addressId: _selectedAddress!.id,
        shippingSetupId: shippingSetupId,
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

  @override
  Widget build(BuildContext context) {
    final bottomPadding = MediaQuery.of(context).viewInsets.bottom;

    return Container(
      decoration: BoxDecoration(
        color: AppColors.neutralWhite,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(20)),
      ),
      constraints: BoxConstraints(
        maxHeight: MediaQuery.of(context).size.height * 0.85,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          // Header
          _buildHeader(context),

          // Content (scrollable)
          Expanded(
            child: SingleChildScrollView(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // Error banner
                  if (_error != null) _buildErrorBanner(),

                  // Address selection
                  _buildAddressSection(),

                  const SizedBox(height: 24),

                  // Delivery options
                  _buildDeliveryOptionsSection(),

                  const SizedBox(height: 20),

                  _buildDiscountField(),

                  const SizedBox(height: 20),

                  _buildCoinToggle(),

                  // Bottom padding for keyboard
                  SizedBox(height: bottomPadding > 0 ? bottomPadding : 16),
                ],
              ),
            ),
          ),

          // Bottom action bar
          _buildBottomBar(context),
        ],
      ),
    );
  }

  Widget _buildHeader(BuildContext context) {
    return Container(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
      decoration: BoxDecoration(
        border: Border(
          bottom: BorderSide(color: AppColors.neutralGray200, width: 1),
        ),
      ),
      child: Column(
        children: [
          // Drag handle
          Container(
            width: 40,
            height: 4,
            decoration: BoxDecoration(
              color: AppColors.neutralGray300,
              borderRadius: BorderRadius.circular(2),
            ),
          ),
          const SizedBox(height: 16),
          // Title
          Row(
            children: [
              Expanded(
                child: Text(
                  'Pilih Pengiriman',
                  style: Theme.of(
                    context,
                  ).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.bold),
                ),
              ),
              // Close button
              if (!_isClaiming)
                IconButton(
                  onPressed: () => Navigator.of(context).pop(),
                  icon: const Icon(Icons.close),
                  padding: EdgeInsets.zero,
                  constraints: const BoxConstraints(),
                ),
            ],
          ),
          const SizedBox(height: 8),
          // Subtitle
          Text(
            'Lengkapi alamat dan pilih opsi pengiriman untuk melanjutkan klaim.',
            style: Theme.of(
              context,
            ).textTheme.bodyMedium?.copyWith(color: AppColors.neutralGray600),
          ),
        ],
      ),
    );
  }

  Widget _buildErrorBanner() {
    return Container(
      margin: const EdgeInsets.only(bottom: 16),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: AppColors.statusError.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: AppColors.statusError.withValues(alpha: 0.3)),
      ),
      child: Row(
        children: [
          Icon(Icons.error_outline, color: AppColors.statusError, size: 20),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              _error!,
              style: TextStyle(color: AppColors.statusError, fontSize: 12),
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
          style: TextStyle(
            fontSize: 16,
            fontWeight: FontWeight.bold,
            color: AppColors.neutralGray900,
          ),
        ),
        const SizedBox(height: 12),
        if (_isLoadingAddresses)
          const Center(
            child: Padding(
              padding: EdgeInsets.all(24),
              child: CircularProgressIndicator(),
            ),
          )
        else if (_addresses.isEmpty)
          _buildEmptyAddressState()
        else
          ..._addresses.map((address) => _buildAddressCard(address)),
      ],
    );
  }

  Widget _buildEmptyAddressState() {
    return Container(
      padding: const EdgeInsets.all(24),
      decoration: BoxDecoration(
        color: AppColors.neutralGray50,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: AppColors.neutralGray200),
      ),
      child: Column(
        children: [
          Icon(
            Icons.location_on_outlined,
            size: 40,
            color: AppColors.neutralGray400,
          ),
          const SizedBox(height: 12),
          Text(
            'Belum ada alamat pengiriman',
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w600,
              color: AppColors.neutralGray700,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            'Tambahkan alamat untuk melanjutkan',
            style: TextStyle(fontSize: 12, color: AppColors.neutralGray600),
          ),
          const SizedBox(height: 16),
          OutlinedButton.icon(
            onPressed: () {
              // Navigate to address creation
              Navigator.of(context).pop();
              // TODO: Navigate to address creation screen
              AppSnackBar.showInfo(
                context,
                'Silakan tambahkan alamat terlebih dahulu',
              );
            },
            icon: const Icon(Icons.add, size: 16),
            label: const Text('Tambah Alamat'),
          ),
        ],
      ),
    );
  }

  Widget _buildAddressCard(AddressEntity address) {
    final isSelected = _selectedAddress?.id == address.id;

    return GestureDetector(
      onTap: () => _onAddressSelected(address),
      child: Container(
        margin: const EdgeInsets.only(bottom: 8),
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: isSelected
              ? AppColors.primaryBlue.withValues(alpha: 0.05)
              : AppColors.neutralWhite,
          borderRadius: BorderRadius.circular(8),
          border: Border.all(
            color: isSelected
                ? AppColors.primaryBlue
                : AppColors.neutralGray200,
            width: isSelected ? 2 : 1,
          ),
        ),
        child: Row(
          children: [
            // Radio indicator
            Container(
              width: 20,
              height: 20,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                border: Border.all(
                  color: isSelected
                      ? AppColors.primaryBlue
                      : AppColors.neutralGray400,
                  width: 2,
                ),
                color: isSelected ? AppColors.primaryBlue : Colors.transparent,
              ),
              child: isSelected
                  ? const Icon(Icons.check, size: 12, color: Colors.white)
                  : null,
            ),
            const SizedBox(width: 12),
            // Address details
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  if (address.nickname != null)
                    Text(
                      address.nickname!,
                      style: TextStyle(
                        fontSize: 12,
                        fontWeight: FontWeight.w600,
                        color: AppColors.primaryBlue,
                      ),
                    ),
                  Text(
                    address.recipientName,
                    style: TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.w600,
                      color: AppColors.neutralGray900,
                    ),
                  ),
                  const SizedBox(height: 2),
                  Text(
                    address.phone,
                    style: TextStyle(
                      fontSize: 12,
                      color: AppColors.neutralGray600,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    address.fullAddress,
                    style: TextStyle(
                      fontSize: 12,
                      color: AppColors.neutralGray700,
                    ),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
              ),
            ),
            // Primary badge
            if (address.isPrimary)
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                decoration: BoxDecoration(
                  color: AppColors.successGreen.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(4),
                ),
                child: Text(
                  'Utama',
                  style: TextStyle(
                    fontSize: 10,
                    fontWeight: FontWeight.w600,
                    color: AppColors.successGreen,
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildDeliveryOptionsSection() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Opsi Pengiriman',
          style: TextStyle(
            fontSize: 16,
            fontWeight: FontWeight.bold,
            color: AppColors.neutralGray900,
          ),
        ),
        const SizedBox(height: 12),
        if (_isLoadingDeliveryOptions)
          const Center(
            child: Padding(
              padding: EdgeInsets.all(24),
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
          style: TextStyle(
            fontSize: 16,
            fontWeight: FontWeight.bold,
            color: AppColors.neutralGray900,
          ),
        ),
        const SizedBox(height: 12),
        TextField(
          controller: _discountController,
          textCapitalization: TextCapitalization.characters,
          decoration: const InputDecoration(
            labelText: 'Kode promo (opsional)',
            border: OutlineInputBorder(),
          ),
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
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        border: Border.all(color: AppColors.neutralGray200),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Gunakan Coins',
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: AppColors.neutralGray900,
                  ),
                ),
                Text(
                  'Saldo: $coinBalance coins',
                  style: TextStyle(
                    fontSize: 12,
                    color: AppColors.neutralGray600,
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
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: AppColors.statusWarning.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(
          color: AppColors.statusWarning.withValues(alpha: 0.3),
        ),
      ),
      child: Row(
        children: [
          Icon(
            Icons.warning_amber_outlined,
            color: AppColors.statusWarning,
            size: 20,
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Tidak ada opsi pengiriman',
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: AppColors.statusWarning,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  'Penjual belum menyediakan opsi pengiriman ke lokasi Anda.',
                  style: TextStyle(
                    fontSize: 11,
                    color: AppColors.neutralGray700,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildDeliveryOptionCard(DeliveryOption option) {
    final isSelected =
        _selectedDeliveryOption?.shippingSetupId == option.shippingSetupId;

    return GestureDetector(
      onTap: () => _onDeliveryOptionSelected(option),
      child: Container(
        margin: const EdgeInsets.only(bottom: 8),
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: isSelected
              ? AppColors.primaryRed.withValues(alpha: 0.05)
              : AppColors.neutralWhite,
          borderRadius: BorderRadius.circular(8),
          border: Border.all(
            color: isSelected ? AppColors.primaryRed : AppColors.neutralGray200,
            width: isSelected ? 2 : 1,
          ),
        ),
        child: Row(
          children: [
            // Radio indicator
            Container(
              width: 20,
              height: 20,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                border: Border.all(
                  color: isSelected
                      ? AppColors.primaryRed
                      : AppColors.neutralGray400,
                  width: 2,
                ),
                color: isSelected ? AppColors.primaryRed : Colors.transparent,
              ),
              child: isSelected
                  ? const Icon(Icons.check, size: 12, color: Colors.white)
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
                    style: TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.w600,
                      color: AppColors.neutralGray900,
                    ),
                  ),
                  if (option.notes != null)
                    Text(
                      option.notes!,
                      style: TextStyle(
                        fontSize: 11,
                        color: AppColors.neutralGray600,
                      ),
                    ),
                ],
              ),
            ),
            // Price
            Text(
              AppFormatters.formatCurrency(option.rate),
              style: TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.bold,
                color: AppColors.primaryRed,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildBottomBar(BuildContext context) {
    final canClaim =
        _selectedAddress != null &&
        _selectedDeliveryOption != null &&
        !_isClaiming;

    return Container(
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 16),
      decoration: BoxDecoration(
        color: AppColors.neutralWhite,
        border: Border(
          top: BorderSide(color: AppColors.neutralGray200, width: 1),
        ),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.05),
            blurRadius: 10,
            offset: const Offset(0, -2),
          ),
        ],
      ),
      child: SafeArea(
        top: false,
        child: Row(
          children: [
            // Cancel button
            Expanded(
              child: OutlinedButton(
                onPressed: _isClaiming
                    ? null
                    : () => Navigator.of(context).pop(),
                style: OutlinedButton.styleFrom(
                  padding: const EdgeInsets.symmetric(vertical: 14),
                  side: BorderSide(color: AppColors.neutralGray300),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(8),
                  ),
                ),
                child: const Text('Batal'),
              ),
            ),
            const SizedBox(width: 12),
            // Claim button
            Expanded(
              flex: 2,
              child: ElevatedButton(
                onPressed: canClaim ? _handleClaim : null,
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppColors.primaryRed,
                  foregroundColor: Colors.white,
                  disabledBackgroundColor: AppColors.neutralGray300,
                  padding: const EdgeInsets.symmetric(vertical: 14),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(8),
                  ),
                ),
                child: _isClaiming
                    ? const SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(
                          strokeWidth: 2,
                          valueColor: AlwaysStoppedAnimation<Color>(
                            Colors.white,
                          ),
                        ),
                      )
                    : const Text('Klaim & Lanjutkan'),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
