import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Widget for displaying existing KTP in preview mode.
class KTPPreviewSection extends StatelessWidget {
  final String? ktpImageUrl;
  final TextEditingController ktpNumberController;
  final TextEditingController ktpNameController;
  final VoidCallback onChangeKTP;
  final bool isDark;

  const KTPPreviewSection({
    super.key,
    required this.ktpImageUrl,
    required this.ktpNumberController,
    required this.ktpNameController,
    required this.onChangeKTP,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          '✅ KTP Already Uploaded',
          style: TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w600,
            color: AppColors.successGreen,
          ),
        ),
        const SizedBox(height: 16),
        Container(
          height: 200,
          decoration: BoxDecoration(
            color: isDark ? AppColors.neutralGray800 : AppColors.neutralGray100,
            borderRadius: BorderRadius.circular(8),
            border: Border.all(
              color: isDark
                  ? AppColors.neutralGray700
                  : AppColors.neutralGray300,
            ),
          ),
          child: ktpImageUrl != null
              ? Image.network(ktpImageUrl!, fit: BoxFit.contain)
              : const Center(child: Icon(Icons.image, size: 64)),
        ),
        const SizedBox(height: 16),
        TextFormField(
          controller: ktpNumberController,
          decoration: const InputDecoration(labelText: 'KTP Number'),
          readOnly: true,
          enabled: false,
        ),
        const SizedBox(height: 16),
        TextFormField(
          controller: ktpNameController,
          decoration: const InputDecoration(labelText: 'Name on KTP'),
          readOnly: true,
          enabled: false,
        ),
        const SizedBox(height: 16),
        OutlinedButton.icon(
          onPressed: onChangeKTP,
          icon: const Icon(Icons.edit),
          label: const Text('Change KTP'),
        ),
      ],
    );
  }
}
