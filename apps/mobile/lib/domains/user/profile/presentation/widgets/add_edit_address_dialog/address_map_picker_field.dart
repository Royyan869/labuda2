import 'package:labuda/core/core.dart';
import 'package:flutter/material.dart';
import 'package:labuda/core/src/config/google_config.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/entities/post_location.dart';

/// Map picker field for address form
class AddressMapPickerField extends StatelessWidget {
  final double? latitude;
  final double? longitude;
  final String streetAddress;
  final Function(double?, double?) onCoordinatesChanged;

  const AddressMapPickerField({
    super.key,
    this.latitude,
    this.longitude,
    required this.streetAddress,
    required this.onCoordinatesChanged,
    
  });

  bool get hasCoordinates => latitude != null && longitude != null;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        InkWell(
          onTap: () => _showLocationPicker(context),
          borderRadius: BorderRadius.circular(AppShape.r12),
          child: Container(
            padding: const EdgeInsets.symmetric(vertical: AppMetrics.p12, horizontal: AppMetrics.p16),
            decoration: BoxDecoration(
              color: scheme.onSurfaceVariant,
              borderRadius: BorderRadius.circular(AppShape.r12),
              border: Border.all(
                color: hasCoordinates
                    ? context.statusColors.success
                    : scheme.onSurfaceVariant,
                width: hasCoordinates ? 2 : 1,
              ),
            ),
            child: Row(
              children: [
                Icon(
                  hasCoordinates ? Icons.check_circle : Icons.map_outlined,
                  size: 20,
                  color: hasCoordinates
                      ? context.statusColors.success
                      : scheme.onSurfaceVariant,
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        hasCoordinates
                            ? 'Pinpoint Location Saved'
                            : 'Select Location on Map',
                        style: TextStyle(
                          fontSize: AppType.s14,
                          fontWeight: FontWeight.w500,
                          color: hasCoordinates
                              ? context.statusColors.success
                              : scheme.onSurfaceVariant,
                        ),
                      ),
                      if (hasCoordinates)
                        Text(
                          '${latitude!.toStringAsFixed(6)}, ${longitude!.toStringAsFixed(6)}',
                          style: TextStyle(
                            fontSize: AppType.s11,
                            fontFamily: 'monospace',
                            color: scheme.onSurfaceVariant,
                          ),
                        ),
                    ],
                  ),
                ),
                Icon(
                  Icons.chevron_right,
                  size: 20,
                  color: scheme.onSurfaceVariant,
                ),
              ],
            ),
          ),
        ),
        const SizedBox(height: 4),
        Text(
          'Pinpoint location to facilitate delivery',
          style: TextStyle(
            fontSize: AppType.s11,
            color: scheme.onSurfaceVariant,
          ),
        ),
      ],
    );
  }

  Future<void> _showLocationPicker(BuildContext context) async {
    FocusManager.instance.primaryFocus?.unfocus();

    final location = await InteractiveMapPickerBottomSheet.show(
      context: context,
      initialLocation: hasCoordinates
          ? PostLocation(
              address: streetAddress,
              latitude: latitude,
              longitude: longitude,
            )
          : null,
      googleApiKey: GoogleConfig.apiKey,
    );

    if (location != null && location.hasCoordinates) {
      onCoordinatesChanged(location.latitude, location.longitude);
    }
  }
}
