import 'package:flutter/material.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Canonical commerce ACCESS GATE — ONE AUTHORITY for every for_sale/auction
/// surface that must block a session which cannot proceed (not signed in,
/// seller profile missing, market authority missing).
///
/// Channels supply only their facts: screen title, headline copy, message and
/// the action. Chrome, icon, typography, spacing and the app bar live here so
/// the same gate can never render two ways across the two sale channels.
class CommerceAccessGate extends StatelessWidget {
  const CommerceAccessGate({
    super.key,
    required this.screenTitle,
    required this.headline,
    required this.message,
    required this.buttonLabel,
    required this.onAction,
  });

  /// The commerce surface that is gated (app-bar title).
  final String screenTitle;

  /// Primary gate copy — e.g. `Login Diperlukan`.
  final String headline;

  /// Supporting copy explaining the gate.
  final String message;

  /// CTA label; the channel owns where it leads.
  final String buttonLabel;

  /// The channel-owned action (navigation lives with the channel).
  final VoidCallback onAction;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Scaffold(
      appBar: AppBarCustom(title: screenTitle),
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(AppMetrics.p24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                Icons.lock_outline,
                size: 56,
                color: scheme.onSurfaceVariant,
              ),
              const SizedBox(height: 16),
              Text(
                headline,
                style: const TextStyle(
                  fontSize: AppType.s20,
                  fontWeight: FontWeight.w700,
                ),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 8),
              Text(
                message,
                style: TextStyle(fontSize: AppType.s14, color: scheme.onSurfaceVariant),
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 20),
              ElevatedButton(
                onPressed: onAction,
                style: ElevatedButton.styleFrom(
                  backgroundColor: scheme.primary,
                  foregroundColor: scheme.onPrimary,
                  padding: const EdgeInsets.symmetric(
                    horizontal: AppMetrics.p24,
                    vertical: AppMetrics.p14,
                  ),
                ),
                child: Text(buttonLabel),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
