import 'package:hishumi/domains/system/support/domain/domain.dart';
import 'package:hishumi/generated/app_localizations.dart';

/// I18N-13 — the ONE canonical Support Category display-label authority.
///
/// This extension holds no copy of its own: it maps each canonical
/// [SupportCategory] to its resource in the canonical `AppLocalizations`
/// authority, so the rendered label always follows the active app locale.
///
/// Category *identity* stays `SupportCategory.wireValue` (the backend / API
/// contract); a localized label is display-only and must never be used for
/// persistence, filtering identity, analytics, routing, or the API payload.
extension SupportCategoryLabel on SupportCategory {
  String label(AppLocalizations l10n) => switch (this) {
    SupportCategory.orderIssue => l10n.supportCategoryOrderIssue,
    SupportCategory.paymentIssue => l10n.supportCategoryPaymentIssue,
    SupportCategory.accountIssue => l10n.supportCategoryAccountIssue,
    SupportCategory.listingIssue => l10n.supportCategoryListingIssue,
    SupportCategory.shippingIssue => l10n.supportCategoryShippingIssue,
    SupportCategory.refundRequest => l10n.supportCategoryRefundRequest,
    SupportCategory.dispute => l10n.supportCategoryDispute,
    SupportCategory.technicalIssue => l10n.supportCategoryTechnicalIssue,
    SupportCategory.other => l10n.supportCategoryOther,
  };
}
