import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Stub widget for SettingsUpgradeCard
/// TODO: Implement seller upgrade flow
class SettingsUpgradeCard extends StatelessWidget {
  final VoidCallback onUpgrade;

  const SettingsUpgradeCard({super.key, required this.onUpgrade});

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.fromLTRB(16, 8, 16, 16),
      decoration: BoxDecoration(
        gradient: const LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [
            Color(0xFF10B981), // Emerald green
            Color(0xFF059669), // Darker emerald
          ],
        ),
        borderRadius: BorderRadius.circular(16),
        boxShadow: [
          BoxShadow(
            color: const Color(0xFF10B981).withValues(alpha: 0.3),
            blurRadius: 12,
            offset: const Offset(0, 6),
          ),
        ],
      ),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: onUpgrade,
          borderRadius: BorderRadius.circular(16),
          child: const Padding(
            padding: EdgeInsets.all(20),
            child: Row(
              children: [
                Icon(
                  Icons.store_outlined,
                  color: AppColors.neutralWhite,
                  size: 28,
                ),
                SizedBox(width: 16),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Become a Seller',
                        style: TextStyle(
                          color: AppColors.neutralWhite,
                          fontSize: 18,
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                      SizedBox(height: 4),
                      Text(
                        'Start selling your koi products',
                        style: TextStyle(
                          color: AppColors.neutralWhite,
                          fontSize: 14,
                          fontWeight: FontWeight.w500,
                        ),
                      ),
                    ],
                  ),
                ),
                Icon(
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
