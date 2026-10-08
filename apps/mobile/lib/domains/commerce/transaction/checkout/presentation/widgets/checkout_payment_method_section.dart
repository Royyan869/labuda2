part of '../screens/checkout_screen_impl.dart';

/// Canonical pre-order payment-method selection.
///
/// Renders the backend-computed options (buyer fee + FINAL payable amount).
/// It is a pure consumer/editor of the checkout state: the parent owns the
/// selected method code, and the summary/CTA read the same state. No fee or
/// total is ever computed here.
class _PaymentMethodSection extends StatelessWidget {
  final PreOrderPaymentPricing? pricing;
  final String? selectedMethodCode;
  final bool isLoading;
  final String? error;
  final ValueChanged<String> onSelected;
  final VoidCallback onRetry;

  const _PaymentMethodSection({
    required this.pricing,
    required this.selectedMethodCode,
    required this.isLoading,
    required this.error,
    required this.onSelected,
    required this.onRetry,
  });

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    Widget body;
    if (isLoading) {
      body = const Padding(
        padding: EdgeInsets.symmetric(vertical: AppMetrics.p16),
        child: Center(child: CircularProgressIndicator()),
      );
    } else if (error != null) {
      body = Row(
        children: [
          Icon(
            Icons.error_outline,
            color: colorScheme.error,
            size: AppIconSize.action,
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              'Gagal memuat metode pembayaran.',
              style: context.typeRoles.bodyDense.copyWith(
                color: colorScheme.onSurfaceVariant,
              ),
            ),
          ),
          TextButton(onPressed: onRetry, child: const Text('Coba lagi')),
        ],
      );
    } else if (pricing == null || pricing!.methods.isEmpty) {
      body = Text(
        'Metode pembayaran belum tersedia.',
        style: context.typeRoles.bodyDense.copyWith(
          color: colorScheme.onSurfaceVariant,
        ),
      );
    } else {
      final methods = pricing!.methods;
      body = Column(
        children: [
          for (var i = 0; i < methods.length; i++) ...[
            _PaymentMethodTile(
              option: methods[i],
              selected: methods[i].methodCode == selectedMethodCode,
              onTap: () => onSelected(methods[i].methodCode),
            ),
            if (i != methods.length - 1) const SizedBox(height: 8),
          ],
        ],
      );
    }

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: colorScheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Metode Pembayaran',
            style: context.typeRoles.titleSection.copyWith(
              fontWeight: FontWeight.bold,
            ),
          ),
          const SizedBox(height: 12),
          body,
        ],
      ),
    );
  }
}

class _PaymentMethodTile extends StatelessWidget {
  final PreOrderPaymentMethodOption option;
  final bool selected;
  final VoidCallback onTap;

  const _PaymentMethodTile({
    required this.option,
    required this.selected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(AppShape.r8),
      child: Container(
        padding: const EdgeInsets.all(AppMetrics.p12),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(AppShape.r8),
          border: Border.all(
            color: selected ? colorScheme.primary : colorScheme.outlineVariant,
            width: selected ? 1.5 : 1,
          ),
          color: selected ? colorScheme.primary.withValues(alpha: 0.06) : null,
        ),
        child: Row(
          children: [
            // Canonical method presentation (ONE mapping authority).
            PaymentMethodLogo(
              visual: PaymentMethodVisuals.visual(option.methodCode),
              size: 24,
              maxWidth: 88,
            ),
            const SizedBox(width: 12),
            Icon(
              selected
                  ? Icons.radio_button_checked
                  : Icons.radio_button_unchecked,
              color: selected
                  ? colorScheme.primary
                  : colorScheme.onSurfaceVariant,
              size: AppIconSize.action,
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    option.displayName,
                    style: context.typeRoles.bodyDense.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  const SizedBox(height: 2),
                  Text(
                    option.buyerPaymentFeeAmount > 0
                        ? 'Biaya layanan: '
                              '${AppFormatters.formatCurrency(option.buyerPaymentFeeAmount.toDouble())}'
                        : 'Tanpa biaya layanan',
                    style: context.typeRoles.labelMicro.copyWith(
                      color: colorScheme.onSurfaceVariant,
                    ),
                  ),
                ],
              ),
            ),
            Text(
              AppFormatters.formatCurrency(
                option.finalPayableAmount.toDouble(),
              ),
              style: context.typeRoles.titleCompact.copyWith(
                fontWeight: FontWeight.bold,
                color: selected ? colorScheme.primary : null,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
