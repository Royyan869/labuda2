import 'package:flutter/material.dart';

import 'payment_method_visuals.dart';

/// Canonical renderer for a [PaymentMethodVisual]: the primary visual (a local
/// asset when defined, otherwise the generic fallback icon) plus optional
/// secondary brand marks.
///
/// Presentation-only. It NEVER owns a method map or payment business logic;
/// callers must resolve the visual through [PaymentMethodVisuals.visual].
class PaymentMethodLogo extends StatelessWidget {
  final PaymentMethodVisual visual;

  /// Height of the primary visual in logical pixels.
  final double size;

  /// Whether to render [PaymentMethodVisual.secondaryBrands] beside the
  /// primary visual.
  final bool showSecondaryBrands;

  /// Hard cap so a multi-brand method can never overflow a tight row; the whole
  /// mark group scales down to fit.
  final double maxWidth;

  const PaymentMethodLogo({
    super.key,
    required this.visual,
    this.size = 28,
    this.showSecondaryBrands = true,
    this.maxWidth = 116,
  });

  @override
  Widget build(BuildContext context) {
    final Color fallbackColor = Theme.of(context).colorScheme.onSurfaceVariant;

    final Widget primary = visual.primaryAsset != null
        ? Image.asset(
            visual.primaryAsset!,
            height: size,
            fit: BoxFit.contain,
            semanticLabel: visual.label,
            errorBuilder: (context, error, stackTrace) => Icon(
              visual.fallbackIcon,
              size: size,
              color: fallbackColor,
            ),
          )
        : Icon(visual.fallbackIcon, size: size, color: fallbackColor);

    final List<Widget> marks = <Widget>[
      primary,
      if (showSecondaryBrands)
        for (final PaymentMethodBrandVisual brand in visual.secondaryBrands)
          Padding(
            padding: const EdgeInsets.only(left: 6),
            child: Image.asset(
              brand.asset,
              height: size * 0.6,
              fit: BoxFit.contain,
              semanticLabel: brand.name,
              errorBuilder: (context, error, stackTrace) =>
                  const SizedBox.shrink(),
            ),
          ),
    ];

    return ConstrainedBox(
      constraints: BoxConstraints(maxWidth: maxWidth),
      child: FittedBox(
        fit: BoxFit.scaleDown,
        alignment: Alignment.centerLeft,
        child: Row(mainAxisSize: MainAxisSize.min, children: marks),
      ),
    );
  }
}
