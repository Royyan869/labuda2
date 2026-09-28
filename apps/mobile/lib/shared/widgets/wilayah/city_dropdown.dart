/// City Dropdown Widget
///
/// Dropdown untuk memilih kota/kabupaten berdasarkan provinsi
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/models/wilayah_models.dart';
import 'package:labuda/shared/providers/wilayah_provider_simple.dart';

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
              ? _buildDisabledDropdown(context, 'Pilih provinsi dulu')
              : citiesAsync.when(
                  data: (cities) => cities.isEmpty
                      ? _buildEmptyDropdown(
                          context,
                          'Tidak ada kota tersedia',
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
                  loading: () =>
                      _buildLoadingDropdown(context, 'Loading kota...'),
                  error: (error, stack) => _buildErrorDropdown(
                    context,
                    'Error loading kota',
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
