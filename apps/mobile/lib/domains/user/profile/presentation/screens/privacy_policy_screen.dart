import 'package:flutter/material.dart';
import 'package:hishumi/shared/shared.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';

/// Privacy Policy Screen
/// Displays the privacy policy for HiShumi platform
///
/// Size: < 200 lines (per GUIDELINES)
class PrivacyPolicyScreen extends StatelessWidget {
  const PrivacyPolicyScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: const AppBarCustom(title: 'Privacy Policy'),
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(AppMetrics.p16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _buildHeader(context),
              const SizedBox(height: 24),
              _buildSection(
                context,
                '1. Information We Collect',
                'We collect information you provide directly to us, including:\n'
                    '• Account information (name, email, phone number)\n'
                    '• Profile information (avatar, bio, location)\n'
                    '• Transaction data (purchases, sales, payments)\n'
                    '• Communication data (messages, reviews, comments)\n'
                    '• Device information (IP address, browser type, OS)',
              ),
              _buildSection(
                context,
                '2. How We Use Your Information',
                'We use the information we collect to:\n'
                    '• Provide, maintain, and improve our services\n'
                    '• Process transactions and send related information\n'
                    '• Send you technical notices and support messages\n'
                    '• Respond to your comments and questions\n'
                    '• Detect, prevent, and address fraud and security issues\n'
                    '• Personalize your experience and show relevant content',
              ),
              _buildSection(
                context,
                '3. Information Sharing',
                'We may share your information in the following situations:\n'
                    '• With other users (profile information, products, reviews)\n'
                    '• With service providers who assist our operations\n'
                    '• For legal compliance and protection of rights\n'
                    '• In connection with business transfers or acquisitions\n'
                    '• With your consent or at your direction',
              ),
              _buildSection(
                context,
                '4. Data Security',
                'We implement appropriate technical and organizational measures to protect your personal information. However, no method of transmission over the internet is 100% secure.',
              ),
              _buildSection(
                context,
                '5. Data Retention',
                'We retain your information for as long as your account is active or as needed to provide services. You may request deletion of your account and data at any time.',
              ),
              _buildSection(
                context,
                '6. Your Rights',
                'You have the right to:\n'
                    '• Access your personal information\n'
                    '• Correct inaccurate information\n'
                    '• Request deletion of your data\n'
                    '• Object to processing of your information\n'
                    '• Export your data in a portable format\n'
                    '• Withdraw consent at any time',
              ),
              _buildSection(
                context,
                '7. Cookies and Tracking',
                'We use cookies and similar tracking technologies to collect information about your browsing activities. You can control cookie preferences through your browser settings.',
              ),
              _buildSection(
                context,
                '8. Third-Party Services',
                'Our service may contain links to third-party websites or services. We are not responsible for the privacy practices of these third parties.',
              ),
              _buildSection(
                context,
                '9. Children\'s Privacy',
                'HiShumi is not intended for users under the age of 13. We do not knowingly collect personal information from children under 13.',
              ),
              _buildSection(
                context,
                '10. International Data Transfers',
                'Your information may be transferred to and processed in countries other than your country of residence. We ensure appropriate safeguards are in place.',
              ),
              _buildSection(
                context,
                '11. Changes to Privacy Policy',
                'We may update this privacy policy from time to time. We will notify you of any changes by posting the new policy on this page and updating the "Last updated" date.',
              ),
              _buildSection(
                context,
                '12. Contact Us',
                'If you have questions about this privacy policy or our data practices, please contact us through the Help & Support section in the app.',
              ),
              const SizedBox(height: 16),
              _buildFooter(context),
              const SizedBox(height: 32),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildHeader(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Privacy Policy',
          style: context.typeRoles.titleProminent.copyWith(
            fontWeight: FontWeight.bold,
          ),
        ),
        const SizedBox(height: 8),
        Text(
          'Last updated: January 5, 2026',
          style: Theme.of(context).textTheme.bodyLarge?.copyWith(
            color: Theme.of(
              context,
            ).textTheme.bodyMedium?.color?.withValues(alpha: 0.6),
          ),
        ),
        const SizedBox(height: 16),
        Text(
          'LABUDA respects your privacy and is committed to protecting your personal data. This policy explains how we collect, use, and safeguard your information.',
          style: Theme.of(context).textTheme.bodyLarge,
        ),
      ],
    );
  }

  Widget _buildSection(BuildContext context, String title, String content) {
    return Padding(
      padding: const EdgeInsets.only(bottom: AppMetrics.p24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: context.typeRoles.titleCompact.copyWith(
              fontWeight: FontWeight.bold,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            content,
            style: Theme.of(context).textTheme.bodyLarge?.copyWith(height: 1.5),
          ),
        ],
      ),
    );
  }

  Widget _buildFooter(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(AppShape.r8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Your Privacy Matters',
            style: Theme.of(
              context,
            ).textTheme.bodyLarge?.copyWith(fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 8),
          Text(
            'We are committed to maintaining the trust and confidence of our users. If you have any concerns about your privacy, please don\'t hesitate to contact us.',
            style: Theme.of(context).textTheme.bodyLarge?.copyWith(height: 1.5),
          ),
        ],
      ),
    );
  }
}
