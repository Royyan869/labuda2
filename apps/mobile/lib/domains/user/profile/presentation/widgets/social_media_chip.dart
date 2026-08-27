import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:url_launcher/url_launcher.dart';

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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return InkWell(
      onTap: () => _launchUrl(context),
      borderRadius: BorderRadius.circular(8),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        decoration: BoxDecoration(
          color: isDark
              ? AppColors.darkGray600.withValues(alpha: 0.3)
              : AppColors.neutralGray50,
          borderRadius: BorderRadius.circular(8),
          border: Border.all(
            color: isDark
                ? AppColors.darkGray500.withValues(alpha: 0.5)
                : AppColors.neutralGray200,
          ),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 16, color: AppColors.primary),
            const SizedBox(width: 8),
            Text(
              label,
              style: TextStyle(
                fontSize: 13,
                color: isDark
                    ? AppColors.neutralGray300
                    : AppColors.neutralGray700,
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
