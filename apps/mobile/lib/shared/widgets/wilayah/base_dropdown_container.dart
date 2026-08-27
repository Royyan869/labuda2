/// Base Dropdown Container
///
/// Container reusable untuk dropdown wilayah
library;

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Base container untuk dropdown dengan styling konsisten
class BaseDropdownContainer extends StatelessWidget {
  final String? labelText;
  final bool isDark;
  final Widget child;

  const BaseDropdownContainer({
    super.key,
    this.labelText,
    required this.isDark,
    required this.child,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (labelText != null) ...[
          Text(
            labelText!,
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w500,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray700,
            ),
          ),
          const SizedBox(height: 8),
        ],
        Container(
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(12),
            border: Border.all(
              color: isDark ? AppColors.darkGray600 : AppColors.neutralGray300,
            ),
            color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
          ),
          child: child,
        ),
      ],
    );
  }
}
