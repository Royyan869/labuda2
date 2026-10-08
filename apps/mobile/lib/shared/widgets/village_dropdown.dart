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
          child: selectedDistrict == null
              ? DropdownStateBuilders.buildDisabled(
                  context: context,
                  text: 'Pilih kecamatan dulu',
                  prefixIcon: prefixIcon,
                )
              : villagesAsync.when(
                  data: (villages) => villages.isEmpty
                      ? DropdownStateBuilders.buildEmpty(
                          context: context,
                          text: 'Tidak ada desa tersedia',
                          prefixIcon: prefixIcon,
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
                  loading: () => DropdownStateBuilders.buildLoading(
                    context: context,
                    text: 'Loading desa...',
                    prefixIcon: prefixIcon,
                  ),
                  error: (error, stack) => DropdownStateBuilders.buildError(
                    context: context,
                    text: 'Error loading desa',
                    prefixIcon: prefixIcon,
                  ),
                ),
        ),
      ],
    );
  }
}
