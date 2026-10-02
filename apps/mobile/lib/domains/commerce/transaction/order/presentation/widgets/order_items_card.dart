part of 'order_widgets_impl.dart';

class OrderItemsCard extends StatelessWidget {
  final Order order;

  const OrderItemsCard({super.key, required this.order});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

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
          Text(
            'Item Pesanan (${order.items.length})',
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 12),
          ...order.items.map((item) => _OrderItemTile(item: item)),
          const SizedBox(height: 12),
          const Divider(),
          const SizedBox(height: 8),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                'Total',
                style: theme.textTheme.bodyMedium?.copyWith(
                  fontWeight: FontWeight.w600,
                ),
              ),
              Text(
                AppFormatters.formatCurrency(order.pricing.subtotal),
                style: theme.textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.w700,
                  color: colorScheme.primary,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _OrderItemTile extends StatelessWidget {
  final OrderItem item;

  const _OrderItemTile({required this.item});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return Padding(
      padding: const EdgeInsets.only(bottom: core.AppMetrics.p12),
      child: Row(
        children: [
          // Item image
          ClipRRect(
            borderRadius: BorderRadius.circular(core.AppShape.r8),
            child: AppImage(
              imageUrl: item.forSaleImage,
              width: 60,
              height: 60,
              fit: BoxFit.cover,
              backgroundColor: colorScheme.surfaceContainerHighest,
              errorWidget: Container(
                width: 60,
                height: 60,
                decoration: BoxDecoration(
                  color: colorScheme.surfaceContainerHighest,
                  borderRadius: BorderRadius.circular(core.AppShape.r8),
                ),
                child: Icon(
                  Icons.image_not_supported,
                  color: colorScheme.onSurfaceVariant,
                  size: AppIconSize.header,
                ),
              ),
            ),
          ),
          const SizedBox(width: 12),
          // Item details
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  item.forSaleName,
                  style: theme.textTheme.bodyMedium?.copyWith(
                    fontWeight: FontWeight.w500,
                  ),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 4),
                // Variant info
                if (item.variety != null ||
                    item.size != null ||
                    item.gender != null)
                  Wrap(
                    spacing: 4,
                    children: [
                      if (item.variety != null)
                        _VariantChip(label: item.variety!),
                      if (item.size != null) _VariantChip(label: item.size!),
                      if (item.gender != null)
                        _VariantChip(label: item.gender!),
                    ],
                  ),
                const SizedBox(height: 4),
                // Price and quantity
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Text(
                      AppFormatters.formatCurrency(item.price),
                      style: theme.textTheme.bodyMedium?.copyWith(
                        color: colorScheme.primary,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    Text(
                      'x${item.quantity}',
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: colorScheme.onSurfaceVariant,
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _VariantChip extends StatelessWidget {
  final String label;

  const _VariantChip({required this.label});

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: core.AppMetrics.p8, vertical: core.AppMetrics.p4),
      decoration: BoxDecoration(
        color: colorScheme.onSurfaceVariant.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(core.AppShape.r4),
        border: Border.all(
          color: colorScheme.onSurfaceVariant.withValues(alpha: 0.3),
        ),
      ),
      child: Text(
        label,
        style: Theme.of(context).textTheme.bodySmall?.copyWith(
          fontSize: core.AppType.s12,
          color: colorScheme.onSurfaceVariant,
        ),
      ),
    );
  }
}

// =============================================================================
// OrderShippingInfoCard - Order Shipping Info Card
// =============================================================================
