import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/src/config/google_config.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/entities/post_location.dart';

/// Map picker field for address form
class AddressMapPickerField extends StatelessWidget {
  final double? latitude;
  final double? longitude;
  final String streetAddress;
  final Function(double?, double?) onCoordinatesChanged;
  final bool isDark;

  const AddressMapPickerField({
    super.key,
    this.latitude,
    this.longitude,
    required this.streetAddress,
    required this.onCoordinatesChanged,
    required this.isDark,
  });

  bool get hasCoordinates => latitude != null && longitude != null;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        InkWell(
          onTap: () => _showLocationPicker(context),
          borderRadius: BorderRadius.circular(12),
          child: Container(
            padding: const EdgeInsets.symmetric(vertical: 12, horizontal: 16),
            decoration: BoxDecoration(
              color: isDark ? AppColors.darkGray700 : AppColors.neutralGray50,
              borderRadius: BorderRadius.circular(12),
              border: Border.all(
                color: hasCoordinates
                    ? AppColors.success
                    : (isDark
                          ? AppColors.darkGray600
                          : AppColors.neutralGray300),
                width: hasCoordinates ? 2 : 1,
              ),
            ),
            child: Row(
              children: [
                Icon(
                  hasCoordinates ? Icons.check_circle : Icons.map_outlined,
                  size: 20,
                  color: hasCoordinates
                      ? AppColors.success
                      : (isDark
                            ? AppColors.neutralGray400
                            : AppColors.neutralGray600),
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
                          fontSize: 14,
                          fontWeight: FontWeight.w500,
                          color: hasCoordinates
                              ? AppColors.success
                              : (isDark
                                    ? AppColors.neutralGray200
                                    : AppColors.neutralGray900),
                        ),
                      ),
                      if (hasCoordinates)
                        Text(
                          '${latitude!.toStringAsFixed(6)}, ${longitude!.toStringAsFixed(6)}',
                          style: TextStyle(
                            fontSize: 11,
                            fontFamily: 'monospace',
                            color: isDark
                                ? AppColors.neutralGray400
                                : AppColors.neutralGray600,
                          ),
                        ),
                    ],
                  ),
                ),
                Icon(
                  Icons.chevron_right,
                  size: 20,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
              ],
            ),
          ),
        ),
        const SizedBox(height: 4),
        Text(
          'Pinpoint location to facilitate delivery',
          style: TextStyle(
            fontSize: 11,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray500,
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
