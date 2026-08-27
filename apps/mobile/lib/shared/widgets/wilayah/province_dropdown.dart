/// Province Dropdown Widget
///
/// Dropdown untuk memilih provinsi
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/shared/shared.dart'; // TODO: update;

class ProvinceDropdown extends ConsumerWidget {
  final Province? selectedProvince;
  final ValueChanged<Province?> onChanged;
  final String? labelText;
  final String? hintText;
  final IconData? prefixIcon;
  final String? Function(Province?)? validator;

  const ProvinceDropdown({
    super.key,
    required this.selectedProvince,
    required this.onChanged,
    this.labelText,
    this.hintText,
    this.prefixIcon,
    this.validator,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final provincesAsync = ref.watch(provincesProvider);

    return BaseDropdownContainer(
      labelText: labelText,
      isDark: isDark,
      child: provincesAsync.when(
        data: (provinces) => _buildDropdown(context, isDark, provinces),
        loading: () => DropdownStateBuilders.buildLoading(
          context: context,
          isDark: isDark,
          text: 'Loading provinsi...',
          prefixIcon: prefixIcon,
        ),
        error: (error, stack) => DropdownStateBuilders.buildError(
          context: context,
          isDark: isDark,
          text: 'Error loading provinsi',
          prefixIcon: prefixIcon,
        ),
      ),
    );
  }

  Widget _buildDropdown(
    BuildContext context,
    bool isDark,
    List<Province> provinces,
  ) {
    return DropdownButtonFormField<Province>(
      initialValue: selectedProvince,
      onChanged: onChanged,
      validator: validator,
      isExpanded: true,
      decoration: DropdownDecorationHelper.createInputDecoration(
        isDark: isDark,
        hintText: hintText ?? 'Pilih Provinsi',
        prefixIcon: prefixIcon,
      ),
      dropdownColor: DropdownDecorationHelper.getDropdownColor(isDark),
      style: DropdownDecorationHelper.getTextStyle(isDark),
      selectedItemBuilder: (context) {
        return DropdownDecorationHelper.buildSelectedItems<Province>(
          provinces,
          (province) => province.name,
        );
      },
      items: DropdownDecorationHelper.buildDropdownItems<Province>(
        provinces,
        (province) => province.name,
      ),
    );
  }
}
