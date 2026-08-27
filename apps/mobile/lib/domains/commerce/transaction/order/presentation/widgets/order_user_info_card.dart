part of 'order_widgets_impl.dart';

class OrderUserInfoCard extends ConsumerWidget {
  final String currentUserId;
  final String sellerId;
  final String buyerId;
  final String? sellerUsername;
  final String? sellerFarmName;
  final String? sellerAvatarUrl;
  final bool isDark;

  const OrderUserInfoCard({
    super.key,
    required this.currentUserId,
    required this.sellerId,
    required this.buyerId,
    this.sellerUsername,
    this.sellerFarmName,
    this.sellerAvatarUrl,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Determine which user info to show based on current user
    final isSeller = currentUserId == sellerId;
    final isBuyer = currentUserId == buyerId;

    // Show the other party's info
    final showSellerInfo = isBuyer;
    final showBuyerInfo = isSeller;

    // Determine the other party's ID for chat
    final otherPartyId = isSeller ? buyerId : sellerId;

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark ? const Color(0xFF1E1E1E) : Colors.white,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isDark ? const Color(0xFF333333) : const Color(0xFFE0E0E0),
        ),
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
                otherPartyId: otherPartyId,
                currentUserId: currentUserId,
                isDark: isDark,
              ),
            ],
          ),
          const SizedBox(height: 12),
          if (showSellerInfo)
            _UserInfoTile(
              label: 'Penjual',
              userId: sellerId,
              sellerUsername: sellerUsername,
              sellerFarmName: sellerFarmName,
              sellerAvatarUrl: sellerAvatarUrl,
              showSellerIdentity: true,
              isDark: isDark,
            ),
          if (showBuyerInfo)
            _UserInfoTile(label: 'Pembeli', userId: buyerId, isDark: isDark),
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
  final bool isDark;

  const _UserInfoTile({
    required this.label,
    required this.userId,
    this.sellerUsername,
    this.sellerFarmName,
    this.sellerAvatarUrl,
    this.showSellerIdentity = false,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
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
            color: core.AppColors.primaryRed.withValues(alpha: 0.1),
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
                        color: core.AppColors.primaryRed,
                        size: 20,
                      );
                    },
                  ),
                )
              : Icon(
                  Icons.person_outline,
                  color: core.AppColors.primaryRed,
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
                      color: Colors.grey,
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
        Icon(Icons.chevron_right, color: Colors.grey, size: 20),
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
  final String otherPartyId;
  final String currentUserId;
  final bool isDark;

  const _ChatButton({
    required this.otherPartyId,
    required this.currentUserId,
    required this.isDark,
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

    // Show loading
    showDialog(
      context: context,
      barrierDismissible: false,
      builder: (_) => const Center(child: CircularProgressIndicator()),
    );

    try {
      // Get or create chat using chat repository
      final chatRepository = ref.read(chatRepositoryProvider);
      final result = await chatRepository.getOrCreateChat(
        participantIds: [currentUserId, otherPartyId],
      );

      if (!context.mounted) return;

      // Close loading
      Navigator.of(context).pop();

      if (result.isSuccess && result.data != null) {
        final chat = result.data!;
        // Navigate to chat screen
        context.push('/chat/${chat.id}');
      } else {
        // Show error - check if user is blocked
        final error = result.error ?? 'Failed to open chat';
        final errorMessage = error.toLowerCase().contains('blocked')
            ? 'Tidak dapat mengirim pesan. Pengguna ini telah memblokir Anda.'
            : error;
        AppSnackBar.showError(context, errorMessage);
      }
    } catch (e) {
      if (!context.mounted) return;

      // Close loading
      Navigator.of(context).pop();

      // Show error
      AppSnackBar.showError(context, 'Terjadi kesalahan. Coba lagi.');
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return InkWell(
      onTap: () => _handleChatTap(context, ref),
      borderRadius: BorderRadius.circular(8),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
        decoration: BoxDecoration(
          color: isDark
              ? core.AppColors.primaryRed.withValues(alpha: 0.2)
              : core.AppColors.primaryRed.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(8),
          border: Border.all(
            color: isDark
                ? core.AppColors.primaryRed.withValues(alpha: 0.5)
                : core.AppColors.primaryRed.withValues(alpha: 0.3),
          ),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.chat_bubble_outline,
              size: 16,
              color: isDark
                  ? core.AppColors.primaryRed.withValues(alpha: 0.9)
                  : core.AppColors.primaryRed,
            ),
            const SizedBox(width: 4),
            Text(
              'Chat',
              style: TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w500,
                color: isDark
                    ? core.AppColors.primaryRed.withValues(alpha: 0.9)
                    : core.AppColors.primaryRed,
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
