import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/shared/widgets/app_snackbar.dart';
import 'package:labuda/shared/widgets/app_button.dart';
// TODO: import 'package:labuda/shared/providers/authenticated_account_provider.dart';
import 'package:labuda/shared/shared.dart' show authenticatedUserProvider;
import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/use_cases/create_discount_use_case.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/providers/discount_provider.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/widgets/create_discount_form/basic_info_section.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/widgets/create_discount_form/discount_type_section.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/widgets/create_discount_form/scope_section.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/widgets/create_discount_form/validity_section.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/widgets/create_discount_form/limits_section.dart';

/// Screen untuk membuat atau edit discount
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
  double? _maxDiscount;

  // Applicability
  DiscountAppliesTo _appliesTo = DiscountAppliesTo.listing;
  DiscountTargetMode _targetMode = DiscountTargetMode.sellerWide;
  List<String> _applicableListingIds = [];
  List<String> _applicableAuctionIds = [];

  // Validity
  DateTime _validFrom = DateTime.now();
  DateTime _validUntil = DateTime.now().add(const Duration(days: 30));

  // Limits
  double? _minPurchase;
  int? _maxUsagePerUser;
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
    _maxDiscount = discount.maxDiscount;
    _appliesTo = discount.appliesTo;
    _targetMode = discount.targetMode;
    _applicableListingIds = List.from(discount.applicableListingIds ?? []);
    _applicableAuctionIds = List.from(discount.applicableAuctionIds ?? []);
    _validFrom = discount.validFrom;
    _validUntil = discount.validUntil;
    _minPurchase = discount.minPurchase;
    _maxUsagePerUser = discount.maxUsagePerUser;
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

    // Validate code
    if (_code.trim().length < 3) {
      AppSnackBar.showWarning(context, 'Discount code minimum 3 characters');
      return false;
    }

    // Validate value
    if (_value <= 0) {
      AppSnackBar.showWarning(context, 'Discount value must be greater than 0');
      return false;
    }

    if (_type == DiscountType.percentage && _value > 100) {
      AppSnackBar.showWarning(context, 'Discount percentage maximum 100%');
      return false;
    }

    // Validate dates
    if (_validUntil.isBefore(_validFrom)) {
      AppSnackBar.showWarning(context, 'End date must be after start date');
      return false;
    }

    // Validate target-specific fields
    if (_targetMode == DiscountTargetMode.selectedItems) {
      final listingAllowed = _appliesTo != DiscountAppliesTo.auction;
      final auctionAllowed = _appliesTo != DiscountAppliesTo.listing;
      final hasListingTargets = _applicableListingIds.isNotEmpty;
      final hasAuctionTargets = _applicableAuctionIds.isNotEmpty;

      if ((listingAllowed && !hasListingTargets) &&
          (auctionAllowed && !hasAuctionTargets)) {
        AppSnackBar.showWarning(
          context,
          'Select at least one listing or auction target',
        );
        return false;
      }

      if (listingAllowed &&
          !hasListingTargets &&
          _appliesTo == DiscountAppliesTo.listing) {
        AppSnackBar.showWarning(context, 'Select at least 1 listing target');
        return false;
      }

      if (auctionAllowed &&
          !hasAuctionTargets &&
          _appliesTo == DiscountAppliesTo.auction) {
        AppSnackBar.showWarning(context, 'Select at least 1 auction target');
        return false;
      }
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
        maxDiscount: _maxDiscount,
        maxUsagePerUser: _maxUsagePerUser,
        totalUsageLimit: _totalUsageLimit,
        appliesTo: _appliesTo,
        targetMode: _targetMode,
        sellerId: currentUser.id,
        applicableListingIds: _appliesTo != DiscountAppliesTo.auction
            ? _applicableListingIds
            : null,
        applicableAuctionIds: _appliesTo != DiscountAppliesTo.listing
            ? _applicableAuctionIds
            : null,
        validFrom: _validFrom,
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
          // Invalidate provider untuk auto-refresh list
          ref.invalidate(sellerDiscountsProvider(currentUser.id));

          AppSnackBar.showSuccess(
            context,
            _isEditMode
                ? 'Discount updated successfully!'
                : 'Discount created successfully!',
          );
          Navigator.of(context).pop(true); // Return true to indicate success
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
                  maxDiscount: _maxDiscount,
                  onTypeChanged: (value) {
                    setState(() {
                      _type = value;
                      // Reset max discount if not percentage
                      if (_type != DiscountType.percentage) {
                        _maxDiscount = null;
                      }
                      _markChanged();
                    });
                  },
                  onValueChanged: (value) {
                    setState(() {
                      _value = value;
                      _markChanged();
                    });
                  },
                  onMaxDiscountChanged: (value) {
                    setState(() {
                      _maxDiscount = value;
                      _markChanged();
                    });
                  },
                ),
                const SizedBox(height: 12),

                // Scope Section
                ScopeSection(
                  appliesTo: _appliesTo,
                  targetMode: _targetMode,
                  applicableListingIds: _applicableListingIds,
                  applicableAuctionIds: _applicableAuctionIds,
                  onAppliesToChanged: (value) {
                    setState(() {
                      _appliesTo = value;
                      _applicableListingIds = [];
                      _applicableAuctionIds = [];
                      _markChanged();
                    });
                  },
                  onTargetModeChanged: (value) {
                    setState(() {
                      _targetMode = value;
                      _markChanged();
                    });
                  },
                  onListingIdsChanged: (value) {
                    setState(() {
                      _applicableListingIds = value;
                      _markChanged();
                    });
                  },
                  onAuctionIdsChanged: (value) {
                    setState(() {
                      _applicableAuctionIds = value;
                      _markChanged();
                    });
                  },
                ),
                const SizedBox(height: 12),

                // Validity Period Section
                ValiditySection(
                  validFrom: _validFrom,
                  validUntil: _validUntil,
                  onValidFromChanged: (value) {
                    setState(() {
                      _validFrom = value;
                      _markChanged();
                    });
                  },
                  onValidUntilChanged: (value) {
                    setState(() {
                      _validUntil = value;
                      _markChanged();
                    });
                  },
                ),
                const SizedBox(height: 12),

                // Limits Section
                LimitsSection(
                  minPurchase: _minPurchase,
                  maxUsagePerUser: _maxUsagePerUser,
                  totalUsageLimit: _totalUsageLimit,
                  isActive: _isActive,
                  onMinPurchaseChanged: (value) {
                    setState(() {
                      _minPurchase = value;
                      _markChanged();
                    });
                  },
                  onMaxUsagePerUserChanged: (value) {
                    setState(() {
                      _maxUsagePerUser = value;
                      _markChanged();
                    });
                  },
                  onTotalUsageLimitChanged: (value) {
                    setState(() {
                      _totalUsageLimit = value;
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
