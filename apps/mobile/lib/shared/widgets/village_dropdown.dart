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
    final scheme = Theme.of(context).colorScheme;
    final villagesAsync = ref.watch(villagesProvider(selectedDistrict?.id));

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
          child: selectedDistrict == null
              ? _buildDisabledDropdown(context, 'Pilih kecamatan dulu')
              : villagesAsync.when(
                  data: (villages) => villages.isEmpty
                      ? _buildEmptyDropdown(
                          context,
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
                      _buildLoadingDropdown(context, 'Loading desa...'),
                  error: (error, stack) => _buildErrorDropdown(
                    context,
                    'Error loading desa',
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
