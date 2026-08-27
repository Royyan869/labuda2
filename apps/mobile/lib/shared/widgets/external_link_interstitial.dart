library;

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:url_launcher/url_launcher.dart';

/// Shows an interstitial dialog before opening an external URL.
///
/// Returns `true` if the URL was launched, `false` if cancelled or failed.
Future<bool> showExternalLinkInterstitial(
  BuildContext context, {
  required String url,
}) async {
  final uri = Uri.tryParse(url);
  if (uri == null || (uri.scheme != 'https' && uri.scheme != 'http')) {
    if (context.mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Tautan tidak valid'),
          backgroundColor: AppColors.primaryRed,
        ),
      );
    }
    return false;
  }

  final confirmed = await showDialog<bool>(
    context: context,
    builder: (dialogContext) => _ExternalLinkDialog(uri: uri),
  );

  if (confirmed != true) return false;

  try {
    return await launchUrl(uri, mode: LaunchMode.externalApplication);
  } catch (_) {
    if (context.mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Gagal membuka tautan'),
          backgroundColor: AppColors.primaryRed,
        ),
      );
    }
    return false;
  }
}

class _ExternalLinkDialog extends StatelessWidget {
  final Uri uri;

  const _ExternalLinkDialog({required this.uri});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return AlertDialog(
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      title: Row(
        children: [
          Icon(Icons.open_in_new, color: AppColors.primaryBlue, size: 24),
          const SizedBox(width: 8),
          const Expanded(
            child: Text(
              'Buka tautan eksternal?',
              style: TextStyle(fontSize: 18, fontWeight: FontWeight.w700),
            ),
          ),
        ],
      ),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Anda akan meninggalkan Labuda dan membuka situs eksternal.',
            style: TextStyle(
              fontSize: 14,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray600,
            ),
          ),
          const SizedBox(height: 16),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(8),
              color: isDark ? AppColors.darkGray800 : AppColors.neutralGray100,
              border: Border.all(
                color: isDark
                    ? AppColors.darkGray700
                    : AppColors.neutralGray200,
              ),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  uri.host,
                  style: TextStyle(
                    fontSize: 15,
                    fontWeight: FontWeight.w600,
                    color: isDark
                        ? AppColors.neutralWhite
                        : AppColors.neutralGray900,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  uri.toString(),
                  maxLines: 3,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: 12,
                    color: AppColors.neutralGray400,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              Icon(Icons.warning_amber_rounded, size: 16, color: Colors.orange),
              const SizedBox(width: 6),
              Expanded(
                child: Text(
                  'Labuda tidak bertanggung jawab atas konten di situs eksternal.',
                  style: TextStyle(fontSize: 12, color: Colors.orange.shade700),
                ),
              ),
            ],
          ),
        ],
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(false),
          child: Text(
            'Batal',
            style: TextStyle(color: AppColors.neutralGray500),
          ),
        ),
        ElevatedButton.icon(
          onPressed: () => Navigator.of(context).pop(true),
          icon: const Icon(Icons.open_in_new, size: 16),
          label: const Text('Buka'),
          style: ElevatedButton.styleFrom(
            backgroundColor: AppColors.primaryBlue,
            foregroundColor: Colors.white,
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(8),
            ),
          ),
        ),
      ],
    );
  }
}
