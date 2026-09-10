import 'package:flutter/material.dart';

enum CommerceDetailValueLayout { auto, horizontal, vertical }

class CommerceDetailSectionCard extends StatelessWidget {
  final Widget child;
  final EdgeInsetsGeometry padding;
  final EdgeInsetsGeometry margin;
  final BorderRadiusGeometry borderRadius;
  final Color? backgroundColor;
  final BorderSide? borderSide;

  const CommerceDetailSectionCard({
    super.key,
    required this.child,
    this.padding = const EdgeInsets.all(16),
    this.margin = EdgeInsets.zero,
    this.borderRadius = const BorderRadius.all(Radius.circular(16)),
    this.backgroundColor,
    this.borderSide,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Container(
      margin: margin,
      padding: padding,
      decoration: BoxDecoration(
        color: backgroundColor ?? theme.colorScheme.surfaceContainerHighest,
        borderRadius: borderRadius,
        border: Border.all(
          color: borderSide?.color ?? theme.colorScheme.outlineVariant,
          width: borderSide?.width ?? 1,
        ),
      ),
      child: child,
    );
  }
}

class CommerceDetailAppBarActionButton extends StatelessWidget {
  final IconData icon;
  final IconData? activeIcon;
  final bool isActive;
  final bool isLoading;
  final String tooltip;
  final String semanticsLabel;
  final String? loadingTooltip;
  final String? loadingSemanticsLabel;
  final VoidCallback? onPressed;
  final Color? activeColor;
  final Color? inactiveColor;
  final Color? disabledColor;

  const CommerceDetailAppBarActionButton({
    super.key,
    required this.icon,
    required this.tooltip,
    required this.semanticsLabel,
    required this.onPressed,
    this.activeIcon,
    this.isActive = false,
    this.isLoading = false,
    this.loadingTooltip,
    this.loadingSemanticsLabel,
    this.activeColor,
    this.inactiveColor,
    this.disabledColor,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final enabled = onPressed != null && !isLoading;
    final resolvedColor = !enabled
        ? (disabledColor ??
              theme.colorScheme.onSurfaceVariant.withValues(alpha: 0.38))
        : isActive
        ? (activeColor ?? theme.colorScheme.primary)
        : (inactiveColor ?? theme.colorScheme.onSurfaceVariant);
    final effectiveTooltip = isLoading ? (loadingTooltip ?? tooltip) : tooltip;
    final effectiveSemanticsLabel = isLoading
        ? (loadingSemanticsLabel ?? semanticsLabel)
        : semanticsLabel;
    final resolvedIcon = isActive && activeIcon != null ? activeIcon! : icon;

    final iconWidget = isLoading
        ? SizedBox.square(
            dimension: 20,
            child: CircularProgressIndicator(
              strokeWidth: 2,
              valueColor: AlwaysStoppedAnimation<Color>(resolvedColor),
            ),
          )
        : Icon(resolvedIcon, size: 20, color: resolvedColor);

    return Semantics(
      button: true,
      selected: isActive,
      enabled: enabled,
      label: effectiveSemanticsLabel,
      child: Tooltip(
        message: effectiveTooltip,
        child: Material(
          type: MaterialType.transparency,
          child: InkResponse(
            onTap: enabled ? onPressed : null,
            containedInkWell: true,
            customBorder: const CircleBorder(),
            highlightShape: BoxShape.circle,
            radius: 24,
            child: SizedBox(
              width: 48,
              height: 48,
              child: Center(child: iconWidget),
            ),
          ),
        ),
      ),
    );
  }
}

class CommerceDetailLabelValue extends StatelessWidget {
  final String label;
  final String value;
  final CommerceDetailValueLayout layout;
  final EdgeInsetsGeometry padding;
  final TextStyle? labelStyle;
  final TextStyle? valueStyle;

  const CommerceDetailLabelValue({
    super.key,
    required this.label,
    required this.value,
    this.layout = CommerceDetailValueLayout.auto,
    this.padding = const EdgeInsets.only(bottom: 8),
    this.labelStyle,
    this.valueStyle,
  });

  bool get _shouldStack {
    if (layout == CommerceDetailValueLayout.vertical) {
      return true;
    }
    if (layout == CommerceDetailValueLayout.horizontal) {
      return false;
    }

    final normalizedLabel = label.trim().toLowerCase();
    const stackedLabels = <String>{
      'origin',
      'opsi pengiriman',
      'breeder',
      'bloodline',
      'sertifikat',
      'certificate',
      'certificates',
    };

    if (stackedLabels.contains(normalizedLabel)) {
      return true;
    }

    if (value.contains('\n')) {
      return true;
    }

    if (value.length >= 32) {
      return true;
    }

    if (value.contains(',') && value.length >= 24) {
      return true;
    }

    return false;
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final resolvedLabelStyle =
        labelStyle ??
        theme.textTheme.bodySmall?.copyWith(
          color: theme.colorScheme.onSurfaceVariant,
        );
    final resolvedValueStyle =
        valueStyle ??
        theme.textTheme.bodyMedium?.copyWith(
          fontWeight: FontWeight.w600,
          color: theme.colorScheme.onSurface,
        );

    return Padding(
      padding: padding,
      child: _shouldStack
          ? Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(label, style: resolvedLabelStyle, softWrap: true),
                const SizedBox(height: 4),
                Text(value, style: resolvedValueStyle, softWrap: true),
              ],
            )
          : Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(
                  flex: 4,
                  child: Text(label, style: resolvedLabelStyle, softWrap: true),
                ),
                const SizedBox(width: 12),
                Expanded(
                  flex: 6,
                  child: Text(
                    value,
                    style: resolvedValueStyle,
                    textAlign: TextAlign.end,
                    softWrap: true,
                  ),
                ),
              ],
            ),
    );
  }
}