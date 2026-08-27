import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Widget for uploading KTP with proper frame and validation.
class KTPUploadSection extends StatelessWidget {
  final String? ktpImageUrl;
  final TextEditingController ktpNumberController;
  final TextEditingController ktpNameController;
  final VoidCallback onKTPUpload;
  final bool isDark;

  const KTPUploadSection({
    super.key,
    required this.ktpImageUrl,
    required this.ktpNumberController,
    required this.ktpNameController,
    required this.onKTPUpload,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // KTP Card with proper ratio (85.6mm x 53.98mm = 1.585:1)
        AspectRatio(
          aspectRatio: 1.585,
          child: Container(
            decoration: BoxDecoration(
              color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
              borderRadius: BorderRadius.circular(12),
              border: Border.all(
                color: ktpImageUrl != null
                    ? AppColors.successGreen
                    : (isDark
                          ? AppColors.darkGray600
                          : AppColors.neutralGray200),
                width: 2,
              ),
            ),
            child: ktpImageUrl != null ? _buildUploadedKTP() : _buildEmptyKTP(),
          ),
        ),
        const SizedBox(height: 24),
        _buildKTPNumberField(),
        const SizedBox(height: 16),
        _buildKTPNameField(),
      ],
    );
  }

  Widget _buildUploadedKTP() {
    return Stack(
      children: [
        ClipRRect(
          borderRadius: BorderRadius.circular(10),
          child: Image.network(
            ktpImageUrl!,
            width: double.infinity,
            height: double.infinity,
            fit: BoxFit.cover,
          ),
        ),
        Positioned(
          top: 8,
          right: 8,
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
            decoration: BoxDecoration(
              color: AppColors.successGreen,
              borderRadius: BorderRadius.circular(12),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: const [
                Icon(Icons.check_circle, color: Colors.white, size: 16),
                SizedBox(width: 4),
                Text(
                  'Verified',
                  style: TextStyle(
                    color: Colors.white,
                    fontSize: 12,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ],
            ),
          ),
        ),
        Positioned(
          bottom: 8,
          right: 8,
          child: Material(
            color: Colors.transparent,
            child: InkWell(
              onTap: onKTPUpload,
              borderRadius: BorderRadius.circular(8),
              child: Container(
                padding: const EdgeInsets.symmetric(
                  horizontal: 12,
                  vertical: 8,
                ),
                decoration: BoxDecoration(
                  color: const Color(0xCC000000),
                  borderRadius: BorderRadius.circular(8),
                  border: Border.all(color: AppColors.primaryRed, width: 1.5),
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: const [
                    Icon(Icons.camera_alt, color: Colors.white, size: 16),
                    SizedBox(width: 6),
                    Text(
                      'Retake Photo',
                      style: TextStyle(
                        color: Colors.white,
                        fontSize: 12,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildEmptyKTP() {
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onKTPUpload,
        borderRadius: BorderRadius.circular(12),
        child: Stack(
          children: [
            Center(
              child: Container(
                margin: const EdgeInsets.all(20),
                decoration: BoxDecoration(
                  border: Border.all(
                    color: isDark
                        ? AppColors.neutralGray600
                        : AppColors.neutralGray400,
                    width: 2,
                    strokeAlign: BorderSide.strokeAlignInside,
                  ),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Icon(
                      Icons.credit_card,
                      size: 48,
                      color: isDark
                          ? AppColors.neutralGray500
                          : AppColors.neutralGray500,
                    ),
                    const SizedBox(height: 12),
                    Text(
                      'Take Photo of Your KTP',
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w600,
                        color: isDark
                            ? AppColors.neutralGray400
                            : AppColors.neutralGray700,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      'Position KTP within frame',
                      style: TextStyle(
                        fontSize: 12,
                        color: isDark
                            ? AppColors.neutralGray500
                            : AppColors.neutralGray600,
                      ),
                    ),
                  ],
                ),
              ),
            ),
            Positioned(
              bottom: 12,
              right: 12,
              child: Container(
                padding: const EdgeInsets.all(12),
                decoration: const BoxDecoration(
                  color: AppColors.primaryRed,
                  shape: BoxShape.circle,
                ),
                child: const Icon(
                  Icons.camera_alt,
                  color: Colors.white,
                  size: 24,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildKTPNumberField() {
    return TextFormField(
      controller: ktpNumberController,
      decoration: InputDecoration(
        labelText: 'KTP Number *',
        hintText: '16 digits',
        hintStyle: TextStyle(
          color: isDark
              ? AppColors.neutralGray600.withValues(alpha: 0.5)
              : AppColors.neutralGray400.withValues(alpha: 0.6),
        ),
        floatingLabelBehavior: FloatingLabelBehavior.always,
        border: OutlineInputBorder(borderRadius: BorderRadius.circular(12)),
      ),
      keyboardType: TextInputType.number,
      maxLength: 16,
      validator: (value) {
        if (value?.isEmpty ?? true) return 'Required';
        if (value!.length != 16) return 'KTP must be 16 digits';
        return null;
      },
    );
  }

  Widget _buildKTPNameField() {
    return TextFormField(
      controller: ktpNameController,
      decoration: InputDecoration(
        labelText: 'Name on KTP *',
        hintText: 'Full name according to KTP',
        hintStyle: TextStyle(
          color: isDark
              ? AppColors.neutralGray600.withValues(alpha: 0.5)
              : AppColors.neutralGray400.withValues(alpha: 0.6),
        ),
        floatingLabelBehavior: FloatingLabelBehavior.always,
        border: OutlineInputBorder(borderRadius: BorderRadius.circular(12)),
      ),
      validator: (value) => value?.isEmpty ?? true ? 'Required' : null,
    );
  }
}
