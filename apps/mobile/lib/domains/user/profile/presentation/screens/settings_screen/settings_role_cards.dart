import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Admin Panel card for settings screen
class SettingsAdminPanelCard extends StatelessWidget {
  final VoidCallback onTap;

  const SettingsAdminPanelCard({super.key, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return _SettingsGradientCard(
      gradientColors: const [
        Color(0xFF6366F1), // Indigo
        Color(0xFF8B5CF6), // Purple
      ],
      icon: Icons.admin_panel_settings,
      title: 'Admin Panel',
      subtitle: 'Manage platform, users, and content',
      onTap: onTap,
    );
  }
}

/// Seller Dashboard card for settings screen
class SettingsSellerDashboardCard extends StatelessWidget {
  final VoidCallback onTap;

  const SettingsSellerDashboardCard({super.key, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return _SettingsGradientCard(
      gradientColors: const [
        Color(0xFF10B981), // Emerald green
        Color(0xFF059669), // Darker emerald
      ],
      icon: Icons.dashboard,
      title: 'Seller Dashboard',
      subtitle: 'Manage your store and sales',
      onTap: onTap,
    );
  }
}

/// Coins card for settings screen
class SettingsCoinsCard extends StatelessWidget {
  final VoidCallback onTap;

  const SettingsCoinsCard({super.key, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return _SettingsGradientCard(
      gradientColors: const [
        Color(0xFFFFA726), // Amber
        Color(0xFFFF9800), // Darker amber
      ],
      icon: Icons.stars,
      title: 'Coins',
      subtitle: 'Poin loyalitas untuk diskon',
      onTap: onTap,
    );
  }
}

/// Reusable gradient card widget for settings
class _SettingsGradientCard extends StatelessWidget {
  final List<Color> gradientColors;
  final IconData icon;
  final String title;
  final String subtitle;
  final VoidCallback onTap;

  const _SettingsGradientCard({
    required this.gradientColors,
    required this.icon,
    required this.title,
    required this.subtitle,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.fromLTRB(16, 8, 16, 16),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: gradientColors,
        ),
        borderRadius: BorderRadius.circular(16),
        boxShadow: [
          BoxShadow(
            color: gradientColors.first.withValues(alpha: 0.3),
            blurRadius: 12,
            offset: const Offset(0, 6),
          ),
        ],
      ),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(16),
          child: Padding(
            padding: const EdgeInsets.all(20),
            child: Row(
              children: [
                Container(
                  padding: const EdgeInsets.all(12),
                  decoration: BoxDecoration(
                    color: AppColors.neutralWhite.withValues(alpha: 0.2),
                    borderRadius: BorderRadius.circular(12),
                  ),
                  child: Icon(icon, color: AppColors.neutralWhite, size: 28),
                ),
                const SizedBox(width: 16),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        title,
                        style: const TextStyle(
                          color: AppColors.neutralWhite,
                          fontSize: 18,
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        subtitle,
                        style: const TextStyle(
                          color: AppColors.neutralWhite,
                          fontSize: 14,
                          fontWeight: FontWeight.w500,
                        ),
                      ),
                    ],
                  ),
                ),
                const Icon(
                  Icons.arrow_forward_ios,
                  color: AppColors.neutralWhite,
                  size: 18,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
