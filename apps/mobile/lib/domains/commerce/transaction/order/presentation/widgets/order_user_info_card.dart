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

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: Theme.of(context).colorScheme.outlineVariant),
      ),
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
              _ChatButton(
                order: order,
                currentUserId: currentUserId,
              ),
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
            _UserInfoTile(
              label: 'Pembeli',
              userId: order.buyerId,
            ),
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
    final sellerIdentity = showSellerIdentity
        ? buildCommerceSellerIdentity(
            username: sellerUsername,
            storeName: sellerFarmName,
          )
        : null;

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
                  child: Image.network(
                    sellerAvatarUrl!,
                    fit: BoxFit.cover,
                    errorBuilder: (context, error, stackTrace) {
                      return Icon(
                        Icons.person_outline,
                        color: colorScheme.primary,
                        size: 20,
                      );
                    },
                  ),
                )
              : Icon(
                  Icons.person_outline,
                  color: colorScheme.primary,
                  size: 20,
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
              if (sellerIdentity != null) ...[
                Text(
                  sellerIdentity.line1,
                  style: theme.textTheme.bodyMedium?.copyWith(
                    fontWeight: FontWeight.w600,
                    fontSize: 12,
                  ),
                ),
                if (sellerIdentity.line2 != null) ...[
                  const SizedBox(height: 2),
                  Text(
                    sellerIdentity.line2!,
                    style: theme.textTheme.bodyMedium?.copyWith(fontSize: 12),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
              ] else if (!showSellerIdentity)
                Text(
                  userId.length > 20 ? '${userId.substring(0, 20)}...' : userId,
                  style: theme.textTheme.bodyMedium?.copyWith(
                    fontFamily: 'monospace',
                    fontSize: 12,
                  ),
                ),
            ],
          ),
        ),
        Icon(Icons.chevron_right, color: colorScheme.onSurfaceVariant, size: 20),
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

  const _ChatButton({
    required this.order,
    required this.currentUserId,
  });

  Future<void> _handleChatTap(BuildContext context, WidgetRef ref) async {
    // Check email verification before starting chat
    final isEmailVerified = ref.read(isEmailVerifiedProvider);
    if (!isEmailVerified) {
      AppSnackBar.showWarning(
        context,
        'Please verify your email to send messages.',
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
      borderRadius: BorderRadius.circular(8),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
        decoration: BoxDecoration(
          color: colorScheme.primary.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(8),
          border: Border.all(
            color: colorScheme.primary.withValues(alpha: 0.3),
          ),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.chat_bubble_outline,
              size: 16,
              color: colorScheme.primary,
            ),
            const SizedBox(width: 4),
            Text(
              'Chat',
              style: TextStyle(
                fontSize: 13,
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
