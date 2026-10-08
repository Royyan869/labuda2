import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Canonical presentation modes for the Metadata semantic family.
///
/// Owner-locked: ONE metadata family, TWO canonical presentation modes.
///
/// - [compact]: a single bounded line; truncates with an ellipsis so the
///   fact can never widen its row. The row must not grow vertically because
///   of metadata text.
/// - [detail]: full readability is preferred; text may wrap onto multiple
///   lines and the row/container may grow vertically. A caller-supplied
///   [MetadataText.maxLines]/[MetadataView.maxLines] limit is honoured only
///   when the surrounding context intentionally requires one.
///
/// A standalone secondary fact (icon + email, icon + date, a timestamp, a
/// count) belongs here. A label → value relationship (Email: value) belongs
/// to the Label/Value family, never to this authority.
enum MetadataMode { compact, detail }

/// THE canonical Metadata text authority.
///
/// It owns the *text strategy* for standalone secondary facts and nothing
/// else, so no screen independently invents a bare `Text`, an arbitrary
/// `maxLines`, or arbitrary wrap/ellipsis behaviour for metadata.
///
/// - compact: `maxLines: 1` + `TextOverflow.ellipsis`.
/// - detail: soft-wrapped; no line cap unless the surrounding context
///   supplies one through [maxLines] (then with an ellipsis).
///
/// Use this directly when the caller already owns its row/column; use
/// [MetadataView] for the canonical leading-visual + fact composition.
class MetadataText extends StatelessWidget {
  const MetadataText({
    super.key,
    required this.text,
    this.mode = MetadataMode.compact,
    this.maxLines,
    this.style,
    this.textAlign,
  });

  final String text;
  final MetadataMode mode;
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
      case MetadataMode.compact:
        return Text(
          text,
          style: resolvedStyle,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          textAlign: textAlign,
        );
      case MetadataMode.detail:
        return Text(
          text,
          style: resolvedStyle,
          maxLines: maxLines,
          overflow: maxLines == null ? null : TextOverflow.ellipsis,
          textAlign: textAlign,
          softWrap: true,
        );
    }
  }
}

/// THE canonical Metadata horizontal composition.
///
/// A bounded leading slot (an [icon], or a caller-supplied [leading])
/// followed by the dynamic fact text, which is flex-constrained, plus an
/// optional trailing affordance. Compact keeps the fact on one ellipsised
/// line; detail lets it wrap within the incoming width. It performs no
/// viewport arithmetic and never fixes a width.
///
/// The leading/trailing slots must be intrinsically bounded content (an
/// icon or similar glyph-sized visual). The authority cannot accept
/// arbitrary unconstrained horizontal content there without defeating the
/// contract, so callers must not pass expanding widgets as [leading] or
/// [trailing].
class MetadataView extends StatelessWidget {
  const MetadataView({
    super.key,
    required this.text,
    this.mode = MetadataMode.compact,
    this.icon,
    this.iconSize = AppIconSize.inlineGlyph,
    this.iconColor,
    this.leading,
    this.trailing,
    this.spacing = 8,
    this.maxLines,
    this.style,
    this.textAlign,
    this.crossAxisAlignment = CrossAxisAlignment.center,
    this.mainAxisSize = MainAxisSize.max,
  });

  final String text;
  final MetadataMode mode;
  final IconData? icon;
  final double iconSize;
  final Color? iconColor;
  final Widget? leading;
  final Widget? trailing;
  final double spacing;
  final int? maxLines;
  final TextStyle? style;
  final TextAlign? textAlign;
  final CrossAxisAlignment crossAxisAlignment;
  final MainAxisSize mainAxisSize;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    final Widget body = MetadataText(
      text: text,
      mode: mode,
      maxLines: maxLines,
      style: style,
      textAlign: textAlign,
    );

    final Widget? head =
        leading ??
        (icon == null
            ? null
            : Icon(
                icon,
                size: iconSize,
                color: iconColor ?? scheme.onSurfaceVariant,
              ));

    return Row(
      mainAxisSize: mainAxisSize,
      crossAxisAlignment: crossAxisAlignment,
      children: [
        if (head != null) ...[
          head,
          SizedBox(width: spacing),
        ],
        if (mode == MetadataMode.detail)
          Expanded(child: body)
        else
          Flexible(child: body),
        if (trailing != null) ...[
          SizedBox(width: spacing),
          trailing!,
        ],
      ],
    );
  }
}
