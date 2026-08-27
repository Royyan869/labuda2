import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

/// Step 2: Store Information Widget
///
/// Collects the seller's store/farm name plus an optional logo/photo.
/// Canonical description fields live in the account step via AuthUser.bio.
class SellerWizardStep2Widget extends StatelessWidget {
  final GlobalKey<FormState> formKey;
  final TextEditingController farmNameController;
  final VoidCallback onStorePhotoUpload;
  final String? farmPhotoUrl;
  final String? selectedStorePhotoPath;
  final bool isDark;
  final Widget? feeNoticeWidget;

  const SellerWizardStep2Widget({
    super.key,
    required this.formKey,
    required this.farmNameController,
    required this.onStorePhotoUpload,
    this.farmPhotoUrl,
    this.selectedStorePhotoPath,
    required this.isDark,
    this.feeNoticeWidget,
  });

  @override
  Widget build(BuildContext context) {
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
              color: isDark
                  ? AppColors.neutralGray200
                  : AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Isi nama toko/farm dan unggah logo atau foto opsional jika tersedia.',
            style: TextStyle(
              fontSize: 14,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
          ),
          const SizedBox(height: 24),

          Center(
            child: Column(
              children: [
                _buildStoreLogoSection(),
                const SizedBox(height: 8),
                Text(
                  'Logo/Foto Opsional',
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w500,
                    color: isDark
                        ? AppColors.neutralGray300
                        : AppColors.neutralGray700,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 24),

          AppTextField(
            controller: farmNameController,
            labelText: 'Nama Toko/Farm *',
            hintText: 'Example: Mutiara Koi Farm',
            prefixIcon: Icons.store_outlined,
            validator: (value) =>
                value == null || value.trim().isEmpty ? 'Required' : null,
          ),
        ],
      ),
    );
  }

  Widget _buildStoreLogoSection() {
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
          color: isDark ? AppColors.darkGray700 : AppColors.neutralGray100,
          border: Border.all(
            color: isDark ? AppColors.darkGray600 : AppColors.neutralGray300,
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
                            _fallback(),
                      )
                    : Image.file(
                        File(selectedStorePhotoPath!),
                        fit: BoxFit.cover,
                        errorBuilder: (context, error, stackTrace) =>
                            _fallback(),
                      ))
              : hasRemoteImage
              ? AppImage.avatar(imageUrl: farmPhotoUrl!, size: 120)
              : _fallback(),
        ),
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
