import 'package:flutter/material.dart';

import 'package:hishumi/core/src/theme/app_theme.dart';

/// THE count badge renderer — one authority for every app-bar counter dot.
///
/// LAYOUT lives here (position, pill, padding, shadow, digit style); the
/// COUNT belongs to the caller's domain seam (chat unread, notification
/// unread, saved items, …). Before this widget the same ~40-line Stack was
/// copy-pasted across three badge widgets — the definition of two spellings
/// of one design drifting into two truths.
///
/// Digit style: the SIZE is the enshrined role `labelMicro` (the badge tail
/// converged into the ladder when these widgets were deduped); the semibold
/// weight, the tight 1.1 line height and the onError ink stay the renderer's
/// own decisions at the call site (recipes 3/4: a role is the size, not the
/// voice).
class CountBadgeOverlay extends StatelessWidget {
  const CountBadgeOverlay({
    super.key,
    required this.count,
    required this.child,
  });

  /// The count beyond which the label turns into `99+`. A SEMANTIC threshold,
  /// not geometry — it used to hide inside `AppMetrics.p99` and died with the
  /// spacing ladder.
  static const int plusThreshold = 99;

  /// Current count. Zero or negative renders the plain child, no badge.
  final int count;

  /// The icon/label the badge sits on.
  final Widget child;

  @override
  Widget build(BuildContext context) {
    if (count <= 0) {
      return child;
    }
    final colorScheme = Theme.of(context).colorScheme;

    return Stack(
      clipBehavior: Clip.none,
      children: [
        child,
        Positioned(
          right: -6,
          top: -6,
          child: Container(
            padding: const EdgeInsets.symmetric(
              horizontal: AppMetrics.p4,
              vertical: AppMetrics.p4,
            ),
            decoration: BoxDecoration(
              color: context.statusColors.error,
              borderRadius: BorderRadius.circular(AppShape.r10),
            ),
            constraints: const BoxConstraints(minWidth: 16, minHeight: 16),
            child: Text(
              count > plusThreshold ? '99+' : count.toString(),
              style: context.typeRoles.labelMicro.copyWith(
                color: colorScheme.onError,
                fontWeight: FontWeight.w600,
                height: 1.1,
              ),
              textAlign: TextAlign.center,
            ),
          ),
        ),
      ],
    );
  }
}
