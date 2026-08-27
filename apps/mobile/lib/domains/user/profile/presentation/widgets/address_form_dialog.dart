import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/src/config/google_config.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/entities/post_location.dart';
import 'package:labuda/shared/helpers/canonical_phone_validator.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show addressRepositoryProvider;
import 'package:labuda/domains/user/profile/presentation/providers/profile_core_provider.dart'
    show profileProvider;
import 'package:labuda/generated/app_localizations.dart';

/// Address Form Bottom Sheet - Modal for adding/editing address
class AddressFormDialog extends ConsumerStatefulWidget {
  final AddressEntity? addressToEdit;
  final AddressPurpose? initialPurpose; // Pre-select purpose when creating new

  /// If set, locks the purpose dropdown to this value (cannot be changed)
  /// Used when adding address from a specific tab (shipping/sender)
  final AddressPurpose? forcedPurpose;

  const AddressFormDialog({
    super.key,
    this.addressToEdit,
    this.initialPurpose,
    this.forcedPurpose,
  });

  @override
  ConsumerState<AddressFormDialog> createState() => _AddressFormDialogState();
}

class _AddressFormDialogState extends ConsumerState<AddressFormDialog> {
  final _formKey = GlobalKey<FormState>();

  // Controllers
  final _recipientNameController = TextEditingController();
  final _phoneController = TextEditingController();
  final _streetAddressController = TextEditingController();
  final _postalCodeController = TextEditingController();
  final _customNicknameController = TextEditingController();

  // Nickname dropdown options for shipping addresses
  static const List<String> _nicknameOptions = [
    'Home',
    'Shop',
    'Apartment',
    'Custom',
  ];
  String? _selectedNickname;
  bool get _isCustomNickname => _selectedNickname == 'Custom';

  // Wilayah selection
  Province? _selectedProvince;
  City? _selectedCity;
  District? _selectedDistrict;
  Village? _selectedVillage;

  // Address fields
  AddressPurpose _selectedPurpose = AddressPurpose.shipping;
  bool _isLoading = false;

  // Map coordinates
  double? _latitude;
  double? _longitude;

  // Flag to indicate if name field should be locked (for seller sender address)
  bool _isNameLocked = false;

  bool get hasCoordinates => _latitude != null && _longitude != null;

  @override
  void initState() {
    super.initState();
    if (widget.addressToEdit != null) {
      _loadAddressData(widget.addressToEdit!);
    } else {
      // New address - set purpose and autofill from profile
      if (widget.forcedPurpose != null) {
        _selectedPurpose = widget.forcedPurpose!;
      } else if (widget.initialPurpose != null) {
        _selectedPurpose = widget.initialPurpose!;
      }
      // Autofill after build (needs ref)
      WidgetsBinding.instance.addPostFrameCallback((_) {
        _autofillFromProfile();
      });
    }
  }

  /// Autofill name and phone from user profile
  Future<void> _autofillFromProfile() async {
    final currentUser = ref.read(authenticatedUserProvider);
    if (currentUser == null) return;

    final effectivePurpose = widget.forcedPurpose ?? _selectedPurpose;

    if (effectivePurpose == AddressPurpose.sender &&
        currentUser.hasCreatedSellerProfile) {
      // Seller sender address: use business name (locked)
      final profileAsync = await ref.read(
        profileProvider(currentUser.id).future,
      );
      final farmName = profileAsync?.farmInfo?.farmName;

      if (farmName != null && farmName.isNotEmpty && mounted) {
        setState(() {
          _recipientNameController.text = farmName;
          _isNameLocked = true; // Lock name for seller
        });
      }
    } else {
      // Buyer shipping address: use personal name (editable)
      if (mounted) {
        setState(() {
          _recipientNameController.text = currentUser.username;
          _isNameLocked = false;
        });
      }
    }

    // Autofill phone for both
    if (currentUser.phoneNumber != null &&
        currentUser.phoneNumber!.isNotEmpty &&
        mounted) {
      setState(() {
        _phoneController.text = currentUser.phoneNumber!;
      });
    }
  }

  @override
  void dispose() {
    _recipientNameController.dispose();
    _phoneController.dispose();
    _streetAddressController.dispose();
    _postalCodeController.dispose();
    _customNicknameController.dispose();
    super.dispose();
  }

  void _loadAddressData(AddressEntity address) {
    setState(() {
      _recipientNameController.text = address.recipientName;
      _phoneController.text = address.phone;
      _selectedProvince = address.province;
      _selectedCity = address.city;
      _selectedDistrict = address.district;
      _selectedVillage = address.village;
      _streetAddressController.text = address.streetAddress;
      _postalCodeController.text = address.postalCode;
      _selectedPurpose = address.purpose;
      // Map coordinates
      _latitude = address.latitude;
      _longitude = address.longitude;

      // Load nickname for shipping addresses
      if (address.purpose == AddressPurpose.shipping &&
          address.nickname != null) {
        if (_nicknameOptions.contains(address.nickname)) {
          _selectedNickname = address.nickname;
        } else {
          // Custom nickname
          _selectedNickname = 'Custom';
          _customNicknameController.text = address.nickname!;
        }
      }
    });
  }

  List<AddressPurpose> _getAvailablePurposes() {
    final currentUser = ref.watch(authenticatedUserProvider);

    // If user is seller, show all purposes
    if (currentUser?.hasCreatedSellerProfile ?? false) {
      return AddressPurpose.values;
    }

    // If user is buyer, only shipping purpose
    return [AddressPurpose.shipping];
  }

  /// Auto-fill postal code ketika village dipilih
  Future<void> _autoFillPostalCode() async {
    if (_selectedProvince == null ||
        _selectedCity == null ||
        _selectedDistrict == null ||
        _selectedVillage == null) {
      return;
    }

    // Normalize IDs (remove dots for postal code lookup)
    final normalizedProvinceId = _selectedProvince!.id.replaceAll('.', '');
    final normalizedCityId = _selectedCity!.id.replaceAll('.', '');
    final normalizedDistrictId = _selectedDistrict!.id.replaceAll('.', '');
    final normalizedVillageId = _selectedVillage!.id.replaceAll('.', '');

    final postalCode = await PostalCodeService.getPostalCodeByWilayah(
      provinceId: normalizedProvinceId,
      cityId: normalizedCityId,
      districtId: normalizedDistrictId,
      villageId: normalizedVillageId,
    );

    if (postalCode != null && mounted) {
      setState(() {
        _postalCodeController.text = postalCode;
      });
    }
  }

  Future<void> _handleSave() async {
    if (!_formKey.currentState!.validate()) {
      AppSnackBar.showError(context, 'Please fix the errors in the form');
      return;
    }

    if (_selectedProvince == null ||
        _selectedCity == null ||
        _selectedDistrict == null ||
        _selectedVillage == null) {
      AppSnackBar.showError(context, 'Please complete all address fields');
      return;
    }

    setState(() => _isLoading = true);

    try {
      final currentUser = ref.read(authenticatedUserProvider);
      if (currentUser == null) {
        throw Exception('User not authenticated');
      }

      final repository = ref.read(addressRepositoryProvider);
      final now = DateTime.now();

      // Determine nickname based on purpose
      String? nickname;
      if (_selectedPurpose == AddressPurpose.shipping) {
        // Only shipping addresses have nickname
        if (_selectedNickname != null) {
          if (_isCustomNickname) {
            nickname = _customNicknameController.text.trim().isEmpty
                ? null
                : _customNicknameController.text.trim();
          } else {
            nickname = _selectedNickname;
          }
        }
      }
      // Sender addresses don't have nickname (null)

      final address = AddressEntity(
        id: widget.addressToEdit?.id ?? '',
        userId: currentUser.id,
        purpose: _selectedPurpose,
        nickname: nickname,
        recipientName: _recipientNameController.text.trim(),
        phone: _phoneController.text.trim(),
        province: _selectedProvince!,
        city: _selectedCity!,
        district: _selectedDistrict!,
        village: _selectedVillage!,
        streetAddress: _streetAddressController.text.trim(),
        postalCode: _postalCodeController.text.trim(),
        notes: widget.addressToEdit?.notes, // Preserve existing notes
        isPrimary:
            widget.addressToEdit?.isPrimary ??
            false, // Preserve existing isPrimary
        latitude: _latitude,
        longitude: _longitude,
        createdAt: widget.addressToEdit?.createdAt ?? now,
        updatedAt: now,
      );

      Result<void> result;

      if (widget.addressToEdit != null) {
        // Update existing
        result = await repository.updateAddress(address);
      } else {
        // Add new
        result = await repository.addAddress(address);
      }

      if (!mounted) return;

      setState(() => _isLoading = false);

      if (result.isSuccess) {
        AppSnackBar.showSuccess(
          context,
          widget.addressToEdit != null
              ? 'Address updated successfully'
              : 'Address added successfully',
        );
        Navigator.of(context).pop(true); // Return true to indicate success
      } else {
        throw Exception(result.error);
      }
    } catch (e) {
      if (mounted) {
        setState(() => _isLoading = false);
        AppSnackBar.showError(context, 'Gagal menyimpan alamat. Coba lagi.');
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final availablePurposes = _getAvailablePurposes();

    return DraggableScrollableSheet(
      initialChildSize: 0.9,
      minChildSize: 0.5,
      maxChildSize: 0.95,
      expand: false,
      builder: (context, scrollController) => Container(
        decoration: BoxDecoration(
          color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
          borderRadius: const BorderRadius.only(
            topLeft: Radius.circular(20),
            topRight: Radius.circular(20),
          ),
        ),
        child: Column(
          children: [
            // Header
            Container(
              padding: const EdgeInsets.all(20),
              decoration: BoxDecoration(
                color: isDark ? AppColors.darkGray700 : AppColors.neutralGray50,
                borderRadius: const BorderRadius.only(
                  topLeft: Radius.circular(20),
                  topRight: Radius.circular(20),
                ),
              ),
              child: Row(
                children: [
                  Icon(
                    widget.addressToEdit != null
                        ? Icons.edit
                        : Icons.add_location,
                    color: AppColors.primaryRed,
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Text(
                      widget.addressToEdit != null
                          ? 'Edit Address'
                          : 'Add New Address',
                      style: TextStyle(
                        fontSize: 18,
                        fontWeight: FontWeight.bold,
                        color: isDark
                            ? AppColors.neutralWhite
                            : AppColors.neutralGray900,
                      ),
                    ),
                  ),
                  IconButton(
                    onPressed: () => Navigator.of(context).pop(),
                    icon: const Icon(Icons.close),
                    tooltip: 'Close',
                  ),
                ],
              ),
            ),

            // Form Content
            Expanded(
              child: Form(
                key: _formKey,
                child: ListView(
                  controller: scrollController,
                  padding: EdgeInsets.fromLTRB(
                    20,
                    20,
                    20,
                    20 + MediaQuery.of(context).viewInsets.bottom,
                  ),
                  children: [
                    // Purpose - show locked indicator if forcedPurpose is set
                    if (widget.forcedPurpose != null)
                      _buildLockedPurposeIndicator(isDark)
                    else
                      DropdownButtonFormField<AddressPurpose>(
                        initialValue:
                            availablePurposes.contains(_selectedPurpose)
                            ? _selectedPurpose
                            : availablePurposes.first,
                        items: availablePurposes.map((purpose) {
                          return DropdownMenuItem<AddressPurpose>(
                            value: purpose,
                            child: Text(purpose.label),
                          );
                        }).toList(),
                        onChanged: (value) {
                          if (value != null) {
                            setState(() => _selectedPurpose = value);
                          }
                        },
                        decoration: InputDecoration(
                          labelText: 'Purpose *',
                          hintText: 'Select address purpose',
                          prefixIcon: Icon(_getPurposeIcon(_selectedPurpose)),
                          border: OutlineInputBorder(
                            borderRadius: BorderRadius.circular(12),
                          ),
                        ),
                        validator: (value) {
                          if (value == null) {
                            return 'Please select a purpose';
                          }
                          return null;
                        },
                      ),
                    const SizedBox(height: 16),

                    // Nickname field (only for shipping addresses)
                    if (_selectedPurpose == AddressPurpose.shipping) ...[
                      _buildNicknameDropdown(isDark),
                      const SizedBox(height: 16),
                    ],

                    // Recipient/Sender Name
                    if (_isNameLocked)
                      // Locked display for seller business name
                      _buildLockedNameField(isDark)
                    else
                      AppTextField(
                        controller: _recipientNameController,
                        labelText: _selectedPurpose == AddressPurpose.sender
                            ? 'Sender Name *'
                            : 'Recipient Name *',
                        hintText: _selectedPurpose == AddressPurpose.sender
                            ? 'Store/farm name'
                            : 'Full name of recipient',
                        prefixIcon: Icons.person_outline,
                        validator: (value) {
                          if (value == null || value.trim().isEmpty) {
                            return _selectedPurpose == AddressPurpose.sender
                                ? 'Sender name is required'
                                : 'Recipient name is required';
                          }
                          return null;
                        },
                      ),
                    const SizedBox(height: 16),

                    // Phone Number
                    AppTextField(
                      controller: _phoneController,
                      labelText: 'Phone Number *',
                      hintText: '08xxxxxxxxxx',
                      prefixIcon: Icons.phone_outlined,
                      keyboardType: TextInputType.phone,
                      validator: (value) =>
                          CanonicalPhoneValidator.validationMessage(value),
                    ),
                    const SizedBox(height: 16),

                    // Province
                    ProvinceDropdown(
                      selectedProvince: _selectedProvince,
                      onChanged: (province) {
                        setState(() {
                          _selectedProvince = province;
                          _selectedCity = null;
                          _selectedDistrict = null;
                          _selectedVillage = null;
                        });
                      },
                      labelText: 'Province *',
                      hintText: 'Select Province',
                      prefixIcon: Icons.map_outlined,
                      validator: (value) =>
                          value == null ? 'Province is required' : null,
                    ),
                    const SizedBox(height: 16),

                    // City
                    CityDropdown(
                      selectedCity: _selectedCity,
                      selectedProvince: _selectedProvince,
                      onChanged: (city) {
                        setState(() {
                          _selectedCity = city;
                          _selectedDistrict = null;
                          _selectedVillage = null;
                        });
                      },
                      labelText: 'City/Regency *',
                      hintText: 'Select City/Regency',
                      prefixIcon: Icons.location_city_outlined,
                      validator: (value) =>
                          value == null ? 'City/Regency is required' : null,
                    ),
                    const SizedBox(height: 16),

                    // District
                    DistrictDropdown(
                      selectedDistrict: _selectedDistrict,
                      selectedCity: _selectedCity,
                      onChanged: (district) {
                        setState(() {
                          _selectedDistrict = district;
                          _selectedVillage = null;
                        });
                      },
                      labelText: 'District *',
                      hintText: 'Select District',
                      prefixIcon: Icons.location_on_outlined,
                      validator: (value) =>
                          value == null ? 'District is required' : null,
                    ),
                    const SizedBox(height: 16),

                    // Village
                    VillageDropdown(
                      selectedVillage: _selectedVillage,
                      selectedDistrict: _selectedDistrict,
                      onChanged: (village) {
                        setState(() => _selectedVillage = village);
                        // Auto-fill postal code after village is selected
                        _autoFillPostalCode();
                      },
                      labelText: 'Village/Subdistrict *',
                      hintText: 'Select Village/Subdistrict',
                      prefixIcon: Icons.home_work_outlined,
                      validator: (value) => value == null
                          ? 'Village/Subdistrict is required'
                          : null,
                    ),
                    const SizedBox(height: 16),

                    // Street Address
                    AppTextField(
                      controller: _streetAddressController,
                      labelText: 'Full Address *',
                      hintText: 'Street name, house number, RT/RW, etc.',
                      prefixIcon: Icons.home_outlined,
                      maxLines: 3,
                      validator: (value) {
                        if (value == null || value.trim().isEmpty) {
                          return 'Full address is required';
                        }
                        return null;
                      },
                    ),
                    const SizedBox(height: 12),

                    // Map Picker Button
                    _buildMapPickerButton(isDark),
                    const SizedBox(height: 16),

                    // Postal Code
                    AppTextField(
                      controller: _postalCodeController,
                      labelText: AppLocalizations.of(context)!.postalCode,
                      hintText: AppLocalizations.of(context)!.enterPostalCode,
                      prefixIcon: Icons.local_post_office_outlined,
                      keyboardType: TextInputType.number,
                      validator: (value) {
                        if (value == null || value.trim().isEmpty) {
                          return AppLocalizations.of(
                            context,
                          )!.postalCodeRequired;
                        }
                        if (value.length != 5) {
                          return 'Postal code must be 5 digits';
                        }
                        return null;
                      },
                    ),
                  ],
                ),
              ),
            ),

            // Footer Actions dengan SafeArea
            SafeArea(
              top: false,
              child: Container(
                padding: const EdgeInsets.all(20),
                decoration: BoxDecoration(
                  color: isDark
                      ? AppColors.darkGray700
                      : AppColors.neutralGray50,
                ),
                child: SizedBox(
                  width: double.infinity,
                  child: ElevatedButton(
                    onPressed: _isLoading ? null : _handleSave,
                    style: ElevatedButton.styleFrom(
                      backgroundColor: AppColors.primaryRed,
                      foregroundColor: Colors.white,
                      padding: const EdgeInsets.symmetric(vertical: 16),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(12),
                      ),
                    ),
                    child: _isLoading
                        ? const SizedBox(
                            height: 20,
                            width: 20,
                            child: CircularProgressIndicator(
                              strokeWidth: 2,
                              valueColor: AlwaysStoppedAnimation<Color>(
                                Colors.white,
                              ),
                            ),
                          )
                        : Text(
                            widget.addressToEdit != null
                                ? 'Update Address'
                                : 'Save Address',
                            style: const TextStyle(
                              fontSize: 16,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  IconData _getPurposeIcon(AddressPurpose purpose) {
    switch (purpose) {
      case AddressPurpose.shipping:
        return Icons.home;
      case AddressPurpose.sender:
        return Icons.agriculture;
    }
  }

  /// Build locked purpose indicator when forcedPurpose is set
  Widget _buildLockedPurposeIndicator(bool isDark) {
    final purpose = widget.forcedPurpose!;
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
                      ? 'Shipping destination address'
                      : 'Shipping origin address',
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

  /// Build nickname dropdown for shipping addresses
  Widget _buildNicknameDropdown(bool isDark) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        DropdownButtonFormField<String>(
          initialValue: _selectedNickname,
          decoration: InputDecoration(
            labelText: 'Address Label (Optional)',
            hintText: 'Select label',
            prefixIcon: const Icon(Icons.label_outline),
            border: OutlineInputBorder(borderRadius: BorderRadius.circular(12)),
          ),
          items: _nicknameOptions.map((option) {
            return DropdownMenuItem<String>(value: option, child: Text(option));
          }).toList(),
          onChanged: (value) {
            setState(() {
              _selectedNickname = value;
              if (value != 'Custom') {
                _customNicknameController.clear();
              }
            });
          },
        ),
        // Show custom input field when "Custom" is selected
        if (_isCustomNickname) ...[
          const SizedBox(height: 12),
          AppTextField(
            controller: _customNicknameController,
            labelText: 'Custom Label',
            hintText: 'Example: Villa, Boarding House, Warehouse',
            prefixIcon: Icons.edit_outlined,
          ),
        ],
      ],
    );
  }

  /// Build locked name field for seller - prominent display (not faded like hint)
  Widget _buildLockedNameField(bool isDark) {
    return Container(
      padding: const EdgeInsets.symmetric(vertical: 14, horizontal: 16),
      decoration: BoxDecoration(
        color: isDark
            ? AppColors.primaryRed.withValues(alpha: 0.1)
            : AppColors.primaryRed.withValues(alpha: 0.05),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isDark
              ? AppColors.primaryRed.withValues(alpha: 0.3)
              : AppColors.primaryRed.withValues(alpha: 0.2),
        ),
      ),
      child: Row(
        children: [
          Container(
            padding: const EdgeInsets.all(8),
            decoration: BoxDecoration(
              color: isDark
                  ? AppColors.primaryRed.withValues(alpha: 0.2)
                  : AppColors.primaryRed.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(8),
            ),
            child: Icon(
              Icons.storefront,
              size: 20,
              color: AppColors.primaryRed,
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Sender Name',
                  style: TextStyle(
                    fontSize: 11,
                    fontWeight: FontWeight.w500,
                    color: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray600,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  _recipientNameController.text.isNotEmpty
                      ? _recipientNameController.text
                      : 'Store Name',
                  style: TextStyle(
                    fontSize: 15,
                    fontWeight: FontWeight.w600,
                    color: isDark
                        ? AppColors.neutralWhite
                        : AppColors.neutralGray900,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
            decoration: BoxDecoration(
              color: isDark ? AppColors.darkGray600 : AppColors.neutralGray200,
              borderRadius: BorderRadius.circular(6),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(
                  Icons.lock_outline,
                  size: 12,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
                const SizedBox(width: 4),
                Text(
                  'From Profile',
                  style: TextStyle(
                    fontSize: 10,
                    fontWeight: FontWeight.w500,
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

  /// Build map picker button with coordinate indicator
  Widget _buildMapPickerButton(bool isDark) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        InkWell(
          onTap: _showLocationPicker,
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
                          '${_latitude!.toStringAsFixed(6)}, ${_longitude!.toStringAsFixed(6)}',
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
  Future<void> _showLocationPicker() async {
    // Dismiss keyboard before showing map picker
    FocusManager.instance.primaryFocus?.unfocus();

    final location = await InteractiveMapPickerBottomSheet.show(
      context: context,
      initialLocation: hasCoordinates
          ? PostLocation(
              address: _streetAddressController.text,
              latitude: _latitude,
              longitude: _longitude,
            )
          : null,
      googleApiKey: GoogleConfig.apiKey,
    );

    if (location != null && location.hasCoordinates) {
      setState(() {
        _latitude = location.latitude;
        _longitude = location.longitude;
      });
    }
  }
}
