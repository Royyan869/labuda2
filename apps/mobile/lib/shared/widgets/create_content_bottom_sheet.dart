import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/widgets/app_bottom_sheet_base.dart';
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
/// - State-based options (Seller-only for ForSale, Auction)
/// - Close via: drag down, tap outside, or back button
/// - No cancel button needed
///
/// **STATE MAPPING:**
/// - unknown: neutral loading/pending row
/// - nonSeller: "Mulai Jualan" CTA
/// - seller + active: "Jual Koi (For Sale)", "Lelang (Auction)" enabled
/// - seller + inactive: "Jual Koi (For Sale)", "Lelang (Auction)" disabled, plus a
///   subscription CTA whose COPY follows the canonical expiry axis:
///   expired subscription → "Perpanjang Langganan" / "Langganan berakhir",
///   not yet active ('none') → "Aktifkan Langganan" / "Langganan belum aktif".
///   Capability `inactive` alone never claims expiry (RF-02).
class CreateContentBottomSheet extends StatelessWidget {
  final VoidCallback onCreateContent;
  final VoidCallback? onCreateForSale;
  final VoidCallback? onCreateAuction;
  final VoidCallback? onStartSelling;
  final VoidCallback? onRenewSubscription;
  final SellerIdentityStatus sellerIdentityStatus;
  final SellerCapabilityStatus sellerCapabilityStatus;

  /// Canonical expiry axis (`sellerSubscriptionStatus == 'expired'`).
  /// Gates the expiry wording only — the capability axis still gates access.
  final bool isSubscriptionExpired;

  const CreateContentBottomSheet({
    super.key,
    required this.onCreateContent,
    this.onCreateForSale,
    this.onCreateAuction,
    this.onStartSelling,
    this.onRenewSubscription,
    required this.sellerIdentityStatus,
    required this.sellerCapabilityStatus,
    this.isSubscriptionExpired = false,
  });

  /// Show the bottom sheet
  static void show({
    required BuildContext context,
    required VoidCallback onCreateContent,
    VoidCallback? onCreateForSale,
    VoidCallback? onCreateAuction,
    VoidCallback? onStartSelling,
    VoidCallback? onRenewSubscription,
    required SellerIdentityStatus sellerIdentityStatus,
    required SellerCapabilityStatus sellerCapabilityStatus,
    bool isSubscriptionExpired = false,
  }) {
    AppBottomSheetBase.show<void>(
      context: context,
      title: 'Create',
      padding: EdgeInsets.zero,
      content: CreateContentBottomSheet(
        onCreateContent: onCreateContent,
        onCreateForSale: onCreateForSale,
        onCreateAuction: onCreateAuction,
        onStartSelling: onStartSelling,
        onRenewSubscription: onRenewSubscription,
        sellerIdentityStatus: sellerIdentityStatus,
        sellerCapabilityStatus: sellerCapabilityStatus,
        isSubscriptionExpired: isSubscriptionExpired,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    // Surface, shape, handle, scroll and safe area come from the base.
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        // Options list - Vertical layout
        _buildOptionsList(context, scheme),

        const SizedBox(height: 16),
      ],
    );
  }

  Widget _buildOptionsList(BuildContext context, ColorScheme scheme) {
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
        color: scheme.primary,
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
          color: scheme.onSurfaceVariant,
          onTap: null,
        ),

      // NOT_SELLER: Show "Mulai Jualan" CTA
      if (isConfirmedNonSeller && onStartSelling != null)
        _CreateOption(
          icon: Icons.store_outlined,
          label: 'Mulai Jualan',
          description: 'Mulai jualan koi di LABUDA',
          color: context.statusColors.success,
          onTap: () {
            Navigator.pop(context);
            onStartSelling!();
          },
        ),

      // ACTIVE: Show enabled forSale/auction options
      if (isActiveSeller) ...[
        _CreateOption(
          icon: Icons.store_outlined,
          label: 'Jual Koi (For Sale)',
          description: 'Jual koi langsung atau tawar harga',
          color: context.statusColors.success,
          onTap: onCreateForSale != null
              ? () {
                  Navigator.pop(context);
                  onCreateForSale!();
                }
              : null,
        ),
        _CreateOption(
          icon: Icons.gavel_outlined,
          label: 'Lelang (Auction)',
          description: 'Buat lelang dari mobile',
          color: context.statusColors.warning,
          onTap: onCreateAuction != null
              ? () {
                  Navigator.pop(context);
                  onCreateAuction!();
                }
              : null,
        ),
      ],

      // NO ACTIVE SUBSCRIPTION: forSale/auction stay disabled. Copy follows the
      // canonical expiry axis — only an ENDED period is "berakhir/perpanjang".
      if (isInactiveSeller) ...[
        _CreateOption(
          icon: Icons.store_outlined,
          label: 'Jual Koi (For Sale)',
          description: isSubscriptionExpired
              ? 'Langganan berakhir - perpanjang untuk menjual'
              : 'Langganan belum aktif - aktifkan untuk menjual',
          color: scheme.onSurfaceVariant,
          onTap: null, // Disabled - no active subscription
        ),
        _CreateOption(
          icon: Icons.gavel_outlined,
          label: 'Lelang (Auction)',
          description: isSubscriptionExpired
              ? 'Langganan berakhir - perpanjang untuk lelang'
              : 'Langganan belum aktif - aktifkan untuk lelang',
          color: scheme.onSurfaceVariant,
          onTap: null, // Disabled - no active subscription
        ),
        if (onRenewSubscription != null)
          _CreateOption(
            icon: Icons.refresh_outlined,
            label: isSubscriptionExpired
                ? 'Perpanjang Langganan'
                : 'Aktifkan Langganan',
            description: isSubscriptionExpired
                ? 'Perbarui langganan untuk jual dan lelang koi'
                : 'Berlangganan untuk mulai jual dan lelang koi',
            color: scheme.primary,
            onTap: () {
              Navigator.pop(context);
              onRenewSubscription!();
            },
          ),
      ],
    ];

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p8),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          for (var i = 0; i < options.length; i++) ...[
            if (i > 0) const SizedBox(height: 4),
            _buildOptionItem(context, options[i], scheme),
          ],
        ],
      ),
    );
  }

  Widget _buildOptionItem(
    BuildContext context,
    _CreateOption option,
    ColorScheme scheme,
  ) {
    final isEnabled = option.onTap != null;

    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: option.onTap,
        borderRadius: BorderRadius.circular(AppShape.r12),
        child: Container(
          padding: const EdgeInsets.symmetric(
            horizontal: AppMetrics.p16,
            vertical: AppMetrics.p12,
          ),
          child: Row(
            children: [
              // Icon with colored background
              Container(
                padding: const EdgeInsets.all(AppMetrics.p12),
                decoration: BoxDecoration(
                  color: isEnabled
                      ? option.color.withValues(alpha: 0.1)
                      : (scheme.onSurfaceVariant),
                  borderRadius: BorderRadius.circular(AppShape.r12),
                ),
                child: Icon(
                  option.icon,
                  color: isEnabled ? option.color : (scheme.onSurfaceVariant),
                  size: AppIconSize.header,
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
                      style: context.typeRoles.titleCompact.copyWith(
                        fontWeight: FontWeight.w600,
                        color: isEnabled
                            ? (scheme.onSurfaceVariant)
                            : (scheme.onSurfaceVariant),
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      option.description,
                      style: context.typeRoles.bodyDense.copyWith(
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                  ],
                ),
              ),

              // Arrow indicator
              if (isEnabled)
                Icon(
                  Icons.arrow_forward_ios,
                  size: AppIconSize.inlineGlyph,
                  color: scheme.onSurfaceVariant,
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
