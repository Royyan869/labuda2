import 'package:flutter/material.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/user/preference/seller/presentation/widgets/wizard/store_photo_preview.dart';
import 'package:hishumi/shared/shared.dart';

/// Preview step for seller onboarding.
///
/// This screen summarizes the backend-authoritative account prerequisites and
/// the seller/store data before the user moves to payment.
class SellerWizardPreviewWidget extends StatelessWidget {
  final String username;
  final String phoneNumber;
  final String primaryAddress;
  final bool emailVerified;
  final double packageFee;
  final int packageDurationDays;

  final String farmName;
  final String? farmPhotoUrl;
  final String? selectedStorePhotoPath;
  final bool isStorePhotoUploading;

  final bool agreeToTerms;
  final ValueChanged<bool> onAgreeToTermsChanged;

  const SellerWizardPreviewWidget({
    super.key,
    required this.username,
    required this.phoneNumber,
    required this.primaryAddress,
    required this.emailVerified,
    required this.packageFee,
    required this.packageDurationDays,
    required this.farmName,
    this.farmPhotoUrl,
    this.selectedStorePhotoPath,
    this.isStorePhotoUploading = false,
    required this.agreeToTerms,
    required this.onAgreeToTermsChanged,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return SingleChildScrollView(
      padding: const EdgeInsets.all(AppMetrics.p24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _buildSectionHeader(context, 'Preview & Confirmation'),
          const SizedBox(height: 8),
          Text(
            'Review the package, account data, and store details before you continue to payment.',
            style: context.typeRoles.bodyDense.copyWith(
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
              _buildInfoRow(context, 'Alamat Utama', primaryAddress),
            ],
          ),

          const SizedBox(height: 24),

          _buildSection(
            context,
            title: 'Store Information',
            children: [
              if (selectedStorePhotoPath != null || farmPhotoUrl != null)
                Center(
                  child: Padding(
                    padding: const EdgeInsets.only(bottom: AppMetrics.p16),
                    child: StorePhotoPreview(
                      localPath: selectedStorePhotoPath,
                      displayUrl: farmPhotoUrl,
                      isUploading: isStorePhotoUploading,
                      size: 100,
                    ),
                  ),
                ),
              _buildInfoRow(context, 'Store/Farm Name', farmName),
            ],
          ),

          const SizedBox(height: 24),

          _buildTermsAgreement(context),

        ],
      ),
    );
  }

  Widget _buildSectionHeader(BuildContext context, String title) {
    return Text(
      title,
      style: context.typeRoles.titleSection.copyWith(
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
      padding: const EdgeInsets.all(AppMetrics.p24),
      decoration: BoxDecoration(
        color: scheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r16),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: context.typeRoles.titleCompact.copyWith(
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
      padding: const EdgeInsets.only(bottom: AppMetrics.p12),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: AppContentSize.termLabel,
            child: Text(
              label,
              style: context.typeRoles.bodyDense.copyWith(
                fontWeight: FontWeight.w600,
                color: scheme.onSurfaceVariant,
              ),
            ),
          ),
          Expanded(
            child: Text(
              value,
              style: context.typeRoles.bodyDense.copyWith(
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
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: scheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: MergeSemantics(
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Checkbox(
              value: agreeToTerms,
              onChanged: (value) => onAgreeToTermsChanged(value ?? false),
            ),
            Expanded(
              child: Padding(
                padding: const EdgeInsets.only(top: AppMetrics.p12),
                child: Text(
                  'I agree to the Seller Terms and understand that seller authority starts after payment is confirmed.',
                  style: context.typeRoles.bodyDense.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
