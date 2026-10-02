/// City Dropdown Widget
///
/// Dropdown untuk memilih kota/kabupaten berdasarkan provinsi
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/models/wilayah_models.dart';
import 'package:labuda/shared/providers/wilayah_provider_simple.dart';
import 'package:labuda/shared/widgets/wilayah/dropdown_state_builders.dart';

class CityDropdown extends ConsumerWidget {
  final City? selectedCity;
  final Province? selectedProvince;
  final ValueChanged<City?> onChanged;
  final String? labelText;
  final String? hintText;
  final IconData? prefixIcon;
  final String? Function(City?)? validator;

  const CityDropdown({
    super.key,
    required this.selectedCity,
    required this.selectedProvince,
    required this.onChanged,
    this.labelText,
    this.hintText,
    this.prefixIcon,
    this.validator,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final scheme = Theme.of(context).colorScheme;
    final citiesAsync = ref.watch(citiesProvider(selectedProvince?.id));

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
          child: selectedProvince == null
              ? DropdownStateBuilders.buildDisabled(
                  context: context,
                  text: 'Pilih provinsi dulu',
                  prefixIcon: prefixIcon,
                )
              : citiesAsync.when(
                  data: (cities) => cities.isEmpty
                      ? DropdownStateBuilders.buildEmpty(
                          context: context,
                          text: 'Tidak ada kota tersedia',
                          prefixIcon: prefixIcon,
                        )
                      : DropdownButtonFormField<City>(
                          initialValue: selectedCity,
                          onChanged: onChanged,
                          validator: validator,
                          isExpanded: true,
                          decoration: InputDecoration(
                            hintText: hintText ?? 'Pilih Kota/Kabupaten',
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
                          style: TextStyle(
                            color: scheme.onSurface,
                            fontSize: AppType.s16,
                          ),
                          selectedItemBuilder: (context) {
                            return cities.map((city) {
                              return Text(
                                city.name,
                                overflow: TextOverflow.ellipsis,
                                maxLines: 1,
                              );
                            }).toList();
                          },
                          items: cities.map<DropdownMenuItem<City>>((city) {
                            return DropdownMenuItem<City>(
                              value: city,
                              child: Text(
                                city.name,
                                overflow: TextOverflow.ellipsis,
                                maxLines: 1,
                              ),
                            );
                          }).toList(),
                        ),
                  loading: () => DropdownStateBuilders.buildLoading(
                    context: context,
                    text: 'Loading kota...',
                    prefixIcon: prefixIcon,
                  ),
                  error: (error, stack) => DropdownStateBuilders.buildError(
                    context: context,
                    text: 'Error loading kota',
                    prefixIcon: prefixIcon,
                  ),
                ),
        ),
      ],
    );
  }
}
