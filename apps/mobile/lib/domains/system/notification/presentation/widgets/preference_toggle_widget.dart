import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

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
    final scheme = Theme.of(context).colorScheme;
    final effectiveEnabled = enabled;
    final effectiveValue = enabled && value;

    return Material(
      color: scheme.surface,
      child: InkWell(
        onTap: effectiveEnabled ? () => onChanged(!value) : null,
        child: Padding(
          padding: const EdgeInsets.symmetric(
            horizontal: AppMetrics.p16,
            vertical: AppMetrics.p12,
          ),
          child: Row(
            children: [
              // Icon
              Container(
                width: 40,
                height: 40,
                decoration: BoxDecoration(
                  color: effectiveEnabled
                      ? iconColor.withValues(alpha: 0.1)
                      : scheme.surfaceContainerHighest,
                  borderRadius: BorderRadius.circular(AppShape.r10),
                ),
                child: Icon(
                  icon,
                  color: effectiveEnabled ? iconColor : scheme.onSurfaceVariant,
                  size: AppIconSize.header,
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
                      style: context.typeRoles.titleCompact.copyWith(
                        fontWeight: FontWeight.w500,
                        color: effectiveEnabled
                            ? scheme.onSurface
                            : scheme.onSurfaceVariant,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      subtitle,
                      style: context.typeRoles.bodyDense.copyWith(
                        color: effectiveEnabled
                            ? scheme.onSurfaceVariant
                            : scheme.onSurfaceVariant,
                        height: 1.3,
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(width: 12),

              // Toggle switch — colour/state comes from `switchTheme`
              // (AppTheme), the one selection-control authority.
              Switch(
                value: effectiveValue,
                onChanged: effectiveEnabled ? onChanged : null,
              ),
            ],
          ),
        ),
      ),
    );
  }
}
