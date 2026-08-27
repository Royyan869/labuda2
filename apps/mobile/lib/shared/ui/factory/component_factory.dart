import 'package:labuda/core/src/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:labuda/shared/ui/base/base_component.dart';
import 'package:labuda/shared/ui/atomic/input/title_input_component.dart';
import 'package:labuda/shared/ui/atomic/input/description_input_component.dart';
import 'package:labuda/shared/ui/atomic/input/price_input_component.dart';
import 'package:labuda/shared/ui/atomic/media/media_upload_component.dart';
import 'package:labuda/shared/ui/atomic/location/location_picker_component.dart';
import 'package:labuda/shared/ui/atomic/tagging/user_tagging_component.dart';
import 'package:labuda/shared/ui/atomic/settings/visibility_settings_component.dart';

/// Factory untuk create components dengan consistent API
/// Mencegah custom implementations dan enforce standards
class ComponentFactory {
  // Private constructor - force use of static methods
  ComponentFactory._();

  /// Configuration untuk component factory
  static ComponentFactoryConfig? _config;

  /// Initialize factory dengan configuration
  static void initialize(ComponentFactoryConfig config) {
    _config = config;
  }

  /// Get current config atau default
  static ComponentFactoryConfig get config =>
      _config ?? ComponentFactoryConfig.defaultConfig();

  // === INPUT COMPONENTS ===

  /// Standard title input field
  static Widget titleInput({
    String? initialValue,
    String? label,
    String? hint,
    String? errorMessage,
    int maxLength = 100,
    bool isRequired = true,
    bool isDisabled = false,
    void Function(String)? onChanged,
    String? Function(String?)? validator,
  }) {
    return TitleInputComponent(
      initialValue: initialValue,
      label: label ?? config.defaultLabels['title'] ?? 'Title',
      hint: hint ?? config.defaultHints['title'] ?? 'Enter title...',
      errorMessage: errorMessage,
      maxLength: maxLength,
      isRequired: isRequired,
      isDisabled: isDisabled,
      onChanged: onChanged,
      validator: validator,
    );
  }

  /// Rich text description input
  static Widget descriptionInput({
    String? initialValue,
    String? label,
    String? hint,
    String? errorMessage,
    int maxLines = 3,
    int maxLength = 500,
    bool enableRichText = false,
    bool isRequired = false,
    bool isDisabled = false,
    void Function(String)? onChanged,
    String? Function(String?)? validator,
  }) {
    return DescriptionInputComponent(
      initialValue: initialValue,
      label: label ?? config.defaultLabels['description'] ?? 'Description',
      hint:
          hint ?? config.defaultHints['description'] ?? 'Enter description...',
      errorMessage: errorMessage,
      maxLines: maxLines,
      maxLength: maxLength,
      enableRichText: enableRichText,
      isRequired: isRequired,
      isDisabled: isDisabled,
      onChanged: onChanged,
      validator: validator,
    );
  }

  /// Price input dengan currency formatting
  static Widget priceInput({
    double? initialValue,
    String? label,
    String? hint,
    String? errorMessage,
    double? minPrice,
    double? maxPrice,
    String currency = 'IDR',
    bool isRequired = true,
    bool isDisabled = false,
    void Function(double?)? onChanged,
    String? Function(double?)? validator,
  }) {
    return PriceInputComponent(
      initialValue: initialValue,
      label: label ?? config.defaultLabels['price'] ?? 'Price',
      hint: hint ?? config.defaultHints['price'] ?? 'Enter price...',
      errorMessage: errorMessage,
      minPrice: minPrice,
      maxPrice: maxPrice,
      currency: currency,
      isRequired: isRequired,
      isDisabled: isDisabled,
      onChanged: onChanged,
      validator: validator,
    );
  }

  // === MEDIA COMPONENTS ===

  /// Media upload component dengan preview
  static Widget mediaUpload({
    List<String>? initialMediaUrls,
    String? errorMessage,
    int maxFiles = 5,
    List<String> allowedTypes = const ['image', 'video'],
    double maxFileSizeMB = 10.0,
    bool showPreview = true,
    bool allowReorder = true,
    bool isRequired = false,
    bool isDisabled = false,
    void Function(List<String>)? onMediaChanged,
    String? Function(List<String>)? validator,
  }) {
    return MediaUploadComponent(
      initialMediaUrls: initialMediaUrls,
      errorMessage: errorMessage,
      maxFiles: maxFiles,
      allowedTypes: allowedTypes,
      maxFileSizeMB: maxFileSizeMB,
      showPreview: showPreview,
      allowReorder: allowReorder,
      isRequired: isRequired,
      isDisabled: isDisabled,
      onMediaChanged: onMediaChanged,
      validator: validator,
    );
  }

  // === LOCATION COMPONENTS ===

  /// Location picker dengan GPS dan manual input
  static Widget locationPicker({
    String? initialLocation,
    String? label,
    String? hint,
    String? errorMessage,
    bool enableGPS = true,
    bool enableManualInput = true,
    bool isRequired = false,
    bool isDisabled = false,
    void Function(String?)? onLocationChanged,
    String? Function(String?)? validator,
  }) {
    return LocationPickerComponent(
      initialLocation: initialLocation,
      label: label ?? config.defaultLabels['location'] ?? 'Location',
      hint: hint ?? config.defaultHints['location'] ?? 'Select location...',
      errorMessage: errorMessage,
      enableGPS: enableGPS,
      enableManualInput: enableManualInput,
      isRequired: isRequired,
      isDisabled: isDisabled,
      onLocationChanged: onLocationChanged,
      validator: validator,
    );
  }

  // === TAGGING COMPONENTS ===

  /// User tagging dengan autocomplete
  static Widget userTagging({
    List<String>? initialTaggedUsers,
    String? label,
    String? hint,
    String? errorMessage,
    int maxTags = 10,
    bool isRequired = false,
    bool isDisabled = false,
    void Function(List<String>)? onTagsChanged,
    String? Function(List<String>)? validator,
  }) {
    return UserTaggingComponent(
      initialTaggedUsers: initialTaggedUsers,
      label: label ?? config.defaultLabels['tagging'] ?? 'Tag People',
      hint: hint ?? config.defaultHints['tagging'] ?? 'Tag people...',
      errorMessage: errorMessage,
      maxTags: maxTags,
      isRequired: isRequired,
      isDisabled: isDisabled,
      onTagsChanged: onTagsChanged,
      validator: validator,
    );
  }

  // === SETTINGS COMPONENTS ===

  /// Visibility settings (public/private/friends)
  static Widget visibilitySettings({
    String? initialVisibility,
    String? label,
    String? errorMessage,
    List<String> availableOptions = const ['Public', 'Private', 'Friends'],
    bool showDescription = true,
    bool isRequired = false,
    bool isDisabled = false,
    void Function(String)? onVisibilityChanged,
    String? Function(String?)? validator,
  }) {
    return VisibilitySettingsComponent(
      initialVisibility: initialVisibility ?? availableOptions.first,
      label: label ?? config.defaultLabels['visibility'] ?? 'Visibility',
      errorMessage: errorMessage,
      availableOptions: availableOptions,
      showDescription: showDescription,
      isRequired: isRequired,
      isDisabled: isDisabled,
      onVisibilityChanged: onVisibilityChanged,
      validator: validator,
    );
  }

  // === CUSTOM COMPONENT WRAPPER ===

  /// Wrapper untuk custom components dengan standard behavior
  static Widget custom({
    required Widget child,
    String? componentId,
    String? errorMessage,
    bool isRequired = false,
    bool isLoading = false,
    bool isDisabled = false,
  }) {
    return ComponentWrapper(
      componentId: componentId,
      errorMessage: errorMessage,
      isRequired: isRequired,
      isLoading: isLoading,
      isDisabled: isDisabled,
      child: child,
    );
  }

  // === UTILITY METHODS ===

  /// Create spacer dengan standard spacing
  static Widget spacing(ComponentSpacing spacing) {
    double height;
    switch (spacing) {
      case ComponentSpacing.xxs:
        height = ComponentSpacingValues.XXS;
        break;
      case ComponentSpacing.xs:
        height = ComponentSpacingValues.XS;
        break;
      case ComponentSpacing.sm:
        height = ComponentSpacingValues.SM;
        break;
      case ComponentSpacing.md:
        height = ComponentSpacingValues.MD;
        break;
      case ComponentSpacing.lg:
        height = ComponentSpacingValues.LG;
        break;
      case ComponentSpacing.xl:
        height = ComponentSpacingValues.XL;
        break;
      case ComponentSpacing.xxl:
        height = ComponentSpacingValues.XXL;
        break;
    }
    return SizedBox(height: height);
  }

  /// Create section header
  static Widget sectionHeader({
    required String title,
    String? subtitle,
    Widget? trailing,
    bool isRequired = false,
  }) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8.0),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Text(
                      title,
                      style: const TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    if (isRequired) ...[
                      const SizedBox(width: 4),
                      const Text(
                        '*',
                        style: TextStyle(color: AppColors.error, fontSize: 16),
                      ),
                    ],
                  ],
                ),
                if (subtitle != null) ...[
                  const SizedBox(height: 2),
                  Text(
                    subtitle,
                    style: TextStyle(
                      fontSize: 12,
                      color: AppColors.neutralGray600,
                    ),
                  ),
                ],
              ],
            ),
          ),
          ?trailing,
        ],
      ),
    );
  }
}

/// Configuration untuk component factory
class ComponentFactoryConfig {
  /// Default labels untuk components
  final Map<String, String> defaultLabels;

  /// Default hints untuk components
  final Map<String, String> defaultHints;

  /// Default validation messages
  final Map<String, String> defaultValidationMessages;

  /// Theme configuration
  final ComponentThemeConfig theme;

  const ComponentFactoryConfig({
    required this.defaultLabels,
    required this.defaultHints,
    required this.defaultValidationMessages,
    required this.theme,
  });

  /// Default configuration
  factory ComponentFactoryConfig.defaultConfig() {
    return ComponentFactoryConfig(
      defaultLabels: {
        'title': 'Title',
        'description': 'Description',
        'price': 'Price',
        'location': 'Location',
        'tagging': 'Tag People',
        'visibility': 'Visibility',
      },
      defaultHints: {
        'title': 'Enter title...',
        'description': 'Enter description...',
        'price': 'Enter price...',
        'location': 'Select location...',
        'tagging': 'Tag people...',
        'visibility': 'Select visibility...',
      },
      defaultValidationMessages: {
        'required': 'This field is required',
        'minLength': 'Minimum {min} characters required',
        'maxLength': 'Maximum {max} characters allowed',
        'invalidFormat': 'Invalid format',
      },
      theme: ComponentThemeConfig.defaultTheme(),
    );
  }

  /// Indonesian configuration
  factory ComponentFactoryConfig.indonesian() {
    return ComponentFactoryConfig(
      defaultLabels: {
        'title': 'Judul',
        'description': 'Deskripsi',
        'price': 'Harga',
        'location': 'Lokasi',
        'tagging': 'Tag Orang',
        'visibility': 'Visibilitas',
      },
      defaultHints: {
        'title': 'Masukkan judul...',
        'description': 'Masukkan deskripsi...',
        'price': 'Masukkan harga...',
        'location': 'Pilih lokasi...',
        'tagging': 'Tag orang...',
        'visibility': 'Pilih visibilitas...',
      },
      defaultValidationMessages: {
        'required': 'Field ini wajib diisi',
        'minLength': 'Minimal {min} karakter diperlukan',
        'maxLength': 'Maksimal {max} karakter diizinkan',
        'invalidFormat': 'Format tidak valid',
      },
      theme: ComponentThemeConfig.defaultTheme(),
    );
  }
}

/// Theme configuration untuk components
class ComponentThemeConfig {
  final Color primaryColor;
  final Color errorColor;
  final Color disabledColor;
  final Color backgroundColor;
  final double borderRadius;
  final double spacing;

  const ComponentThemeConfig({
    required this.primaryColor,
    required this.errorColor,
    required this.disabledColor,
    required this.backgroundColor,
    required this.borderRadius,
    required this.spacing,
  });

  factory ComponentThemeConfig.defaultTheme() {
    return const ComponentThemeConfig(
      primaryColor: AppColors.primary,
      errorColor: AppColors.error,
      disabledColor: AppColors.neutral,
      backgroundColor: AppColors.light,
      borderRadius: 8.0,
      spacing: 16.0,
    );
  }
}
