import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/shared/shared.dart';
import 'package:hishumi/domains/user/profile/domain/entities/bank_account_entity.dart';
import 'package:hishumi/domains/user/profile/presentation/providers/bank_account_provider.dart'
    show bankAccountRepositoryProvider;
import 'package:hishumi/core/src/theme/app_theme.dart';

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
    final scheme = Theme.of(context).colorScheme;
    final isEdit = widget.account != null;

    return Dialog(
      backgroundColor: Colors.transparent,
      insetPadding: const EdgeInsets.all(AppMetrics.p24),
      child: Container(
        constraints: const BoxConstraints(maxWidth: 500),
        decoration: BoxDecoration(
          color: scheme.surfaceContainerHigh,
          borderRadius: BorderRadius.circular(AppShape.r20),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Header
            Container(
              padding: const EdgeInsets.all(AppMetrics.p24),
              decoration: BoxDecoration(
                color: scheme.surfaceContainerHighest,
                borderRadius: const BorderRadius.only(
                  topLeft: Radius.circular(AppShape.r20),
                  topRight: Radius.circular(AppShape.r20),
                ),
              ),
              child: Row(
                children: [
                  Container(
                    padding: const EdgeInsets.all(AppMetrics.p8),
                    decoration: BoxDecoration(
                      color: scheme.primary.withValues(alpha: 0.1),
                      borderRadius: BorderRadius.circular(AppShape.r8),
                    ),
                    child: Icon(
                      Icons.account_balance,
                      color: scheme.primary,
                      size: AppIconSize.header,
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Text(
                      isEdit ? 'Edit Bank Account' : 'Add Bank Account',
                      style: context.typeRoles.titleSection.copyWith(
                        fontWeight: FontWeight.bold,
                        color: scheme.onSurface,
                      ),
                    ),
                  ),
                  IconButton(
                    onPressed: () => Navigator.pop(context),
                    icon: Icon(Icons.close, color: scheme.onSurfaceVariant, semanticLabel: 'Tutup'),
                  ),
                ],
              ),
            ),

            // Form
            Flexible(
              child: SingleChildScrollView(
                padding: const EdgeInsets.all(AppMetrics.p24),
                child: Form(
                  key: _formKey,
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      // Bank selection
                      _buildLabel(context, 'Bank'),
                      const SizedBox(height: 8),
                      _buildBankDropdown(context),
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
              padding: const EdgeInsets.all(AppMetrics.p24),
              decoration: BoxDecoration(
                color: scheme.surfaceContainerHighest,
                borderRadius: const BorderRadius.only(
                  bottomLeft: Radius.circular(AppShape.r20),
                  bottomRight: Radius.circular(AppShape.r20),
                ),
              ),
              child: Row(
                children: [
                  Expanded(
                    child: OutlinedButton(
                      onPressed: () => Navigator.pop(context),
                      child: const Text('Cancel'),
                    ),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: ElevatedButton(
                      onPressed: _isLoading ? null : _handleSubmit,
                      child: _isLoading
                          ? SizedBox(
                              height: 20,
                              width: 20,
                              child: CircularProgressIndicator(
                                strokeWidth: 2,
                                valueColor: AlwaysStoppedAnimation(
                                  Theme.of(context).colorScheme.onPrimary,
                                ),
                              ),
                            )
                          : Text(isEdit ? 'Update' : 'Add Account'),
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

  Widget _buildLabel(BuildContext context, String text) {
    return Text(
      text,
      style: context.typeRoles.bodyDense.copyWith(
        fontWeight: FontWeight.w600,
        color: Theme.of(context).colorScheme.onSurface,
      ),
    );
  }

  Widget _buildBankDropdown(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return DropdownButtonFormField<String>(
      initialValue: _selectedBankCode,
      // Border/fill/geometry come from `inputDecorationTheme` (AppTheme) —
      // the one form-field authority (fill = surfaceContainerHigh).
      decoration: const InputDecoration(hintText: 'Select bank'),
      dropdownColor: scheme.surfaceContainerHigh,
      items: _indonesianBanks.map((bank) {
        return DropdownMenuItem(
          value: bank.code,
          child: Row(
            children: [
              Text(bank.icon, style: context.typeRoles.titleSection),
              const SizedBox(width: 12),
              Expanded(
                child: Text(
                  bank.name,
                  style: TextStyle(color: scheme.onSurface),
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
      AppSnackBar.showError(context, result.error ?? 'Gagal menyimpan rekening');
    }

    setState(() => _isLoading = false);
  }
}
