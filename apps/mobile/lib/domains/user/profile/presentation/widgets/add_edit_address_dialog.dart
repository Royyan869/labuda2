import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:uuid/uuid.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show addressRepositoryProvider;
import 'add_edit_address_dialog/address_dialog_header.dart';
import 'add_edit_address_dialog/address_dialog_actions.dart';
import 'add_edit_address_dialog/address_form_fields.dart';

/// Add/Edit Address Dialog
/// Modal dialog untuk menambah atau edit address
class AddEditAddressDialog extends ConsumerStatefulWidget {
  final AddressEntity? address;
  final String userId;

  /// If set, locks the purpose dropdown to this value
  /// Used when adding address from a specific tab (shipping/sender)
  final AddressPurpose? forcedPurpose;

  const AddEditAddressDialog({
    super.key,
    this.address,
    required this.userId,
    this.forcedPurpose,
  });

  @override
  ConsumerState<AddEditAddressDialog> createState() =>
      _AddEditAddressDialogState();
}

class _AddEditAddressDialogState extends ConsumerState<AddEditAddressDialog> {
  final _formKey = GlobalKey<FormState>();
  final _recipientNameController = TextEditingController();
  final _phoneController = TextEditingController();
  final _streetAddressController = TextEditingController();
  final _postalCodeController = TextEditingController();
  final _notesController = TextEditingController();
  final _nicknameController = TextEditingController();

  AddressPurpose? _selectedPurpose;
  Province? _selectedProvince;
  City? _selectedCity;
  District? _selectedDistrict;
  Village? _selectedVillage;

  // Map coordinates
  double? _latitude;
  double? _longitude;

  bool _isLoading = false;

  @override
  void initState() {
    super.initState();
    if (widget.address != null) {
      // Edit mode - populate fields
      _selectedPurpose = widget.address!.purpose;
      _nicknameController.text = widget.address!.nickname ?? '';
      _recipientNameController.text = widget.address!.recipientName;
      _phoneController.text = widget.address!.phone;
      _selectedProvince = widget.address!.province;
      _selectedCity = widget.address!.city;
      _selectedDistrict = widget.address!.district;
      _selectedVillage = widget.address!.village;
      _streetAddressController.text = widget.address!.streetAddress;
      _postalCodeController.text = widget.address!.postalCode;
      _notesController.text = widget.address!.notes ?? '';
      // Map coordinates
      _latitude = widget.address!.latitude;
      _longitude = widget.address!.longitude;
    } else if (widget.forcedPurpose != null) {
      // New address with forced purpose from tab
      _selectedPurpose = widget.forcedPurpose;
    }
  }

  @override
  void dispose() {
    _recipientNameController.dispose();
    _phoneController.dispose();
    _streetAddressController.dispose();
    _postalCodeController.dispose();
    _notesController.dispose();
    _nicknameController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final isEdit = widget.address != null;

    return Dialog(
      backgroundColor: Colors.transparent,
      insetPadding: const EdgeInsets.all(24),
      child: Container(
        constraints: const BoxConstraints(maxWidth: 600, maxHeight: 700),
        decoration: BoxDecoration(
          color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
          borderRadius: BorderRadius.circular(20),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Header
            AddressDialogHeader(
              isEdit: isEdit,
              onClose: () => Navigator.pop(context),
            ),

            // Form
            Flexible(
              child: SingleChildScrollView(
                padding: const EdgeInsets.all(24),
                child: Form(
                  key: _formKey,
                  child: AddressFormFields(
                    selectedPurpose: _selectedPurpose,
                    nicknameController: _nicknameController,
                    recipientNameController: _recipientNameController,
                    phoneController: _phoneController,
                    selectedProvince: _selectedProvince,
                    selectedCity: _selectedCity,
                    selectedDistrict: _selectedDistrict,
                    selectedVillage: _selectedVillage,
                    streetAddressController: _streetAddressController,
                    postalCodeController: _postalCodeController,
                    notesController: _notesController,
                    latitude: _latitude,
                    longitude: _longitude,
                    forcedPurpose: widget.forcedPurpose,
                    onPurposeChanged: (value) {
                      setState(() => _selectedPurpose = value);
                    },
                    onProvinceChanged: (province) {
                      setState(() {
                        _selectedProvince = province;
                        _selectedCity = null;
                        _selectedDistrict = null;
                        _selectedVillage = null;
                      });
                    },
                    onCityChanged: (city) {
                      setState(() {
                        _selectedCity = city;
                        _selectedDistrict = null;
                        _selectedVillage = null;
                      });
                    },
                    onDistrictChanged: (district) {
                      setState(() {
                        _selectedDistrict = district;
                        _selectedVillage = null;
                      });
                    },
                    onVillageChanged: (village) {
                      setState(() => _selectedVillage = village);
                    },
                    onCoordinatesChanged: (lat, lng) {
                      setState(() {
                        _latitude = lat;
                        _longitude = lng;
                      });
                    },
                  ),
                ),
              ),
            ),

            // Actions
            AddressDialogActions(
              isEdit: isEdit,
              isLoading: _isLoading,
              onCancel: () => Navigator.pop(context),
              onSubmit: _handleSubmit,
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _handleSubmit() async {
    // Prevent double-tap/multiple submissions
    if (_isLoading) return;

    if (!_formKey.currentState!.validate()) return;

    // Validate wilayah selection
    if (_selectedProvince == null ||
        _selectedCity == null ||
        _selectedDistrict == null ||
        _selectedVillage == null) {
      AppSnackBar.showError(context, 'Please complete all location fields');
      return;
    }

    setState(() => _isLoading = true);

    final repository = ref.read(addressRepositoryProvider);
    final now = DateTime.now();

    final addressEntity = AddressEntity(
      id: widget.address?.id ?? const Uuid().v4(),
      userId: widget.userId,
      purpose: _selectedPurpose ?? widget.forcedPurpose!,
      nickname: _nicknameController.text.trim().isEmpty
          ? null
          : _nicknameController.text.trim(),
      recipientName: _recipientNameController.text.trim(),
      phone: _phoneController.text.trim(),
      province: _selectedProvince!,
      city: _selectedCity!,
      district: _selectedDistrict!,
      village: _selectedVillage!,
      streetAddress: _streetAddressController.text.trim(),
      postalCode: _postalCodeController.text.trim(),
      notes: _notesController.text.trim().isEmpty
          ? null
          : _notesController.text.trim(),
      isPrimary: widget.address?.isPrimary ?? false,
      latitude: _latitude,
      longitude: _longitude,
      createdAt: widget.address?.createdAt ?? now,
      updatedAt: now,
    );

    // Error handling is done by the Repository via Result object
    final result = widget.address == null
        ? await repository.addAddress(addressEntity)
        : await repository.updateAddress(addressEntity);

    if (!mounted) {
      setState(() => _isLoading = false);
      return;
    }

    if (result.isSuccess) {
      Navigator.pop(context, true);
      AppSnackBar.showSuccess(
        context,
        widget.address == null
            ? 'Address added successfully'
            : 'Address updated successfully',
      );
    } else {
      AppSnackBar.showError(context, result.error ?? 'Failed to save address');
    }

    setState(() => _isLoading = false);
  }
}
