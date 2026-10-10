import 'package:flutter/material.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';

/// Canonical composer action-row buttons.
///
/// One visual authority for every composer action row (chat, comment,
/// share-to-chat, support). The row layout is always:
///
///   [pill textarea (Expanded)] [ComposerAddButton?] [ComposerSendButton]
///
/// The send button is ALWAYS visible — it is disabled (gray) until the
/// composer can submit, so the row never shifts when the draft changes.
/// The `+` attach button only exists in composers that can attach
/// (chat, comment); it sits to the left of send, never to the left of
/// the textarea.
class ComposerAddButton extends StatelessWidget {
  final VoidCallback onPressed;

  const ComposerAddButton({super.key, required this.onPressed});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return IconButton(
      onPressed: onPressed,
      tooltip: 'Tambah lampiran',
      icon: Icon(Icons.add_circle, color: scheme.primary, size: AppIconSize.emphasis),
    );
  }
}

/// Send button: filled primary circle when it can submit, gray circle
/// while disabled, primary circle with spinner while the send is in
/// flight. Pass `onPressed: null` to disable.
class ComposerSendButton extends StatelessWidget {
  final VoidCallback? onPressed;
  final bool loading;
  final String tooltip;

  const ComposerSendButton({
    super.key,
    this.onPressed,
    this.loading = false,
    this.tooltip = 'Kirim',
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final canTap = onPressed != null;
    return Container(
      decoration: BoxDecoration(
        color: loading || canTap
            ? scheme.primary
            : scheme.surfaceContainerHighest,
        shape: BoxShape.circle,
      ),
      child: IconButton(
        onPressed: loading ? null : onPressed,
        tooltip: tooltip,
        icon: loading
            ? SizedBox(
                width: 20,
                height: 20,
                child: CircularProgressIndicator(
                  strokeWidth: 2,
                  valueColor: AlwaysStoppedAnimation<Color>(scheme.onPrimary),
                ),
              )
            : Icon(
                Icons.send,
                color: canTap ? scheme.onPrimary : scheme.onSurfaceVariant,
              ),
      ),
    );
  }
}
