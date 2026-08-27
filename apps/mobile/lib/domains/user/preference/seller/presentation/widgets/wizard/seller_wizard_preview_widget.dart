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
  final String bio;
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
  final bool isDark;

  const SellerWizardPreviewWidget({
    super.key,
    required this.username,
    required this.bio,
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
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _buildSectionHeader('Preview & Confirmation', isDark),
          const SizedBox(height: 8),
          Text(
            'Review the package, account data, and store details before you continue to payment.',
            style: TextStyle(
              fontSize: 14,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
          ),
          const SizedBox(height: 24),

          _buildSection(
            title: 'Package & Fee',
            isDark: isDark,
            children: [
              _buildInfoRow(
                'Fee',
                'Rp ${AppFormatters.formatCurrency(packageFee)}',
                isDark,
              ),
              _buildInfoRow('Duration', '$packageDurationDays days', isDark),
              const SizedBox(height: 4),
              Text(
                'Payment is required before seller authority becomes active. KYC and bank review are handled later for payout access.',
                style: TextStyle(
                  fontSize: 12,
                  height: 1.5,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
              ),
            ],
          ),

          const SizedBox(height: 24),

          _buildSection(
            title: 'Account Prerequisites',
            isDark: isDark,
            children: [
              _buildInfoRow('Username', username, isDark),
              _buildInfoRow(
                'Email Status',
                emailVerified ? 'Verified' : 'Not verified',
                isDark,
              ),
              _buildInfoRow('Phone', phoneNumber, isDark),
              _buildInfoRow('Sender Address', senderAddress, isDark),
              if (bio.isNotEmpty) ...[
                const SizedBox(height: 8),
                Text(
                  'Bio / Description',
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: isDark
                        ? AppColors.neutralGray300
                        : AppColors.neutralGray700,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  bio,
                  style: TextStyle(
                    fontSize: 13,
                    height: 1.5,
                    color: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray600,
                  ),
                ),
              ],
            ],
          ),

          const SizedBox(height: 24),

          _buildSection(
            title: 'Store Information',
            isDark: isDark,
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
                        color: isDark
                            ? AppColors.darkGray600
                            : AppColors.neutralGray300,
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
                                            _fallback(),
                                  )
                                : Image.file(
                                    File(selectedStorePhotoPath!),
                                    width: 96,
                                    height: 96,
                                    fit: BoxFit.cover,
                                    errorBuilder:
                                        (context, error, stackTrace) =>
                                            _fallback(),
                                  ))
                          : AppImage.avatar(imageUrl: farmPhotoUrl!, size: 96),
                    ),
                  ),
                ),
              _buildInfoRow('Store/Farm Name', farmName, isDark),
            ],
          ),

          const SizedBox(height: 24),

          _buildTermsAgreement(),

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
                      color: isDark
                          ? AppColors.neutralGray300
                          : AppColors.neutralGray700,
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

  Widget _buildSectionHeader(String title, bool isDark) {
    return Text(
      title,
      style: TextStyle(
        fontSize: 20,
        fontWeight: FontWeight.bold,
        color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
      ),
    );
  }

  Widget _buildSection({
    required String title,
    required bool isDark,
    required List<Widget> children,
  }) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: TextStyle(
              fontSize: 16,
              fontWeight: FontWeight.bold,
              color: isDark
                  ? AppColors.neutralGray200
                  : AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 16),
          ...children,
        ],
      ),
    );
  }

  Widget _buildInfoRow(String label, String value, bool isDark) {
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
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
              ),
            ),
          ),
          Expanded(
            child: Text(
              value,
              style: TextStyle(
                fontSize: 13,
                color: isDark
                    ? AppColors.neutralGray200
                    : AppColors.neutralGray800,
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildTermsAgreement() {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
        ),
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
                  color: isDark
                      ? AppColors.neutralGray300
                      : AppColors.neutralGray700,
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _fallback() {
    return Icon(
      Icons.store_outlined,
      size: 48,
      color: isDark ? AppColors.neutralGray500 : AppColors.neutralGray400,
    );
  }
}
