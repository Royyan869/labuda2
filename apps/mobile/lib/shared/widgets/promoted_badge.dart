/// Promoted Badge Widget (Phase 4)
///
/// A simple, clear disclosure widget for promoted content.
/// Per Rule 1: DISCLOSURE IS MANDATORY - All promoted items must be clearly labeled.
library;

import 'package:flutter/material.dart';

/// A simple "Promoted" badge for disclosure
///
/// This badge should be displayed on all promoted content to comply with
/// honest advertising practices. The styling is intentionally minimal and clean.
class PromotedBadge extends StatelessWidget {
  final String? label;
  final PromotedBadgeStyle style;

  const PromotedBadge({
    super.key,
    this.label,
    this.style = PromotedBadgeStyle.pill,
  });

  /// Default pill-style badge
  factory PromotedBadge.pill({String? label}) {
    return PromotedBadge(label: label, style: PromotedBadgeStyle.pill);
  }

  /// Compact chip-style badge for tight spaces
  factory PromotedBadge.chip({String? label}) {
    return PromotedBadge(label: label, style: PromotedBadgeStyle.chip);
  }

  /// Text-only minimal badge
  factory PromotedBadge.text({String? label}) {
    return PromotedBadge(label: label, style: PromotedBadgeStyle.text);
  }

  @override
  Widget build(BuildContext context) {
    final badgeLabel = label ?? 'Dipromosikan';
    final theme = Theme.of(context);

    switch (style) {
      case PromotedBadgeStyle.pill:
        return _buildPill(context, badgeLabel, theme);

      case PromotedBadgeStyle.chip:
        return _buildChip(context, badgeLabel, theme);

      case PromotedBadgeStyle.text:
        return _buildText(context, badgeLabel, theme);
    }
  }

  Widget _buildPill(BuildContext context, String label, ThemeData theme) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: theme.colorScheme.secondary.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: theme.colorScheme.secondary.withValues(alpha: 0.3),
          width: 1,
        ),
      ),
      child: Text(
        label,
        style: TextStyle(
          fontSize: 11,
          fontWeight: FontWeight.w600,
          color: theme.colorScheme.secondary.withValues(alpha: 0.8),
          letterSpacing: 0.3,
        ),
      ),
    );
  }

  Widget _buildChip(BuildContext context, String label, ThemeData theme) {
    return Chip(
      label: Text(
        label,
        style: TextStyle(
          fontSize: 10,
          fontWeight: FontWeight.w600,
          color: theme.colorScheme.secondary.withValues(alpha: 0.8),
        ),
      ),
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      labelPadding: const EdgeInsets.symmetric(horizontal: 4),
      backgroundColor: theme.colorScheme.secondary.withValues(alpha: 0.1),
      side: BorderSide(
        color: theme.colorScheme.secondary.withValues(alpha: 0.3),
        width: 1,
      ),
      visualDensity: VisualDensity.compact,
    );
  }

  Widget _buildText(BuildContext context, String label, ThemeData theme) {
    return Text(
      label,
      style: TextStyle(
        fontSize: 10,
        fontWeight: FontWeight.w600,
        color: theme.colorScheme.secondary.withValues(alpha: 0.7),
        letterSpacing: 0.5,
      ),
    );
  }
}

/// Style options for the promoted badge
enum PromotedBadgeStyle {
  /// Pill-shaped container with border and background
  pill,

  /// Compact chip style
  chip,

  /// Minimal text-only style
  text,
}

/// Extension to check if a SearchResult is promoted
extension SearchResultPromotedExtension on dynamic {
  /// Check if this result has promotion data
  /// Works with SearchResult and any object with isPromoted property
  bool get isPromoted {
    if (this is Map<String, dynamic>) {
      return (this as Map<String, dynamic>)['isPromoted'] == true;
    }
    return false;
  }

  /// Get the promotion instance ID if promoted
  String? get promotionInstanceId {
    if (this is Map<String, dynamic>) {
      return (this as Map<String, dynamic>)['promotionInstanceId'] as String?;
    }
    return null;
  }
}

/// A widget that conditionally shows the PromotedBadge based on isPromoted flag
class MaybePromotedBadge extends StatelessWidget {
  final bool isPromoted;
  final String? label;
  final PromotedBadgeStyle style;

  const MaybePromotedBadge({
    super.key,
    required this.isPromoted,
    this.label,
    this.style = PromotedBadgeStyle.pill,
  });

  @override
  Widget build(BuildContext context) {
    if (!isPromoted) {
      return const SizedBox.shrink();
    }
    return PromotedBadge(label: label, style: style);
  }
}
