/// District Dropdown Widget
///
/// Dropdown untuk memilih kecamatan berdasarkan kota
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/models/wilayah_models.dart';
import 'package:labuda/shared/providers/wilayah_provider_simple.dart';

class DistrictDropdown extends ConsumerWidget {
  final District? selectedDistrict;
  final City? selectedCity;
  final ValueChanged<District?> onChanged;
  final String? labelText;
  final String? hintText;
  final IconData? prefixIcon;
  final String? Function(District?)? validator;

  const DistrictDropdown({
    super.key,
    required this.selectedDistrict,
    required this.selectedCity,
    required this.onChanged,
    this.labelText,
    this.hintText,
    this.prefixIcon,
    this.validator,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final districtsAsync = ref.watch(districtsProvider(selectedCity?.id));

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (labelText != null) ...[
          Text(
            labelText!,
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w500,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray700,
            ),
          ),
          const SizedBox(height: 8),
        ],
        Container(
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(12),
            border: Border.all(
              color: isDark ? AppColors.darkGray600 : AppColors.neutralGray300,
            ),
            color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
          ),
          child: selectedCity == null
              ? _buildDisabledDropdown(context, isDark, 'Pilih kota dulu')
              : districtsAsync.when(
                  data: (districts) => districts.isEmpty
                      ? _buildEmptyDropdown(
                          context,
                          isDark,
                          'Tidak ada kecamatan tersedia',
                        )
                      : DropdownButtonFormField<District>(
                          initialValue: selectedDistrict,
                          onChanged: onChanged,
                          validator: validator,
                          isExpanded: true,
                          decoration: InputDecoration(
                            hintText: hintText ?? 'Pilih Kecamatan',
                            prefixIcon: prefixIcon != null
                                ? Icon(
                                    prefixIcon,
                                    color: isDark
                                        ? AppColors.neutralGray400
                                        : AppColors.neutralGray600,
                                  )
                                : null,
                            border: InputBorder.none,
                            contentPadding: const EdgeInsets.symmetric(
                              horizontal: 16,
                              vertical: 14,
                            ),
                            hintStyle: TextStyle(
                              color: isDark
                                  ? AppColors.neutralGray400
                                  : AppColors.neutralGray500,
                            ),
                          ),
                          dropdownColor: isDark
                              ? AppColors.darkGray700
                              : AppColors.neutralWhite,
                          style: TextStyle(
                            color: isDark
                                ? AppColors.neutralGray200
                                : AppColors.neutralGray900,
                            fontSize: 16,
                          ),
                          selectedItemBuilder: (context) {
                            return districts.map((district) {
                              return Text(
                                district.name,
                                overflow: TextOverflow.ellipsis,
                                maxLines: 1,
                              );
                            }).toList();
                          },
                          items: districts.map<DropdownMenuItem<District>>((
                            district,
                          ) {
                            return DropdownMenuItem<District>(
                              value: district,
                              child: Text(
                                district.name,
                                overflow: TextOverflow.ellipsis,
                                maxLines: 1,
                              ),
                            );
                          }).toList(),
                        ),
                  loading: () => _buildLoadingDropdown(
                    context,
                    isDark,
                    'Loading kecamatan...',
                  ),
                  error: (error, stack) => _buildErrorDropdown(
                    context,
                    isDark,
                    'Error loading kecamatan',
                  ),
                ),
        ),
      ],
    );
  }

  Widget _buildDisabledDropdown(
    BuildContext context,
    bool isDark,
    String text,
  ) {
    return Container(
      height: 50,
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: Row(
        children: [
          if (prefixIcon != null) ...[
            Icon(
              prefixIcon,
              color: isDark
                  ? AppColors.neutralGray500
                  : AppColors.neutralGray400,
            ),
            const SizedBox(width: 12),
          ],
          Text(
            text,
            style: TextStyle(
              color: isDark
                  ? AppColors.neutralGray500
                  : AppColors.neutralGray400,
              fontSize: 16,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildEmptyDropdown(BuildContext context, bool isDark, String text) {
    return Container(
      height: 50,
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: Row(
        children: [
          if (prefixIcon != null) ...[
            Icon(
              prefixIcon,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
            const SizedBox(width: 12),
          ],
          Text(
            text,
            style: TextStyle(
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray500,
              fontSize: 16,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildLoadingDropdown(BuildContext context, bool isDark, String text) {
    return Container(
      height: 50,
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: Row(
        children: [
          if (prefixIcon != null) ...[
            Icon(
              prefixIcon,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
            const SizedBox(width: 12),
          ],
          const SizedBox(
            width: 20,
            height: 20,
            child: CircularProgressIndicator(strokeWidth: 2),
          ),
          const SizedBox(width: 12),
          Text(
            text,
            style: TextStyle(
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray500,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildErrorDropdown(BuildContext context, bool isDark, String text) {
    return Container(
      height: 50,
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: Row(
        children: [
          if (prefixIcon != null) ...[
            Icon(prefixIcon, color: AppColors.statusError),
            const SizedBox(width: 12),
          ],
          Icon(Icons.error_outline, color: AppColors.statusError, size: 20),
          const SizedBox(width: 8),
          Text(text, style: TextStyle(color: AppColors.statusError)),
        ],
      ),
    );
  }
}
