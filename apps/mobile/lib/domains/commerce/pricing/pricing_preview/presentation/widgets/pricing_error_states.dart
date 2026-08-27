/// Pricing Error States Widgets
///
/// **STRICT MODE - ERROR HANDLING**
///
/// Displays user-friendly error messages for pricing flow failures.
/// All errors are from backend validation - no frontend assumptions.
library;

import 'package:flutter/material.dart';

/// Pricing Error Type
///
/// Represents different error states in pricing flow
enum PricingErrorType {
  /// Listing is sold or unavailable
  listingSold,

  /// Listing not found
  listingNotFound,

  /// Negotiation expired
  negotiationExpired,

  /// Negotiation not accepted
  negotiationNotAccepted,

  /// Pricing token expired
  tokenExpired,

  /// Pricing token invalid
  tokenInvalid,

  /// Price mismatch between preview and order
  priceMismatch,

  /// Shipping address invalid
  addressInvalid,

  /// Shipping option unavailable
  shippingUnavailable,

  /// Network error
  networkError,

  /// Unknown error
  unknown,
}

/// Pricing Error State Widget
///
/// Displays user-friendly error message with actionable options
class PricingErrorStateWidget extends StatelessWidget {
  final PricingErrorType errorType;
  final String? customMessage;
  final VoidCallback? onRetry;
  final VoidCallback? onBack;

  const PricingErrorStateWidget({
    super.key,
    required this.errorType,
    this.customMessage,
    this.onRetry,
    this.onBack,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Container(
      padding: const EdgeInsets.all(24),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          _buildIcon(theme),
          const SizedBox(height: 24),
          _buildTitle(theme),
          const SizedBox(height: 8),
          _buildMessage(theme),
          if (onRetry != null || onBack != null) ...[
            const SizedBox(height: 24),
            _buildActions(theme),
          ],
        ],
      ),
    );
  }

  Widget _buildIcon(ThemeData theme) {
    IconData icon;
    Color color;

    switch (errorType) {
      case PricingErrorType.listingSold:
      case PricingErrorType.listingNotFound:
        icon = Icons.remove_shopping_cart_outlined;
        color = theme.colorScheme.error;
        break;

      case PricingErrorType.negotiationExpired:
      case PricingErrorType.tokenExpired:
        icon = Icons.access_time_outlined;
        color = theme.colorScheme.error;
        break;

      case PricingErrorType.negotiationNotAccepted:
      case PricingErrorType.tokenInvalid:
      case PricingErrorType.priceMismatch:
        icon = Icons.error_outline;
        color = theme.colorScheme.error;
        break;

      case PricingErrorType.addressInvalid:
      case PricingErrorType.shippingUnavailable:
        icon = Icons.local_shipping_outlined;
        color = theme.colorScheme.tertiary;
        break;

      case PricingErrorType.networkError:
        icon = Icons.wifi_off_outlined;
        color = theme.colorScheme.outline;
        break;

      default:
        icon = Icons.error_outline;
        color = theme.colorScheme.error;
    }

    return Icon(icon, size: 64, color: color);
  }

  Widget _buildTitle(ThemeData theme) {
    String title;

    switch (errorType) {
      case PricingErrorType.listingSold:
        title = 'Barang Sudah Terjual';
        break;
      case PricingErrorType.listingNotFound:
        title = 'Barang Tidak Ditemukan';
        break;
      case PricingErrorType.negotiationExpired:
        title = 'Negosiasi Habis Waktu';
        break;
      case PricingErrorType.negotiationNotAccepted:
        title = 'Negosiasi Belum Disetujui';
        break;
      case PricingErrorType.tokenExpired:
        title = 'Waktu Harga Habis';
        break;
      case PricingErrorType.tokenInvalid:
        title = 'Token Harga Tidak Valid';
        break;
      case PricingErrorType.priceMismatch:
        title = 'Harga Berubah';
        break;
      case PricingErrorType.addressInvalid:
        title = 'Alamat Tidak Valid';
        break;
      case PricingErrorType.shippingUnavailable:
        title = 'Ongkos Kirim Tidak Tersedia';
        break;
      case PricingErrorType.networkError:
        title = 'Koneksi Bermasalah';
        break;
      default:
        title = 'Terjadi Kesalahan';
    }

    return Text(
      title,
      style: theme.textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w700),
      textAlign: TextAlign.center,
    );
  }

  Widget _buildMessage(ThemeData theme) {
    final message = customMessage ?? _getDefaultMessage();

    return Text(
      message,
      style: theme.textTheme.bodyMedium?.copyWith(
        color: theme.colorScheme.onSurfaceVariant,
      ),
      textAlign: TextAlign.center,
    );
  }

  String _getDefaultMessage() {
    switch (errorType) {
      case PricingErrorType.listingSold:
        return 'Maaf, barang ini sudah terjual. Silakan cari barang lain yang serupa.';
      case PricingErrorType.listingNotFound:
        return 'Barang tidak ditemukan atau telah dihapus oleh penjual.';
      case PricingErrorType.negotiationExpired:
        return 'Waktu negosiasi telah habis. Silakan ajukan negosiasi baru jika masih berminat.';
      case PricingErrorType.negotiationNotAccepted:
        return 'Penjual belum menyetujui harga ini. Tunggu hingga negosiasi disetujui.';
      case PricingErrorType.tokenExpired:
        return 'Waktu harga telah habis. Silakan refresh untuk mendapatkan harga terbaru.';
      case PricingErrorType.tokenInvalid:
        return 'Token harga tidak valid. Silakan kembali dan ulangi proses checkout.';
      case PricingErrorType.priceMismatch:
        return 'Harga barang telah berubah. Silakan refresh untuk mendapatkan harga terbaru.';
      case PricingErrorType.addressInvalid:
        return 'Alamat pengiriman tidak lengkap atau tidak valid. Mohon periksa kembali.';
      case PricingErrorType.shippingUnavailable:
        return 'Maaf, ongkos kirim ke alamat ini belum tersedia. Coba alamat lain.';
      case PricingErrorType.networkError:
        return 'Terjadi masalah koneksi. Silakan periksa internet Anda dan coba lagi.';
      default:
        return 'Terjadi kesalahan tidak terduga. Silakan coba lagi.';
    }
  }

  Widget _buildActions(ThemeData theme) {
    return Column(
      children: [
        if (onRetry != null)
          ElevatedButton.icon(
            onPressed: onRetry,
            icon: const Icon(Icons.refresh),
            label: Text(_getRetryLabel()),
            style: ElevatedButton.styleFrom(
              padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 12),
              minimumSize: const Size(200, 48),
            ),
          ),
        if (onRetry != null && onBack != null) const SizedBox(height: 8),
        if (onBack != null)
          TextButton.icon(
            onPressed: onBack,
            icon: const Icon(Icons.arrow_back),
            label: const Text('Kembali'),
          ),
      ],
    );
  }

  String _getRetryLabel() {
    switch (errorType) {
      case PricingErrorType.tokenExpired:
      case PricingErrorType.priceMismatch:
        return 'Refresh Harga';
      case PricingErrorType.negotiationExpired:
        return 'Negosiasi Ulang';
      case PricingErrorType.networkError:
        return 'Coba Lagi';
      default:
        return 'Coba Lagi';
    }
  }
}

/// Pricing Error Dialog
///
/// Shows error in a dialog format for better visibility
class PricingErrorDialog extends StatelessWidget {
  final PricingErrorType errorType;
  final String? customMessage;
  final VoidCallback? onRetry;
  final VoidCallback? onBack;

  const PricingErrorDialog({
    super.key,
    required this.errorType,
    this.customMessage,
    this.onRetry,
    this.onBack,
  });

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      contentPadding: const EdgeInsets.all(24),
      content: PricingErrorStateWidget(
        errorType: errorType,
        customMessage: customMessage,
        onRetry: onRetry,
        onBack: onBack,
      ),
    );
  }

  /// Show the dialog
  static Future<void> show(
    BuildContext context, {
    required PricingErrorType errorType,
    String? customMessage,
    VoidCallback? onRetry,
    VoidCallback? onBack,
  }) {
    return showDialog<void>(
      context: context,
      builder: (context) => PricingErrorDialog(
        errorType: errorType,
        customMessage: customMessage,
        onRetry: onRetry,
        onBack: onBack,
      ),
    );
  }
}

/// Inline Pricing Error Banner
///
/// Shows error in a compact banner format
class PricingErrorBanner extends StatelessWidget {
  final PricingErrorType errorType;
  final String? customMessage;
  final VoidCallback? onRetry;
  final VoidCallback? onDismiss;

  const PricingErrorBanner({
    super.key,
    required this.errorType,
    this.customMessage,
    this.onRetry,
    this.onDismiss,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Container(
      margin: const EdgeInsets.all(16),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: theme.colorScheme.errorContainer.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: theme.colorScheme.error.withValues(alpha: 0.3),
        ),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(Icons.error_outline, color: theme.colorScheme.error),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  _getShortTitle(),
                  style: theme.textTheme.titleSmall?.copyWith(
                    fontWeight: FontWeight.w600,
                    color: theme.colorScheme.error,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  customMessage ?? _getShortMessage(),
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                ),
                if (onRetry != null) ...[
                  const SizedBox(height: 8),
                  TextButton.icon(
                    onPressed: onRetry,
                    icon: const Icon(Icons.refresh, size: 16),
                    label: const Text('Coba Lagi'),
                    style: TextButton.styleFrom(
                      padding: const EdgeInsets.symmetric(horizontal: 8),
                      minimumSize: const Size(0, 32),
                    ),
                  ),
                ],
              ],
            ),
          ),
          if (onDismiss != null)
            IconButton(
              onPressed: onDismiss,
              icon: const Icon(Icons.close),
              iconSize: 20,
            ),
        ],
      ),
    );
  }

  String _getShortTitle() {
    switch (errorType) {
      case PricingErrorType.listingSold:
        return 'Barang Terjual';
      case PricingErrorType.tokenExpired:
        return 'Waktu Habis';
      case PricingErrorType.priceMismatch:
        return 'Harga Berubah';
      default:
        return 'Terjadi Kesalahan';
    }
  }

  String _getShortMessage() {
    switch (errorType) {
      case PricingErrorType.listingSold:
        return 'Barang ini sudah terjual.';
      case PricingErrorType.tokenExpired:
        return 'Silakan refresh harga terbaru.';
      case PricingErrorType.priceMismatch:
        return 'Harga telah berubah sejak preview.';
      default:
        return 'Silakan coba lagi.';
    }
  }
}
