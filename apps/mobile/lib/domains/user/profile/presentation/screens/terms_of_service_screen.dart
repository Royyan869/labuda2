import 'package:flutter/material.dart';
import 'package:labuda/shared/shared.dart';

/// Terms of Service Screen
/// Displays the terms of service for LABUDA platform
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
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _buildHeader(context),
              const SizedBox(height: 24),
              _buildSection(
                '1. Acceptance of Terms',
                'By accessing and using LABUDA, you accept and agree to be bound by the terms and provision of this agreement.',
              ),
              _buildSection(
                '2. Use License',
                'Permission is granted to temporarily download one copy of the materials on LABUDA for personal, non-commercial transitory viewing only.',
              ),
              _buildSection(
                '3. User Account',
                'You are responsible for maintaining the confidentiality of your account and password. You agree to accept responsibility for all activities that occur under your account.',
              ),
              _buildSection(
                '4. User Content',
                'You retain all rights to the content you post on LABUDA. By posting content, you grant LABUDA a worldwide, non-exclusive, royalty-free license to use, reproduce, and display such content.',
              ),
              _buildSection(
                '5. Prohibited Activities',
                'You may not use LABUDA to:\n'
                    '• Post illegal, harmful, or offensive content\n'
                    '• Impersonate others or provide false information\n'
                    '• Engage in fraudulent transactions\n'
                    '• Violate intellectual property rights\n'
                    '• Harass or harm other users',
              ),
              _buildSection(
                '6. Transactions',
                'All transactions conducted through LABUDA are between buyers and sellers. LABUDA acts as a platform facilitator and is not responsible for the quality, safety, or legality of items listed.',
              ),
              _buildSection(
                '7. Payment Terms',
                'Payment processing is handled through secure third-party providers. LABUDA does not store your full payment information.',
              ),
              _buildSection(
                '8. Seller Obligations',
                'Sellers must:\n'
                    '• Provide accurate product descriptions\n'
                    '• Honor listed prices and availability\n'
                    '• Ship items promptly and securely\n'
                    '• Comply with applicable laws and regulations',
              ),
              _buildSection(
                '9. Buyer Obligations',
                'Buyers must:\n'
                    '• Provide accurate delivery information\n'
                    '• Complete payment for purchased items\n'
                    '• Communicate issues directly with sellers first\n'
                    '• Leave honest and fair reviews',
              ),
              _buildSection(
                '10. Intellectual Property',
                'The LABUDA platform, including its design, graphics, and content, is protected by intellectual property laws and remains the property of LABUDA.',
              ),
              _buildSection(
                '11. Limitation of Liability',
                'LABUDA shall not be liable for any indirect, incidental, special, consequential, or punitive damages resulting from your use or inability to use the service.',
              ),
              _buildSection(
                '12. Changes to Terms',
                'LABUDA reserves the right to modify these terms at any time. Continued use of the platform after changes constitutes acceptance of new terms.',
              ),
              _buildSection(
                '13. Termination',
                'LABUDA may terminate or suspend your account immediately, without prior notice, for conduct that violates these terms or is harmful to other users.',
              ),
              _buildSection(
                '14. Governing Law',
                'These terms shall be governed by and construed in accordance with the laws of Indonesia, without regard to its conflict of law provisions.',
              ),
              _buildSection(
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
        const Text(
          'Terms of Service',
          style: TextStyle(fontSize: 24, fontWeight: FontWeight.bold),
        ),
        const SizedBox(height: 8),
        Text(
          'Last updated: January 5, 2026',
          style: TextStyle(
            fontSize: 14,
            color: Theme.of(
              context,
            ).textTheme.bodyMedium?.color?.withValues(alpha: 0.6),
          ),
        ),
        const SizedBox(height: 16),
        const Text(
          'Please read these terms carefully before using LABUDA. By using our service, you agree to these terms.',
          style: TextStyle(fontSize: 14),
        ),
      ],
    );
  }

  Widget _buildSection(String title, String content) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: const TextStyle(fontSize: 16, fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 8),
          Text(content, style: const TextStyle(fontSize: 14, height: 1.5)),
        ],
      ),
    );
  }

  Widget _buildFooter(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(8),
      ),
      child: const Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Agreement',
            style: TextStyle(fontSize: 14, fontWeight: FontWeight.bold),
          ),
          SizedBox(height: 8),
          Text(
            'By creating an account or using LABUDA, you acknowledge that you have read, understood, and agree to be bound by these Terms of Service.',
            style: TextStyle(fontSize: 13, height: 1.5),
          ),
        ],
      ),
    );
  }
}
