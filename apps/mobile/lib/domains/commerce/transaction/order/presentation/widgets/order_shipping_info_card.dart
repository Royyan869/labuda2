part of 'order_widgets_impl.dart';

class OrderShippingInfoCard extends StatelessWidget {
  final Order order;

  const OrderShippingInfoCard({super.key, required this.order});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;
    final shipping = order.shippingInfo;

    return Container(
      padding: const EdgeInsets.all(core.AppMetrics.p16),
      decoration: BoxDecoration(
        color: colorScheme.surface,
        borderRadius: BorderRadius.circular(core.AppShape.r12),
        border: Border.all(color: colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                Icons.local_shipping_outlined,
                size: 20,
                color: colorScheme.secondary,
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
          ),
          const SizedBox(height: 12),
          // Phone
          _ShippingInfoRow(
            icon: Icons.phone_outlined,
            label: 'Telepon',
            value: shipping.phone,
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
          ),
          // Shipping method
          if (shipping.courierName != null) ...[
            const SizedBox(height: 12),
            _ShippingInfoRow(
              icon: Icons.delivery_dining_outlined,
              label: 'Kurir',
              value: shipping.courierName!,
            ),
          ],
          // Tracking number
          if (shipping.trackingNumber != null &&
              shipping.trackingNumber!.isNotEmpty) ...[
            const SizedBox(height: 12),
            _ShippingReferenceRow(shipping: shipping),
          ],

          // SHIPPING CONFIRMATION TRUTH: Shipping note from seller
          if (shipping.shippingNote != null &&
              shipping.shippingNote!.isNotEmpty) ...[
            const SizedBox(height: 12),
            _ShippingNoteSection(note: shipping.shippingNote!),
          ],

          // PHASE 3 HARDENING: Contextual help for shipping issues
          const SizedBox(height: 16),
          _ShippingHelpSection(order: order),
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

  const _ShippingReferenceRow({required this.shipping});

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
      );
    }

    // Tracking or other: show as info row with copy
    return _ShippingInfoRow(
      icon: getIcon(),
      label: getLabel(),
      value: reference,
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

  const _PhoneShippingRow({
    required this.icon,
    required this.label,
    required this.phone,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return InkWell(
      onTap: () => _callPhone(context, phone),
      borderRadius: BorderRadius.circular(core.AppShape.r8),
      child: Container(
        padding: const EdgeInsets.all(core.AppMetrics.p8),
        decoration: BoxDecoration(
          color: context.statusColors.success.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(core.AppShape.r8),
        ),
        child: Row(
          children: [
            Icon(icon, size: 18, color: context.statusColors.success),
            const SizedBox(width: 8),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    label,
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: context.statusColors.success,
                    ),
                  ),
                  Text(
                    phone,
                    style: theme.textTheme.bodyMedium?.copyWith(
                      color: context.statusColors.success,
                      fontWeight: FontWeight.w500,
                    ),
                  ),
                ],
              ),
            ),
            Icon(Icons.call, size: 18, color: context.statusColors.success),
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

  const _ShippingNoteSection({required this.note});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return Container(
      padding: const EdgeInsets.all(core.AppMetrics.p12),
      decoration: BoxDecoration(
        color: colorScheme.secondaryContainer,
        borderRadius: BorderRadius.circular(core.AppShape.r8),
        border: Border.all(
          color: colorScheme.secondary.withValues(alpha: 0.4),
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
                color: colorScheme.secondary,
              ),
              const SizedBox(width: 6),
              Text(
                'Catatan Pengiriman',
                style: theme.textTheme.bodySmall?.copyWith(
                  fontWeight: FontWeight.w500,
                  color: colorScheme.secondary,
                ),
              ),
            ],
          ),
          const SizedBox(height: 4),
          Text(
            note,
            style: theme.textTheme.bodySmall?.copyWith(
              color: colorScheme.onSurface,
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
  final bool isMonospace;
  final bool showCopy;

  const _ShippingInfoRow({
    required this.icon,
    required this.label,
    required this.value,
    this.isMonospace = false,
    this.showCopy = false,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(icon, size: 18, color: colorScheme.onSurfaceVariant),
        const SizedBox(width: 8),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                label,
                style: theme.textTheme.bodySmall?.copyWith(
                  color: colorScheme.onSurfaceVariant,
                ),
              ),
              Row(
                children: [
                  Expanded(
                    child: Text(
                      value,
                      style: theme.textTheme.bodyMedium?.copyWith(
                        color: colorScheme.onSurface,
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
                        padding: const EdgeInsets.all(core.AppMetrics.p4),
                        child: Icon(
                          Icons.copy,
                          size: 16,
                          color: colorScheme.secondary,
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

  const _ShippingAddressRow({
    required this.icon,
    required this.label,
    required this.address,
    this.cityName,
    this.districtName,
    this.postalCode,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    // Build full address string
    final addressParts = <String>[address];
    if (districtName != null) addressParts.add(districtName!);
    if (cityName != null) addressParts.add(cityName!);
    if (postalCode != null) addressParts.add(postalCode!);
    final fullAddress = addressParts.join(', ');

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(icon, size: 18, color: colorScheme.onSurfaceVariant),
        const SizedBox(width: 8),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                label,
                style: theme.textTheme.bodySmall?.copyWith(
                  color: colorScheme.onSurfaceVariant,
                ),
              ),
              Text(
                fullAddress,
                style: theme.textTheme.bodyMedium?.copyWith(
                  color: colorScheme.onSurface,
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

  const _ShippingHelpSection({required this.order});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colorScheme = Theme.of(context).colorScheme;
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
      padding: const EdgeInsets.all(core.AppMetrics.p12),
      decoration: BoxDecoration(
        color: colorScheme.secondaryContainer,
        borderRadius: BorderRadius.circular(core.AppShape.r8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                Icons.help_outline,
                size: 16,
                color: colorScheme.secondary,
              ),
              const SizedBox(width: 6),
              Text(
                'Masalah dengan pengiriman?',
                style: TextStyle(
                  fontSize: core.AppType.s12,
                  fontWeight: FontWeight.w600,
                  color: colorScheme.onSurface,
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
                    if (userId == null) return;
                    openOrderCommerceChat(
                      context: context,
                      ref: ref,
                      order: order,
                      currentUserId: userId,
                    );
                  },
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

  const _HelpActionChip({
    required this.icon,
    required this.label,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(core.AppShape.r6),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: core.AppMetrics.p8, vertical: core.AppMetrics.p6),
        decoration: BoxDecoration(
          color: onTap != null
              ? colorScheme.secondary.withValues(alpha: 0.1)
              : colorScheme.surfaceContainerHighest,
          borderRadius: BorderRadius.circular(core.AppShape.r6),
        ),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              icon,
              size: 12,
              color: onTap != null
                  ? colorScheme.secondary
                  : colorScheme.onSurfaceVariant,
            ),
            const SizedBox(width: 4),
            Text(
              label,
              style: TextStyle(
                fontSize: core.AppType.s10,
                fontWeight: FontWeight.w500,
                color: onTap != null
                    ? colorScheme.secondary
                    : colorScheme.onSurfaceVariant,
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
