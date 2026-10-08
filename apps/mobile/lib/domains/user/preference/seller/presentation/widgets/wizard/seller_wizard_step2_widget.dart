import 'package:flutter/material.dart';
import 'package:labuda/domains/user/preference/seller/presentation/widgets/wizard/store_name_form_field.dart';
import 'package:labuda/domains/user/preference/seller/presentation/widgets/wizard/store_photo_preview.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Step 2: Store Information Widget
///
/// Collects the seller's store/farm name plus an optional logo/photo.
class SellerWizardStep2Widget extends StatelessWidget {
  final GlobalKey<FormState> formKey;
  final TextEditingController farmNameController;
  final VoidCallback onStorePhotoUpload;
  final String? farmPhotoUrl;
  final String? selectedStorePhotoPath;
  final bool isStorePhotoUploading;
  final Widget? feeNoticeWidget;

  const SellerWizardStep2Widget({
    super.key,
    required this.formKey,
    required this.farmNameController,
    required this.onStorePhotoUpload,
    this.farmPhotoUrl,
    this.selectedStorePhotoPath,
    this.isStorePhotoUploading = false,
    this.feeNoticeWidget,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Form(
      key: formKey,
      child: ListView(
        padding: const EdgeInsets.all(AppMetrics.p24),
        children: [
          if (feeNoticeWidget != null) ...[
            feeNoticeWidget!,
            const SizedBox(height: 24),
          ],

          Text(
            'Informasi Toko/Farm',
            style: context.typeRoles.titleSection.copyWith(
              fontWeight: FontWeight.bold,
              color: scheme.onSurface,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Isi nama toko/farm dan unggah logo atau foto opsional jika tersedia.',
            style: context.typeRoles.bodyDense.copyWith(
              color: scheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 24),

          Center(
            child: Column(
              children: [
                _buildStoreLogoSection(context),
                const SizedBox(height: 8),
                Text(
                  'Logo/Foto Opsional',
                  style: context.typeRoles.bodyDense.copyWith(
                    fontWeight: FontWeight.w500,
                    color: scheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 24),

          // SINGLE AUTHORITY: canonical store/farm name field shared with
          // edit profile — same label, hint, and validation.
          StoreNameFormField(controller: farmNameController),
        ],
      ),
    );
  }

  Widget _buildStoreLogoSection(BuildContext context) {
    return GestureDetector(
      onTap: onStorePhotoUpload,
      child: StorePhotoPreview(
        localPath: selectedStorePhotoPath,
        displayUrl: farmPhotoUrl,
        isUploading: isStorePhotoUploading,
        size: 120,
      ),
    );
  }
}
