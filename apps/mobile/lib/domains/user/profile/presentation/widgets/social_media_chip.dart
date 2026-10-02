import 'package:flutter/material.dart';
import 'package:labuda/shared/shared.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Social media chip widget
/// Displays a clickable social media link with icon
class SocialMediaChip extends StatelessWidget {
  final IconData icon;
  final String label;
  final String url;

  const SocialMediaChip({
    super.key,
    required this.icon,
    required this.label,
    required this.url,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return InkWell(
      onTap: () => _launchUrl(context),
      borderRadius: BorderRadius.circular(AppShape.r8),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p12, vertical: AppMetrics.p8),
        decoration: BoxDecoration(
          color: scheme.surfaceContainerLow,
          borderRadius: BorderRadius.circular(AppShape.r8),
          border: Border.all(
            color: scheme.outlineVariant,
          ),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: AppIconSize.inlineGlyph, color: scheme.primary),
            const SizedBox(width: 8),
            Text(
              label,
              style: TextStyle(
                fontSize: AppType.s14,
                color: scheme.onSurface,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _launchUrl(BuildContext context) async {
    final uri = Uri.parse(url);
    if (await canLaunchUrl(uri)) {
      await launchUrl(uri, mode: LaunchMode.externalApplication);
    } else {
      if (context.mounted) {
        AppSnackBar.showError(context, 'Cannot open link');
      }
    }
  }
}
