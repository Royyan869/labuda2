import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/domains/user/preference/seller/presentation/providers/current_seller_provider.dart';
import 'package:hishumi/domains/user/preference/seller/domain/entities/seller_state.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';

/// Marketing & Promotion Section — two honest doors: Diskon (product price
/// reductions) and Promosi (canonical campaign budget). Each label names
/// exactly what its destination holds; the old single tile promising
/// "Promotions & Discounts" opened only discounts.
class SettingsMarketingSection extends ConsumerWidget {
  final Function(String) onNavigate;

  const SettingsMarketingSection({super.key, required this.onNavigate});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final scheme = Theme.of(context).colorScheme;
    final sellerCapabilityStatus = ref.watch(sellerCapabilityStatusProvider);
    final isSeller = sellerCapabilityStatus == SellerCapabilityStatus.active;

    // Honest UI: entire section hidden unless hasMarketAuthority.
    // Prevents ghost header for non-seller / pending / expired.
    if (!isSeller) return const SizedBox.shrink();

    return Column(
      children: [
        _buildSectionHeaderWithIcon(
          context,
          Icons.campaign,
          'Marketing & Promotion',
          scheme,
        ),
        _buildSettingsTile(
          context,
          icon: Icons.discount_outlined,
          title: 'Diskon',
          subtitle: 'Buat dan kelola diskon produk',
          onTap: () => onNavigate('discount'),
          scheme: scheme,
        ),
        _buildSettingsTile(
          context,
          // Canonical Promotion icon (same as the Promotion feature,
          // wizard, feed badge, and NotificationDisplayIcon.campaign).
          icon: Icons.campaign_outlined,
          title: 'Promosi',
          subtitle: 'Buat dan kelola promosi & iklan toko',
          onTap: () => onNavigate('promotion'),
          scheme: scheme,
        ),
      ],
    );
  }

  Widget _buildSectionHeaderWithIcon(
    BuildContext context,
    IconData icon,
    String title,
    ColorScheme scheme,
  ) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(
        AppMetrics.p16,
        AppMetrics.p16,
        AppMetrics.p16,
        AppMetrics.p8,
      ),
      child: Row(
        children: [
          Icon(icon, size: AppIconSize.action, color: scheme.onSurfaceVariant),
          const SizedBox(width: 8),
          Text(
            title,
            style: context.typeRoles.bodyDense.copyWith(
              fontWeight: FontWeight.w600,
              color: scheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSettingsTile(
    BuildContext context, {
    required IconData icon,
    required String title,
    required String subtitle,
    required VoidCallback onTap,
    required ColorScheme scheme,
  }) {
    return ListTile(
      leading: Icon(icon, color: scheme.onSurfaceVariant),
      title: Text(title, style: TextStyle(color: scheme.onSurface)),
      subtitle: Text(
        subtitle,
        style: context.typeRoles.bodyDense.copyWith(
          color: scheme.onSurfaceVariant,
        ),
      ),
      trailing: Icon(
        Icons.arrow_forward_ios,
        size: AppIconSize.inlineGlyph,
        color: scheme.onSurfaceVariant,
      ),
      onTap: onTap,
    );
  }
}
