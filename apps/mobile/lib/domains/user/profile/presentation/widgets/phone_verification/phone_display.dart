import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Displays the phone number being verified
class PhoneDisplay extends StatelessWidget {
  final String phoneNumber;
  final bool isDark;

  const PhoneDisplay({
    super.key,
    required this.phoneNumber,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
      decoration: BoxDecoration(
        color: isDark
            ? AppColors.darkGray700.withValues(alpha: 0.5)
            : AppColors.neutralGray50,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
        ),
      ),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.phone,
            size: 16,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
          ),
          const SizedBox(width: 8),
          Text(
            phoneNumber,
            style: TextStyle(
              fontSize: 15,
              fontWeight: FontWeight.w600,
              color: isDark
                  ? AppColors.neutralGray200
                  : AppColors.neutralGray900,
            ),
          ),
        ],
      ),
    );
  }
}
