import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/src/config/google_config.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/entities/post_location.dart';
import 'package:labuda/shared/helpers/canonical_phone_validator.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';

/// Form fields for address dialog
class AddressFormFields extends StatelessWidget {
  final AddressPurpose? selectedPurpose;
  final TextEditingController nicknameController;
  final TextEditingController recipientNameController;
  final TextEditingController phoneController;
  final Province? selectedProvince;
  final City? selectedCity;
  final District? selectedDistrict;
  final Village? selectedVillage;
  final TextEditingController streetAddressController;
  final TextEditingController postalCodeController;
  final TextEditingController notesController;
  final ValueChanged<AddressPurpose?> onPurposeChanged;
  final ValueChanged<Province?> onProvinceChanged;
  final ValueChanged<City?> onCityChanged;
  final ValueChanged<District?> onDistrictChanged;
  final ValueChanged<Village?> onVillageChanged;

  // Map location fields
  final double? latitude;
  final double? longitude;
  final Function(double?, double?)? onCoordinatesChanged;

  /// If set, purpose dropdown will be hidden and this value will be used
  /// This locks the form to a specific purpose (e.g., from tab selection)
  final AddressPurpose? forcedPurpose;

  const AddressFormFields({
    super.key,
    required this.selectedPurpose,
    required this.nicknameController,
    required this.recipientNameController,
    required this.phoneController,
    required this.selectedProvince,
    required this.selectedCity,
    required this.selectedDistrict,
    required this.selectedVillage,
    required this.streetAddressController,
    required this.postalCodeController,
    required this.notesController,
    required this.onPurposeChanged,
    required this.onProvinceChanged,
    required this.onCityChanged,
    required this.onDistrictChanged,
    required this.onVillageChanged,
    this.latitude,
    this.longitude,
    this.onCoordinatesChanged,
    this.forcedPurpose,
  });

  bool get hasCoordinates => latitude != null && longitude != null;

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Purpose selection - show locked indicator if forcedPurpose is set
        if (forcedPurpose != null)
          _buildLockedPurposeIndicator(isDark)
        else ...[
          _buildLabel('Address Purpose', isDark),
          const SizedBox(height: 8),
          _buildPurposeDropdown(isDark),
        ],
        const SizedBox(height: 16),

        // Nickname (optional)
        AppTextField(
          controller: nicknameController,
          labelText: 'Nickname (Optional)',
          hintText: 'E.g., "Rumah Utama", "Kantor", "Farm"',
          prefixIcon: Icons.bookmark_outline,
        ),
        const SizedBox(height: 16),

        // Recipient Name
        AppTextField(
          controller: recipientNameController,
          labelText: 'Recipient Name *',
          hintText: 'e.g., John Doe',
          prefixIcon: Icons.person_outline,
          validator: (value) {
            if (value == null || value.trim().isEmpty) {
              return 'Recipient name is required';
            }
            return null;
          },
        ),
        const SizedBox(height: 16),

        // Phone Number
        AppTextField(
          controller: phoneController,
          labelText: 'Phone Number *',
          hintText: 'e.g., 081234567890',
          prefixIcon: Icons.phone_outlined,
          keyboardType: TextInputType.phone,
          validator: (value) =>
              CanonicalPhoneValidator.validationMessage(value),
        ),
        const SizedBox(height: 16),

        // Province
        ProvinceDropdown(
          selectedProvince: selectedProvince,
          onChanged: onProvinceChanged,
          validator: (value) {
            if (value == null) return 'Province is required';
            return null;
          },
        ),
        const SizedBox(height: 16),

        // City
        if (selectedProvince != null) ...[
          CityDropdown(
            selectedProvince: selectedProvince,
            selectedCity: selectedCity,
            onChanged: onCityChanged,
            validator: (value) {
              if (value == null) return 'City is required';
              return null;
            },
          ),
          const SizedBox(height: 16),
        ],

        // District
        if (selectedCity != null) ...[
          DistrictDropdown(
            selectedCity: selectedCity,
            selectedDistrict: selectedDistrict,
            onChanged: onDistrictChanged,
            validator: (value) {
              if (value == null) return 'District is required';
              return null;
            },
          ),
          const SizedBox(height: 16),
        ],

        // Village
        if (selectedDistrict != null) ...[
          VillageDropdown(
            selectedDistrict: selectedDistrict,
            selectedVillage: selectedVillage,
            onChanged: onVillageChanged,
            validator: (value) {
              if (value == null) return 'Village is required';
              return null;
            },
          ),
          const SizedBox(height: 16),
        ],

        // Street Address (multiline - no icon per standard)
        AppTextField(
          controller: streetAddressController,
          labelText: 'Street Address *',
          hintText: 'Enter street address',
          maxLines: 2,
          validator: (value) {
            if (value == null || value.isEmpty) {
              return 'Street address is required';
            }
            if (value.length < 5) {
              return 'Address must be at least 5 characters';
            }
            return null;
          },
        ),
        const SizedBox(height: 12),

        // Map Picker Button
        if (onCoordinatesChanged != null)
          _buildMapPickerSection(context, isDark),

        const SizedBox(height: 16),

        // Postal Code
        AppTextField(
          controller: postalCodeController,
          labelText: 'Postal Code *',
          hintText: 'Enter postal code',
          prefixIcon: Icons.markunread_mailbox_outlined,
          keyboardType: TextInputType.number,
          inputFormatters: [
            FilteringTextInputFormatter.digitsOnly,
            LengthLimitingTextInputFormatter(5),
          ],
          validator: (value) {
            if (value == null || value.isEmpty) {
              return 'Postal code is required';
            }
            if (value.length != 5) {
              return 'Postal code must be 5 digits';
            }
            return null;
          },
        ),
        const SizedBox(height: 16),

        // Notes (optional, multiline - no icon per standard)
        AppTextField(
          controller: notesController,
          labelText: 'Delivery Notes (Optional)',
          hintText: 'e.g., House with red gate, near minimarket',
          maxLines: 2,
        ),
        const SizedBox(height: 8),
        Text(
          'Add notes to help delivery find your location',
          style: TextStyle(
            fontSize: 12,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
          ),
        ),
      ],
    );
  }

  Widget _buildLabel(String text, bool isDark) {
    return Text(
      text,
      style: TextStyle(
        fontSize: 14,
        fontWeight: FontWeight.w600,
        color: isDark ? AppColors.neutralGray200 : AppColors.neutralGray900,
      ),
    );
  }

  InputDecoration _inputDecoration(bool isDark, String hintText) {
    return InputDecoration(
      hintText: hintText,
      filled: true,
      fillColor: isDark ? AppColors.darkGray700 : AppColors.neutralGray50,
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(12),
        borderSide: BorderSide(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
        ),
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(12),
        borderSide: BorderSide(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
        ),
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(12),
        borderSide: BorderSide(color: AppColors.primaryRed, width: 2),
      ),
      errorBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(12),
        borderSide: BorderSide(color: AppColors.error),
      ),
    );
  }

  Widget _buildPurposeDropdown(bool isDark) {
    return DropdownButtonFormField<AddressPurpose>(
      initialValue: selectedPurpose,
      decoration: _inputDecoration(isDark, 'Select address purpose'),
      dropdownColor: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
      items: AddressPurpose.values.map((purpose) {
        return DropdownMenuItem(
          value: purpose,
          child: Text(
            purpose.label,
            style: TextStyle(
              color: isDark
                  ? AppColors.neutralGray200
                  : AppColors.neutralGray900,
            ),
          ),
        );
      }).toList(),
      onChanged: onPurposeChanged,
      validator: (value) {
        if (value == null) {
          return 'Please select a purpose';
        }
        return null;
      },
    );
  }

  /// Build locked purpose indicator when forcedPurpose is set
  Widget _buildLockedPurposeIndicator(bool isDark) {
    final purpose = forcedPurpose!;
    final isShipping = purpose == AddressPurpose.shipping;

    return Container(
      padding: const EdgeInsets.symmetric(vertical: 12, horizontal: 16),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralGray50,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
        ),
      ),
      child: Row(
        children: [
          Icon(
            isShipping
                ? Icons.local_shipping_outlined
                : Icons.storefront_outlined,
            size: 20,
            color: AppColors.primaryRed,
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  isShipping
                      ? 'Recipient Address (Buyer)'
                      : 'Sender Address (Seller)',
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: isDark
                        ? AppColors.neutralGray200
                        : AppColors.neutralGray900,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  isShipping
                      ? 'Destination address for shipping'
                      : 'Origin address for shipping',
                  style: TextStyle(
                    fontSize: 12,
                    color: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray600,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  /// Build map picker section with button and coordinate indicator
  Widget _buildMapPickerSection(BuildContext context, bool isDark) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Map Picker Button
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

  /// Show interactive map picker bottom sheet
  Future<void> _showLocationPicker(BuildContext context) async {
    // Dismiss keyboard before showing map picker
    FocusManager.instance.primaryFocus?.unfocus();

    final location = await InteractiveMapPickerBottomSheet.show(
      context: context,
      initialLocation: hasCoordinates
          ? PostLocation(
              address: streetAddressController.text,
              latitude: latitude,
              longitude: longitude,
            )
          : null,
      googleApiKey: GoogleConfig.apiKey,
    );

    if (location != null && location.hasCoordinates) {
      onCoordinatesChanged?.call(location.latitude, location.longitude);
    }
  }
}
