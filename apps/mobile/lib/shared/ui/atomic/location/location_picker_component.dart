import 'package:labuda/core/src/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:labuda/shared/ui/base/base_component.dart';

/// Atomic component untuk location picker dengan GPS dan manual input
/// Single responsibility: Handle location selection
/// MAKSIMAL 100 LINES - ENFORCED BY GUIDELINES
class LocationPickerComponent extends BaseComponent
    implements
        ValidatableComponent,
        DataComponent<String>,
        ResettableComponent {
  final String? initialLocation;
  final String label;
  final String hint;
  final bool enableGPS;
  final bool enableManualInput;
  final void Function(String?)? onLocationChanged;
  final String? Function(String?)? validator;

  const LocationPickerComponent({
    super.key,
    this.initialLocation,
    required this.label,
    required this.hint,
    this.enableGPS = true,
    this.enableManualInput = true,
    this.onLocationChanged,
    this.validator,
    super.componentId,
    super.isRequired,
    super.errorMessage,
    super.isLoading,
    super.isDisabled,
  });

  @override
  Widget buildContent(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (enableManualInput) _buildManualInput(context),
        if (enableGPS && enableManualInput) const SizedBox(height: 8),
        if (enableGPS) _buildGPSButton(context),
        if (initialLocation != null) ...[
          const SizedBox(height: 8),
          _buildCurrentLocation(context),
        ],
      ],
    );
  }

  Widget _buildManualInput(BuildContext context) {
    return TextFormField(
      initialValue: initialLocation,
      onChanged: onLocationChanged,
      validator: validator ?? _defaultValidator,
      enabled: !isDisabled,
      decoration: InputDecoration(
        labelText: isRequired ? '$label *' : label,
        hintText: hint,
        border: const OutlineInputBorder(),
        prefixIcon: const Icon(Icons.location_on_outlined),
        suffixIcon: isRequired
            ? const Icon(Icons.star, size: 12, color: AppColors.error)
            : null,
      ),
    );
  }

  Widget _buildGPSButton(BuildContext context) {
    return SizedBox(
      width: double.infinity,
      child: OutlinedButton.icon(
        onPressed: isDisabled ? null : () => _getCurrentLocation(),
        icon: const Icon(Icons.my_location),
        label: const Text('Use Current Location'),
      ),
    );
  }

  Widget _buildCurrentLocation(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: AppColors.neutralGray50,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: AppColors.neutralGray200),
      ),
      child: Row(
        children: [
          const Icon(Icons.location_on, color: AppColors.primary),
          const SizedBox(width: 8),
          Expanded(
            child: Text(initialLocation!, style: const TextStyle(fontSize: 14)),
          ),
          IconButton(
            onPressed: () => _clearLocation(),
            icon: const Icon(Icons.close, size: 18),
          ),
        ],
      ),
    );
  }

  void _getCurrentLocation() async {
    // TODO: Implement GPS location fetching
    // This would integrate dengan geolocator package

    // Simulasi GPS result
    const mockLocation = 'Jakarta, Indonesia';
    onLocationChanged?.call(mockLocation);
  }

  void _clearLocation() {
    onLocationChanged?.call(null);
  }

  @override
  String? validate() {
    return validator?.call(getData()) ?? _defaultValidator(getData());
  }

  @override
  String? getData() {
    return initialLocation;
  }

  @override
  void reset() {
    onLocationChanged?.call(null);
  }

  String? _defaultValidator(String? value) {
    if (isRequired && (value?.trim().isEmpty ?? true)) {
      return 'Location is required';
    }
    return null;
  }
}
