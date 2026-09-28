import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

/// Preview step for seller onboarding.
///
/// This screen summarizes the backend-authoritative account prerequisites and
/// the seller/store data before the user moves to payment.
class SellerWizardPreviewWidget extends StatelessWidget {
  final String username;
  final String phoneNumber;
  final String senderAddress;
  final bool emailVerified;
  final double packageFee;
  final int packageDurationDays;

  final String farmName;
  final String? farmPhotoUrl;
  final String? selectedStorePhotoPath;

  final bool agreeToTerms;
  final ValueChanged<bool> onAgreeToTermsChanged;

  const SellerWizardPreviewWidget({
    super.key,
    required this.username,
    required this.phoneNumber,
    required this.senderAddress,
    required this.emailVerified,
    required this.packageFee,
    required this.packageDurationDays,
    required this.farmName,
    this.farmPhotoUrl,
    this.selectedStorePhotoPath,
    required this.agreeToTerms,
    required this.onAgreeToTermsChanged,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _buildSectionHeader(context, 'Preview & Confirmation'),
          const SizedBox(height: 8),
          Text(
            'Review the package, account data, and store details before you continue to payment.',
            style: TextStyle(
              fontSize: 14,
              color: scheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 24),

          _buildSection(
            context,
            title: 'Package & Fee',
            children: [
              _buildInfoRow(
                context,
                'Fee',
                AppFormatters.formatCurrency(packageFee),
              ),
              _buildInfoRow(context, 'Duration', '$packageDurationDays days'),
              const SizedBox(height: 4),
              Text(
                'Payment is required before seller authority becomes active. KYC and bank review are handled later for payout access.',
                style: TextStyle(
                  fontSize: 12,
                  height: 1.5,
                  color: scheme.onSurfaceVariant,
                ),
              ),
            ],
          ),

          const SizedBox(height: 24),

          _buildSection(
            context,
            title: 'Account Prerequisites',
            children: [
              _buildInfoRow(context, 'Username', username),
              _buildInfoRow(
                context,
                'Email Status',
                emailVerified ? 'Verified' : 'Not verified',
              ),
              _buildInfoRow(context, 'Phone', phoneNumber),
              _buildInfoRow(context, 'Sender Address', senderAddress),
            ],
          ),

          const SizedBox(height: 24),

          _buildSection(
            context,
            title: 'Store Information',
            children: [
              if (selectedStorePhotoPath != null || farmPhotoUrl != null)
                Center(
                  child: Container(
                    width: 100,
                    height: 100,
                    margin: const EdgeInsets.only(bottom: 16),
                    decoration: BoxDecoration(
                      shape: BoxShape.circle,
                      border: Border.all(
                        color: scheme.outlineVariant,
                        width: 2,
                      ),
                    ),
                    child: ClipOval(
                      child: selectedStorePhotoPath != null
                          ? (kIsWeb
                                ? Image.network(
                                    selectedStorePhotoPath!,
                                    width: 96,
                                    height: 96,
                                    fit: BoxFit.cover,
                                    errorBuilder:
                                        (context, error, stackTrace) =>
                                            _fallback(context),
                                  )
                                : Image.file(
                                    File(selectedStorePhotoPath!),
                                    width: 96,
                                    height: 96,
                                    fit: BoxFit.cover,
                                    errorBuilder:
                                        (context, error, stackTrace) =>
                                            _fallback(context),
                                  ))
                          : AppImage.avatar(imageUrl: farmPhotoUrl!, size: 96),
                    ),
                  ),
                ),
              _buildInfoRow(context, 'Store/Farm Name', farmName),
            ],
          ),

          const SizedBox(height: 24),

          _buildTermsAgreement(context),

          const SizedBox(height: 24),

          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: AppColors.warningYellow.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(12),
              border: Border.all(
                color: AppColors.warningYellow.withValues(alpha: 0.3),
              ),
            ),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Icon(
                  Icons.info_outline,
                  color: AppColors.warningYellow,
                  size: 20,
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(
                    'Payment activates seller authority. KYC and bank review are handled later for payout access.',
                    style: TextStyle(
                      fontSize: 13,
                      color: scheme.onSurfaceVariant,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSectionHeader(BuildContext context, String title) {
    return Text(
      title,
      style: TextStyle(
        fontSize: 20,
        fontWeight: FontWeight.bold,
        color: Theme.of(context).colorScheme.onSurface,
      ),
    );
  }

  Widget _buildSection(
    BuildContext context, {
    required String title,
    required List<Widget> children,
  }) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: scheme.surface,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: TextStyle(
              fontSize: 16,
              fontWeight: FontWeight.bold,
              color: scheme.onSurface,
            ),
          ),
          const SizedBox(height: 16),
          ...children,
        ],
      ),
    );
  }

  Widget _buildInfoRow(BuildContext context, String label, String value) {
    final scheme = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 110,
            child: Text(
              label,
              style: TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w600,
                color: scheme.onSurfaceVariant,
              ),
            ),
          ),
          Expanded(
            child: Text(
              value,
              style: TextStyle(
                fontSize: 13,
                color: scheme.onSurface,
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildTermsAgreement(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: scheme.surface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Checkbox(
            value: agreeToTerms,
            onChanged: (value) => onAgreeToTermsChanged(value ?? false),
          ),
          Expanded(
            child: Padding(
              padding: const EdgeInsets.only(top: 12),
              child: Text(
                'I agree to the Seller Terms and understand that seller authority starts after payment is confirmed.',
                style: TextStyle(
                  fontSize: 13,
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _fallback(BuildContext context) {
    return Icon(
      Icons.store_outlined,
      size: 48,
      color: Theme.of(context).colorScheme.onSurfaceVariant,
    );
  }
}
