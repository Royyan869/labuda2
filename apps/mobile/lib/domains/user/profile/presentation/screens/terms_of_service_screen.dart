import 'package:flutter/material.dart';
import 'package:hishumi/shared/shared.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';

/// Terms of Service Screen
/// Displays the terms of service for HiShumi platform
///
/// Size: < 200 lines (per GUIDELINES)
class TermsOfServiceScreen extends StatelessWidget {
  const TermsOfServiceScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: const AppBarCustom(title: 'Terms of Service'),
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
                '1. Acceptance of Terms',
                'By accessing and using HiShumi, you accept and agree to be bound by the terms and provision of this agreement.',
              ),
              _buildSection(
                context,
                '2. Use License',
                'Permission is granted to temporarily download one copy of the materials on HiShumi for personal, non-commercial transitory viewing only.',
              ),
              _buildSection(
                context,
                '3. User Account',
                'You are responsible for maintaining the confidentiality of your account and password. You agree to accept responsibility for all activities that occur under your account.',
              ),
              _buildSection(
                context,
                '4. User Content',
                'You retain all rights to the content you post on HiShumi. By posting content, you grant LABUDA a worldwide, non-exclusive, royalty-free license to use, reproduce, and display such content.',
              ),
              _buildSection(
                context,
                '5. Prohibited Activities',
                'You may not use HiShumi to:\n'
                    '• Post illegal, harmful, or offensive content\n'
                    '• Impersonate others or provide false information\n'
                    '• Engage in fraudulent transactions\n'
                    '• Violate intellectual property rights\n'
                    '• Harass or harm other users',
              ),
              _buildSection(
                context,
                '6. Transactions',
                'All transactions conducted through HiShumi are between buyers and sellers. LABUDA acts as a platform facilitator and is not responsible for the quality, safety, or legality of items listed.',
              ),
              _buildSection(
                context,
                '7. Payment Terms',
                'Payment processing is handled through secure third-party providers. HiShumi does not store your full payment information.',
              ),
              _buildSection(
                context,
                '8. Seller Obligations',
                'Sellers must:\n'
                    '• Provide accurate product descriptions\n'
                    '• Honor listed prices and availability\n'
                    '• Ship items promptly and securely\n'
                    '• Comply with applicable laws and regulations',
              ),
              _buildSection(
                context,
                '9. Buyer Obligations',
                'Buyers must:\n'
                    '• Provide accurate delivery information\n'
                    '• Complete payment for purchased items\n'
                    '• Communicate issues directly with sellers first\n'
                    '• Leave honest and fair reviews',
              ),
              _buildSection(
                context,
                '10. Intellectual Property',
                'The HiShumi platform, including its design, graphics, and content, is protected by intellectual property laws and remains the property of LABUDA.',
              ),
              _buildSection(
                context,
                '11. Limitation of Liability',
                'LABUDA shall not be liable for any indirect, incidental, special, consequential, or punitive damages resulting from your use or inability to use the service.',
              ),
              _buildSection(
                context,
                '12. Changes to Terms',
                'LABUDA reserves the right to modify these terms at any time. Continued use of the platform after changes constitutes acceptance of new terms.',
              ),
              _buildSection(
                context,
                '13. Termination',
                'LABUDA may terminate or suspend your account immediately, without prior notice, for conduct that violates these terms or is harmful to other users.',
              ),
              _buildSection(
                context,
                '14. Governing Law',
                'These terms shall be governed by and construed in accordance with the laws of Indonesia, without regard to its conflict of law provisions.',
              ),
              _buildSection(
                context,
                '15. Contact Information',
                'For questions about these terms, please contact us through the Help & Support section in the app.',
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
          'Terms of Service',
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
          'Please read these terms carefully before using HiShumi. By using our service, you agree to these terms.',
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
            'Agreement',
            style: Theme.of(
              context,
            ).textTheme.bodyLarge?.copyWith(fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 8),
          Text(
            'By creating an account or using HiShumi, you acknowledge that you have read, understood, and agree to be bound by these Terms of Service.',
            style: Theme.of(context).textTheme.bodyLarge?.copyWith(height: 1.5),
          ),
        ],
      ),
    );
  }
}
