import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

/// Empty State Widget untuk Address List
class AddressEmptyStateWidget extends StatelessWidget {
  final VoidCallback onAddAddress;
  final bool isDark;

  const AddressEmptyStateWidget({
    super.key,
    required this.onAddAddress,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          // Icon
          Container(
            padding: const EdgeInsets.all(24),
            decoration: BoxDecoration(
              color: AppColors.primaryRed.withValues(alpha: 0.1),
              shape: BoxShape.circle,
            ),
            child: Icon(
              Icons.location_on_outlined,
              size: 64,
              color: AppColors.primaryRed,
            ),
          ),
          const SizedBox(height: 24),

          // Title
          Text(
            'No Address Yet',
            style: TextStyle(
              fontSize: 20,
              fontWeight: FontWeight.bold,
              color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 8),

          // Description
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 48),
            child: Text(
              'Add your shipping address to start shopping',
              textAlign: TextAlign.center,
              style: TextStyle(
                fontSize: 14,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
              ),
            ),
          ),
          const SizedBox(height: 32),

          // Add Button
          SizedBox(
            width: 200,
            child: AppButton.primary(
              text: 'Add Address',
              onPressed: onAddAddress,
            ),
          ),
        ],
      ),
    );
  }
}
