part of 'order_widgets_impl.dart';

class OrderShippingInfoCard extends StatelessWidget {
  final Order order;
  final bool isDark;

  const OrderShippingInfoCard({
    super.key,
    required this.order,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final shipping = order.shippingInfo;

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
            children: [
              Icon(
                Icons.local_shipping_outlined,
                size: 20,
                color: core.AppColors.primaryBlue,
              ),
              const SizedBox(width: 8),
              Text(
                'Info Pengiriman',
                style: theme.textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.w600,
                ),
              ),
            ],
          ),
          const SizedBox(height: 16),
          // Recipient name
          _ShippingInfoRow(
            icon: Icons.person_outline,
            label: 'Penerima',
            value: shipping.recipientName,
            isDark: isDark,
          ),
          const SizedBox(height: 12),
          // Phone
          _ShippingInfoRow(
            icon: Icons.phone_outlined,
            label: 'Telepon',
            value: shipping.phone,
            isDark: isDark,
          ),
          const SizedBox(height: 12),
          // Address
          _ShippingAddressRow(
            icon: Icons.location_on_outlined,
            label: 'Alamat',
            address: shipping.address,
            cityName: shipping.cityName,
            districtName: shipping.districtName,
            postalCode: shipping.postalCode,
            isDark: isDark,
          ),
          // Shipping method
          if (shipping.courierName != null) ...[
            const SizedBox(height: 12),
            _ShippingInfoRow(
              icon: Icons.delivery_dining_outlined,
              label: 'Kurir',
              value: shipping.courierName!,
              isDark: isDark,
            ),
          ],
          // Tracking number
          if (shipping.trackingNumber != null &&
              shipping.trackingNumber!.isNotEmpty) ...[
            const SizedBox(height: 12),
            _ShippingReferenceRow(shipping: shipping, isDark: isDark),
          ],

          // SHIPPING CONFIRMATION TRUTH: Shipping note from seller
          if (shipping.shippingNote != null &&
              shipping.shippingNote!.isNotEmpty) ...[
            const SizedBox(height: 12),
            _ShippingNoteSection(note: shipping.shippingNote!, isDark: isDark),
          ],

          // PHASE 3 HARDENING: Contextual help for shipping issues
          const SizedBox(height: 16),
          _ShippingHelpSection(order: order, isDark: isDark),
        ],
      ),
    );
  }
}

/// SHIPPING CONFIRMATION TRUTH: Shipping reference row with honest label
///
/// Displays the shipping reference with appropriate labeling:
/// - "tracking" → "Nomor Resi" with receipt icon
/// - "phone" → "No. HP / WA Pengiriman" with phone icon, tap-to-call
/// - "other" → "Referensi Pengiriman" with description icon
class _ShippingReferenceRow extends StatelessWidget {
  final ShippingInfo shipping;
  final bool isDark;

  const _ShippingReferenceRow({required this.shipping, required this.isDark});

  @override
  Widget build(BuildContext context) {
    final referenceType = shipping.referenceType ?? 'tracking';
    final reference = shipping.trackingNumber!;

    // Get honest label and icon based on reference type
    IconData getIcon() {
      switch (referenceType) {
        case 'phone':
          return Icons.phone_outlined;
        case 'other':
          return Icons.description_outlined;
        case 'tracking':
        default:
          return Icons.receipt_long_outlined;
      }
    }

    String getLabel() {
      switch (referenceType) {
        case 'phone':
          return 'No. HP / WA Pengiriman';
        case 'other':
          return 'Referensi Pengiriman';
        case 'tracking':
        default:
          return 'Nomor Resi';
      }
    }

    // Phone-type reference: show as tappable phone number
    if (referenceType == 'phone') {
      return _PhoneShippingRow(
        icon: getIcon(),
        label: getLabel(),
        phone: reference,
        isDark: isDark,
      );
    }

    // Tracking or other: show as info row with copy
    return _ShippingInfoRow(
      icon: getIcon(),
      label: getLabel(),
      value: reference,
      isDark: isDark,
      isMonospace: true,
      showCopy: true,
    );
  }
}

/// Phone-type shipping reference row with tap-to-call functionality
class _PhoneShippingRow extends StatelessWidget {
  final IconData icon;
  final String label;
  final String phone;
  final bool isDark;

  const _PhoneShippingRow({
    required this.icon,
    required this.label,
    required this.phone,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return InkWell(
      onTap: () => _callPhone(context, phone),
      borderRadius: BorderRadius.circular(8),
      child: Container(
        padding: const EdgeInsets.all(8),
        decoration: BoxDecoration(
          color: core.AppColors.primaryGreen.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(8),
        ),
        child: Row(
          children: [
            Icon(icon, size: 18, color: core.AppColors.primaryGreen),
            const SizedBox(width: 8),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    label,
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: core.AppColors.primaryGreen,
                    ),
                  ),
                  Text(
                    phone,
                    style: theme.textTheme.bodyMedium?.copyWith(
                      color: core.AppColors.primaryGreen,
                      fontWeight: FontWeight.w500,
                    ),
                  ),
                ],
              ),
            ),
            Icon(Icons.call, size: 18, color: core.AppColors.primaryGreen),
          ],
        ),
      ),
    );
  }

  Future<void> _callPhone(BuildContext context, String phone) async {
    // In a real implementation, you would use url_launcher
    // For now, show a snackbar as fallback
    if (context.mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('Hubungi: $phone'),
          duration: const Duration(seconds: 2),
        ),
      );
    }
  }
}

/// SHIPPING CONFIRMATION TRUTH: Shipping note section
///
/// Displays seller's shipping note to provide buyer context
/// like "berangkat malam ini", "dititip ke sopir travel"
class _ShippingNoteSection extends StatelessWidget {
  final String note;
  final bool isDark;

  const _ShippingNoteSection({required this.note, required this.isDark});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: isDark
            ? const Color(0xFF2A2A2A)
            : core.AppColors.primaryBlue.withValues(alpha: 0.05),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(
          color: isDark
              ? const Color(0xFF333333)
              : core.AppColors.primaryBlue.withValues(alpha: 0.2),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                Icons.note_alt_outlined,
                size: 14,
                color: core.AppColors.primaryBlue,
              ),
              const SizedBox(width: 6),
              Text(
                'Catatan Pengiriman',
                style: theme.textTheme.bodySmall?.copyWith(
                  fontWeight: FontWeight.w500,
                  color: core.AppColors.primaryBlue,
                ),
              ),
            ],
          ),
          const SizedBox(height: 4),
          Text(
            note,
            style: theme.textTheme.bodySmall?.copyWith(
              color: isDark ? Colors.white70 : Colors.black87,
              fontStyle: FontStyle.italic,
            ),
          ),
        ],
      ),
    );
  }
}

class _ShippingInfoRow extends StatelessWidget {
  final IconData icon;
  final String label;
  final String value;
  final bool isDark;
  final bool isMonospace;
  final bool showCopy;

  const _ShippingInfoRow({
    required this.icon,
    required this.label,
    required this.value,
    required this.isDark,
    this.isMonospace = false,
    this.showCopy = false,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(icon, size: 18, color: Colors.grey),
        const SizedBox(width: 8),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                label,
                style: theme.textTheme.bodySmall?.copyWith(color: Colors.grey),
              ),
              Row(
                children: [
                  Expanded(
                    child: Text(
                      value,
                      style: theme.textTheme.bodyMedium?.copyWith(
                        color: isDark ? Colors.white : Colors.black87,
                        fontFamily: isMonospace ? 'monospace' : null,
                      ),
                    ),
                  ),
                  if (showCopy)
                    InkWell(
                      onTap: () {
                        // Copy to clipboard functionality could be added here
                      },
                      child: Padding(
                        padding: const EdgeInsets.all(4),
                        child: Icon(
                          Icons.copy,
                          size: 16,
                          color: core.AppColors.primaryBlue,
                        ),
                      ),
                    ),
                ],
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _ShippingAddressRow extends StatelessWidget {
  final IconData icon;
  final String label;
  final String address;
  final String? cityName;
  final String? districtName;
  final String? postalCode;
  final bool isDark;

  const _ShippingAddressRow({
    required this.icon,
    required this.label,
    required this.address,
    this.cityName,
    this.districtName,
    this.postalCode,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    // Build full address string
    final addressParts = <String>[address];
    if (districtName != null) addressParts.add(districtName!);
    if (cityName != null) addressParts.add(cityName!);
    if (postalCode != null) addressParts.add(postalCode!);
    final fullAddress = addressParts.join(', ');

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(icon, size: 18, color: Colors.grey),
        const SizedBox(width: 8),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                label,
                style: theme.textTheme.bodySmall?.copyWith(color: Colors.grey),
              ),
              Text(
                fullAddress,
                style: theme.textTheme.bodyMedium?.copyWith(
                  color: isDark ? Colors.white : Colors.black87,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

// =============================================================================
// _ShippingHelpSection - Contextual help for shipping issues (PHASE 3 HARDENING)
// =============================================================================

class _ShippingHelpSection extends ConsumerWidget {
  final Order order;
  final bool isDark;

  const _ShippingHelpSection({required this.order, required this.isDark});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final authState = ref.watch(core.authControllerProvider);
    final userId = authState is core.AuthStateAuthenticated
        ? authState.user.id
        : null;
    final userName = authState is core.AuthStateAuthenticated
        ? authState.user.username
        : null;
    final userAvatar = authState is core.AuthStateAuthenticated
        ? authState.user.avatarUrl
        : null;

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: core.AppColors.primaryBlue.withValues(alpha: 0.05),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                Icons.help_outline,
                size: 16,
                color: core.AppColors.primaryBlue,
              ),
              const SizedBox(width: 6),
              Text(
                'Masalah dengan pengiriman?',
                style: TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.w600,
                  color: isDark
                      ? core.AppColors.neutralWhite
                      : core.AppColors.neutralGray900,
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Row(
            children: [
              Expanded(
                child: _HelpActionChip(
                  icon: Icons.chat_bubble_outline,
                  label: 'Chat Penjual',
                  onTap: () {
                    // Navigate to seller chat (already implemented in order detail)
                    final currentUserId = userId;
                    if (currentUserId == null) return;

                    final otherUserId = currentUserId == order.sellerId
                        ? order.buyerId
                        : order.sellerId;

                    context.go('/chat?userId=$otherUserId&orderId=${order.id}');
                  },
                  isDark: isDark,
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: _HelpActionChip(
                  icon: Icons.support_agent,
                  label: 'Hubungi Support',
                  onTap: userId != null
                      ? () {
                          showPreChatFormRefactored(
                            context,
                            userId: userId,
                            userName: userName ?? 'User',
                            userAvatar: userAvatar,
                            linkedOrderId: order.id,
                          );
                        }
                      : null,
                  isDark: isDark,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _HelpActionChip extends StatelessWidget {
  final IconData icon;
  final String label;
  final VoidCallback? onTap;
  final bool isDark;

  const _HelpActionChip({
    required this.icon,
    required this.label,
    required this.onTap,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(6),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
        decoration: BoxDecoration(
          color: onTap != null
              ? core.AppColors.primaryBlue.withValues(alpha: 0.1)
              : core.AppColors.neutralGray200,
          borderRadius: BorderRadius.circular(6),
        ),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              icon,
              size: 12,
              color: onTap != null
                  ? core.AppColors.primaryBlue
                  : core.AppColors.neutralGray400,
            ),
            const SizedBox(width: 4),
            Text(
              label,
              style: TextStyle(
                fontSize: 10,
                fontWeight: FontWeight.w500,
                color: onTap != null
                    ? core.AppColors.primaryBlue
                    : core.AppColors.neutralGray400,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// =============================================================================
// OrderPaymentInfoCard - Order Payment Info Card
// =============================================================================
