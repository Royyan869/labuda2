import 'package:flutter/material.dart';

/// The one tone a confirming action may carry.
///
/// This is deliberately NOT a general colour knob. The app has exactly two
/// action roles inside a dialog: the neutral confirming action (brand ink from
/// the button theme) and the DESTRUCTIVE one (`scheme.error`). A status tone
/// (success/warning) as a CTA fill is the anti-pattern the button authority
/// already outlawed, so it is not a legal intent here.
enum AppDialogIntent { neutral, destructive }

/// The canonical dialog authority.
///
/// ONE compositor for the app's common dialog grammar:
///  * [confirm] — a yes/no decision that resolves to a `bool`;
///  * [info] — an acknowledge-only notice.
///
/// The shell is Flutter's [AlertDialog] on purpose: it already owns action
/// geometry, overflow, scrolling, inset policy, semantics and focus. This
/// authority owns only what every call site used to re-decide for itself:
///  1. the ACTION GRAMMAR — a cancel button plus exactly one confirming
///     button, in that order;
///  2. the TONE of the confirming action — [AppDialogIntent.neutral] leaves the
///     fill to the button theme, [AppDialogIntent.destructive] uses
///     `scheme.error`; and
///  3. the RESULT CONTRACT — [confirm] always resolves a non-nullable `bool`,
///     and a dismissed barrier is a cancel (`false`), never `null`.
///
/// It does NOT decide surface, shape, elevation, inset, padding or typography.
/// Those come from `dialogTheme`, the resolved [TextTheme] and Flutter's own
/// `AlertDialog`; a call site that restates them is a competing authority.
class AppDialog {
  AppDialog._();

  /// Show a confirmation dialog and resolve to the user's decision.
  ///
  /// Returns `true` only when the confirming action is tapped. Cancelling the
  /// dialog or dismissing it through the barrier resolves `false`.
  ///
  /// BOUNDED-CONTENT CONTRACT: same principle as [info] — the dialog must
  /// never overflow merely because caller-provided `content` is taller than
  /// the available viewport. [AlertDialog.scrollable] is enabled so the
  /// framework bounds the dialog to the available height and scrolls the
  /// title+body internally while pinning both actions. Short content renders
  /// exactly as before — a scroll view that fits its child paints no
  /// scrollbar and takes the child's height.
  static Future<bool> confirm({
    required BuildContext context,
    required String title,
    String? message,
    Widget? content,
    String confirmLabel = 'OK',
    String cancelLabel = 'Batal',
    AppDialogIntent intent = AppDialogIntent.neutral,
    bool barrierDismissible = true,
  }) async {
    assert(
      message != null || content != null,
      'AppDialog.confirm needs a message or custom content',
    );

    final confirmed = await showDialog<bool>(
      context: context,
      barrierDismissible: barrierDismissible,
      builder: (dialogContext) {
        final scheme = Theme.of(dialogContext).colorScheme;
        return AlertDialog(
          // Bounded height + internal scroll, owned by the framework — the
          // same contract as [info]. Surface/shape/inset/typography still
          // come from `dialogTheme` and the resolved [TextTheme]; this is a
          // layout-mechanic flag, not a style.
          scrollable: true,
          title: Text(title),
          content: content ?? Text(message!),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(dialogContext).pop(false),
              child: Text(cancelLabel),
            ),
            ElevatedButton(
              onPressed: () => Navigator.of(dialogContext).pop(true),
              style: intent == AppDialogIntent.destructive
                  ? ElevatedButton.styleFrom(
                      backgroundColor: scheme.error,
                      foregroundColor: scheme.onError,
                    )
                  : null,
              child: Text(confirmLabel),
            ),
          ],
        );
      },
    );

    return confirmed ?? false;
  }

  /// Show an acknowledge-only dialog. Resolves when the dialog closes.
  ///
  /// LONG-CONTENT CONTRACT: this is the canonical Page Info / Help surface, so
  /// it must never overflow on a long payload. The shell is still Flutter's
  /// [AlertDialog]; [AlertDialog.scrollable] is enabled so the framework bounds
  /// the dialog to the available height and scrolls the title+body internally
  /// while pinning the single close action. Short content renders exactly as
  /// before — a scroll view that fits its child paints no scrollbar and takes
  /// the child's height. No call site may add its own height/scroll padding
  /// workaround.
  static Future<void> info({
    required BuildContext context,
    required String title,
    String? message,
    Widget? content,
    String closeLabel = 'OK',
    bool barrierDismissible = true,
  }) async {
    assert(
      message != null || content != null,
      'AppDialog.info needs a message or custom content',
    );

    await showDialog<void>(
      context: context,
      barrierDismissible: barrierDismissible,
      builder: (dialogContext) => AlertDialog(
        // Bounded height + internal scroll, owned by the framework. The
        // surface/shape/inset/typography still come from `dialogTheme` and the
        // resolved [TextTheme]; this is a layout-mechanic flag, not a style.
        scrollable: true,
        title: Text(title),
        content: content ?? Text(message!),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(),
            child: Text(closeLabel),
          ),
        ],
      ),
    );
  }
}
