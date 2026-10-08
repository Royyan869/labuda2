/// Dropdown State Builders
///
/// ONE authority for the states a wilayah field shows while it has no dropdown
/// to show: disabled, empty, loading, error.
///
/// This file used to be the canonical version of a row that three other files
/// (`wilayah/city_dropdown`, `wilayah/district_dropdown`, `village_dropdown`)
/// re-implemented privately, verbatim, four methods each — while
/// `wilayah/province_dropdown` consumed this one. So the app held two competing
/// authorities for the same row and only one of them was maintained.
library;

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Helper class untuk membuat state builders pada dropdown wilayah
class DropdownStateBuilders {
  const DropdownStateBuilders._();

  /// The row that STANDS IN for the field while the real dropdown is not shown.
  ///
  /// CONTENT-DRIVEN, deliberately. This used to be `height: 50` — one literal
  /// promise repeated in all four states here AND in the three private copies
  /// above, so a single step moving in the type ladder could overflow every
  /// wilayah field in the app at once, and nothing in the source said how much
  /// room was left. The padding now mirrors exactly what the live
  /// `DropdownButtonFormField` gets in the callers (`contentPadding` horizontal
  /// `p16` + vertical `p16`), so a stand-in row and the real field are the same
  /// height without anyone summing up 50, and a taller type step grows the row
  /// instead of clipping it.
  static Widget fieldRow({
    required List<Widget> children,
    BoxDecoration? decoration,
  }) {
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: decoration,
      child: Row(children: children),
    );
  }

  /// The optional leading icon and its gap — one spelling for four states.
  static List<Widget> _leading(
    BuildContext context,
    IconData? prefixIcon, {
    Color? color,
  }) {
    if (prefixIcon == null) return const <Widget>[];
    return <Widget>[
      Icon(
        prefixIcon,
        color: color ?? Theme.of(context).colorScheme.onSurfaceVariant,
      ),
      const SizedBox(width: AppMetrics.p12),
    ];
  }

  /// Build disabled dropdown state
  static Widget buildDisabled({
    required BuildContext context,
    required String text,
    IconData? prefixIcon,
  }) {
    final scheme = Theme.of(context).colorScheme;
    return fieldRow(
      children: [
        ..._leading(context, prefixIcon),
        Text(
          text,
          style: Theme.of(
            context,
          ).textTheme.bodyLarge?.copyWith(color: scheme.onSurfaceVariant),
        ),
      ],
    );
  }

  /// Build empty dropdown state
  ///
  /// The SAME body as [buildDisabled], on purpose: an empty list and a locked
  /// field read identically to the eye, and the two identical bodies that used
  /// to sit here are how a helper grows a second spelling. The two names stay
  /// because the CALLER means two different things.
  static Widget buildEmpty({
    required BuildContext context,
    required String text,
    IconData? prefixIcon,
  }) {
    return buildDisabled(context: context, text: text, prefixIcon: prefixIcon);
  }

  /// Build loading dropdown state
  static Widget buildLoading({
    required BuildContext context,
    required String text,
    IconData? prefixIcon,
  }) {
    final scheme = Theme.of(context).colorScheme;
    return fieldRow(
      children: [
        ..._leading(context, prefixIcon),
        const SizedBox(
          width: AppIconSize.action,
          height: AppIconSize.action,
          child: CircularProgressIndicator(strokeWidth: 2),
        ),
        const SizedBox(width: AppMetrics.p12),
        Text(text, style: TextStyle(color: scheme.onSurfaceVariant)),
      ],
    );
  }

  /// Build error dropdown state
  static Widget buildError({
    required BuildContext context,
    required String text,
    IconData? prefixIcon,
  }) {
    final error = context.statusColors.error;
    return fieldRow(
      children: [
        ..._leading(context, prefixIcon, color: error),
        Icon(Icons.error_outline, color: error, size: AppIconSize.action),
        const SizedBox(width: AppMetrics.p8),
        Text(text, style: TextStyle(color: error)),
      ],
    );
  }
}
