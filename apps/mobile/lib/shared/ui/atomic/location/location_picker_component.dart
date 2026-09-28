import 'package:labuda/core/src/theme/app_theme.dart';
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
            ? Icon(Icons.star, size: 12, color: context.statusColors.error)
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
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerLow,
        borderRadius: BorderRadius.circular(AppShape.r8),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Row(
        children: [
          Icon(Icons.location_on, color: scheme.primary),
          const SizedBox(width: 8),
          Expanded(
            child: Text(initialLocation!, style: const TextStyle(fontSize: AppType.s14)),
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
