import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:labuda/domains/user/preference/seller/presentation/widgets/wizard/store_name_form_field.dart';
import 'package:labuda/shared/shared.dart';

/// Step 2: Store Information Widget
///
/// Collects the seller's store/farm name plus an optional logo/photo.
class SellerWizardStep2Widget extends StatelessWidget {
  final GlobalKey<FormState> formKey;
  final TextEditingController farmNameController;
  final VoidCallback onStorePhotoUpload;
  final String? farmPhotoUrl;
  final String? selectedStorePhotoPath;
  final Widget? feeNoticeWidget;

  const SellerWizardStep2Widget({
    super.key,
    required this.formKey,
    required this.farmNameController,
    required this.onStorePhotoUpload,
    this.farmPhotoUrl,
    this.selectedStorePhotoPath,
    this.feeNoticeWidget,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Form(
      key: formKey,
      child: ListView(
        padding: const EdgeInsets.all(24),
        children: [
          if (feeNoticeWidget != null) ...[
            feeNoticeWidget!,
            const SizedBox(height: 24),
          ],

          Text(
            'Informasi Toko/Farm',
            style: TextStyle(
              fontSize: 20,
              fontWeight: FontWeight.bold,
              color: scheme.onSurface,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Isi nama toko/farm dan unggah logo atau foto opsional jika tersedia.',
            style: TextStyle(
              fontSize: 14,
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
                  style: TextStyle(
                    fontSize: 14,
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
    final scheme = Theme.of(context).colorScheme;
    final hasLocalSelection =
        selectedStorePhotoPath != null && selectedStorePhotoPath!.isNotEmpty;
    final hasRemoteImage = farmPhotoUrl != null && farmPhotoUrl!.isNotEmpty;

    return GestureDetector(
      onTap: onStorePhotoUpload,
      child: Container(
        width: 120,
        height: 120,
        decoration: BoxDecoration(
          shape: BoxShape.circle,
          color: scheme.surfaceContainer,
          border: Border.all(
            color: scheme.outlineVariant,
            width: 2,
          ),
        ),
        child: ClipOval(
          child: hasLocalSelection
              ? (kIsWeb
                    ? Image.network(
                        selectedStorePhotoPath!,
                        fit: BoxFit.cover,
                        errorBuilder: (context, error, stackTrace) =>
                            _fallback(context),
                      )
                    : Image.file(
                        File(selectedStorePhotoPath!),
                        fit: BoxFit.cover,
                        errorBuilder: (context, error, stackTrace) =>
                            _fallback(context),
                      ))
              : hasRemoteImage
              ? AppImage.avatar(imageUrl: farmPhotoUrl!, size: 120)
              : _fallback(context),
        ),
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
