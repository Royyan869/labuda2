/// District Dropdown Widget
///
/// Dropdown untuk memilih kecamatan berdasarkan kota
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/models/wilayah_models.dart';
import 'package:labuda/shared/providers/wilayah_provider_simple.dart';
import 'package:labuda/shared/widgets/wilayah/dropdown_state_builders.dart';

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
            style: context.typeRoles.bodyDense.copyWith(
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
              ? DropdownStateBuilders.buildDisabled(
                  context: context,
                  text: 'Pilih kota dulu',
                  prefixIcon: prefixIcon,
                )
              : districtsAsync.when(
                  data: (districts) => districts.isEmpty
                      ? DropdownStateBuilders.buildEmpty(
                          context: context,
                          text: 'Tidak ada kecamatan tersedia',
                          prefixIcon: prefixIcon,
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
                              vertical: AppMetrics.p16,
                            ),
                            hintStyle: TextStyle(
                              color: scheme.onSurfaceVariant,
                            ),
                          ),
                          dropdownColor: scheme.surfaceContainerHigh,
                          style: Theme.of(context).textTheme.bodyLarge
                              ?.copyWith(color: scheme.onSurface),
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
                  loading: () => DropdownStateBuilders.buildLoading(
                    context: context,
                    text: 'Loading kecamatan...',
                    prefixIcon: prefixIcon,
                  ),
                  error: (error, stack) => DropdownStateBuilders.buildError(
                    context: context,
                    text: 'Error loading kecamatan',
                    prefixIcon: prefixIcon,
                  ),
                ),
        ),
      ],
    );
  }
}
