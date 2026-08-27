import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Widget for selfie verification with KTP.
class SelfieVerificationSection extends StatelessWidget {
  final String? selfieImageUrl;
  final VoidCallback onSelfieUpload;
  final bool isDark;

  const SelfieVerificationSection({
    super.key,
    required this.selfieImageUrl,
    required this.onSelfieUpload,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _buildHeader(),
        const SizedBox(height: 16),
        _buildSelfieFrame(),
        const SizedBox(height: 12),
        _buildInstructions(),
      ],
    );
  }

  Widget _buildHeader() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Icon(
              Icons.camera_alt_outlined,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
              size: 20,
            ),
            const SizedBox(width: 8),
            Text(
              'Selfie with KTP',
              style: TextStyle(
                fontSize: 18,
                fontWeight: FontWeight.bold,
                color: isDark
                    ? AppColors.neutralGray200
                    : AppColors.neutralGray900,
              ),
            ),
          ],
        ),
        const SizedBox(height: 8),
        Text(
          'Take a selfie holding your KTP for identity verification',
          style: TextStyle(
            fontSize: 13,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
          ),
        ),
      ],
    );
  }

  Widget _buildSelfieFrame() {
    return Center(
      child: AspectRatio(
        aspectRatio: 3 / 4,
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 280),
          child: Container(
            decoration: BoxDecoration(
              color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
              borderRadius: BorderRadius.circular(16),
              border: Border.all(
                color: selfieImageUrl != null
                    ? AppColors.successGreen
                    : (isDark
                          ? AppColors.darkGray600
                          : AppColors.neutralGray200),
                width: 2,
              ),
            ),
            child: selfieImageUrl != null
                ? _buildUploadedSelfie()
                : _buildEmptySelfie(),
          ),
        ),
      ),
    );
  }

  Widget _buildUploadedSelfie() {
    return Stack(
      children: [
        ClipRRect(
          borderRadius: BorderRadius.circular(14),
          child: Image.network(
            selfieImageUrl!,
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
          bottom: 12,
          left: 0,
          right: 0,
          child: Center(
            child: Material(
              color: Colors.transparent,
              child: InkWell(
                onTap: onSelfieUpload,
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
        ),
      ],
    );
  }

  Widget _buildEmptySelfie() {
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onSelfieUpload,
        borderRadius: BorderRadius.circular(16),
        child: Stack(
          children: [
            Center(
              child: Container(
                margin: const EdgeInsets.all(24),
                decoration: BoxDecoration(
                  border: Border.all(
                    color: isDark
                        ? AppColors.neutralGray600
                        : AppColors.neutralGray400,
                    width: 2,
                    strokeAlign: BorderSide.strokeAlignInside,
                  ),
                  borderRadius: BorderRadius.circular(100),
                ),
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Icon(
                      Icons.face_retouching_natural,
                      size: 64,
                      color: isDark
                          ? AppColors.neutralGray500
                          : AppColors.neutralGray500,
                    ),
                    const SizedBox(height: 16),
                    Text(
                      'Take Selfie Photo',
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w600,
                        color: isDark
                            ? AppColors.neutralGray400
                            : AppColors.neutralGray700,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 16),
                      child: Text(
                        'Hold KTP beside face',
                        textAlign: TextAlign.center,
                        style: TextStyle(
                          fontSize: 12,
                          color: isDark
                              ? AppColors.neutralGray500
                              : AppColors.neutralGray600,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
            Positioned(
              bottom: 12,
              left: 0,
              right: 0,
              child: Center(
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
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildInstructions() {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
          width: 1,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                Icons.info_outline,
                size: 16,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
              ),
              const SizedBox(width: 6),
              Text(
                'Tips for a good selfie photo:',
                style: TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.w600,
                  color: isDark
                      ? AppColors.neutralGray300
                      : AppColors.neutralGray800,
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          ...[
            'Ensure face and KTP are clearly visible',
            'Use sufficient lighting',
            'KTP held beside face',
            'No filters or editing used',
          ].map(
            (tip) => Padding(
              padding: const EdgeInsets.only(left: 22, bottom: 4),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    '• ',
                    style: TextStyle(
                      color: isDark
                          ? AppColors.neutralGray400
                          : AppColors.neutralGray600,
                      fontSize: 12,
                    ),
                  ),
                  Expanded(
                    child: Text(
                      tip,
                      style: TextStyle(
                        fontSize: 11,
                        color: isDark
                            ? AppColors.neutralGray400
                            : AppColors.neutralGray600,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}
