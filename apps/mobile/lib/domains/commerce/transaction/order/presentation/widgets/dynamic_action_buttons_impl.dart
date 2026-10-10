library;

// =============================================================================
// DYNAMIC ACTION BUTTONS - Decision V2 Contract
// =============================================================================
//
// Backend is the SINGLE SOURCE OF TRUTH for all UI actions.
//
// This widget:
// 1. Loops through primary_action + secondary_actions from backend
// 2. Renders buttons based on action metadata (label, endpoint, method)
// 3. Executes actions using backend-provided endpoint + method
// 4. NO hardcoded button visibility logic
// 5. NO fallback to status-based checks
//
// Action structure from backend:
// - type: Action type enum
// - label_key: Localization key for UI
// - enabled: Whether action is currently enabled
// - blocked: Why action is disabled (with resolution)
// - endpoint: API endpoint to call
// - method: HTTP method (POST, PATCH, etc.)
// - requires_idempotency: Whether action requires idempotency key
// - financial: Whether action affects money (ledger validation)
// - input_schema: Structured input definition with validation
//
// PRESENTATION: chrome (surface, separator, padding, Safe Area, keyboard,
// button height) is owned by [BottomActionBar]. This file only maps the
// backend decision to bar actions — labels, icons, and destructive marking.
// =============================================================================

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';
import 'package:hishumi/domains/commerce/transaction/order/domain/domain.dart'
    as order_domain;
import 'package:hishumi/domains/commerce/transaction/order/presentation/widgets/order_action_label_resolver.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/shared/shared.dart';

/// Action Button Callbacks - handlers for different action types
class ActionCallbacks {
  final void Function(order_domain.Action action) onAction;
  final VoidCallback? onRequestSupport;
  final VoidCallback? onChatSeller;

  const ActionCallbacks({
    required this.onAction,
    this.onRequestSupport,
    this.onChatSeller,
  });
}

/// Dynamic Action Buttons Widget
///
/// Renders buttons dynamically based on backend Decision V2 contract.
/// NO hardcoded logic - all button visibility and behavior comes from backend.
class DynamicActionButtons extends ConsumerWidget {
  final order_domain.DecisionContract decision;
  final ActionCallbacks callbacks;

  const DynamicActionButtons({
    super.key,
    required this.decision,
    required this.callbacks,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Canonical localization authority for order action labels: the backend
    // `label_key` is resolved through AppLocalizations only (no local maps).
    final l10n = AppLocalizations.of(context)!;

    // Collect all actions (primary + secondary)
    final allActions = decision.allActions;

    // Filter to only enabled actions
    final enabledActions = allActions.where((a) => a.enabled).toList();

    // No actions available
    if (enabledActions.isEmpty) {
      return BottomActionBar(
        footer: _SupportLinks(callbacks: callbacks),
      );
    }

    // Separate primary and secondary actions
    final primaryAction = decision.primaryAction?.enabled == true
        ? decision.primaryAction
        : null;
    final selectedPrimaryAction = primaryAction;
    final secondaryActions = enabledActions
        .where(
          (a) =>
              selectedPrimaryAction == null ||
              a.type != selectedPrimaryAction.type,
        )
        .toList();

    return BottomActionBar(
      primary: primaryAction == null
          ? null
          : BottomBarAction(
              label: resolveOrderActionLabel(l10n, primaryAction.labelKey),
              icon: _primaryIcon(primaryAction.type),
              onPressed: () => callbacks.onAction(primaryAction),
            ),
      stacked: secondaryActions
          .map(
            (action) => BottomBarAction(
              label: resolveOrderActionLabel(l10n, action.labelKey),
              icon: _secondaryIcon(action.type),
              isDestructive: _isDestructiveAction(action.type),
              onPressed: () => callbacks.onAction(action),
            ),
          )
          .toList(),
      footer: _SupportLinks(callbacks: callbacks),
    );
  }
}

/// Tertiary support links below the actions (content owned here, chrome
/// owned by the bar).
class _SupportLinks extends StatelessWidget {
  final ActionCallbacks callbacks;

  const _SupportLinks({required this.callbacks});

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final colorScheme = Theme.of(context).colorScheme;
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        // Support button (always available)
        TextButton.icon(
          onPressed: callbacks.onRequestSupport,
          icon: const Icon(
            Icons.support_agent,
            size: AppIconSize.inlineGlyph,
          ),
          label: const Text('Butuh Bantuan?'),
          style: TextButton.styleFrom(
            foregroundColor: colorScheme.onSurfaceVariant,
          ),
        ),

        // Chat Seller button (BATCH 2B - DIRECT ORDER → CHAT CONTINUITY)
        // Allows buyer↔seller communication through canonical commerce chat
        if (callbacks.onChatSeller != null)
          TextButton.icon(
            onPressed: callbacks.onChatSeller,
            icon: const Icon(
              Icons.chat_bubble_outline,
              size: AppIconSize.inlineGlyph,
            ),
            label: Text(l10n.orderActionChatSeller),
          ),
      ],
    );
  }
}

IconData _primaryIcon(String actionType) {
  switch (actionType) {
    case 'mark_shipped':
    case 'update_tracking':
      return Icons.local_shipping;
    case 'complete':
      return Icons.check_circle;
    case 'request_refund':
      return Icons.currency_exchange;
    case 'open_dispute':
      return Icons.report_problem;
    case 'cancel':
      return Icons.cancel;
    case 'pay':
      return Icons.payment;
    case 'extend_confirmation':
      return Icons.add_alarm;
    default:
      return Icons.arrow_forward;
  }
}

IconData _secondaryIcon(String actionType) {
  switch (actionType) {
    case 'mark_shipped':
    case 'update_tracking':
      return Icons.local_shipping;
    case 'complete':
      return Icons.check_circle;
    case 'request_refund':
      return Icons.currency_exchange;
    case 'open_dispute':
      return Icons.report_problem;
    case 'cancel':
      return Icons.cancel;
    case 'extend_confirmation':
      return Icons.add_alarm;
    default:
      return Icons.arrow_forward;
  }
}

bool _isDestructiveAction(String actionType) {
  return actionType == 'cancel' || actionType == 'request_refund';
}
