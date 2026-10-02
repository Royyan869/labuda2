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
///
/// This is the ONE address form in the app (Settings list, checkout CTA and
/// the seller wizard all open it). The old per-tab copy with its own rules
/// (`AddEditAddressDialog`) was the competing authority and is gone.
class AddressFormDialog extends ConsumerStatefulWidget {
  final AddressEntity? addressToEdit;

  /// Tags pre-selected when creating a new address.
  final List<AddressTag>? presetTags;

  const AddressFormDialog({
    super.key,
    this.addressToEdit,
    this.presetTags,
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
  final Set<AddressTag> _selectedTags = {AddressTag.shipping};
  bool _isLoading = false;

  /// Total active addresses of this account. Role tags are only a real
  /// CHOICE at 2+ addresses — a lone address is simply everything
  /// (backend reconciler enforces both tags + primary), so the selector
  /// stands down below that.
  int? _addressCount;

  bool get _showTagSelector => (_addressCount ?? 0) >= 2;

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
      // New address - preselect the tags this flow requires.
      final preset = <AddressTag>{...?widget.presetTags};
      if (preset.isNotEmpty) {
        _selectedTags
          ..clear()
          ..addAll(preset);
      }
      // Autofill after build (needs ref)
      WidgetsBinding.instance.addPostFrameCallback((_) {
        _autofillFromProfile();
      });
    }
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _loadAddressCount();
    });
  }

  Future<void> _loadAddressCount() async {
    final currentUser = ref.read(authenticatedUserProvider);
    if (currentUser == null) return;
    final result = await ref
        .read(addressRepositoryProvider)
        .countAddresses(currentUser.id);
    if (!mounted || !result.isSuccess) return;
    setState(() => _addressCount = result.data);
  }

  /// Autofill name and phone from user profile
  Future<void> _autofillFromProfile() async {
    final currentUser = ref.read(authenticatedUserProvider);
    if (currentUser == null) return;

    final isSenderFlow = _selectedTags.contains(AddressTag.sender);

    if (isSenderFlow &&
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
      _selectedTags
        ..clear()
        ..addAll(address.tags);
      // Map coordinates
      _latitude = address.latitude;
      _longitude = address.longitude;

      // Load nickname for shipping addresses
      if (address.hasTag(AddressTag.shipping) &&
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

  List<AddressTag> _sortedSelectedTags() => AddressTag.values
      .where(_selectedTags.contains)
      .toList();

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

    if (_selectedTags.isEmpty) {
      AppSnackBar.showError(context, 'Pick at least one role for this address');
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

      // Nickname belongs to the shipping role of this address
      String? nickname;
      if (_selectedTags.contains(AddressTag.shipping)) {
        // Only shipping-tagged addresses have nickname
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
      // Sender-only addresses don't have nickname (null)

      final address = AddressEntity(
        id: widget.addressToEdit?.id ?? '',
        userId: currentUser.id,
        // Below 2 addresses the account's address is everything (and the
        // backend reconciler forces it anyway): never save a roleless
        // single address.
        tags: _showTagSelector
            ? _sortedSelectedTags()
            : const [AddressTag.shipping, AddressTag.sender],
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
    final scheme = Theme.of(context).colorScheme;

    return DraggableScrollableSheet(
      initialChildSize: 0.9,
      minChildSize: 0.5,
      maxChildSize: 0.95,
      expand: false,
      builder: (context, scrollController) => Container(
        // Sheet surface follows the canonical transparent-sheet convention
        // (LinkPickerModal): `surface` + r20 top. `onSurfaceVariant` used to
        // be the LAYER colour here — a mid-grey slab.
        decoration: BoxDecoration(
          color: scheme.surface,
          borderRadius: const BorderRadius.only(
            topLeft: Radius.circular(AppShape.r20),
            topRight: Radius.circular(AppShape.r20),
          ),
        ),
        child: Column(
          children: [
            // Handle bar — ONE authority: `AppDragHandle` beside the bottom-sheet
            // base. What matched the link picker by hand is now guaranteed by it.
            const AppDragHandle(padding: EdgeInsets.only(top: AppMetrics.p12)),
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
                      style: TextStyle(
                        fontSize: AppType.s20,
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
                  controller: scrollController,
                  padding: EdgeInsets.fromLTRB(
                    AppMetrics.p24,
                    AppMetrics.p24,
                    AppMetrics.p24,
                    AppMetrics.p24 + MediaQuery.of(context).viewInsets.bottom,
                  ),
                  children: [
                    // Role tags — only a CHOICE when the account owns 2+
                    // addresses. A lone address is simply everything.
                    if (_showTagSelector) ...[
                      _buildTagSelector(scheme),
                      const SizedBox(height: 16),
                    ] else ...[
                      Text(
                        'Applies to shipping & sender',
                        style: TextStyle(
                          fontSize: AppType.s12,
                          color: scheme.onSurfaceVariant,
                        ),
                      ),
                      const SizedBox(height: 16),
                    ],

                    // Nickname field (only for shipping-tagged addresses)
                    if (_selectedTags.contains(AddressTag.shipping)) ...[
                      _buildNicknameDropdown(scheme),
                      const SizedBox(height: 16),
                    ],

                    // Recipient/Sender Name
                    if (_isNameLocked)
                      // Locked display for seller business name
                      _buildLockedNameField(scheme)
                    else
                      AppTextField(
                        controller: _recipientNameController,
                        labelText: _isSenderTagged
                            ? 'Sender Name *'
                            : 'Recipient Name *',
                        hintText: _isSenderTagged
                            ? 'Store/farm name'
                            : 'Full name of recipient',
                        prefixIcon: Icons.person_outline,
                        validator: (value) {
                          if (value == null || value.trim().isEmpty) {
                            return _isSenderTagged
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
                padding: const EdgeInsets.all(AppMetrics.p24),
                // Sticky action bar = the same surface + top divider the
                // address list's own sticky bar uses.
                decoration: BoxDecoration(
                  color: scheme.surface,
                  border: Border(
                    top: BorderSide(color: scheme.outlineVariant),
                  ),
                ),
                child: SizedBox(
                  width: double.infinity,
                  child: ElevatedButton(
                    onPressed: _isLoading ? null : _handleSave,
                    style: ElevatedButton.styleFrom(
                      padding: const EdgeInsets.symmetric(vertical: AppMetrics.p16),
                    ),
                    child: _isLoading
? SizedBox(
                             height: 20,
                            width: 20,
                            child: CircularProgressIndicator(
                              strokeWidth: 2,
                              valueColor: AlwaysStoppedAnimation<Color>(
                                scheme.onPrimary,
                              ),
                            ),
                          )
                        : Text(
                            widget.addressToEdit != null
                                ? 'Update Address'
                                : 'Save Address',
                            style: const TextStyle(
                              fontSize: AppType.s16,
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

  bool get _isSenderTagged => _selectedTags.contains(AddressTag.sender);

  IconData _getTagIcon(AddressTag tag) {
    switch (tag) {
      case AddressTag.shipping:
        return Icons.home;
      case AddressTag.sender:
        return Icons.agriculture;
    }
  }

  /// Role tags: what this address is FOR. Multi-select — one address may be
  /// both a delivery destination and a shipping origin. Shown only at 2+
  /// addresses; below that the lone address is everything.
  Widget _buildTagSelector(ColorScheme scheme) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Use this address for *',
          style: TextStyle(
            fontSize: AppType.s14,
            fontWeight: FontWeight.w600,
            color: scheme.onSurface,
          ),
        ),
        const SizedBox(height: 4),
        Text(
          'One address can serve more than one role.',
          style: TextStyle(
            fontSize: AppType.s12,
            color: scheme.onSurfaceVariant,
          ),
        ),
        const SizedBox(height: 8),
        Wrap(
          spacing: AppMetrics.p8,
          runSpacing: AppMetrics.p8,
          children: [
            for (final tag in AddressTag.values)
              ChoiceChip(
                avatar: Icon(_getTagIcon(tag), size: 16),
                label: Text(tag.shortLabel),
                selected: _selectedTags.contains(tag),
                onSelected: (selected) {
                  setState(() {
                    if (selected) {
                      _selectedTags.add(tag);
                    } else {
                      _selectedTags.remove(tag);
                    }
                  });
                },
              ),
          ],
        ),
        if (_selectedTags.isEmpty)
          Padding(
            padding: const EdgeInsets.only(top: AppMetrics.p8),
            child: Text(
              'Pick at least one role',
              style: TextStyle(fontSize: AppType.s12, color: scheme.error),
            ),
          ),
      ],
    );
  }

  /// Build nickname dropdown for shipping addresses
  Widget _buildNicknameDropdown(ColorScheme scheme) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        DropdownButtonFormField<String>(
          initialValue: _selectedNickname,
          decoration: InputDecoration(
            labelText: 'Address Label (Optional)',
            hintText: 'Select label',
            prefixIcon: const Icon(Icons.label_outline),
            border: OutlineInputBorder(borderRadius: BorderRadius.circular(AppShape.r12)),
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
  Widget _buildLockedNameField(ColorScheme scheme) {
    return Container(
      padding: const EdgeInsets.symmetric(vertical: AppMetrics.p16, horizontal: AppMetrics.p16),
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
                  'Sender Name',
                  style: TextStyle(
                    fontSize: AppType.s12,
                    fontWeight: FontWeight.w500,
                    color: scheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  _recipientNameController.text.isNotEmpty
                      ? _recipientNameController.text
                      : 'Store Name',
                  style: TextStyle(
                    fontSize: AppType.s16,
                    fontWeight: FontWeight.w600,
                    color: scheme.onSurface,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p8, vertical: AppMetrics.p4),
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
                  style: TextStyle(
                    fontSize: AppType.s12,
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
            padding: const EdgeInsets.symmetric(vertical: AppMetrics.p12, horizontal: AppMetrics.p16),
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
                        style: TextStyle(
                          fontSize: AppType.s14,
                          fontWeight: FontWeight.w500,
                          color: hasCoordinates
                              ? context.statusColors.success
                              : scheme.onSurface,
                        ),
                      ),
                      if (hasCoordinates)
                        Text(
                          '${_latitude!.toStringAsFixed(6)}, ${_longitude!.toStringAsFixed(6)}',
                          style: TextStyle(
                            fontSize: AppType.s12,
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
          style: TextStyle(
            fontSize: AppType.s12,
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
