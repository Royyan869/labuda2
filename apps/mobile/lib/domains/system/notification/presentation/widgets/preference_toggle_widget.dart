import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Preference Toggle Widget
///
/// Reusable toggle widget untuk notification preferences.
/// Professional design dengan disabled state.
///
/// Size: < 150 lines (per GUIDELINES)
class PreferenceToggleWidget extends StatelessWidget {
  final IconData icon;
  final Color iconColor;
  final String title;
  final String subtitle;
  final bool value;
  final bool enabled;
  final ValueChanged<bool> onChanged;

  const PreferenceToggleWidget({
    super.key,
    required this.icon,
    required this.iconColor,
    required this.title,
    required this.subtitle,
    required this.value,
    this.enabled = true,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final effectiveEnabled = enabled;
    final effectiveValue = enabled && value;

    return Material(
      color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
      child: InkWell(
        onTap: effectiveEnabled ? () => onChanged(!value) : null,
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          child: Row(
            children: [
              // Icon
              Container(
                width: 40,
                height: 40,
                decoration: BoxDecoration(
                  color: effectiveEnabled
                      ? iconColor.withValues(alpha: 0.1)
                      : (isDark
                            ? AppColors.darkGray700
                            : AppColors.neutralGray100),
                  borderRadius: BorderRadius.circular(10),
                ),
                child: Icon(
                  icon,
                  color: effectiveEnabled
                      ? iconColor
                      : (isDark
                            ? AppColors.neutralGray600
                            : AppColors.neutralGray400),
                  size: 22,
                ),
              ),
              const SizedBox(width: 12),

              // Text
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      title,
                      style: TextStyle(
                        fontSize: 15,
                        fontWeight: FontWeight.w500,
                        color: effectiveEnabled
                            ? (isDark
                                  ? AppColors.neutralGray100
                                  : AppColors.neutralGray900)
                            : (isDark
                                  ? AppColors.neutralGray600
                                  : AppColors.neutralGray500),
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      subtitle,
                      style: TextStyle(
                        fontSize: 13,
                        color: effectiveEnabled
                            ? (isDark
                                  ? AppColors.neutralGray400
                                  : AppColors.neutralGray600)
                            : (isDark
                                  ? AppColors.neutralGray700
                                  : AppColors.neutralGray400),
                        height: 1.3,
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(width: 12),

              // Toggle switch
              Switch(
                value: effectiveValue,
                onChanged: effectiveEnabled ? onChanged : null,
                activeTrackColor: AppColors.primaryRed,
                activeThumbColor: AppColors.neutralWhite,
                inactiveTrackColor: isDark
                    ? AppColors.darkGray600
                    : AppColors.neutralGray300,
              ),
            ],
          ),
        ),
      ),
    );
  }
}
