import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_state.dart';

/// Create Content Bottom Sheet - Modal for opening the universal content composer
///
/// **SELLER UX ALIGNMENT:**
/// - Tri-state identity/capability model with a neutral unknown state
/// - UI reflects honest backend state
/// - No fake affordance - disabled buttons show why
///
/// Features:
/// - Vertical list layout (not grid)
/// - Universal content composer entry for all users
/// - State-based options (Seller-only for Listing, Auction)
/// - Close via: drag down, tap outside, or back button
/// - No cancel button needed
///
/// **STATE MAPPING:**
/// - unknown: neutral loading/pending row
/// - nonSeller: "Mulai Jualan" CTA
/// - seller + active: "Jual Koi (Listing)", "Lelang (Auction)" enabled
/// - seller + inactive: "Jual Koi (Listing)", "Lelang (Auction)" disabled + "Perpanjang Langganan" CTA
class CreateContentBottomSheet extends StatelessWidget {
  final VoidCallback onCreateContent;
  final VoidCallback? onCreateListing;
  final VoidCallback? onCreateAuction;
  final VoidCallback? onStartSelling;
  final VoidCallback? onRenewSubscription;
  final SellerIdentityStatus sellerIdentityStatus;
  final SellerCapabilityStatus sellerCapabilityStatus;

  const CreateContentBottomSheet({
    super.key,
    required this.onCreateContent,
    this.onCreateListing,
    this.onCreateAuction,
    this.onStartSelling,
    this.onRenewSubscription,
    required this.sellerIdentityStatus,
    required this.sellerCapabilityStatus,
  });

  /// Show the bottom sheet
  static void show({
    required BuildContext context,
    required VoidCallback onCreateContent,
    VoidCallback? onCreateListing,
    VoidCallback? onCreateAuction,
    VoidCallback? onStartSelling,
    VoidCallback? onRenewSubscription,
    required SellerIdentityStatus sellerIdentityStatus,
    required SellerCapabilityStatus sellerCapabilityStatus,
  }) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      enableDrag: true,
      isDismissible: true,
      builder: (context) => CreateContentBottomSheet(
        onCreateContent: onCreateContent,
        onCreateListing: onCreateListing,
        onCreateAuction: onCreateAuction,
        onStartSelling: onStartSelling,
        onRenewSubscription: onRenewSubscription,
        sellerIdentityStatus: sellerIdentityStatus,
        sellerCapabilityStatus: sellerCapabilityStatus,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(20)),
      ),
      child: SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Drag handle
            Container(
              width: 40,
              height: 4,
              margin: const EdgeInsets.only(top: 12, bottom: 8),
              decoration: BoxDecoration(
                color: isDark
                    ? AppColors.neutralGray600
                    : AppColors.neutralGray300,
                borderRadius: BorderRadius.circular(2),
              ),
            ),

            // Header
            Padding(
              padding: const EdgeInsets.fromLTRB(20, 8, 20, 16),
              child: Text(
                'Create',
                style: TextStyle(
                  fontSize: 20,
                  fontWeight: FontWeight.w600,
                  color: isDark
                      ? AppColors.neutralWhite
                      : AppColors.neutralGray900,
                ),
              ),
            ),

            // Options list - Vertical layout
            _buildOptionsList(context, isDark),

            const SizedBox(height: 16),
          ],
        ),
      ),
    );
  }

  Widget _buildOptionsList(BuildContext context, bool isDark) {
    final isIdentityUnknown =
        sellerIdentityStatus == SellerIdentityStatus.unknown;
    final isCapabilityUnknown =
        sellerCapabilityStatus == SellerCapabilityStatus.unknown;
    final isConfirmedSeller =
        sellerIdentityStatus == SellerIdentityStatus.seller;
    final isConfirmedNonSeller =
        sellerIdentityStatus == SellerIdentityStatus.nonSeller;
    final isActiveSeller =
        isConfirmedSeller &&
        sellerCapabilityStatus == SellerCapabilityStatus.active;
    final isInactiveSeller =
        isConfirmedSeller &&
        sellerCapabilityStatus == SellerCapabilityStatus.inactive;

    final options = <_CreateOption>[
      // Always visible option for ALL users
      _CreateOption(
        icon: Icons.post_add_outlined,
        label: 'Buat Konten',
        description: 'Cerita, showcase, atau update',
        color: AppColors.primaryRed,
        onTap: () {
          Navigator.pop(context);
          onCreateContent();
        },
      ),

      // UNKNOWN: stay neutral while the account snapshot is still resolving.
      if (isIdentityUnknown || isCapabilityUnknown)
        _CreateOption(
          icon: Icons.hourglass_top_outlined,
          label: 'Checking seller status',
          description:
              'Seller tools will appear once your account data is ready.',
          color: AppColors.neutralGray400,
          onTap: null,
        ),

      // NOT_SELLER: Show "Mulai Jualan" CTA
      if (isConfirmedNonSeller && onStartSelling != null)
        _CreateOption(
          icon: Icons.store_outlined,
          label: 'Mulai Jualan',
          description: 'Mulai jualan koi di LABUDA',
          color: AppColors.primaryGreen,
          onTap: () {
            Navigator.pop(context);
            onStartSelling!();
          },
      ),

      // ACTIVE: Show enabled listing/auction options
      if (isActiveSeller) ...[
        _CreateOption(
          icon: Icons.store_outlined,
          label: 'Jual Koi (Listing)',
          description: 'Jual koi langsung atau tawar harga',
          color: AppColors.primaryGreen,
          onTap: onCreateListing != null
              ? () {
                  Navigator.pop(context);
                  onCreateListing!();
                }
              : null,
        ),
        _CreateOption(
          icon: Icons.gavel_outlined,
          label: 'Lelang (Auction)',
          description: 'Buat lelang dari mobile',
          color: AppColors.statusWarning,
          onTap: onCreateAuction != null
              ? () {
                  Navigator.pop(context);
                  onCreateAuction!();
                }
              : null,
        ),
      ],

      // EXPIRED: Show disabled listing/auction options with "Perpanjang Langganan" CTA
      if (isInactiveSeller) ...[
        _CreateOption(
          icon: Icons.store_outlined,
          label: 'Jual Koi (Listing)',
          description: 'Langganan berakhir - perpanjang untuk menjual',
          color: AppColors.neutralGray400,
          onTap: null, // Disabled - subscription expired
        ),
        _CreateOption(
          icon: Icons.gavel_outlined,
          label: 'Lelang (Auction)',
          description: 'Langganan berakhir - perpanjang untuk lelang',
          color: AppColors.neutralGray400,
          onTap: null, // Disabled - subscription expired
        ),
        if (onRenewSubscription != null)
          _CreateOption(
            icon: Icons.refresh_outlined,
            label: 'Perpanjang Langganan',
            description: 'Perbarui langganan untuk jual dan lelang koi',
            color: AppColors.primaryRed,
            onTap: () {
              Navigator.pop(context);
              onRenewSubscription!();
            },
          ),
      ],
    ];

    return ListView.separated(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      padding: const EdgeInsets.symmetric(horizontal: 8),
      itemCount: options.length,
      separatorBuilder: (context, index) => const SizedBox(height: 4),
      itemBuilder: (context, index) {
        final option = options[index];
        return _buildOptionItem(option, isDark);
      },
    );
  }

  Widget _buildOptionItem(_CreateOption option, bool isDark) {
    final isEnabled = option.onTap != null;

    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: option.onTap,
        borderRadius: BorderRadius.circular(12),
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          child: Row(
            children: [
              // Icon with colored background
              Container(
                width: 48,
                height: 48,
                decoration: BoxDecoration(
                  color: isEnabled
                      ? option.color.withValues(alpha: 0.1)
                      : (isDark
                            ? AppColors.neutralGray700
                            : AppColors.neutralGray200),
                  borderRadius: BorderRadius.circular(12),
                ),
                child: Icon(
                  option.icon,
                  color: isEnabled
                      ? option.color
                      : (isDark
                            ? AppColors.neutralGray500
                            : AppColors.neutralGray400),
                  size: 24,
                ),
              ),

              const SizedBox(width: 16),

              // Label and description
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      option.label,
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w600,
                        color: isEnabled
                            ? (isDark
                                  ? AppColors.neutralWhite
                                  : AppColors.neutralGray900)
                            : (isDark
                                  ? AppColors.neutralGray500
                                  : AppColors.neutralGray400),
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      option.description,
                      style: TextStyle(
                        fontSize: 13,
                        color: isDark
                            ? AppColors.neutralGray400
                            : AppColors.neutralGray600,
                      ),
                    ),
                  ],
                ),
              ),

              // Arrow indicator
              if (isEnabled)
                Icon(
                  Icons.arrow_forward_ios,
                  size: 16,
                  color: isDark
                      ? AppColors.neutralGray500
                      : AppColors.neutralGray400,
                ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Model for create option
class _CreateOption {
  final IconData icon;
  final String label;
  final String description;
  final Color color;
  final VoidCallback? onTap;

  const _CreateOption({
    required this.icon,
    required this.label,
    required this.description,
    required this.color,
    this.onTap,
  });
}
