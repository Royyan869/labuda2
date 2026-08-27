import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/bank_account_card_widget.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/bank_account_empty_state_widget.dart';
import 'package:labuda/domains/user/profile/presentation/widgets/add_edit_bank_account_dialog.dart';
import 'package:labuda/domains/user/profile/domain/entities/bank_account_entity.dart';
import 'package:labuda/domains/user/profile/presentation/providers/bank_account_provider.dart';

/// Bank Account Management Screen untuk seller payment settings
///
/// Features:
/// - Bank account registration untuk seller payouts
/// - Multiple bank account support
/// - Account verification status
/// - Bank account validation
/// - Secure account information handling
/// - Integration dengan payment flow
class BankAccountScreen extends ConsumerStatefulWidget {
  const BankAccountScreen({super.key});

  @override
  ConsumerState<BankAccountScreen> createState() => _BankAccountScreenState();
}

class _BankAccountScreenState extends ConsumerState<BankAccountScreen> {
  @override
  void initState() {
    super.initState();
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final authState = ref.watch(authControllerProvider);

    // Get current user ID
    if (authState is! AuthStateAuthenticated) {
      return const Scaffold(
        appBar: AppBarCustom(title: 'Bank Accounts'),
        body: Center(child: Text('Please login to continue')),
      );
    }

    final userId = authState.user.id;

    // Watch bank accounts stream
    final bankAccountsAsync = ref.watch(bankAccountsStreamProvider(userId));

    return Scaffold(
      appBar: const AppBarCustom(title: 'Bank Accounts'),
      body: bankAccountsAsync.when(
        data: (result) {
          if (result.isError) {
            return Center(
              child: Text(
                result.error ?? 'Failed to load bank accounts',
                style: TextStyle(
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
              ),
            );
          }

          final bankAccounts = result.data ?? [];
          return _buildContent(context, isDark, bankAccounts, userId);
        },
        loading: () => const Center(child: LoadingIndicator()),
        error: (error, stack) => Center(
          child: Text(
            'Error: $error',
            style: TextStyle(
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildContent(
    BuildContext context,
    bool isDark,
    List<BankAccountEntity> bankAccounts,
    String userId,
  ) {
    return SafeArea(
      child: Column(
        children: [
          // Existing accounts list
          if (bankAccounts.isNotEmpty) ...[
            Expanded(
              child: ListView(
                padding: const EdgeInsets.all(24),
                children: [
                  _buildSectionHeader('Your Bank Accounts'),
                  const SizedBox(height: 16),
                  ...bankAccounts.map(
                    (account) => BankAccountCardWidget(
                      account: account,
                      onEdit: () => _editAccount(account, userId),
                      onDelete: () =>
                          _deleteAccount(account, bankAccounts.length),
                      onSetPrimary: () => _setPrimaryAccount(account, userId),
                      isDark: isDark,
                    ),
                  ),
                  const SizedBox(height: 16),
                  _buildAddAccountButton(isDark, userId),
                ],
              ),
            ),
          ] else ...[
            // Empty state
            Expanded(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: BankAccountEmptyStateWidget(
                  onAddAccount: () => _showAddAccountDialog(userId),
                  isDark: isDark,
                ),
              ),
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildSectionHeader(String title) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Text(
      title,
      style: TextStyle(
        fontSize: 18,
        fontWeight: FontWeight.bold,
        color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
      ),
    );
  }

  Widget _buildAddAccountButton(bool isDark, String userId) {
    return SizedBox(
      width: double.infinity,
      child: AppButton.secondary(
        text: 'Add Another Account',
        onPressed: () => _showAddAccountDialog(userId),
      ),
    );
  }

  void _showAddAccountDialog(
    String userId, [
    BankAccountEntity? account,
  ]) async {
    await showDialog<bool>(
      context: context,
      builder: (context) =>
          AddEditBankAccountDialog(userId: userId, account: account),
    );
  }

  void _editAccount(BankAccountEntity account, String userId) {
    _showAddAccountDialog(userId, account);
  }

  void _deleteAccount(BankAccountEntity account, int totalAccounts) async {
    // Show confirmation dialog
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Delete Bank Account'),
        content: Text(
          'Are you sure you want to delete this bank account?\n\n${account.bankName} - ${account.accountNumber}',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(context, true),
            style: TextButton.styleFrom(foregroundColor: AppColors.error),
            child: const Text('Delete'),
          ),
        ],
      ),
    );

    if (confirmed != true || !mounted) return;

    // Perform delete
    final repository = ref.read(bankAccountRepositoryProvider);
    final result = await repository.deleteBankAccount(account.id);

    if (!mounted) return;

    if (result.isSuccess) {
      AppSnackBar.showSuccess(context, 'Bank account deleted successfully');
    } else {
      AppSnackBar.showError(
        context,
        result.error ?? 'Failed to delete account',
      );
    }
  }

  void _setPrimaryAccount(BankAccountEntity account, String userId) async {
    if (account.isDefault) {
      AppSnackBar.showInfo(context, 'This account is already set as primary');
      return;
    }

    final repository = ref.read(bankAccountRepositoryProvider);
    final result = await repository.setPrimaryBankAccount(account.id, userId);

    if (!mounted) return;

    if (result.isSuccess) {
      AppSnackBar.showSuccess(context, 'Primary account updated successfully');
    } else {
      AppSnackBar.showError(
        context,
        result.error ?? 'Failed to set primary account',
      );
    }
  }
}
