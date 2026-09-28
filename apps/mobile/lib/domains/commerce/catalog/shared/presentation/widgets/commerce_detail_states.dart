import 'package:flutter/material.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Canonical DETAIL state surfaces — ONE AUTHORITY for both sale channels.
///
/// Loading, not-found and load-error render the same skeleton, typography and
/// app bar on ForSale and Auction detail, so a channel never owns its own
/// state vocabulary. Error copy must stay user-facing (no raw exception text).
class CommerceDetailStates {
  const CommerceDetailStates._();

  static Scaffold loading({required String title}) {
    return Scaffold(
      appBar: AppBarCustom(title: title),
      body: const Center(child: CircularProgressIndicator()),
    );
  }

  static Scaffold notFound({
    required String title,
    required String headline,
    String? message,
    String actionLabel = 'Kembali',
    VoidCallback? onAction,
  }) {
    return Scaffold(
      appBar: AppBarCustom(title: title),
      body: _StateBody(
        icon: Icons.search_off,
        iconColorRole: _IconColorRole.neutral,
        headline: headline,
        message: message,
        actionLabel: actionLabel,
        onAction: onAction ?? () {},
      ),
    );
  }

  static Scaffold error({
    required String title,
    required String headline,
    required String message,
    String actionLabel = 'Coba Lagi',
    VoidCallback? onAction,
  }) {
    return Scaffold(
      appBar: AppBarCustom(title: title),
      body: _StateBody(
        icon: Icons.error_outline,
        iconColorRole: _IconColorRole.error,
        headline: headline,
        message: message,
        actionLabel: actionLabel,
        onAction: onAction,
      ),
    );
  }
}

enum _IconColorRole { neutral, error }

class _StateBody extends StatelessWidget {
  final IconData icon;
  final _IconColorRole iconColorRole;
  final String headline;
  final String? message;
  final String actionLabel;
  final VoidCallback? onAction;

  const _StateBody({
    required this.icon,
    required this.iconColorRole,
    required this.headline,
    required this.message,
    required this.actionLabel,
    required this.onAction,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final color = iconColorRole == _IconColorRole.error
        ? scheme.error
        : scheme.onSurfaceVariant;

    return Center(
      child: Padding(
        padding: const EdgeInsets.all(AppMetrics.p24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(icon, size: 64, color: color),
            const SizedBox(height: 16),
            Text(
              headline,
              style: const TextStyle(
                fontSize: AppType.s20,
                fontWeight: FontWeight.bold,
              ),
              textAlign: TextAlign.center,
            ),
            if (message != null) ...[
              const SizedBox(height: 8),
              Text(
                message!,
                style: TextStyle(fontSize: AppType.s14, color: scheme.onSurfaceVariant),
                textAlign: TextAlign.center,
              ),
            ],
            if (onAction != null) ...[
              const SizedBox(height: 24),
              ElevatedButton(
                onPressed: onAction,
                style: ElevatedButton.styleFrom(
                  backgroundColor: scheme.primary,
                  foregroundColor: scheme.onPrimary,
                  padding: const EdgeInsets.symmetric(
                    horizontal: AppMetrics.p24,
                    vertical: AppMetrics.p12,
                  ),
                ),
                child: Text(actionLabel),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
