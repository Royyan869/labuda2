library;

/// Help Center Screen
///
/// Provides self-help resources before escalating to human support.
/// This is the MATURITY LAYER that reduces unnecessary support tickets.
///
/// LOCALIZATION AUTHORITY: every user-facing string on these surfaces is
/// resolved from `AppLocalizations` (lib/l10n/app_en.arb + app_id.arb) at
/// build time, so the active app locale controls the Help Center language.
/// The former private `_Strings` bag is purged — there is no second content
/// authority and no manual localization map.

import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/support/support.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/shared.dart';

/// Main Help Center Screen
class HelpCenterScreen extends StatelessWidget {
  final String? userId;
  final String? userName;
  final String? userAvatar;

  const HelpCenterScreen({
    super.key,
    this.userId,
    this.userName,
    this.userAvatar,
  });

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;

    return Scaffold(
      appBar: AppBarCustom(title: l10n.helpSupport),
      // Canonical body-level bottom-inset authority (SAFE-AREA-22): the ONE
      // `SafeArea` consumes the live system bottom inset for the whole body,
      // so the scroll end (the "Hubungi Support" CTA) clears the system
      // navigation region. The scroll view's explicit `p16` padding below is
      // DESIGN spacing only — an explicit `ScrollView.padding` never inherits
      // MediaQuery padding, so nothing else reserves the inset.
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(AppMetrics.p16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Header
              _buildHeader(context, l10n),
              const SizedBox(height: 24),
              // Search Bar
              _buildSearchBar(context, l10n),
              const SizedBox(height: 24),
              // Quick Help Cards
              _buildSectionTitle(context, l10n.quickHelp),
              const SizedBox(height: 12),
              _buildQuickHelpCards(context, l10n),
              const SizedBox(height: 24),
              // Browse by Category
              _buildSectionTitle(context, l10n.browseByCategory),
              const SizedBox(height: 12),
              _buildCategoryCards(context, l10n),
              const SizedBox(height: 24),
              // Popular Articles
              _buildSectionTitle(context, l10n.popularArticles),
              const SizedBox(height: 12),
              _buildPopularArticles(context, l10n),
              const SizedBox(height: 24),
              // Still Need Help Section
              _buildStillNeedHelpSection(context, l10n),
              const SizedBox(height: 32),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildHeader(BuildContext context, AppLocalizations l10n) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          l10n.howCanWeHelp,
          style: context.typeRoles.titleProminent.copyWith(
            fontWeight: FontWeight.bold,
            color: Theme.of(context).colorScheme.onSurface,
          ),
        ),
        const SizedBox(height: 8),
        Text(
          l10n.helpCenterDescription,
          style: context.typeRoles.bodyDense.copyWith(
            color: Theme.of(context).colorScheme.onSurfaceVariant,
          ),
        ),
      ],
    );
  }

  Widget _buildSearchBar(BuildContext context, AppLocalizations l10n) {
    // Canonical editable-search decoration (one authority). Help Center search
    // is intended product behavior, but the field is not wired to a query yet:
    // the search backend (Elasticsearch) is still an implementation gap.
    return TextField(
      decoration: AppTheme.searchDecoration(
        Theme.of(context).colorScheme,
        hintText: l10n.searchHelpArticles,
      ).copyWith(prefixIcon: const Icon(Icons.search)),
    );
  }

  Widget _buildSectionTitle(BuildContext context, String title) {
    return Padding(
      padding: const EdgeInsets.only(left: AppMetrics.p4),
      child: Text(
        title,
        style: context.typeRoles.titleSection.copyWith(
          fontWeight: FontWeight.w600,
          color: Theme.of(context).colorScheme.onSurface,
        ),
      ),
    );
  }

  Widget _buildQuickHelpCards(BuildContext context, AppLocalizations l10n) {
    return Row(
      children: [
        Expanded(
          child: _QuickHelpCard(
            icon: Icons.shopping_cart_outlined,
            title: l10n.orders,
            color: Theme.of(context).colorScheme.primary,
            onTap: () => _navigateToCategory(context, HelpCategory.order),
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: _QuickHelpCard(
            icon: Icons.payment_outlined,
            title: l10n.payments,
            color: context.statusColors.warning,
            onTap: () => _navigateToCategory(context, HelpCategory.payment),
          ),
        ),
      ],
    );
  }

  Widget _buildCategoryCards(BuildContext context, AppLocalizations l10n) {
    final categories = [
      _CategoryItem(
        icon: Icons.shopping_bag_outlined,
        title: l10n.orders,
        subtitle: l10n.orderHelpSubtitle,
        color: Theme.of(context).colorScheme.primary,
        category: HelpCategory.order,
      ),
      _CategoryItem(
        icon: Icons.account_balance_wallet_outlined,
        title: l10n.payments,
        subtitle: l10n.paymentHelpSubtitle,
        color: context.statusColors.warning,
        category: HelpCategory.payment,
      ),
      _CategoryItem(
        icon: Icons.store_outlined,
        title: l10n.selling,
        subtitle: l10n.sellingHelpSubtitle,
        color: context.statusColors.success,
        category: HelpCategory.selling,
      ),
      _CategoryItem(
        icon: Icons.person_outlined,
        title: l10n.account,
        subtitle: l10n.accountHelpSubtitle,
        color: Theme.of(context).colorScheme.secondary,
        category: HelpCategory.account,
      ),
      _CategoryItem(
        icon: Icons.verified_user_outlined,
        title: l10n.verification,
        subtitle: l10n.verificationHelpSubtitle,
        color: Theme.of(context).colorScheme.primary,
        category: HelpCategory.verification,
      ),
      _CategoryItem(
        icon: Icons.build_outlined,
        title: l10n.technical,
        subtitle: l10n.technicalHelpSubtitle,
        color: Theme.of(context).colorScheme.onSurfaceVariant,
        category: HelpCategory.technical,
      ),
    ];

    return GridView.builder(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: 2,
        childAspectRatio: 1.0,
        crossAxisSpacing: 12,
        mainAxisSpacing: 12,
      ),
      itemCount: categories.length,
      itemBuilder: (context, index) {
        final item = categories[index];
        return _CategoryCard(
          icon: item.icon,
          title: item.title,
          subtitle: item.subtitle,
          color: item.color,
          onTap: () => _navigateToCategory(context, item.category),
        );
      },
    );
  }

  Widget _buildPopularArticles(BuildContext context, AppLocalizations l10n) {
    final articles = _getPopularArticles(l10n);

    return Column(
      children: articles.map((article) {
        return _ArticleTile(
          title: article.title,
          category: article.category,
          onTap: () => _showArticle(context, article),
        );
      }).toList(),
    );
  }

  Widget _buildStillNeedHelpSection(
    BuildContext context,
    AppLocalizations l10n,
  ) {
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p24),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          colors: [
            Theme.of(context).colorScheme.primary.withValues(alpha: 0.1),
            Theme.of(context).colorScheme.primary.withValues(alpha: 0.05),
          ],
        ),
        borderRadius: BorderRadius.circular(AppShape.r16),
        border: Border.all(
          color: Theme.of(context).colorScheme.primary.withValues(alpha: 0.2),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                Icons.support_agent,
                color: Theme.of(context).colorScheme.primary,
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Text(
                  l10n.stillNeedHelp,
                  style: context.typeRoles.titleSection.copyWith(
                    fontWeight: FontWeight.bold,
                    color: Theme.of(context).colorScheme.onSurface,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text(
            l10n.contactSupportDescription,
            style: context.typeRoles.bodyDense.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 16),
          SizedBox(
            width: double.infinity,
            child: ElevatedButton(
              onPressed: () => _navigateToSupportForm(context),
              style: ElevatedButton.styleFrom(
                padding: const EdgeInsets.symmetric(vertical: AppMetrics.p16),
              ),
              child: Text(l10n.contactSupport),
            ),
          ),
        ],
      ),
    );
  }

  void _navigateToCategory(BuildContext context, HelpCategory category) {
    // Canonical category route: the enum name is the stable, shareable
    // identity; the reader identity is resolved by the route itself.
    context.push(RoutePaths.helpCategoryPath(category.name));
  }

  void _showArticle(BuildContext context, HelpArticle article) {
    // Articles carry localized content with no stable content id yet, so the
    // article travels as route extra. The content is resolved from the active
    // locale by the screen that builds the list.
    context.push(RoutePaths.helpArticle, extra: article);
  }

  void _navigateToSupportForm(BuildContext context) {
    // The canonical support flow is the pre-chat form used by the rest of the
    // app (verification, earnings, order detail). Without a session, the
    // sign-in route is the canonical entry — the silent pop(true) dead-end is
    // gone.
    if (userId != null && userName != null) {
      showPreChatFormRefactored(
        context,
        userId: userId!,
        userName: userName!,
        userAvatar: userAvatar,
      );
    } else {
      context.push(RoutePaths.signIn);
    }
  }

  List<HelpArticle> _getPopularArticles(AppLocalizations l10n) {
    return [
      HelpArticle(
        title: l10n.articleHowToPay,
        category: l10n.payments,
        content: l10n.articleHowToPayContent,
      ),
      HelpArticle(
        title: l10n.articleTrackOrder,
        category: l10n.orders,
        content: l10n.articleTrackOrderContent,
      ),
      HelpArticle(
        title: l10n.articleRequestRefund,
        category: l10n.orders,
        content: l10n.articleRequestRefundContent,
      ),
      HelpArticle(
        title: l10n.articleBecomeSeller,
        category: l10n.selling,
        content: l10n.articleBecomeSellerContent,
      ),
    ];
  }
}

// =============================================================================
// SUBSCREENS
// =============================================================================

/// Category-specific help screen
class HelpCategoryScreen extends StatelessWidget {
  final HelpCategory category;
  final String? userId;
  final String? userName;
  final String? userAvatar;

  const HelpCategoryScreen({
    super.key,
    required this.category,
    this.userId,
    this.userName,
    this.userAvatar,
  });

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final articles = _getArticlesForCategory(l10n);

    return Scaffold(
      appBar: AppBarCustom(title: _getCategoryTitle(l10n)),
      // Canonical body-level bottom-inset authority (SAFE-AREA-24): the ONE
      // `SafeArea` consumes the live system bottom inset for the whole body, so
      // the last article row clears the system navigation region. The list's
      // explicit `p16` padding below is DESIGN spacing only — an explicit
      // `ListView.padding` never inherits MediaQuery padding.
      body: SafeArea(
        child: ListView.separated(
          padding: const EdgeInsets.all(AppMetrics.p16),
          itemCount: articles.length,
          separatorBuilder: (_, _) => const SizedBox(height: 8),
          itemBuilder: (context, index) {
            final article = articles[index];
            return _ArticleTile(
              title: article.title,
              category: article.category,
              onTap: () =>
                  context.push(RoutePaths.helpArticle, extra: article),
            );
          },
        ),
      ),
    );
  }

  String _getCategoryTitle(AppLocalizations l10n) {
    switch (category) {
      case HelpCategory.order:
        return l10n.orders;
      case HelpCategory.payment:
        return l10n.payments;
      case HelpCategory.selling:
        return l10n.selling;
      case HelpCategory.account:
        return l10n.account;
      case HelpCategory.verification:
        return l10n.verification;
      case HelpCategory.technical:
        return l10n.technical;
    }
  }

  List<HelpArticle> _getArticlesForCategory(AppLocalizations l10n) {
    switch (category) {
      case HelpCategory.order:
        return [
          HelpArticle(
            title: l10n.articleTrackOrder,
            category: l10n.orders,
            content: l10n.articleTrackOrderContent,
          ),
          HelpArticle(
            title: l10n.articleRequestRefund,
            category: l10n.orders,
            content: l10n.articleRequestRefundContent,
          ),
          HelpArticle(
            title: l10n.articleCancelOrder,
            category: l10n.orders,
            content: l10n.articleCancelOrderContent,
          ),
          HelpArticle(
            title: l10n.articleConfirmDelivery,
            category: l10n.orders,
            content: l10n.articleConfirmDeliveryContent,
          ),
          HelpArticle(
            title: l10n.articleOrderShipmentHelp,
            category: l10n.orders,
            content: l10n.articleOrderShipmentHelpContent,
          ),
          HelpArticle(
            title: l10n.articleItemNotReceived,
            category: l10n.orders,
            content: l10n.articleItemNotReceivedContent,
          ),
        ];
      case HelpCategory.payment:
        return [
          HelpArticle(
            title: l10n.articleHowToPay,
            category: l10n.payments,
            content: l10n.articleHowToPayContent,
          ),
          HelpArticle(
            title: l10n.articlePaymentFailed,
            category: l10n.payments,
            content: l10n.articlePaymentFailedContent,
          ),
          HelpArticle(
            title: l10n.articleRefundTime,
            category: l10n.payments,
            content: l10n.articleRefundTimeContent,
          ),
        ];
      case HelpCategory.selling:
        return [
          HelpArticle(
            title: l10n.articleBecomeSeller,
            category: l10n.selling,
            content: l10n.articleBecomeSellerContent,
          ),
          HelpArticle(
            title: l10n.articleCreateForSale,
            category: l10n.selling,
            content: l10n.articleCreateForSaleContent,
          ),
          HelpArticle(
            title: l10n.articleShippingSetup,
            category: l10n.selling,
            content: l10n.articleShippingSetupContent,
          ),
          HelpArticle(
            title: l10n.articleWithdrawalFailed,
            category: l10n.selling,
            content: l10n.articleWithdrawalFailedContent,
          ),
          HelpArticle(
            title: l10n.articleForSaleNotVisible,
            category: l10n.selling,
            content: l10n.articleForSaleNotVisibleContent,
          ),
          HelpArticle(
            title: l10n.articleSellerPaymentPending,
            category: l10n.selling,
            content: l10n.articleSellerPaymentPendingContent,
          ),
        ];
      case HelpCategory.account:
        return [
          HelpArticle(
            title: l10n.articleEditProfile,
            category: l10n.account,
            content: l10n.articleEditProfileContent,
          ),
          HelpArticle(
            title: l10n.articleChangePassword,
            category: l10n.account,
            content: l10n.articleChangePasswordContent,
          ),
        ];
      case HelpCategory.verification:
        return [
          HelpArticle(
            title: l10n.articleSellerVerification,
            category: l10n.verification,
            content: l10n.articleSellerVerificationContent,
          ),
        ];
      case HelpCategory.technical:
        return [
          HelpArticle(
            title: l10n.articleAppNotWorking,
            category: l10n.technical,
            content: l10n.articleAppNotWorkingContent,
          ),
          HelpArticle(
            title: l10n.articleAppSlowOrNotLoading,
            category: l10n.technical,
            content: l10n.articleAppSlowOrNotLoadingContent,
          ),
        ];
    }
  }
}

/// Individual article screen
class HelpArticleScreen extends StatelessWidget {
  final HelpArticle article;
  final String? userId;
  final String? userName;
  final String? userAvatar;

  const HelpArticleScreen({
    super.key,
    required this.article,
    this.userId,
    this.userName,
    this.userAvatar,
  });

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;

    return Scaffold(
      appBar: AppBarCustom(title: l10n.helpArticle),
      // Canonical body-level bottom-inset authority (SAFE-AREA-25): the ONE
      // `SafeArea` consumes the live system bottom inset for the whole body, so
      // the "Was this helpful" block (the last meaningful content, carrying
      // the Yes/No actions) clears the system navigation region. The scroll
      // view's explicit `p24` padding below is DESIGN spacing only — an
      // explicit `ScrollView.padding` never inherits MediaQuery padding.
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(AppMetrics.p24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Category Badge
              Container(
                padding: const EdgeInsets.symmetric(
                  horizontal: AppMetrics.p12,
                  vertical: AppMetrics.p8,
                ),
                decoration: BoxDecoration(
                  color: Theme.of(
                    context,
                  ).colorScheme.secondary.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(AppShape.r16),
                ),
                child: Text(
                  article.category,
                  style: context.typeRoles.labelMicro.copyWith(
                    fontWeight: FontWeight.w600,
                    color: Theme.of(context).colorScheme.secondary,
                  ),
                ),
              ),
              const SizedBox(height: 16),
              // Title
              Text(
                article.title,
                style: context.typeRoles.titleProminent.copyWith(
                  fontWeight: FontWeight.bold,
                  color: Theme.of(context).colorScheme.onSurface,
                ),
              ),
              const SizedBox(height: 24),
              // Content
              Text(
                article.content,
                style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                  height: 1.6,
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
              ),
              const SizedBox(height: 32),
              // Helpful Section
              _buildHelpfulSection(context, l10n),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildHelpfulSection(BuildContext context, AppLocalizations l10n) {
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surfaceContainer,
        borderRadius: BorderRadius.circular(AppShape.r12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            l10n.wasThisHelpful,
            style: context.typeRoles.titleCompact.copyWith(
              fontWeight: FontWeight.w600,
              color: Theme.of(context).colorScheme.onSurface,
            ),
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              Expanded(
                child: OutlinedButton.icon(
                  // Persisting the article rating is a separate, unimplemented
                  // feedback backend; the UI stays honest and claims nothing
                  // beyond an acknowledgement.
                  onPressed: () {
                    Navigator.of(context).pop();
                    AppSnackBar.showSuccess(context, l10n.feedbackThanks);
                  },
                  icon: const Icon(
                    Icons.thumb_up_outlined,
                    size: AppIconSize.action,
                  ),
                  label: Text(l10n.yes),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: OutlinedButton.icon(
                  onPressed: () {
                    if (userId != null && userName != null) {
                      showPreChatFormRefactored(
                        context,
                        userId: userId!,
                        userName: userName!,
                        userAvatar: userAvatar,
                      );
                    } else {
                      // Authentication required: the canonical flow is the
                      // sign-in route, not a transient toast. Capture the
                      // router before closing this sheet.
                      final router = GoRouter.of(context);
                      Navigator.of(context).pop();
                      router.push(RoutePaths.signIn);
                    }
                  },
                  icon: const Icon(
                    Icons.thumb_down_outlined,
                    size: AppIconSize.action,
                  ),
                  label: Text(l10n.no),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

// =============================================================================
// WIDGETS
// =============================================================================

class _QuickHelpCard extends StatelessWidget {
  final IconData icon;
  final String title;
  final Color color;
  final VoidCallback onTap;

  const _QuickHelpCard({
    required this.icon,
    required this.title,
    required this.color,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(AppShape.r12),
      child: Container(
        padding: const EdgeInsets.all(AppMetrics.p16),
        decoration: BoxDecoration(
          color: Theme.of(context).colorScheme.surface,
          borderRadius: BorderRadius.circular(AppShape.r12),
          border: Border.all(
            color: Theme.of(context).colorScheme.outlineVariant,
          ),
        ),
        child: Column(
          children: [
            Container(
              padding: const EdgeInsets.all(AppMetrics.p12),
              decoration: BoxDecoration(
                color: color.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(AppShape.r10),
              ),
              child: Icon(icon, color: color, size: AppIconSize.header),
            ),
            const SizedBox(height: 8),
            Text(
              title,
              style: context.typeRoles.labelMicro.copyWith(
                fontWeight: FontWeight.w500,
                color: Theme.of(context).colorScheme.onSurface,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _CategoryCard extends StatelessWidget {
  final IconData icon;
  final String title;
  final String subtitle;
  final Color color;
  final VoidCallback onTap;

  const _CategoryCard({
    required this.icon,
    required this.title,
    required this.subtitle,
    required this.color,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(AppShape.r12),
      child: Container(
        padding: const EdgeInsets.all(AppMetrics.p16),
        decoration: BoxDecoration(
          color: Theme.of(context).colorScheme.surface,
          borderRadius: BorderRadius.circular(AppShape.r12),
          border: Border.all(
            color: Theme.of(context).colorScheme.outlineVariant,
          ),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Icon(icon, color: color, size: AppIconSize.header),
            const SizedBox(height: 12),
            Text(
              title,
              style: context.typeRoles.titleCompact.copyWith(
                fontWeight: FontWeight.w600,
                color: Theme.of(context).colorScheme.onSurface,
              ),
            ),
            const SizedBox(height: 4),
            Expanded(
              child: Text(
                subtitle,
                style: context.typeRoles.labelMicro.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _ArticleTile extends StatelessWidget {
  final String title;
  final String category;
  final VoidCallback onTap;

  const _ArticleTile({
    required this.title,
    required this.category,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(AppShape.r10),
      child: Container(
        padding: const EdgeInsets.all(AppMetrics.p16),
        decoration: BoxDecoration(
          color: Theme.of(context).colorScheme.surface,
          borderRadius: BorderRadius.circular(AppShape.r10),
          border: Border.all(
            color: Theme.of(context).colorScheme.outlineVariant,
          ),
        ),
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    title,
                    style: context.typeRoles.titleCompact.copyWith(
                      fontWeight: FontWeight.w500,
                      color: Theme.of(context).colorScheme.onSurface,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    category,
                    style: context.typeRoles.labelMicro.copyWith(
                      color: Theme.of(context).colorScheme.secondary,
                    ),
                  ),
                ],
              ),
            ),
            Icon(
              Icons.chevron_right,
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ],
        ),
      ),
    );
  }
}

// =============================================================================
// MODELS
// =============================================================================

enum HelpCategory { order, payment, selling, account, verification, technical }

class _CategoryItem {
  final IconData icon;
  final String title;
  final String subtitle;
  final Color color;
  final HelpCategory category;

  _CategoryItem({
    required this.icon,
    required this.title,
    required this.subtitle,
    required this.color,
    required this.category,
  });
}

class HelpArticle {
  final String title;
  final String category;
  final String content;

  HelpArticle({
    required this.title,
    required this.category,
    required this.content,
  });
}
