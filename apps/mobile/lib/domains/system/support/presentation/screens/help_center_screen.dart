library;

/// Help Center Screen
///
/// Provides self-help resources before escalating to human support.
/// This is the MATURITY LAYER that reduces unnecessary support tickets.

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/support/support.dart';
import 'package:labuda/shared/shared.dart';

// Hardcoded strings for now - localization will be added after fixing gen-l10n issue
class _Strings {
  static const String helpSupport = 'Help & Support';
  static const String howCanWeHelp = 'How can we help you today?';
  static const String helpCenterDescription =
      'Find answers quickly or get in touch with our support team.';
  static const String searchHelpArticles = 'Search help articles...';
  static const String quickHelp = 'Quick Help';
  static const String browseByCategory = 'Browse by Category';
  static const String popularArticles = 'Popular Articles';
  static const String orders = 'Orders';
  static const String payments = 'Payments';
  static const String selling = 'Selling';
  static const String account = 'Account';
  static const String verification = 'Verification';
  static const String technical = 'Technical';
  static const String orderHelpSubtitle = 'Track, cancel, or refund orders';
  static const String paymentHelpSubtitle =
      'Payment methods and failed transactions';
  static const String sellingHelpSubtitle =
      'Become a seller and manage your store';
  static const String accountHelpSubtitle = 'Profile, password, and settings';
  static const String verificationHelpSubtitle = 'Seller verification and KTP';
  static const String technicalHelpSubtitle = 'App issues and troubleshooting';
  static const String stillNeedHelp = 'Still need help?';
  static const String contactSupportDescription =
      'Our support team is here to help you with any questions or issues.';
  static const String contactSupport = 'Contact Support';
  static const String helpArticle = 'Help Article';
  static const String wasThisHelpful = 'Was this helpful?';
  static const String yes = 'Yes';
  static const String no = 'No';
  static const String feedbackThanks = 'Thank you for your feedback!';
  static const String pleaseLoginToManage =
      'Please login to access this feature';

  // Article titles
  static const String articleHowToPay = 'How to pay for my order?';
  static const String articleHowToPayContent = '''To pay for your order:

1. Go to your order from the Orders screen
2. Tap the 'Pay Now' button
3. Select your payment method (GoPay, Bank Transfer, etc.)
4. Follow the instructions to complete payment
5. Your payment will be confirmed within 24 hours

If payment fails, you can retry from the order screen.''';

  static const String articleTrackOrder = 'How to track my order?';
  static const String articleTrackOrderContent = '''To track your order:

1. Go to 'My Orders' from the profile menu
2. Tap on the order you want to track
3. You'll see the current status and tracking information

Order statuses:
- Pending: Waiting for payment
- Processing: Seller is preparing your order
- Shipped: Order is on the way
- Delivered: Order has arrived
- Completed: Order is finished''';

  static const String articleRequestRefund = 'How to request a refund?';
  static const String articleRequestRefundContent = '''To request a refund:

1. Open the order details
2. Tap 'Request Refund'
3. Select a reason and describe the issue
4. Upload unboxing video as proof (required)
5. Submit your request

The seller will review your request. If rejected, you can escalate to admin for final decision.''';

  static const String articleBecomeSeller = 'How to become a seller?';
  static const String articleBecomeSellerContent =
      '''To become a seller on LABUDA:

1. Go to Settings → Upgrade to Seller
2. Choose your plan (Basic or Pro)
3. Fill in your business information
4. Complete payment for the subscription
5. Wait for verification approval

Once approved, you can start listing your koi for sale!''';

  static const String articleCancelOrder = 'How to cancel an order?';
  static const String articleCancelOrderContent =
      '''Order cancellation depends on the status:

- Pending payment: Auto-cancelled if not paid within 24 hours
- Processing: Contact seller to cancel
- Shipped: Cannot cancel, use refund process instead

To request cancellation, tap the order and select 'Contact Seller' to discuss.''';

  static const String articleConfirmDelivery = 'How to complete an order?';
  static const String articleConfirmDeliveryContent =
      '''To complete your order after receiving items:

1. Open the order details
2. Tap 'Terima Barang' button
3. Confirm that items match your order

If you don't confirm within 5 days of shipment, the order will auto-complete.''';

  static const String articlePaymentFailed = 'Payment failed, what to do?';
  static const String articlePaymentFailedContent = '''If your payment fails:

1. Check your payment method balance
2. Try a different payment method
3. Ensure you have stable internet connection
4. Retry payment from the order screen

If the issue persists, contact support with your order number.''';

  static const String articleRefundTime = 'How long does refund take?';
  static const String articleRefundTimeContent = '''Refund processing time:

1. Seller review: 1-3 days
2. If approved: 3-7 business days for funds to return

The exact time depends on your payment method. GoPay refunds are usually faster than bank transfers.''';

  static const String articleCreateListing = 'How to create a listing?';
  static const String articleCreateListingContent = '''To create a new listing:

1. Tap the + button on the home screen
2. Select 'Listing'
3. Add photos of your koi (multiple angles recommended)
4. Fill in details (variety, size, price, location)
5. Write a description
6. Publish your listing

Your listing will be visible to buyers immediately!''';

  static const String articleShippingSetup = 'How to set up shipping?';
  static const String articleShippingSetupContent = '''To set up shipping:

1. Go to Settings → Pengiriman (or Seller Dashboard → Atur Pengiriman)
2. Add a shipping option (train, bus, travel, plane, or custom)
3. Add province coverage with the rate you charge for each province
4. Toggle the option active to make it available for your listings
5. When creating a listing, choose which of your options apply to that listing

Shipping is seller-managed: you decide the options, rates, and courier.
For irregular cases (large fish, special handling), send a shipping quote to the buyer in chat as a fallback.

Always use proper packaging with oxygen for live koi shipping!''';

  static const String articleEditProfile = 'How to edit my profile?';
  static const String articleEditProfileContent = '''To edit your profile:

1. Go to your profile screen
2. Tap the edit icon
3. Update your information:
   - Profile photo
   - Display name
   - Bio
   - Location
4. Tap 'Save' to apply changes''';

  static const String articleChangePassword = 'How to change my password?';
  static const String articleChangePasswordContent = '''To change your password:

1. Go to Settings → Security
2. Tap 'Change Password'
3. Enter your current password
4. Enter your new password (min 8 characters)
5. Confirm the new password
6. Tap 'Update Password'

You'll be logged out from other devices after changing password.''';

  static const String articleSellerVerification =
      'Seller verification requirements';
  static const String articleSellerVerificationContent =
      '''Seller verification requires:

1. Valid KTP (Indonesian ID card)
2. Clear photo of KTP
3. KTP number (16 digits)
4. Name matching KTP
5. Business address
6. Active phone number

Verification usually takes 1-3 business days.''';

  static const String articleAppNotWorking = 'App not working properly?';
  static const String articleAppNotWorkingContent =
      '''If the app is not working:

1. Check your internet connection
2. Close and reopen the app
3. Clear app cache (Settings → Clear Cache)
4. Update to the latest app version
5. Restart your phone

If issues persist, contact support with details of what's not working.''';

  static const String articleClearCache = 'How to clear app cache?';
  static const String articleClearCacheContent = '''To clear app cache:

1. Go to Settings
2. Scroll to 'App Preferences'
3. Tap 'Clear Cache'
4. Confirm when prompted

This will free up storage but won't delete your data. You'll need to log in again.''';

  // ========== SELLER-SPECIFIC ARTICLES (PHASE 3 HARDENING) ==========

  static const String articleWithdrawalFailed =
      'Withdrawal failed, what to do?';
  static const String articleWithdrawalFailedContent =
      '''If your withdrawal fails:

1. Check your bank account details are correct
2. Ensure you've completed seller verification (KTP)
3. Minimum withdrawal is Rp 10.000
4. Bank transfers process within 1-3 business days

Next steps:
• Go to Earnings → Check withdrawal status
• If status shows "failed", update bank details and try again
• Contact support if funds are deducted but not received

👉 Contact Support for withdrawal issues''';

  static const String articleListingNotVisible =
      'Why is my listing not visible?';
  static const String articleListingNotVisibleContent =
      '''Your listing may not be visible because:

1. Not yet published - Check status is "Active"
2. Incomplete information - Fill all required fields
3. Pending review - Some listings need approval
4. Search ranking - Optimize title and description

Next steps:
• Go to My Listings → Check listing status
• Edit listing to complete missing information
• Use clear photos and detailed descriptions
• Share listing link to social media

👉 Contact Support if listing is active but not shown''';

  static const String articleSellerPaymentPending =
      'Payment from order not received?';
  static const String articleSellerPaymentPendingContent =
      '''Order payments go through these stages:

1. Paid → In escrow (waiting for delivery)
2. Shipped → Still in escrow
3. Delivered → Buyer must confirm (3 days)
4. Completed → Funds mature and become withdrawable

Your earnings show as:
• Pending Balance: In escrow, not yet withdrawable
• Available Balance: Ready to withdraw

Check:
• Order status in Seller Dashboard
• Earnings screen for balance breakdown
• Funds typically mature 3-7 days after delivery

👉 Contact Support if completed orders not showing in earnings''';

  static const String articleOrderShipmentHelp =
      'Track shipment and delivery issues';
  static const String articleOrderShipmentHelpContent = '''For shipment issues:

1. Open order details
2. Check tracking number in Shipping Info
3. Track with courier's website or app

Common issues:
• Tracking not updating: Allow 24h for courier scan
• Delivery delayed: Contact seller for update
• Wrong address: Message seller immediately

If package not received:
• Check delivery confirmation status
• Contact seller through order chat
• Request refund if delivery deadline passed

👉 Still having issues? Contact Support with order number''';

  static const String articleItemNotReceived = 'Item paid but not received?';
  static const String articleItemNotReceivedContent =
      '''If you paid but haven't received your item:

Step 1: Check order status
• Paid: Seller is preparing your order
• Shipped: Check tracking number
• Delivered: Confirm within 3 days or auto-confirm

Step 2: Contact seller
• Use "Chat Penjual" button in order
• Ask for shipping update or tracking info

Step 3: Escalate if needed
• If seller doesn't respond in 24h
• If delivery deadline has passed
• Open dispute for refund

Next actions:
1. Chat seller first (fastest resolution)
2. If no response, use Request Support button
3. For urgent cases, select "Order Problems" category

👉 Contact Support now with your order details''';
}

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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Scaffold(
      appBar: AppBarCustom(title: _Strings.helpSupport),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Header
            _buildHeader(context, isDark),
            const SizedBox(height: 24),
            // Search Bar
            _buildSearchBar(context, isDark),
            const SizedBox(height: 24),
            // Quick Help Cards
            _buildSectionTitle(context, _Strings.quickHelp),
            const SizedBox(height: 12),
            _buildQuickHelpCards(context, isDark),
            const SizedBox(height: 24),
            // Browse by Category
            _buildSectionTitle(context, _Strings.browseByCategory),
            const SizedBox(height: 12),
            _buildCategoryCards(context, isDark),
            const SizedBox(height: 24),
            // Popular Articles
            _buildSectionTitle(context, _Strings.popularArticles),
            const SizedBox(height: 12),
            _buildPopularArticles(context, isDark),
            const SizedBox(height: 24),
            // Still Need Help Section
            _buildStillNeedHelpSection(context, isDark),
            const SizedBox(height: 32),
          ],
        ),
      ),
    );
  }

  Widget _buildHeader(BuildContext context, bool isDark) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          _Strings.howCanWeHelp,
          style: TextStyle(
            fontSize: 28,
            fontWeight: FontWeight.bold,
            color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
          ),
        ),
        const SizedBox(height: 8),
        Text(
          _Strings.helpCenterDescription,
          style: TextStyle(
            fontSize: 14,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
          ),
        ),
      ],
    );
  }

  Widget _buildSearchBar(BuildContext context, bool isDark) {
    return Container(
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralGray100,
        borderRadius: BorderRadius.circular(12),
      ),
      child: TextField(
        decoration: InputDecoration(
          hintText: _Strings.searchHelpArticles,
          prefixIcon: const Icon(Icons.search),
          border: InputBorder.none,
          contentPadding: const EdgeInsets.symmetric(
            horizontal: 16,
            vertical: 12,
          ),
        ),
      ),
    );
  }

  Widget _buildSectionTitle(BuildContext context, String title) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Padding(
      padding: const EdgeInsets.only(left: 4),
      child: Text(
        title,
        style: TextStyle(
          fontSize: 16,
          fontWeight: FontWeight.w600,
          color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
        ),
      ),
    );
  }

  Widget _buildQuickHelpCards(BuildContext context, bool isDark) {
    return Row(
      children: [
        Expanded(
          child: _QuickHelpCard(
            icon: Icons.shopping_cart_outlined,
            title: _Strings.orders,
            color: AppColors.primaryRed,
            onTap: () => _navigateToCategory(context, HelpCategory.order),
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: _QuickHelpCard(
            icon: Icons.payment_outlined,
            title: _Strings.payments,
            color: AppColors.warning,
            onTap: () => _navigateToCategory(context, HelpCategory.payment),
          ),
        ),
      ],
    );
  }

  Widget _buildCategoryCards(BuildContext context, bool isDark) {
    final categories = [
      _CategoryItem(
        icon: Icons.shopping_bag_outlined,
        title: _Strings.orders,
        subtitle: _Strings.orderHelpSubtitle,
        color: AppColors.primaryRed,
        category: HelpCategory.order,
      ),
      _CategoryItem(
        icon: Icons.account_balance_wallet_outlined,
        title: _Strings.payments,
        subtitle: _Strings.paymentHelpSubtitle,
        color: AppColors.warning,
        category: HelpCategory.payment,
      ),
      _CategoryItem(
        icon: Icons.store_outlined,
        title: _Strings.selling,
        subtitle: _Strings.sellingHelpSubtitle,
        color: AppColors.successGreen,
        category: HelpCategory.selling,
      ),
      _CategoryItem(
        icon: Icons.person_outlined,
        title: _Strings.account,
        subtitle: _Strings.accountHelpSubtitle,
        color: AppColors.primaryBlue,
        category: HelpCategory.account,
      ),
      _CategoryItem(
        icon: Icons.verified_user_outlined,
        title: _Strings.verification,
        subtitle: _Strings.verificationHelpSubtitle,
        color: AppColors.primary,
        category: HelpCategory.verification,
      ),
      _CategoryItem(
        icon: Icons.build_outlined,
        title: _Strings.technical,
        subtitle: _Strings.technicalHelpSubtitle,
        color: AppColors.neutralGray600,
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

  Widget _buildPopularArticles(BuildContext context, bool isDark) {
    final articles = _getPopularArticles();

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

  Widget _buildStillNeedHelpSection(BuildContext context, bool isDark) {
    return Container(
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          colors: [
            AppColors.primaryRed.withValues(alpha: 0.1),
            AppColors.primaryRed.withValues(alpha: 0.05),
          ],
        ),
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: AppColors.primaryRed.withValues(alpha: 0.2)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.support_agent, color: AppColors.primaryRed),
              const SizedBox(width: 12),
              Expanded(
                child: Text(
                  _Strings.stillNeedHelp,
                  style: TextStyle(
                    fontSize: 18,
                    fontWeight: FontWeight.bold,
                    color: isDark
                        ? AppColors.neutralWhite
                        : AppColors.neutralGray900,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text(
            _Strings.contactSupportDescription,
            style: TextStyle(
              fontSize: 14,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
          ),
          const SizedBox(height: 16),
          SizedBox(
            width: double.infinity,
            child: ElevatedButton(
              onPressed: () => _navigateToSupportForm(context),
              style: ElevatedButton.styleFrom(
                backgroundColor: AppColors.primaryRed,
                foregroundColor: AppColors.neutralWhite,
                padding: const EdgeInsets.symmetric(vertical: 14),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(10),
                ),
              ),
              child: Text(_Strings.contactSupport),
            ),
          ),
        ],
      ),
    );
  }

  void _navigateToCategory(BuildContext context, HelpCategory category) {
    Navigator.of(context).push(
      MaterialPageRoute(
        builder: (context) => HelpCategoryScreen(
          category: category,
          userId: userId,
          userName: userName,
          userAvatar: userAvatar,
        ),
      ),
    );
  }

  void _showArticle(BuildContext context, HelpArticle article) {
    Navigator.of(context).push(
      MaterialPageRoute(
        builder: (context) => HelpArticleScreen(
          article: article,
          userId: userId,
          userName: userName,
          userAvatar: userAvatar,
        ),
      ),
    );
  }

  void _navigateToSupportForm(BuildContext context) {
    // Navigate back with result indicating user wants to contact support
    Navigator.of(context).pop(true);
  }

  List<HelpArticle> _getPopularArticles() {
    return [
      HelpArticle(
        title: _Strings.articleHowToPay,
        category: _Strings.payments,
        content: _Strings.articleHowToPayContent,
      ),
      HelpArticle(
        title: _Strings.articleTrackOrder,
        category: _Strings.orders,
        content: _Strings.articleTrackOrderContent,
      ),
      HelpArticle(
        title: _Strings.articleRequestRefund,
        category: _Strings.orders,
        content: _Strings.articleRequestRefundContent,
      ),
      HelpArticle(
        title: _Strings.articleBecomeSeller,
        category: _Strings.selling,
        content: _Strings.articleBecomeSellerContent,
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
    final articles = _getArticlesForCategory();

    return Scaffold(
      appBar: AppBarCustom(title: _getCategoryTitle()),
      body: ListView.separated(
        padding: const EdgeInsets.all(16),
        itemCount: articles.length,
        separatorBuilder: (_, _) => const SizedBox(height: 8),
        itemBuilder: (context, index) {
          final article = articles[index];
          return _ArticleTile(
            title: article.title,
            category: article.category,
            onTap: () => Navigator.of(context).push(
              MaterialPageRoute(
                builder: (context) => HelpArticleScreen(
                  article: article,
                  userId: userId,
                  userName: userName,
                  userAvatar: userAvatar,
                ),
              ),
            ),
          );
        },
      ),
    );
  }

  String _getCategoryTitle() {
    switch (category) {
      case HelpCategory.order:
        return _Strings.orders;
      case HelpCategory.payment:
        return _Strings.payments;
      case HelpCategory.selling:
        return _Strings.selling;
      case HelpCategory.account:
        return _Strings.account;
      case HelpCategory.verification:
        return _Strings.verification;
      case HelpCategory.technical:
        return _Strings.technical;
    }
  }

  List<HelpArticle> _getArticlesForCategory() {
    switch (category) {
      case HelpCategory.order:
        return [
          HelpArticle(
            title: _Strings.articleTrackOrder,
            category: _Strings.orders,
            content: _Strings.articleTrackOrderContent,
          ),
          HelpArticle(
            title: _Strings.articleRequestRefund,
            category: _Strings.orders,
            content: _Strings.articleRequestRefundContent,
          ),
          HelpArticle(
            title: _Strings.articleCancelOrder,
            category: _Strings.orders,
            content: _Strings.articleCancelOrderContent,
          ),
          HelpArticle(
            title: _Strings.articleConfirmDelivery,
            category: _Strings.orders,
            content: _Strings.articleConfirmDeliveryContent,
          ),
          // PHASE 3 HARDENING: Shipping and delivery articles
          HelpArticle(
            title: _Strings.articleOrderShipmentHelp,
            category: _Strings.orders,
            content: _Strings.articleOrderShipmentHelpContent,
          ),
          HelpArticle(
            title: _Strings.articleItemNotReceived,
            category: _Strings.orders,
            content: _Strings.articleItemNotReceivedContent,
          ),
        ];
      case HelpCategory.payment:
        return [
          HelpArticle(
            title: _Strings.articleHowToPay,
            category: _Strings.payments,
            content: _Strings.articleHowToPayContent,
          ),
          HelpArticle(
            title: _Strings.articlePaymentFailed,
            category: _Strings.payments,
            content: _Strings.articlePaymentFailedContent,
          ),
          HelpArticle(
            title: _Strings.articleRefundTime,
            category: _Strings.payments,
            content: _Strings.articleRefundTimeContent,
          ),
        ];
      case HelpCategory.selling:
        return [
          HelpArticle(
            title: _Strings.articleBecomeSeller,
            category: _Strings.selling,
            content: _Strings.articleBecomeSellerContent,
          ),
          HelpArticle(
            title: _Strings.articleCreateListing,
            category: _Strings.selling,
            content: _Strings.articleCreateListingContent,
          ),
          HelpArticle(
            title: _Strings.articleShippingSetup,
            category: _Strings.selling,
            content: _Strings.articleShippingSetupContent,
          ),
          // PHASE 3 HARDENING: Seller-specific support articles
          HelpArticle(
            title: _Strings.articleWithdrawalFailed,
            category: _Strings.selling,
            content: _Strings.articleWithdrawalFailedContent,
          ),
          HelpArticle(
            title: _Strings.articleListingNotVisible,
            category: _Strings.selling,
            content: _Strings.articleListingNotVisibleContent,
          ),
          HelpArticle(
            title: _Strings.articleSellerPaymentPending,
            category: _Strings.selling,
            content: _Strings.articleSellerPaymentPendingContent,
          ),
        ];
      case HelpCategory.account:
        return [
          HelpArticle(
            title: _Strings.articleEditProfile,
            category: _Strings.account,
            content: _Strings.articleEditProfileContent,
          ),
          HelpArticle(
            title: _Strings.articleChangePassword,
            category: _Strings.account,
            content: _Strings.articleChangePasswordContent,
          ),
        ];
      case HelpCategory.verification:
        return [
          HelpArticle(
            title: _Strings.articleSellerVerification,
            category: _Strings.verification,
            content: _Strings.articleSellerVerificationContent,
          ),
        ];
      case HelpCategory.technical:
        return [
          HelpArticle(
            title: _Strings.articleAppNotWorking,
            category: _Strings.technical,
            content: _Strings.articleAppNotWorkingContent,
          ),
          HelpArticle(
            title: _Strings.articleClearCache,
            category: _Strings.technical,
            content: _Strings.articleClearCacheContent,
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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Scaffold(
      appBar: AppBarCustom(title: _Strings.helpArticle),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Category Badge
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
              decoration: BoxDecoration(
                color: AppColors.primaryBlue.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(16),
              ),
              child: Text(
                article.category,
                style: const TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.w600,
                  color: AppColors.primaryBlue,
                ),
              ),
            ),
            const SizedBox(height: 16),
            // Title
            Text(
              article.title,
              style: TextStyle(
                fontSize: 24,
                fontWeight: FontWeight.bold,
                color: isDark
                    ? AppColors.neutralWhite
                    : AppColors.neutralGray900,
              ),
            ),
            const SizedBox(height: 24),
            // Content
            Text(
              article.content,
              style: TextStyle(
                fontSize: 16,
                height: 1.6,
                color: isDark
                    ? AppColors.neutralGray300
                    : AppColors.neutralGray700,
              ),
            ),
            const SizedBox(height: 32),
            // Helpful Section
            _buildHelpfulSection(context, isDark),
          ],
        ),
      ),
    );
  }

  Widget _buildHelpfulSection(BuildContext context, bool isDark) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralGray100,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            _Strings.wasThisHelpful,
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w600,
              color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              Expanded(
                child: OutlinedButton.icon(
                  onPressed: () {
                    Navigator.of(context).pop();
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(
                        content: Text(_Strings.feedbackThanks),
                        backgroundColor: AppColors.successGreen,
                      ),
                    );
                  },
                  icon: const Icon(Icons.thumb_up_outlined, size: 18),
                  label: Text(_Strings.yes),
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
                      Navigator.of(context).pop();
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(content: Text(_Strings.pleaseLoginToManage)),
                      );
                    }
                  },
                  icon: const Icon(Icons.thumb_down_outlined, size: 18),
                  label: Text(_Strings.no),
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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(12),
      child: Container(
        padding: const EdgeInsets.all(16),
        decoration: BoxDecoration(
          color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(
            color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
          ),
        ),
        child: Column(
          children: [
            Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: color.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(10),
              ),
              child: Icon(icon, color: color, size: 24),
            ),
            const SizedBox(height: 8),
            Text(
              title,
              style: TextStyle(
                fontSize: 12,
                fontWeight: FontWeight.w500,
                color: isDark
                    ? AppColors.neutralWhite
                    : AppColors.neutralGray900,
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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(12),
      child: Container(
        padding: const EdgeInsets.all(16),
        decoration: BoxDecoration(
          color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(
            color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
          ),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Icon(icon, color: color, size: 24),
            const SizedBox(height: 12),
            Text(
              title,
              style: TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.w600,
                color: isDark
                    ? AppColors.neutralWhite
                    : AppColors.neutralGray900,
              ),
            ),
            const SizedBox(height: 4),
            Expanded(
              child: Text(
                subtitle,
                style: TextStyle(
                  fontSize: 11,
                  color: isDark
                      ? AppColors.neutralGray500
                      : AppColors.neutralGray600,
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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(10),
      child: Container(
        padding: const EdgeInsets.all(16),
        decoration: BoxDecoration(
          color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
          borderRadius: BorderRadius.circular(10),
          border: Border.all(
            color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
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
                    style: TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.w500,
                      color: isDark
                          ? AppColors.neutralWhite
                          : AppColors.neutralGray900,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    category,
                    style: const TextStyle(
                      fontSize: 12,
                      color: AppColors.primaryBlue,
                    ),
                  ),
                ],
              ),
            ),
            Icon(
              Icons.chevron_right,
              color: isDark
                  ? AppColors.neutralGray600
                  : AppColors.neutralGray400,
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
