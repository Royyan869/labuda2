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
    final scheme = Theme.of(context).colorScheme;
    final districtsAsync = ref.watch(districtsProvider(selectedCity?.id));

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (labelText != null) ...[
          Text(
            labelText!,
            style: TextStyle(
              fontSize: AppType.s14,
              fontWeight: FontWeight.w500,
              color: scheme.onSurface,
            ),
          ),
          const SizedBox(height: 8),
        ],
        Container(
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(AppShape.r12),
            border: Border.all(color: scheme.outlineVariant),
            color: scheme.surface,
          ),
          child: selectedCity == null
              ? _buildDisabledDropdown(context, 'Pilih kota dulu')
              : districtsAsync.when(
                  data: (districts) => districts.isEmpty
                      ? _buildEmptyDropdown(
                          context,
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
                                    color: scheme.onSurfaceVariant,
                                  )
                                : null,
                            border: InputBorder.none,
                            contentPadding: const EdgeInsets.symmetric(
                              horizontal: AppMetrics.p16,
                              vertical: AppMetrics.p14,
                            ),
                            hintStyle: TextStyle(
                              color: scheme.onSurfaceVariant,
                            ),
                          ),
                          dropdownColor: scheme.surfaceContainerHigh,
                          style: TextStyle(
                            color: scheme.onSurface,
                            fontSize: AppType.s16,
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
                    'Loading kecamatan...',
                  ),
                  error: (error, stack) => _buildErrorDropdown(
                    context,
                    'Error loading kecamatan',
                  ),
                ),
        ),
      ],
    );
  }

  Widget _buildDisabledDropdown(BuildContext context, String text) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      height: 50,
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16),
      child: Row(
        children: [
          if (prefixIcon != null) ...[
            Icon(prefixIcon, color: scheme.onSurfaceVariant),
            const SizedBox(width: 12),
          ],
          Text(
            text,
            style: TextStyle(
              color: scheme.onSurfaceVariant,
              fontSize: AppType.s16,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildEmptyDropdown(BuildContext context, String text) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      height: 50,
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16),
      child: Row(
        children: [
          if (prefixIcon != null) ...[
            Icon(prefixIcon, color: scheme.onSurfaceVariant),
            const SizedBox(width: 12),
          ],
          Text(
            text,
            style: TextStyle(
              color: scheme.onSurfaceVariant,
              fontSize: AppType.s16,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildLoadingDropdown(BuildContext context, String text) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      height: 50,
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16),
      child: Row(
        children: [
          if (prefixIcon != null) ...[
            Icon(prefixIcon, color: scheme.onSurfaceVariant),
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
            style: TextStyle(color: scheme.onSurfaceVariant),
          ),
        ],
      ),
    );
  }

  Widget _buildErrorDropdown(BuildContext context, String text) {
    return Container(
      height: 50,
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16),
      child: Row(
        children: [
          if (prefixIcon != null) ...[
            Icon(prefixIcon, color: context.statusColors.error),
            const SizedBox(width: 12),
          ],
          Icon(Icons.error_outline, color: context.statusColors.error, size: 20),
          const SizedBox(width: 8),
          Text(text, style: TextStyle(color: context.statusColors.error)),
        ],
      ),
    );
  }
}
