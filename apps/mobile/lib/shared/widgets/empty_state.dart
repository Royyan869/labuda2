import 'package:flutter/material.dart';
import 'package:hishumi/core/core.dart';

/// Semantic marker for the empty-state family. One value only: a successful
/// zero-item result is the ONLY thing this foundation renders.
enum EmptyStateType {
  noData,
}

/// Empty State Widget — the SINGLE visual authority for empty states.
///
/// CANONICAL scope: a **successful** request whose result has zero items.
/// Never Loading (data still fetching) and Never Error (the request failed) —
/// those live in `LoadingIndicator` and `PageErrorState`. The widget takes no
/// retry behavior and no technical error input.
///
/// SEMANTIC FAMILIES share this one renderer; the family shows up as copy +
/// at most ONE primary action:
/// - Collection empty — title/subtitle/icon, no action.
/// - Search / filter empty — "no match" copy plus a reset action whose
///   callback clears the active query/filter. Only pass an action when the
///   screen really can be reset.
/// - First-use empty — copy that invites the user to start, plus one primary
///   action (create / explore). Never two primary actions.
///
/// SCOPE: full-page / page-body empty. Section-bounded empty keeps its own
/// specialized widget when the layout is genuinely different (card, info
/// note, badge). Not for inline validation, not-found, or unavailable states.
class EmptyState extends StatelessWidget {
  const EmptyState({
    super.key,
    required this.title,
    this.subtitle,
    this.icon,
    this.type = EmptyStateType.noData,
    this.actionLabel,
    this.onAction,
  });

  /// Headline of the empty state (localized copy).
  final String title;

  /// Supporting explanation (localized copy).
  final String? subtitle;

  /// Domain glyph; falls back to the canonical empty icon for [type].
  final IconData? icon;

  final EmptyStateType type;

  /// The ONE primary action label (localized). Requires [onAction].
  final String? actionLabel;

  /// The ONE primary action callback: reset, create or explore.
  final VoidCallback? onAction;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final hasAction = actionLabel != null && onAction != null;

    // One semantic container so a screen reader announces the whole state;
    // no live region — an empty state is static, not an announcement.
    return Semantics(
      container: true,
      child: Padding(
        padding: const EdgeInsets.all(AppMetrics.p48),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          mainAxisSize: MainAxisSize.min,
          children: [
            // Icon is decorative: the text below carries the meaning.
            ExcludeSemantics(
              child: _buildIcon(context, scheme),
            ),
            const SizedBox(height: 24),
            Text(
              title,
              style: context.typeRoles.titleProminent.copyWith(
                fontWeight: FontWeight.w600,
                color: scheme.onSurface,
              ),
              textAlign: TextAlign.center,
            ),
            if (subtitle != null) ...[
              const SizedBox(height: 12),
              Text(
                subtitle!,
                style: context.typeRoles.bodyDense.copyWith(
                  color: scheme.onSurfaceVariant,
                ),
                textAlign: TextAlign.center,
              ),
            ],
            if (hasAction) ...[
              const SizedBox(height: 32),
              SizedBox(
                width: AppContentSize.actionWidth,
                child: FilledButton(
                  onPressed: onAction,
                  child: Text(actionLabel!),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildIcon(BuildContext context, ColorScheme scheme) {
    // Empty is never the error role: neutral variant ink, canonical size.
    final IconData iconData = icon ?? switch (type) {
      EmptyStateType.noData => Icons.inbox_outlined,
    };
    final Color iconColor = scheme.onSurfaceVariant;

    return Container(
      width: 80,
      height: 80,
      decoration: BoxDecoration(
        color: iconColor.withValues(alpha: 0.1),
        shape: BoxShape.circle,
      ),
      child: Icon(iconData, size: AppIconSize.display, color: iconColor),
    );
  }
}
