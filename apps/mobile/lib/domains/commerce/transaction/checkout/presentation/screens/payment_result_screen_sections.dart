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
  final bool isDark;

  const _NextStepsSection({required this.isDark});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: isDark
            ? core.AppColors.darkGray800
            : core.AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(
          color: isDark
              ? core.AppColors.darkGray700
              : core.AppColors.neutralGray200,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Section header
          Row(
            children: [
              Container(
                padding: const EdgeInsets.all(8),
                decoration: BoxDecoration(
                  color: core.AppColors.successGreen.withValues(alpha: 0.1),
                  shape: BoxShape.circle,
                ),
                child: const Icon(
                  Icons.info_outline,
                  color: core.AppColors.successGreen,
                  size: 20,
                ),
              ),
              const SizedBox(width: 12),
              Text(
                'Apa Selanjutnya?',
                style: TextStyle(
                  fontSize: 16,
                  fontWeight: FontWeight.bold,
                  color: isDark
                      ? core.AppColors.neutralWhite
                      : core.AppColors.neutralGray900,
                ),
              ),
            ],
          ),
          const SizedBox(height: 16),

          // Next steps list
          _NextStepItem(
            icon: Icons.store_outlined,
            title: 'Penjual mempersiapkan pesanan',
            description:
                'Penjual akan menyiapkan ikan sesuai dengan spesifikasi pesanan',
            isDark: isDark,
          ),
          const SizedBox(height: 12),
          _NextStepItem(
            icon: Icons.local_shipping_outlined,
            title: 'Pengiriman diatur oleh penjual',
            description:
                'Setelah siap, penjual akan mengirim pesanan dan mengupdate resi',
            isDark: isDark,
          ),
          const SizedBox(height: 12),
          _NextStepItem(
            icon: Icons.chat_bubble_outline,
            title: 'Pantau melalui Pesanan / Chat',
            description:
                'Anda dapat memantau status pesanan dan berkomunikasi dengan penjual',
            isDark: isDark,
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
  final bool isDark;

  const _NextStepItem({
    required this.icon,
    required this.title,
    required this.description,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Container(
          padding: const EdgeInsets.all(8),
          decoration: BoxDecoration(
            color: core.AppColors.primaryBlue.withValues(alpha: 0.1),
            shape: BoxShape.circle,
          ),
          child: Icon(icon, size: 16, color: core.AppColors.primaryBlue),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                title,
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w600,
                  color: isDark
                      ? core.AppColors.neutralWhite
                      : core.AppColors.neutralGray900,
                ),
              ),
              const SizedBox(height: 2),
              Text(
                description,
                style: TextStyle(
                  fontSize: 12,
                  color: core.AppColors.neutralGray600,
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
