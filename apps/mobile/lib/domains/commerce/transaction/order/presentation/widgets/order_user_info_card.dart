part of 'order_widgets_impl.dart';

class OrderUserInfoCard extends ConsumerWidget {
  final Order order;
  final String currentUserId;

  const OrderUserInfoCard({
    super.key,
    required this.order,
    required this.currentUserId,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Determine which user info to show based on current user
    final isSeller = currentUserId == order.sellerId;
    final isBuyer = currentUserId == order.buyerId;

    // Show the other party's info
    final showSellerInfo = isBuyer;
    final showBuyerInfo = isSeller;

    return OrderSectionCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                isSeller ? 'Info Pembeli' : 'Info Penjual',
                style: Theme.of(
                  context,
                ).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w600),
              ),
              // Chat button for continuity - Contact the other party
              _ChatButton(order: order, currentUserId: currentUserId),
            ],
          ),
          const SizedBox(height: 12),
          if (showSellerInfo)
            _UserInfoTile(
              label: 'Penjual',
              userId: order.sellerId,
              sellerUsername: order.sellerUsername,
              sellerFarmName: order.sellerFarmName,
              sellerAvatarUrl: order.sellerAvatarUrl,
              showSellerIdentity: true,
            ),
          if (showBuyerInfo)
            _UserInfoTile(label: 'Pembeli', userId: order.buyerId),
        ],
      ),
    );
  }
}

class _UserInfoTile extends ConsumerWidget {
  final String label;
  final String userId;
  final String? sellerUsername;
  final String? sellerFarmName;
  final String? sellerAvatarUrl;
  final bool showSellerIdentity;

  const _UserInfoTile({
    required this.label,
    required this.userId,
    this.sellerUsername,
    this.sellerFarmName,
    this.sellerAvatarUrl,
    this.showSellerIdentity = false,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;
    // ONE identity authority: the canonical pairing (store name primary,
    // handle secondary) comes from the identity model.
    final sellerIdentity = showSellerIdentity
        ? SellerIdentityData(
            userId: userId,
            username: sellerUsername,
            storeName: sellerFarmName,
            avatarUrl: sellerAvatarUrl,
          )
        : null;
    final sellerPrimaryLabel = sellerIdentity?.primaryLabel;
    final sellerSecondaryLabel = sellerIdentity?.secondaryLabel;

    return Row(
      children: [
        Container(
          width: 40,
          height: 40,
          decoration: BoxDecoration(
            color: colorScheme.primary.withValues(alpha: 0.1),
            shape: BoxShape.circle,
          ),
          child: sellerAvatarUrl != null && sellerAvatarUrl!.isNotEmpty
              ? ClipOval(
                  child: AppImage(
                    imageUrl: sellerAvatarUrl,
                    fit: BoxFit.cover,
                    isCircle: true,
                    width: 40,
                    height: 40,
                    backgroundColor: colorScheme.primary.withValues(alpha: 0.1),
                    errorWidget: Icon(
                      Icons.person_outline,
                      color: colorScheme.primary,
                      size: AppIconSize.action,
                    ),
                  ),
                )
              : Icon(
                  Icons.person_outline,
                  color: colorScheme.primary,
                  size: AppIconSize.action,
                ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Text(
                    label,
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: colorScheme.onSurfaceVariant,
                    ),
                  ),
                  const SizedBox(width: 8),
                ],
              ),
              if (sellerPrimaryLabel != null) ...[
                Text(
                  sellerPrimaryLabel,
                  style: context.typeRoles.titleCompact.copyWith(
                    fontWeight: FontWeight.w600,
                  ),
                ),
                if (sellerSecondaryLabel != null) ...[
                  const SizedBox(height: 2),
                  Text(
                    sellerSecondaryLabel,
                    style: context.typeRoles.labelMicro,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
              ] else if (!showSellerIdentity)
                Text(
                  userId.length > 20 ? '${userId.substring(0, 20)}...' : userId,
                  style: context.typeRoles.labelMicro.copyWith(
                    fontFamily: 'monospace',
                  ),
                ),
            ],
          ),
        ),
        Icon(
          Icons.chevron_right,
          color: colorScheme.onSurfaceVariant,
          size: AppIconSize.action,
        ),
      ],
    );
  }
}

/// Chat Button - Opens chat with the other party in the order
///
/// This widget provides the critical ORDER -> CHAT continuity:
/// - Buyer can message seller about their order
/// - Seller can message buyer about shipping, payment, etc.
class _ChatButton extends ConsumerWidget {
  final Order order;
  final String currentUserId;

  const _ChatButton({required this.order, required this.currentUserId});

  Future<void> _handleChatTap(BuildContext context, WidgetRef ref) async {
    // Check email verification before starting chat
    final isEmailVerified = ref.read(isEmailVerifiedProvider);
    if (!isEmailVerified) {
      AppSnackBar.showWarning(
        context,
        'Verifikasi email Anda untuk mengirim pesan.',
      );
      return;
    }

    // Canonical Order → commerce chat entry point (same authority as every
    // other Order chat affordance: order-linked room + `/chat/<room-id>`).
    await openOrderCommerceChat(
      context: context,
      ref: ref,
      order: order,
      currentUserId: currentUserId,
    );
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colorScheme = Theme.of(context).colorScheme;
    return InkWell(
      onTap: () => _handleChatTap(context, ref),
      borderRadius: BorderRadius.circular(core.AppShape.r8),
      child: Container(
        padding: const EdgeInsets.symmetric(
          horizontal: core.AppMetrics.p12,
          vertical: core.AppMetrics.p8,
        ),
        decoration: BoxDecoration(
          color: colorScheme.primary.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(core.AppShape.r8),
          border: Border.all(color: colorScheme.primary.withValues(alpha: 0.3)),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.chat_bubble_outline,
              size: AppIconSize.inlineGlyph,
              color: colorScheme.primary,
            ),
            const SizedBox(width: 4),
            Text(
              'Chat',
              style: Theme.of(context).textTheme.labelLarge?.copyWith(
                fontWeight: FontWeight.w500,
                color: colorScheme.primary,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// =============================================================================
// OrderItemsCard - Order Items Card
// =============================================================================
