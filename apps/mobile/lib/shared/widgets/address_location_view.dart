import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Canonical presentation modes for the Address/Location semantic family.
///
/// Owner-locked: ONE semantic family, TWO canonical presentation modes.
///
/// - [compact]: a single bounded line; truncates with an ellipsis so the
///   address can never widen its row.
/// - [detail]: wrapping text, horizontally bounded by its parent; it may grow
///   vertically and is only line-capped when the surrounding context requires
///   one (passed through `maxLines`).
enum AddressLocationMode { compact, detail }

/// THE canonical Address/Location text authority.
///
/// It owns the *text strategy* for address/location content and nothing else,
/// so no screen independently invents a bare `Text`, an arbitrary `maxLines`,
/// or arbitrary wrap/ellipsis behaviour.
///
/// - compact: `maxLines: 1` + `TextOverflow.ellipsis`.
/// - detail:  soft-wrapped; no line cap unless the surrounding context supplies
///   one through [maxLines].
///
/// Use this directly when the caller already owns its row/column (a card, a
/// labelled section); use [AddressLocationView] for the canonical icon +
/// location composition.
class AddressLocationText extends StatelessWidget {
  const AddressLocationText({
    super.key,
    required this.location,
    this.mode = AddressLocationMode.compact,
    this.maxLines,
    this.style,
    this.textAlign,
  });

  final String location;
  final AddressLocationMode mode;
  final int? maxLines;
  final TextStyle? style;
  final TextAlign? textAlign;

  @override
  Widget build(BuildContext context) {
    final resolvedStyle =
        style ??
        context.typeRoles.bodyDense.copyWith(
          color: Theme.of(context).colorScheme.onSurfaceVariant,
        );

    switch (mode) {
      case AddressLocationMode.compact:
        return Text(
          location,
          style: resolvedStyle,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          textAlign: textAlign,
        );
      case AddressLocationMode.detail:
        return Text(
          location,
          style: resolvedStyle,
          maxLines: maxLines,
          overflow: maxLines == null ? null : TextOverflow.ellipsis,
          textAlign: textAlign,
        );
    }
  }
}

/// THE canonical Address/Location horizontal composition.
///
/// A bounded leading slot (an [icon], or a caller-supplied [leading]) followed
/// by the dynamic location text, which is flex-constrained, plus an optional
/// trailing affordance. Compact keeps the address on one ellipsised line;
/// detail lets it wrap within the incoming width. It performs no viewport
/// arithmetic and never fixes a width.
class AddressLocationView extends StatelessWidget {
  const AddressLocationView({
    super.key,
    required this.location,
    this.mode = AddressLocationMode.compact,
    this.icon = Icons.location_on_outlined,
    this.iconSize = AppIconSize.inlineGlyph,
    this.iconColor,
    this.leading,
    this.trailing,
    this.spacing = 8,
    this.maxLines,
    this.style,
    this.crossAxisAlignment = CrossAxisAlignment.center,
    this.mainAxisSize = MainAxisSize.max,
  });

  final String location;
  final AddressLocationMode mode;
  final IconData icon;
  final double iconSize;
  final Color? iconColor;
  final Widget? leading;
  final Widget? trailing;
  final double spacing;
  final int? maxLines;
  final TextStyle? style;
  final CrossAxisAlignment crossAxisAlignment;
  final MainAxisSize mainAxisSize;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    final Widget text = AddressLocationText(
      location: location,
      mode: mode,
      maxLines: maxLines,
      style: style,
    );

    return Row(
      mainAxisSize: mainAxisSize,
      crossAxisAlignment: crossAxisAlignment,
      children: [
        leading ??
            Icon(
              icon,
              size: iconSize,
              color: iconColor ?? scheme.onSurfaceVariant,
            ),
        SizedBox(width: spacing),
        if (mode == AddressLocationMode.detail)
          Expanded(child: text)
        else
          Flexible(child: text),
        if (trailing != null) ...[
          SizedBox(width: spacing),
          trailing!,
        ],
      ],
    );
  }
}
