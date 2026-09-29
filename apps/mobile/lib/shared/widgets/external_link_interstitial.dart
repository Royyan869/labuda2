library;

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';
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
      AppSnackBar.showError(context, 'Tautan tidak valid');
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
      AppSnackBar.showError(context, 'Gagal membuka tautan');
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
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(AppShape.r16)),
      title: Row(
        children: [
          Icon(Icons.open_in_new, color: scheme.secondary, size: 24),
          const SizedBox(width: 8),
          const Expanded(
            child: Text(
              'Buka tautan eksternal?',
              style: TextStyle(fontSize: AppType.s18, fontWeight: FontWeight.w700),
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
              fontSize: AppType.s14,
              color: scheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 16),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(AppMetrics.p12),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(AppShape.r8),
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
                    fontSize: AppType.s15,
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
                    fontSize: AppType.s12,
                    color: scheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              Icon(Icons.warning_amber_rounded, size: 16, color: context.statusColors.warning),
              const SizedBox(width: 6),
              Expanded(
                child: Text(
                  'Labuda tidak bertanggung jawab atas konten di situs eksternal.',
                  style: TextStyle(fontSize: AppType.s12, color: context.statusColors.warning),
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
          ),
        ),
      ],
    );
  }
}
