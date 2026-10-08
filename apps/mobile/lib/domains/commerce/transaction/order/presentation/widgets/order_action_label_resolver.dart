library;

import 'package:labuda/generated/app_localizations.dart';

/// Mechanical `label_key → AppLocalizations` resolver for Order Actions.
///
/// This is NOT a translation authority: it contains no translated strings.
/// Every branch returns a canonical [AppLocalizations] getter, so
/// AppLocalizations stays the single localization authority for order action
/// labels (Audit 06 canonical end state).
///
/// The backend wire contract (`Action.label_key`, e.g. `action.mark_shipped`)
/// is preserved verbatim — only the client-side lookup lives here.
///
/// Unknown keys resolve to the smallest existing localized generic CTA:
/// never a raw `action.*` key, never a Title-Case transformation.
String resolveOrderActionLabel(AppLocalizations l10n, String labelKey) {
  switch (labelKey) {
    case 'action.mark_shipped':
      return l10n.orderActionMarkShipped;
    case 'action.confirm_receipt':
      return l10n.orderActionConfirmReceipt;
    case 'action.provide_evidence':
      return l10n.orderActionProvideEvidence;
    case 'action.cancel_order_overdue':
      return l10n.orderActionCancelOrderOverdue;
    case 'action.pay_now':
      return l10n.orderActionPayNow;
    case 'action.payment_continue':
      return l10n.orderActionPaymentContinue;
    case 'action.payment_check_status':
      return l10n.orderActionPaymentCheckStatus;
    case 'action.pay_again':
      return l10n.orderActionPayAgain;
    case 'action.cancel_order':
      return l10n.orderActionCancelOrder;
    case 'action.extend_confirmation':
      return l10n.orderActionExtendConfirmation;
    case 'action.request_refund':
      return l10n.orderActionRequestRefund;
    case 'action.open_dispute':
      return l10n.orderActionOpenDispute;
    case 'action.update_tracking':
      return l10n.orderActionUpdateTracking;
    case 'action.chat_seller':
      return l10n.orderActionChatSeller;
    case 'action.contact_support':
      return l10n.orderActionContactSupport;
    default:
      return l10n.continueButton;
  }
}
