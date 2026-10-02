part of 'payment_result_screen_impl.dart';

/// Get appropriate message based on elapsed time
String _paymentResultGetElapsedTimeMessage(PaymentResultState state) {
  final elapsed = state.elapsed;
  if (elapsed == null) return '';

  final seconds = elapsed.inSeconds;

  if (seconds > 30) {
    return 'Jika sudah membayar, silakan cek kembali di halaman pesanan';
  } else if (seconds > 15) {
    return 'Pembayaran sedang diverifikasi sistem';
  }
  return '';
}

// =============================================================================
// NEXT STEPS SECTION (PHASE 2 HARDENING)
// =============================================================================
/// "Apa Selanjutnya?" section shown after successful payment
/// Provides buyers with clarity on what happens next in the order journey
class _NextStepsSection extends StatelessWidget {
  const _NextStepsSection();

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    return Container(
      padding: const EdgeInsets.all(core.AppMetrics.p24),
      decoration: BoxDecoration(
        color: colorScheme.surface,
        borderRadius: BorderRadius.circular(core.AppShape.r16),
        border: Border.all(color: colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Section header
          Row(
            children: [
              Container(
                padding: const EdgeInsets.all(core.AppMetrics.p8),
                decoration: BoxDecoration(
                  color: context.statusColors.success.withValues(alpha: 0.1),
                  shape: BoxShape.circle,
                ),
                child: Icon(
                  Icons.info_outline,
                  color: context.statusColors.success,
                  size: AppIconSize.action,
                ),
              ),
              const SizedBox(width: 12),
              Text(
                'Apa Selanjutnya?',
                style: TextStyle(
                  fontSize: core.AppType.s16,
                  fontWeight: FontWeight.bold,
                  color: colorScheme.onSurface,
                ),
              ),
            ],
          ),
          const SizedBox(height: 16),

          // Next steps list
          const _NextStepItem(
            icon: Icons.store_outlined,
            title: 'Penjual mempersiapkan pesanan',
            description:
                'Penjual akan menyiapkan ikan sesuai dengan spesifikasi pesanan',
          ),
          const SizedBox(height: 12),
          const _NextStepItem(
            icon: Icons.local_shipping_outlined,
            title: 'Pengiriman diatur oleh penjual',
            description:
                'Setelah siap, penjual akan mengirim pesanan dan mengupdate resi',
          ),
          const SizedBox(height: 12),
          const _NextStepItem(
            icon: Icons.chat_bubble_outline,
            title: 'Pantau melalui Pesanan / Chat',
            description:
                'Anda dapat memantau status pesanan dan berkomunikasi dengan penjual',
          ),
        ],
      ),
    );
  }
}

/// Single next step item
class _NextStepItem extends StatelessWidget {
  final IconData icon;
  final String title;
  final String description;

  const _NextStepItem({
    required this.icon,
    required this.title,
    required this.description,
  });

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Container(
          padding: const EdgeInsets.all(core.AppMetrics.p8),
          decoration: BoxDecoration(
            color: colorScheme.secondary.withValues(alpha: 0.1),
            shape: BoxShape.circle,
          ),
          child: Icon(icon, size: AppIconSize.inlineGlyph, color: colorScheme.secondary),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                title,
                style: TextStyle(
                  fontSize: core.AppType.s14,
                  fontWeight: FontWeight.w600,
                  color: colorScheme.onSurface,
                ),
              ),
              const SizedBox(height: 2),
              Text(
                description,
                style: TextStyle(
                  fontSize: core.AppType.s12,
                  color: colorScheme.onSurfaceVariant,
                  height: 1.4,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}
