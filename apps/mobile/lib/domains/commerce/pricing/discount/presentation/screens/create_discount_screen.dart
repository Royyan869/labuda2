import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/shared/widgets/app_snackbar.dart';
import 'package:labuda/shared/widgets/app_button.dart';
import 'package:labuda/shared/shared.dart' show authenticatedUserProvider;
import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/use_cases/create_discount_use_case.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/providers/discount_provider.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/widgets/create_discount_form/basic_info_section.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/widgets/create_discount_form/discount_type_section.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/widgets/create_discount_form/applies_to_section.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/widgets/create_discount_form/validity_section.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/widgets/create_discount_form/limits_section.dart';

/// Screen untuk membuat discount
///
/// CANONICAL MODEL (DISCOUNT-003): code, description, type, value,
/// minPurchase, applies_to (for_sale/auction/both), validUntil, totalUsageLimit.
/// No specific-item targeting.
class CreateDiscountScreen extends ConsumerStatefulWidget {
  final Discount? discountToEdit;

  const CreateDiscountScreen({super.key, this.discountToEdit});

  @override
  ConsumerState<CreateDiscountScreen> createState() =>
      _CreateDiscountScreenState();
}

class _CreateDiscountScreenState extends ConsumerState<CreateDiscountScreen> {
  final _formKey = GlobalKey<FormState>();

  // Basic info
  String _code = '';
  String _description = '';

  // Type & value
  DiscountType _type = DiscountType.percentage;
  double _value = 0;

  // Applicability (surface type only)
  DiscountAppliesTo _appliesTo = DiscountAppliesTo.forSale;

  // Minimum purchase
  double _minPurchase = 0.0;

  // Validity (expiry-only)
  DateTime _validUntil = DateTime.now().add(const Duration(days: 30));

  // Limits
  int? _totalUsageLimit;

  // Status
  bool _isActive = true;

  // State
  bool _isLoading = false;
  bool _hasUnsavedChanges = false;

  bool get _isEditMode => widget.discountToEdit != null;

  @override
  void initState() {
    super.initState();
    if (_isEditMode) {
      _initializeEditMode();
    }
  }

  void _initializeEditMode() {
    final discount = widget.discountToEdit!;
    _code = discount.code;
    _description = discount.description;
    _type = discount.type;
    _value = discount.value;
    _minPurchase = discount.minPurchase;
    _appliesTo = discount.appliesTo;
    _validUntil = discount.validUntil;
    _totalUsageLimit = discount.totalUsageLimit;
    _isActive = discount.isActive;
  }

  void _markChanged() {
    if (!_hasUnsavedChanges) {
      setState(() => _hasUnsavedChanges = true);
    }
  }

  bool _validateForm() {
    if (!_formKey.currentState!.validate()) {
      return false;
    }

    if (_code.trim().length < 3) {
      AppSnackBar.showWarning(context, 'Discount code minimum 3 characters');
      return false;
    }

    if (_value <= 0) {
      AppSnackBar.showWarning(context, 'Discount value must be greater than 0');
      return false;
    }

    if (_type == DiscountType.percentage && _value > 100) {
      AppSnackBar.showWarning(context, 'Discount percentage maximum 100%');
      return false;
    }

    if (_validUntil.isBefore(DateTime.now())) {
      AppSnackBar.showWarning(context, 'Expiry date must be in the future');
      return false;
    }

    return true;
  }

  Future<void> _submit() async {
    if (!_validateForm()) return;

    final currentUser = ref.read(authenticatedUserProvider);
    if (currentUser == null) {
      AppSnackBar.showError(context, 'User not found');
      return;
    }

    setState(() => _isLoading = true);

    try {
      final useCase = ref.read(createDiscountUseCaseProvider);

      final params = CreateDiscountParams(
        code: _code.trim().toUpperCase(),
        description: _description.trim(),
        type: _type,
        value: _value,
        minPurchase: _minPurchase,
        totalUsageLimit: _totalUsageLimit,
        appliesTo: _appliesTo,
        sellerId: currentUser.id,
        validUntil: _validUntil,
        isActive: _isActive,
        createdBy: currentUser.id,
      );

      final result = await useCase(params);

      if (!mounted) return;

      result.fold(
        (error) {
          AppSnackBar.showError(context, error);
        },
        (discount) {
          ref.invalidate(sellerDiscountsProvider(currentUser.id));

          AppSnackBar.showSuccess(
            context,
            _isEditMode
                ? 'Discount updated successfully!'
                : 'Discount created successfully!',
          );
          Navigator.of(context).pop(true);
        },
      );
    } catch (e) {
      if (mounted) {
        AppSnackBar.showError(context, 'Terjadi kesalahan. Coba lagi.');
      }
    } finally {
      if (mounted) {
        setState(() => _isLoading = false);
      }
    }
  }

  void _showDiscardDialog() {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Discard Changes?'),
        content: const Text(
          'Anda memiliki perubahan yang belum disimpan. '
          'Yakin ingin keluar?',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () {
              Navigator.of(context).pop();
              Navigator.of(context).pop();
            },
            child: const Text('Discard', style: TextStyle(color: Colors.red)),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) {
        if (didPop) return;
        if (_hasUnsavedChanges) {
          _showDiscardDialog();
        } else {
          Navigator.of(context).pop();
        }
      },
      child: Scaffold(
        backgroundColor: isDark
            ? core.AppColors.darkGray900
            : core.AppColors.neutralGray50,
        appBar: AppBar(
          title: Text(_isEditMode ? 'Edit Discount' : 'Create New Discount'),
          elevation: 0,
          surfaceTintColor: Colors.transparent,
          scrolledUnderElevation: 0,
          leading: IconButton(
            icon: const Icon(Icons.close),
            onPressed: () {
              if (_hasUnsavedChanges) {
                _showDiscardDialog();
              } else {
                Navigator.of(context).pop();
              }
            },
          ),
        ),
        body: Form(
          key: _formKey,
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Basic Info Section
                BasicInfoSection(
                  code: _code,
                  description: _description,
                  isEditMode: _isEditMode,
                  onCodeChanged: (value) {
                    setState(() {
                      _code = value;
                      _markChanged();
                    });
                  },
                  onDescriptionChanged: (value) {
                    setState(() {
                      _description = value;
                      _markChanged();
                    });
                  },
                ),
                const SizedBox(height: 12),

                // Type & Value Section
                DiscountTypeSection(
                  type: _type,
                  value: _value,
                  onTypeChanged: (value) {
                    setState(() {
                      _type = value;
                      _markChanged();
                    });
                  },
                  onValueChanged: (value) {
                    setState(() {
                      _value = value;
                      _markChanged();
                    });
                  },
                ),
                const SizedBox(height: 12),

                // Scope Section (dropdown only — no UUID inputs)
                AppliesToSection(
                  appliesTo: _appliesTo,
                  onAppliesToChanged: (value) {
                    setState(() {
                      _appliesTo = value;
                      _markChanged();
                    });
                  },
                ),
                const SizedBox(height: 12),

                // Validity Period Section (expiry-only)
                ValiditySection(
                  validUntil: _validUntil,
                  onValidUntilChanged: (value) {
                    setState(() {
                      _validUntil = value;
                      _markChanged();
                    });
                  },
                ),
                const SizedBox(height: 12),

                // Limits Section (minPurchase + totalUsageLimit + active status)
                LimitsSection(
                  totalUsageLimit: _totalUsageLimit,
                  minPurchase: _minPurchase,
                  isActive: _isActive,
                  onTotalUsageLimitChanged: (value) {
                    setState(() {
                      _totalUsageLimit = value;
                      _markChanged();
                    });
                  },
                  onMinPurchaseChanged: (value) {
                    setState(() {
                      _minPurchase = value;
                      _markChanged();
                    });
                  },
                  onIsActiveChanged: (value) {
                    setState(() {
                      _isActive = value;
                      _markChanged();
                    });
                  },
                ),

                const SizedBox(height: 80), // Space for bottom button
              ],
            ),
          ),
        ),
        bottomNavigationBar: Container(
          padding: const EdgeInsets.all(12),
          decoration: BoxDecoration(
            color: isDark
                ? core.AppColors.darkGray800
                : core.AppColors.neutralWhite,
            boxShadow: [
              BoxShadow(
                color: Colors.black.withValues(alpha: 0.05),
                blurRadius: 10,
                offset: const Offset(0, -2),
              ),
            ],
          ),
          child: SafeArea(
            child: AppButton(
              text: _isEditMode ? 'Save Changes' : 'Create Discount',
              onPressed: _submit,
              isLoading: _isLoading,
            ),
          ),
        ),
      ),
    );
  }
}
