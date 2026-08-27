import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
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
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final sellerCapabilityStatus = ref.watch(sellerCapabilityStatusProvider);
    final isSeller = sellerCapabilityStatus == SellerCapabilityStatus.active;

    return Column(
      children: [
        _buildSectionHeaderWithIcon(
          context,
          Icons.campaign,
          'Marketing & Promotion',
          isDark,
        ),
        if (isSeller)
          _buildSettingsTile(
            icon: Icons.discount_outlined,
            title: 'Promotions & Discounts',
            subtitle: 'Create and manage special offers',
            onTap: () => _navigateToDiscountManagement(context),
            isDark: isDark,
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
    bool isDark,
  ) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
      child: Row(
        children: [
          Icon(
            icon,
            size: 20,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
          ),
          const SizedBox(width: 8),
          Text(
            title,
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w600,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
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
    required bool isDark,
  }) {
    return ListTile(
      leading: Icon(
        icon,
        color: isDark ? AppColors.neutralGray300 : AppColors.neutralGray700,
      ),
      title: Text(
        title,
        style: TextStyle(
          color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
        ),
      ),
      subtitle: Text(
        subtitle,
        style: TextStyle(
          color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
          fontSize: 13,
        ),
      ),
      trailing: Icon(
        Icons.arrow_forward_ios,
        size: 16,
        color: isDark ? AppColors.neutralGray500 : AppColors.neutralGray400,
      ),
      onTap: onTap,
    );
  }
}
