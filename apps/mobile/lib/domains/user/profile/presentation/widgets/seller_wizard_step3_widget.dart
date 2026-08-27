import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'ktp_preview_section.dart';
import 'ktp_upload_section.dart';
import 'selfie_verification_section.dart';

/// Step 3: Identity Verification (KTP) Widget
///
/// Extracted component for seller upgrade wizard
/// Supports 2 modes: Preview (existing KTP) and Upload (new KTP)
/// Includes selfie verification (holding KTP)
class SellerWizardStep3Widget extends StatelessWidget {
  final GlobalKey<FormState> formKey;
  final bool hasExistingKTP;
  final String? ktpImageUrl;
  final String? selfieImageUrl;
  final TextEditingController ktpNumberController;
  final TextEditingController ktpNameController;
  final VoidCallback onKTPUpload;
  final VoidCallback onSelfieUpload;
  final VoidCallback onChangeKTP;
  final bool isDark;

  const SellerWizardStep3Widget({
    super.key,
    required this.formKey,
    required this.hasExistingKTP,
    required this.ktpImageUrl,
    required this.selfieImageUrl,
    required this.ktpNumberController,
    required this.ktpNameController,
    required this.onKTPUpload,
    required this.onSelfieUpload,
    required this.onChangeKTP,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    return Form(
      key: formKey,
      child: ListView(
        padding: const EdgeInsets.all(24),
        children: [
          Text(
            'Identity Verification',
            style: TextStyle(
              fontSize: 20,
              fontWeight: FontWeight.bold,
              color: isDark
                  ? AppColors.neutralGray200
                  : AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 24),
          if (hasExistingKTP)
            KTPPreviewSection(
              ktpImageUrl: ktpImageUrl,
              ktpNumberController: ktpNumberController,
              ktpNameController: ktpNameController,
              onChangeKTP: onChangeKTP,
              isDark: isDark,
            )
          else
            KTPUploadSection(
              ktpImageUrl: ktpImageUrl,
              ktpNumberController: ktpNumberController,
              ktpNameController: ktpNameController,
              onKTPUpload: onKTPUpload,
              isDark: isDark,
            ),
          const SizedBox(height: 32),
          SelfieVerificationSection(
            selfieImageUrl: selfieImageUrl,
            onSelfieUpload: onSelfieUpload,
            isDark: isDark,
          ),
        ],
      ),
    );
  }
}
