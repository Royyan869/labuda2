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
        SnackBar(
          content: const Text('Tautan tidak valid'),
          backgroundColor: Theme.of(context).colorScheme.error,
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
        SnackBar(
          content: const Text('Gagal membuka tautan'),
          backgroundColor: Theme.of(context).colorScheme.error,
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
    final scheme = Theme.of(context).colorScheme;

    return AlertDialog(
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      title: Row(
        children: [
          Icon(Icons.open_in_new, color: scheme.secondary, size: 24),
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
              color: scheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 16),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(8),
              color: scheme.surfaceContainerHigh,
              border: Border.all(
                color: scheme.outlineVariant,
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
                    color: scheme.onSurface,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  uri.toString(),
                  maxLines: 3,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: 12,
                    color: scheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              Icon(Icons.warning_amber_rounded, size: 16, color: AppColors.statusWarning),
              const SizedBox(width: 6),
              Expanded(
                child: Text(
                  'Labuda tidak bertanggung jawab atas konten di situs eksternal.',
                  style: TextStyle(fontSize: 12, color: AppColors.statusWarning),
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
            style: TextStyle(color: scheme.onSurfaceVariant),
          ),
        ),
        ElevatedButton.icon(
          onPressed: () => Navigator.of(context).pop(true),
          icon: const Icon(Icons.open_in_new, size: 16),
          label: const Text('Buka'),
          style: ElevatedButton.styleFrom(
            backgroundColor: scheme.secondary,
            foregroundColor: scheme.onSecondary,
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(8),
            ),
          ),
        ),
      ],
    );
  }
}
