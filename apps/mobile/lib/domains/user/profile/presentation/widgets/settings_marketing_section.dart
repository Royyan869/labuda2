import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/user/preference/seller/presentation/providers/current_seller_provider.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_state.dart';
import 'package:labuda/domains/commerce/pricing/discount/discount.dart';

/// Marketing & Promotion Section
/// Handles: Promotions & Discounts
class SettingsMarketingSection extends ConsumerWidget {
  final Function(String) onNavigate;
  final String userId;

  const SettingsMarketingSection({
    super.key,
    required this.onNavigate,
    required this.userId,
  });

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
          icon: Icons.discount_outlined,
          title: 'Promotions & Discounts',
          subtitle: 'Create and manage special offers',
          onTap: () => _navigateToDiscountManagement(context),
          scheme: scheme,
        ),
      ],
    );
  }

  void _navigateToDiscountManagement(BuildContext context) {
    Navigator.of(
      context,
    ).push(MaterialPageRoute(builder: (_) => const SellerDiscountListScreen()));
  }

  Widget _buildSectionHeaderWithIcon(
    BuildContext context,
    IconData icon,
    String title,
    ColorScheme scheme,
  ) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
      child: Row(
        children: [
          Icon(
            icon,
            size: 20,
            color: scheme.onSurfaceVariant,
          ),
          const SizedBox(width: 8),
          Text(
            title,
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w600,
              color: scheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSettingsTile({
    required IconData icon,
    required String title,
    required String subtitle,
    required VoidCallback onTap,
    required ColorScheme scheme,
  }) {
    return ListTile(
      leading: Icon(
        icon,
        color: scheme.onSurfaceVariant,
      ),
      title: Text(
        title,
        style: TextStyle(
          color: scheme.onSurface,
        ),
      ),
      subtitle: Text(
        subtitle,
        style: TextStyle(
          color: scheme.onSurfaceVariant,
          fontSize: 13,
        ),
      ),
      trailing: Icon(
        Icons.arrow_forward_ios,
        size: 16,
        color: scheme.onSurfaceVariant,
      ),
      onTap: onTap,
    );
  }
}
