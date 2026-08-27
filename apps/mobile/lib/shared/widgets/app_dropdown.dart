import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';

/// Reusable Dropdown Field dengan styling konsisten sesuai LABUDA design
///
/// Features:
/// - Consistent styling dengan AppTextField
/// - OutlineInputBorder untuk proper label floating
/// - Dark/light theme support
/// - Validation support
/// - Prefix icon support
/// - Custom item builder support
class AppDropdown<T> extends StatelessWidget {
  final String? labelText;
  final String? hintText;
  final IconData? prefixIcon;
  final T? value;
  final List<T> items;
  final String Function(T) itemDisplayText;
  final void Function(T?)? onChanged;
  final String? Function(T?)? validator;
  final bool enabled;

  const AppDropdown({
    super.key,
    this.labelText,
    this.hintText,
    this.prefixIcon,
    this.value,
    required this.items,
    required this.itemDisplayText,
    this.onChanged,
    this.validator,
    this.enabled = true,
  });

  /// Factory for Koi Variety dropdown
  static AppDropdown<String> koiVariety({
    Key? key,
    String? value,
    void Function(String?)? onChanged,
    String? Function(String?)? validator,
    bool enabled = true,
  }) {
    return AppDropdown<String>(
      key: key,
      labelText: 'Variety *',
      hintText: 'Pilih variety',
      prefixIcon: Icons.pets,
      value: value,
      items: KoiVarieties.all,
      itemDisplayText: (item) => item,
      onChanged: onChanged,
      validator: validator,
      enabled: enabled,
    );
  }

  /// Factory for Koi Gender dropdown
  static AppDropdown<KoiGender> koiGender({
    Key? key,
    KoiGender? value,
    void Function(KoiGender?)? onChanged,
    String? Function(KoiGender?)? validator,
    bool enabled = true,
  }) {
    return AppDropdown<KoiGender>(
      key: key,
      labelText: 'Gender *',
      hintText: 'Pilih gender',
      prefixIcon: Icons
          .wc, // Gender icon - toilet/restroom symbol (closest to gender symbols)
      value: value,
      items: KoiGender.all,
      itemDisplayText: (item) => item.displayName,
      onChanged: onChanged,
      validator: validator,
      enabled: enabled,
    );
  }

  /// Build label widget with red asterisk if needed
  Widget? _buildLabel(BuildContext context) {
    if (labelText == null) return null;

    final label = labelText!;

    // Check if label ends with " *"
    if (label.endsWith(' *')) {
      final textWithoutAsterisk = label.substring(0, label.length - 2);
      return RichText(
        text: TextSpan(
          text: textWithoutAsterisk,
          style: TextStyle(
            color: Theme.of(context).brightness == Brightness.dark
                ? AppColors.neutralGray300
                : AppColors.neutralGray700,
            fontSize: 16,
          ),
          children: const [
            TextSpan(
              text: ' *',
              style: TextStyle(color: AppColors.error),
            ),
          ],
        ),
      );
    }

    return null; // Use labelText parameter instead
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final customLabel = _buildLabel(context);

    return DropdownButtonFormField<T>(
      decoration: InputDecoration(
        label: customLabel,
        labelText: customLabel == null ? labelText : null,
        hintText: hintText,
        prefixIcon: prefixIcon != null
            ? Icon(
                prefixIcon,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray500,
              )
            : null,
        // Consistent OutlineInputBorder with AppTextField
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(
            color: isDark ? AppColors.darkGray600 : AppColors.neutralGray300,
          ),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide(
            color: isDark ? AppColors.darkGray600 : AppColors.neutralGray300,
          ),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: AppColors.primaryRed, width: 2),
        ),
        errorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: AppColors.error),
        ),
        focusedErrorBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: AppColors.error, width: 2),
        ),
      ),
      initialValue: value,
      validator: validator,
      items: items.map((item) {
        return DropdownMenuItem<T>(
          value: item,
          child: Text(
            itemDisplayText(item),
            style: TextStyle(color: Theme.of(context).colorScheme.onSurface),
          ),
        );
      }).toList(),
      onChanged: enabled ? onChanged : null,
      icon: Icon(
        Icons.arrow_drop_down,
        color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray500,
      ),
      style: TextStyle(color: Theme.of(context).colorScheme.onSurface),
      dropdownColor: Theme.of(context).colorScheme.surface,
    );
  }
}

/// Koi varieties data - centralized for reuse
class KoiVarieties {
  static const String DEFAULT_VARIETY = 'Other';

  static const List<String> _baseVarieties = [
    'Kohaku',
    'Sanke',
    'Showa',
    'Asagi',
    'Shusui',
    'Bekko',
    'Utsurimono',
    'Gosanke',
    'Tancho',
    'Kinginrin',
    'Doitsu',
    'Kawarimono',
    'Hikarimono',
    'Ogon',
    'Kujaku',
    'Hariwake',
    'Matsuba',
    'Goshiki',
    'Koromo',
    'Shiro Utsuri',
    'Hi/Ki Utsuri',
    'Midorigoi',
    'Ochiba',
    'Soragoi',
    'Chagoi',
    'Kigoi',
    'Benigoi',
    'Aka Muji',
    'Shiro Muji',
    'Kumonryu',
    'Butterfly',
  ];

  /// Default variety constant
  static const String defaultVariety = DEFAULT_VARIETY;

  /// Get sorted varieties with 'Other' at top
  static List<String> get all {
    final sorted = [..._baseVarieties]..sort();
    return [defaultVariety, ...sorted];
  }
}

/// Koi gender options
