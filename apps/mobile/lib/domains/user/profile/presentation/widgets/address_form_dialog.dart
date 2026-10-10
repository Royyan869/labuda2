import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/core/src/config/google_config.dart';
import 'package:hishumi/shared/shared.dart';
import 'package:hishumi/shared/entities/post_location.dart';
import 'package:hishumi/shared/helpers/canonical_phone_validator.dart';
import 'package:hishumi/domains/user/profile/domain/entities/address_entity.dart';
import 'package:hishumi/domains/user/profile/presentation/providers/notifiers/address_notifier.dart';
import 'package:hishumi/domains/user/profile/presentation/providers/profile_core_provider.dart'
    show profileProvider;
import 'package:hishumi/generated/app_localizations.dart';

/// Address Form Bottom Sheet - Modal for adding/editing address
///
/// This is the ONE address form in the app (Settings list, checkout CTA and
/// the seller wizard all open it). The old per-tab copy with its own rules
/// (`AddEditAddressDialog`) was the competing authority and is gone.
class AddressFormDialog extends ConsumerStatefulWidget {
  final AddressEntity? addressToEdit;

  const AddressFormDialog({super.key, this.addressToEdit});

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

  // Label dropdown options — recognition only, never a business role.
  static const List<String> _nicknameOptions = [
    'Rumah',
    'Kantor',
    'Apartemen',
    'Gudang',
    'Kost',
    'Lainnya',
  ];
  String? _selectedNickname;
  bool get _isCustomNickname => _selectedNickname == 'Lainnya';

  // Wilayah selection
  Province? _selectedProvince;
  City? _selectedCity;
  District? _selectedDistrict;
  Village? _selectedVillage;

  // Address fields
  bool _isLoading = false;

  // Map coordinates
  double? _latitude;
  double? _longitude;

  // Flag to indicate if name field should be locked (seller business name)
  bool _isNameLocked = false;

  bool get hasCoordinates => _latitude != null && _longitude != null;

  @override
  void initState() {
    super.initState();
    if (widget.addressToEdit != null) {
      _loadAddressData(widget.addressToEdit!);
    } else {
      // Autofill after build (needs ref)
      WidgetsBinding.instance.addPostFrameCallback((_) {
        _autofillFromProfile();
      });
    }
  }

  /// Autofill name and phone from user profile.
  ///
  /// A seller's business name is used for the recipient (locked) so the farm
  /// name is the origin identity; every other account uses its personal name.
  Future<void> _autofillFromProfile() async {
    final currentUser = ref.read(authenticatedUserProvider);
    if (currentUser == null) return;

    if (currentUser.hasCreatedSellerProfile) {
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
      // Map coordinates
      _latitude = address.latitude;
      _longitude = address.longitude;

      // Load label
      if (address.nickname != null) {
        if (_nicknameOptions.contains(address.nickname)) {
          _selectedNickname = address.nickname;
        } else {
          // Custom label
          _selectedNickname = 'Lainnya';
          _customNicknameController.text = address.nickname!;
        }
      }
    });
  }

  /// Auto-fill postal code when a village is selected. The postal code is an
  /// attribute of the canonical village, so no separate lookup authority is
  /// involved.
  Future<void> _autoFillPostalCode() async {
    final village = _selectedVillage;
    if (village == null) {
      return;
    }
    final postalCode = village.postalCode;
    if (postalCode != null && postalCode.isNotEmpty && mounted) {
      setState(() {
        _postalCodeController.text = postalCode;
      });
    }
  }

  Future<void> _handleSave() async {
    if (!_formKey.currentState!.validate()) {
      AppSnackBar.showError(context, 'Perbaiki kesalahan pada formulir');
      return;
    }

    if (_selectedProvince == null ||
        _selectedCity == null ||
        _selectedDistrict == null ||
        _selectedVillage == null) {
      AppSnackBar.showError(context, 'Lengkapi semua kolom alamat');
      return;
    }

    setState(() => _isLoading = true);

    try {
      final currentUser = ref.read(authenticatedUserProvider);
      if (currentUser == null) {
        throw Exception('User not authenticated');
      }

      final now = DateTime.now();

      // Label is free recognition text for every address — never a role.
      String? nickname;
      if (_selectedNickname != null) {
        if (_isCustomNickname) {
          nickname = _customNicknameController.text.trim().isEmpty
              ? null
              : _customNicknameController.text.trim();
        } else {
          nickname = _selectedNickname;
        }
      }

      final address = AddressEntity(
        id: widget.addressToEdit?.id ?? '',
        userId: currentUser.id,
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

      // Persist through the canonical Address Book authority (shared with
      // Address List and checkout), which reloads its own collection.
      final notifier = ref.read(addressProvider.notifier);
      final saved = widget.addressToEdit != null
          ? await notifier.updateAddress(address)
          : await notifier.addAddress(address);

      if (!mounted) return;

      setState(() => _isLoading = false);

      if (saved) {
        AppSnackBar.showSuccess(
          context,
          widget.addressToEdit != null
              ? 'Address updated successfully'
              : 'Address added successfully',
        );
        Navigator.of(context).pop(true); // Return true to indicate success
      } else {
        AppSnackBar.showError(context, 'Gagal menyimpan alamat. Coba lagi.');
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
    final scheme = Theme.of(context).colorScheme;

    // Standard NON-DRAGGABLE modal sheet (owner decision B): the surface and
    // top-r20 shape come from `bottomSheetTheme`; the form content scrolls
    // internally. No `DraggableScrollableSheet`, no local surface/radius.
    //
    // The body bound is the sheet's LIVE content allocation
    // ([AppBottomSheetBase.contentAllocationOf]): the ceiling and everything
    // spent around the content region (handle, content wrap, system spacer)
    // belong to the base alone. Re-spelling the sheet's `0.9` ceiling here
    // instead gave the form the WHOLE body height while its region is
    // `handle + wrap + spacer` smaller — the footer/bar rode 72+N px past
    // the allocation and the CTA parked 36 px below the content clip
    // (geometry-proven: BOTTOMSHEET-02-FIT-GAP). The form owns NO ceiling:
    // it fills exactly what the sheet allocates to it.
    return ConstrainedBox(
      constraints: BoxConstraints(
        maxHeight: AppBottomSheetBase.contentAllocationOf(context),
      ),
      child: Column(
        children: [
          // Drag handle is owned once by `AppBottomSheetBase` (hosts use the
          // default drag behavior) — no local handle here.
          // Header
          Container(
            padding: const EdgeInsets.all(AppMetrics.p24),
            child: Row(
              children: [
                Icon(
                  widget.addressToEdit != null
                      ? Icons.edit
                      : Icons.add_location,
                  color: scheme.primary,
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(
                    widget.addressToEdit != null
                        ? 'Edit Address'
                        : 'Add New Address',
                    style: context.typeRoles.titleSection.copyWith(
                      fontWeight: FontWeight.bold,
                      // Primary ink on a surface — was `onSurfaceVariant`
                      // ink on an `onSurfaceVariant` surface (invisible).
                      color: scheme.onSurface,
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
                // Tail = design spacing only (BOTTOMSHEET-02). The ONE
                // keyboard lift lives in `AppBottomSheetBase` — the whole
                // sheet already sits above the keyboard — so a `+
                // viewInsets.bottom` here would be a SECOND reservation:
                // it grows the tail by the keyboard height and parks the
                // last field out of reach (geometry-proven: 324 px tail at
                // keyboard 300 vs the canonical p24).
                padding: EdgeInsets.all(AppMetrics.p24),
                children: [
                  // Label (recognition only)
                  _buildNicknameDropdown(scheme),
                  const SizedBox(height: 16),

                  // Recipient Name
                  if (_isNameLocked)
                    // Locked display for seller business name
                    _buildLockedNameField(scheme)
                  else
                    AppTextField(
                      controller: _recipientNameController,
                      labelText: 'Recipient Name *',
                      hintText: 'Full name of recipient',
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
                  _buildMapPickerButton(scheme),
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
                        return AppLocalizations.of(context)!.postalCodeRequired;
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

          // Footer actions ride the canonical bar presentation (embedded in
          // dialog content, not a viewport bar — the foundation still owns
          // the chrome). The sheet itself is already lifted above the
          // keyboard by `AppBottomSheetBase`, so the embedded mode suppresses
          // the bar's own keyboard rise (no second lift) and its system-bottom
          // reservation (the base spacer owns that inset — BOTTOMSHEET-03).
          BottomActionBar(
            embeddedInLiftedSheet: true,
            primary: BottomBarAction(
              label: widget.addressToEdit != null
                  ? 'Update Address'
                  : 'Save Address',
              onPressed: _isLoading ? null : _handleSave,
              isLoading: _isLoading,
            ),
          ),
        ],
      ),
    );
  }

  /// Build the recognition-label dropdown. The label is optional and carries
  /// no business meaning.
  Widget _buildNicknameDropdown(ColorScheme scheme) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        DropdownButtonFormField<String>(
          initialValue: _selectedNickname,
          // Border/fill/geometry come from `inputDecorationTheme`
          // (AppTheme) — the one form-field authority.
          decoration: const InputDecoration(
            labelText: 'Address Label (Optional)',
            hintText: 'Select label',
            prefixIcon: Icon(Icons.label_outline),
          ),
          items: _nicknameOptions.map((option) {
            return DropdownMenuItem<String>(value: option, child: Text(option));
          }).toList(),
          onChanged: (value) {
            setState(() {
              _selectedNickname = value;
              if (value != 'Lainnya') {
                _customNicknameController.clear();
              }
            });
          },
        ),
        // Show custom input field when "Lainnya" is selected
        if (_isCustomNickname) ...[
          const SizedBox(height: 12),
          AppTextField(
            controller: _customNicknameController,
            labelText: 'Custom Label',
            hintText: 'Example: Rumah Orang Tua, Rumah Baru',
            prefixIcon: Icons.edit_outlined,
          ),
        ],
      ],
    );
  }

  /// Build locked name field for seller - prominent display (not faded like hint)
  Widget _buildLockedNameField(ColorScheme scheme) {
    return Container(
      padding: const EdgeInsets.symmetric(
        vertical: AppMetrics.p16,
        horizontal: AppMetrics.p16,
      ),
      decoration: BoxDecoration(
        // Container roles: neutral subtle fill + outline border (was ink at
        // 5%/20% alpha, which dark-mode flips to a light grey blob).
        color: scheme.surfaceContainerHigh,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Row(
        children: [
          Container(
            padding: const EdgeInsets.all(AppMetrics.p8),
            decoration: BoxDecoration(
              color: scheme.surfaceContainerHighest,
              borderRadius: BorderRadius.circular(AppShape.r8),
            ),
            child: Icon(
              Icons.storefront,
              size: AppIconSize.action,
              color: scheme.primary,
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Recipient Name',
                  style: context.typeRoles.labelMicro.copyWith(
                    fontWeight: FontWeight.w500,
                    color: scheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  _recipientNameController.text.isNotEmpty
                      ? _recipientNameController.text
                      : 'Store Name',
                  style: context.typeRoles.titleCompact.copyWith(
                    fontWeight: FontWeight.w600,
                    color: scheme.onSurface,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          Container(
            padding: const EdgeInsets.symmetric(
              horizontal: AppMetrics.p8,
              vertical: AppMetrics.p4,
            ),
            // M3 chip pair: secondaryContainer/onSecondaryContainer — the old
            // copy was a solid ink box with ink text on it.
            decoration: BoxDecoration(
              color: scheme.secondaryContainer,
              borderRadius: BorderRadius.circular(AppShape.r6),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(
                  Icons.lock_outline,
                  size: AppIconSize.inlineGlyph,
                  color: scheme.onSecondaryContainer,
                ),
                const SizedBox(width: 4),
                Text(
                  'From Profile',
                  style: context.typeRoles.labelMicro.copyWith(
                    fontWeight: FontWeight.w500,
                    color: scheme.onSecondaryContainer,
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
  Widget _buildMapPickerButton(ColorScheme scheme) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        InkWell(
          onTap: _showLocationPicker,
          borderRadius: BorderRadius.circular(AppShape.r12),
          child: Container(
            padding: const EdgeInsets.symmetric(
              vertical: AppMetrics.p12,
              horizontal: AppMetrics.p16,
            ),
            decoration: BoxDecoration(
              color: scheme.surfaceContainerHigh,
              borderRadius: BorderRadius.circular(AppShape.r12),
              border: Border.all(
                color: hasCoordinates
                    ? context.statusColors.success
                    : scheme.outlineVariant,
                width: hasCoordinates ? 2 : 1,
              ),
            ),
            child: Row(
              children: [
                Icon(
                  hasCoordinates ? Icons.check_circle : Icons.map_outlined,
                  size: AppIconSize.action,
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
                        style: context.typeRoles.bodyDense.copyWith(
                          fontWeight: FontWeight.w500,
                          color: hasCoordinates
                              ? context.statusColors.success
                              : scheme.onSurface,
                        ),
                      ),
                      if (hasCoordinates)
                        Text(
                          '${_latitude!.toStringAsFixed(6)}, ${_longitude!.toStringAsFixed(6)}',
                          style: context.typeRoles.labelMicro.copyWith(
                            fontFamily: 'monospace',
                            color: scheme.onSurfaceVariant,
                          ),
                        ),
                    ],
                  ),
                ),
                Icon(
                  Icons.chevron_right,
                  size: AppIconSize.action,
                  color: scheme.onSurfaceVariant,
                ),
              ],
            ),
          ),
        ),
        const SizedBox(height: 4),
        Text(
          'Pinpoint location to facilitate delivery',
          style: context.typeRoles.labelMicro.copyWith(
            color: scheme.onSurfaceVariant,
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
