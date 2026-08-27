import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

class VillageDropdown extends ConsumerWidget {
  final Village? selectedVillage;
  final District? selectedDistrict;
  final ValueChanged<Village?> onChanged;
  final String? labelText;
  final String? hintText;
  final IconData? prefixIcon;
  final String? Function(Village?)? validator;

  const VillageDropdown({
    super.key,
    required this.selectedVillage,
    required this.selectedDistrict,
    required this.onChanged,
    this.labelText,
    this.hintText,
    this.prefixIcon,
    this.validator,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final villagesAsync = ref.watch(villagesProvider(selectedDistrict?.id));

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
          child: selectedDistrict == null
              ? _buildDisabledDropdown(context, isDark, 'Pilih kecamatan dulu')
              : villagesAsync.when(
                  data: (villages) => villages.isEmpty
                      ? _buildEmptyDropdown(
                          context,
                          isDark,
                          'Tidak ada desa tersedia',
                        )
                      : DropdownButtonFormField<Village>(
                          initialValue: selectedVillage,
                          onChanged: onChanged,
                          validator: validator,
                          isExpanded: true,
                          decoration: InputDecoration(
                            hintText: hintText ?? 'Pilih Desa/Kelurahan',
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
                            return villages.map((village) {
                              return Text(
                                village.name,
                                overflow: TextOverflow.ellipsis,
                                maxLines: 1,
                              );
                            }).toList();
                          },
                          items: villages.map((village) {
                            return DropdownMenuItem<Village>(
                              value: village,
                              child: Text(
                                village.name,
                                overflow: TextOverflow.ellipsis,
                                maxLines: 1,
                              ),
                            );
                          }).toList(),
                        ),
                  loading: () =>
                      _buildLoadingDropdown(context, isDark, 'Loading desa...'),
                  error: (error, stack) => _buildErrorDropdown(
                    context,
                    isDark,
                    'Error loading desa',
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
