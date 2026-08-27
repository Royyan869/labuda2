import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/shared/widgets/app_snackbar.dart';
import 'package:labuda/shared/widgets/app_button.dart';
// TODO: import 'package:labuda/shared/providers/authenticated_account_provider.dart';
import 'package:labuda/shared/shared.dart' show authenticatedUserProvider;
import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/use_cases/discount_usecase_providers.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/providers/discount_provider.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/widgets/create_discount_form/basic_info_section.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/widgets/create_discount_form/discount_type_section.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/widgets/create_discount_form/scope_section.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/widgets/create_discount_form/validity_section.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/widgets/create_discount_form/limits_section.dart';

/// Screen untuk edit discount dengan business rules validation
class EditDiscountScreen extends ConsumerStatefulWidget {
  final Discount discount;

  const EditDiscountScreen({super.key, required this.discount});

  @override
  ConsumerState<EditDiscountScreen> createState() => _EditDiscountScreenState();
}

class _EditDiscountScreenState extends ConsumerState<EditDiscountScreen> {
  final _formKey = GlobalKey<FormState>();

  // Basic info
  late String _code;
  late String _description;

  // Type & value
  late DiscountType _type;
  late double _value;
  late double? _maxDiscount;

  // Applicability
  late DiscountAppliesTo _appliesTo;
  late DiscountTargetMode _targetMode;
  late List<String> _applicableListingIds;
  late List<String> _applicableAuctionIds;

  // Validity
  late DateTime _validFrom;
  late DateTime _validUntil;

  // Limits
  late double? _minPurchase;
  late int? _maxUsagePerUser;
  late int? _totalUsageLimit;

  // Status
  late bool _isActive;

  // State
  bool _isLoading = false;
  bool _hasUnsavedChanges = false;

  /// Check if discount has been used
  bool get _isUsed => widget.discount.currentUsageCount > 0;

  /// Original discount for validation
  Discount get _original => widget.discount;

  @override
  void initState() {
    super.initState();
    _initializeFields();
  }

  void _initializeFields() {
    _code = _original.code;
    _description = _original.description;
    _type = _original.type;
    _value = _original.value;
    _maxDiscount = _original.maxDiscount;
    _appliesTo = _original.appliesTo;
    _targetMode = _original.targetMode;
    _applicableListingIds = List.from(_original.applicableListingIds ?? []);
    _applicableAuctionIds = List.from(_original.applicableAuctionIds ?? []);
    _validFrom = _original.validFrom;
    _validUntil = _original.validUntil;
    _minPurchase = _original.minPurchase;
    _maxUsagePerUser = _original.maxUsagePerUser;
    _totalUsageLimit = _original.totalUsageLimit;
    _isActive = _original.isActive;
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

    // Use usecase for validation
    final validateDiscountUpdate = ref.read(
      validateDiscountUpdateUseCaseProvider,
    );
    final validationResult = validateDiscountUpdate(
      description: _description,
      validUntil: _validUntil,
      totalUsageLimit: _totalUsageLimit,
      originalDiscount: _original,
    );

    if (validationResult.isError) {
      AppSnackBar.showWarning(
        context,
        validationResult.error ?? 'Validation failed',
      );
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
      // Create updated discount entity
      final updatedDiscount = Discount(
        id: _original.id,
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
        sellerId: _original.sellerId,
        applicableListingIds:
            _appliesTo != DiscountAppliesTo.auction &&
                _applicableListingIds.isNotEmpty
            ? _applicableListingIds
            : null,
        applicableAuctionIds:
            _appliesTo != DiscountAppliesTo.listing &&
                _applicableAuctionIds.isNotEmpty
            ? _applicableAuctionIds
            : null,
        validFrom: _validFrom,
        validUntil: _validUntil,
        isActive: _isActive,
        currentUsageCount: _original.currentUsageCount,
        createdAt: _original.createdAt,
        createdBy: _original.createdBy,
      );

      // Call update use case
      final updateUseCase = ref.read(updateDiscountUseCaseProvider);
      final result = await updateUseCase(updatedDiscount);

      if (!mounted) return;

      result.fold(
        (error) {
          AppSnackBar.showError(context, error);
        },
        (discount) {
          // Invalidate provider untuk auto-refresh list
          ref.invalidate(sellerDiscountsProvider(currentUser.id));

          AppSnackBar.showSuccess(context, 'Discount updated successfully!');
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

  Widget _buildUsedDiscountBanner() {
    if (!_isUsed) return const SizedBox.shrink();

    return Container(
      margin: const EdgeInsets.only(bottom: 12),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: Colors.orange.withValues(alpha: 0.1),
        border: Border.all(color: Colors.orange),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.info_outline, color: Colors.orange[700], size: 20),
              const SizedBox(width: 8),
              Text(
                'Discount Already Used',
                style: TextStyle(
                  fontWeight: FontWeight.bold,
                  color: Colors.orange[700],
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text(
            'This discount has been used ${_original.currentUsageCount} times. '
            'Some fields cannot be changed to maintain data consistency.',
            style: TextStyle(fontSize: 13, color: Colors.orange[900]),
          ),
          const SizedBox(height: 12),
          Text(
            'What can be changed:',
            style: TextStyle(
              fontWeight: FontWeight.w600,
              fontSize: 13,
              color: Colors.orange[900],
            ),
          ),
          const SizedBox(height: 4),
          ...[
            '• Active/Inactive Status',
            '• Description',
            '• Extend validity period',
            '• Add usage limit',
          ].map(
            (text) => Padding(
              padding: const EdgeInsets.only(left: 8, top: 2),
              child: Text(
                text,
                style: TextStyle(fontSize: 12, color: Colors.orange[800]),
              ),
            ),
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
          title: const Text('Edit Discount'),
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
                // Info banner for used discounts
                _buildUsedDiscountBanner(),

                // Basic Info Section
                BasicInfoSection(
                  code: _code,
                  description: _description,
                  isEditMode: true,
                  onCodeChanged: (value) {
                    if (!_isUsed) {
                      setState(() {
                        _code = value;
                        _markChanged();
                      });
                    }
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
                IgnorePointer(
                  ignoring: _isUsed,
                  child: Opacity(
                    opacity: _isUsed ? 0.5 : 1.0,
                    child: DiscountTypeSection(
                      type: _type,
                      value: _value,
                      maxDiscount: _maxDiscount,
                      onTypeChanged: (value) {
                        setState(() {
                          _type = value;
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
                  ),
                ),
                const SizedBox(height: 12),

                // Scope Section
                IgnorePointer(
                  ignoring: _isUsed,
                  child: Opacity(
                    opacity: _isUsed ? 0.5 : 1.0,
                    child: ScopeSection(
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
                  ),
                ),
                const SizedBox(height: 12),

                // Validity Period Section
                // Note: validFrom disabled if used, but validUntil can be extended
                ValiditySection(
                  validFrom: _validFrom,
                  validUntil: _validUntil,
                  onValidFromChanged: (value) {
                    if (!_isUsed) {
                      setState(() {
                        _validFrom = value;
                        _markChanged();
                      });
                    }
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
                // Note: minPurchase and maxUsagePerUser disabled if used
                // But totalUsageLimit (can increase) and isActive always editable
                LimitsSection(
                  minPurchase: _minPurchase,
                  maxUsagePerUser: _maxUsagePerUser,
                  totalUsageLimit: _totalUsageLimit,
                  isActive: _isActive,
                  onMinPurchaseChanged: (value) {
                    if (!_isUsed) {
                      setState(() {
                        _minPurchase = value;
                        _markChanged();
                      });
                    }
                  },
                  onMaxUsagePerUserChanged: (value) {
                    if (!_isUsed) {
                      setState(() {
                        _maxUsagePerUser = value;
                        _markChanged();
                      });
                    }
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
              text: 'Save Changes',
              onPressed: _submit,
              isLoading: _isLoading,
            ),
          ),
        ),
      ),
    );
  }
}
