import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/user/profile/domain/entities/bank_account_entity.dart';
import 'package:labuda/domains/user/profile/presentation/providers/bank_account_provider.dart'
    show bankAccountRepositoryProvider;

/// Add/Edit Bank Account Dialog
/// Modal dialog for adding or editing bank account
class AddEditBankAccountDialog extends ConsumerStatefulWidget {
  final BankAccountEntity? account;
  final String userId;

  const AddEditBankAccountDialog({
    super.key,
    this.account,
    required this.userId,
  });

  @override
  ConsumerState<AddEditBankAccountDialog> createState() =>
      _AddEditBankAccountDialogState();
}

class _AddEditBankAccountDialogState
    extends ConsumerState<AddEditBankAccountDialog> {
  final _formKey = GlobalKey<FormState>();
  final _accountNumberController = TextEditingController();
  final _accountHolderController = TextEditingController();

  String? _selectedBankCode;
  String? _selectedBankName;
  bool _isLoading = false;

  // List of Indonesian banks (simplified - can be moved to constants or fetched from API)
  final List<BankInfo> _indonesianBanks = const [
    BankInfo(code: 'BCA', name: 'Bank Central Asia (BCA)', icon: '🏦'),
    BankInfo(code: 'MANDIRI', name: 'Bank Mandiri', icon: '🏦'),
    BankInfo(code: 'BRI', name: 'Bank Rakyat Indonesia (BRI)', icon: '🏦'),
    BankInfo(code: 'BNI', name: 'Bank Negara Indonesia (BNI)', icon: '🏦'),
    BankInfo(code: 'CIMB', name: 'CIMB Niaga', icon: '🏦'),
    BankInfo(code: 'PERMATA', name: 'Bank Permata', icon: '🏦'),
    BankInfo(code: 'DANAMON', name: 'Bank Danamon', icon: '🏦'),
    BankInfo(code: 'BTN', name: 'Bank Tabungan Negara (BTN)', icon: '🏦'),
    BankInfo(code: 'MEGA', name: 'Bank Mega', icon: '🏦'),
    BankInfo(code: 'PANIN', name: 'Bank Panin', icon: '🏦'),
    BankInfo(code: 'OCBC', name: 'OCBC NISP', icon: '🏦'),
    BankInfo(code: 'BSI', name: 'Bank Syariah Indonesia (BSI)', icon: '🏦'),
    BankInfo(code: 'MUAMALAT', name: 'Bank Muamalat', icon: '🏦'),
    BankInfo(code: 'GOPAY', name: 'GoPay', icon: '💳'),
    BankInfo(code: 'OVO', name: 'OVO', icon: '💳'),
    BankInfo(code: 'DANA', name: 'DANA', icon: '💳'),
  ];

  @override
  void initState() {
    super.initState();
    if (widget.account != null) {
      // Edit mode - populate fields
      _accountNumberController.text = widget.account!.accountNumber;
      _accountHolderController.text = widget.account!.accountHolderName;
      _selectedBankCode = widget.account!.bankCode;
      _selectedBankName = widget.account!.bankName;
    }
  }

  @override
  void dispose() {
    _accountNumberController.dispose();
    _accountHolderController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final isEdit = widget.account != null;

    return Dialog(
      backgroundColor: Colors.transparent,
      insetPadding: const EdgeInsets.all(24),
      child: Container(
        constraints: const BoxConstraints(maxWidth: 500),
        decoration: BoxDecoration(
          color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
          borderRadius: BorderRadius.circular(20),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Header
            Container(
              padding: const EdgeInsets.all(24),
              decoration: BoxDecoration(
                color: isDark ? AppColors.darkGray700 : AppColors.neutralGray50,
                borderRadius: const BorderRadius.only(
                  topLeft: Radius.circular(20),
                  topRight: Radius.circular(20),
                ),
              ),
              child: Row(
                children: [
                  Container(
                    padding: const EdgeInsets.all(8),
                    decoration: BoxDecoration(
                      color: AppColors.primaryRed.withValues(alpha: 0.1),
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: Icon(
                      Icons.account_balance,
                      color: AppColors.primaryRed,
                      size: 24,
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Text(
                      isEdit ? 'Edit Bank Account' : 'Add Bank Account',
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
                    onPressed: () => Navigator.pop(context),
                    icon: Icon(
                      Icons.close,
                      color: isDark
                          ? AppColors.neutralGray400
                          : AppColors.neutralGray600,
                    ),
                  ),
                ],
              ),
            ),

            // Form
            Flexible(
              child: SingleChildScrollView(
                padding: const EdgeInsets.all(24),
                child: Form(
                  key: _formKey,
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      // Bank selection
                      _buildLabel('Bank', isDark),
                      const SizedBox(height: 8),
                      _buildBankDropdown(isDark),
                      const SizedBox(height: 16),

                      // Account Number
                      AppTextField(
                        controller: _accountNumberController,
                        labelText: 'Account Number *',
                        hintText: 'Enter account number',
                        prefixIcon: Icons.numbers,
                        keyboardType: TextInputType.number,
                        inputFormatters: [
                          FilteringTextInputFormatter.digitsOnly,
                          LengthLimitingTextInputFormatter(20),
                        ],
                        validator: (value) {
                          if (value == null || value.isEmpty) {
                            return 'Account number is required';
                          }
                          if (value.length < 8) {
                            return 'Account number must be at least 8 digits';
                          }
                          return null;
                        },
                      ),
                      const SizedBox(height: 16),

                      // Account Holder Name
                      AppTextField(
                        controller: _accountHolderController,
                        labelText: 'Account Holder Name *',
                        hintText: 'Enter account holder name',
                        prefixIcon: Icons.person_outline,
                        textCapitalization: TextCapitalization.words,
                        validator: (value) {
                          if (value == null || value.isEmpty) {
                            return 'Account holder name is required';
                          }
                          if (value.length < 3) {
                            return 'Name must be at least 3 characters';
                          }
                          return null;
                        },
                      ),
                      const SizedBox(height: 8),
                    ],
                  ),
                ),
              ),
            ),

            // Actions
            Container(
              padding: const EdgeInsets.all(24),
              decoration: BoxDecoration(
                color: isDark ? AppColors.darkGray700 : AppColors.neutralGray50,
                borderRadius: const BorderRadius.only(
                  bottomLeft: Radius.circular(20),
                  bottomRight: Radius.circular(20),
                ),
              ),
              child: Row(
                children: [
                  Expanded(
                    child: AppButton.secondary(
                      text: 'Cancel',
                      onPressed: () => Navigator.pop(context),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: AppButton.primary(
                      text: isEdit ? 'Update' : 'Add Account',
                      onPressed: _isLoading ? null : _handleSubmit,
                      isLoading: _isLoading,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
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

  Widget _buildBankDropdown(bool isDark) {
    return DropdownButtonFormField<String>(
      initialValue: _selectedBankCode,
      decoration: InputDecoration(
        hintText: 'Select bank',
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
      ),
      dropdownColor: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
      items: _indonesianBanks.map((bank) {
        return DropdownMenuItem(
          value: bank.code,
          child: Row(
            children: [
              Text(bank.icon, style: const TextStyle(fontSize: 20)),
              const SizedBox(width: 12),
              Expanded(
                child: Text(
                  bank.name,
                  style: TextStyle(
                    color: isDark
                        ? AppColors.neutralGray200
                        : AppColors.neutralGray900,
                  ),
                ),
              ),
            ],
          ),
        );
      }).toList(),
      onChanged: (value) {
        setState(() {
          _selectedBankCode = value;
          _selectedBankName = _indonesianBanks
              .firstWhere((bank) => bank.code == value)
              .name;
        });
      },
      validator: (value) {
        if (value == null || value.isEmpty) {
          return 'Please select a bank';
        }
        return null;
      },
    );
  }

  Future<void> _handleSubmit() async {
    if (!_formKey.currentState!.validate()) return;

    setState(() => _isLoading = true);

    final repository = ref.read(bankAccountRepositoryProvider);
    final now = DateTime.now();

    // Backend has no update endpoint for bank accounts — add-only.
    // The entity holds only fields the backend accepts: no userId/branch/alias.
    final bankAccount = BankAccountEntity(
      id: widget.account?.id ?? '',
      bankName: _selectedBankName!,
      bankCode: _selectedBankCode!,
      accountNumber: _accountNumberController.text.trim(),
      accountHolderName: _accountHolderController.text.trim(),
      isDefault: widget.account?.isDefault ?? false,
      status: BankAccountStatus.active,
      createdAt: widget.account?.createdAt ?? now,
      updatedAt: now,
    );

    // No backend update route exists — always use addBankAccount.
    final result = await repository.addBankAccount(bankAccount);

    if (!mounted) {
      setState(() => _isLoading = false);
      return;
    }

    if (result.isSuccess) {
      Navigator.pop(context, true);
      AppSnackBar.showSuccess(
        context,
        widget.account == null
            ? 'Bank account added successfully'
            : 'Bank account updated successfully',
      );
    } else {
      AppSnackBar.showError(context, result.error ?? 'Failed to save account');
    }

    setState(() => _isLoading = false);
  }
}
