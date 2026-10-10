import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/widgets/app_dialog.dart';
import 'package:hishumi/shared/widgets/app_snackbar.dart';
import 'package:hishumi/shared/widgets/empty_state.dart';
import 'package:hishumi/shared/widgets/page_error_state.dart';
import 'package:hishumi/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';
import 'package:hishumi/domains/commerce/pricing/discount/presentation/providers/discount_provider.dart';
import 'package:hishumi/domains/commerce/pricing/discount/presentation/widgets/discount_card.dart';

/// Screen untuk list semua discount milik seller
class SellerDiscountListScreen extends ConsumerStatefulWidget {
  const SellerDiscountListScreen({super.key});

  @override
  ConsumerState<SellerDiscountListScreen> createState() =>
      _SellerDiscountListScreenState();
}

class _SellerDiscountListScreenState
    extends ConsumerState<SellerDiscountListScreen>
    with SingleTickerProviderStateMixin {
  late TabController _tabController;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 3, vsync: this);
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authControllerProvider);
    final currentUser = authState is AuthStateAuthenticated
        ? authState.user
        : null;

    if (currentUser == null) {
      return Scaffold(
        appBar: AppBar(title: const Text('Kelola Diskon')),
        body: const Center(child: Text('Please login first')),
      );
    }

    final discountsAsync = ref.watch(sellerDiscountsProvider(currentUser.id));

    return Scaffold(
      appBar: AppBar(
        title: const Text('Kelola Diskon'),
        // Canonical Page Info trigger: the page owns the action, the
        // information surface is the canonical AppDialog.info.
        actions: [
          IconButton(
            icon: const Icon(Icons.help_outline),
            tooltip: 'Panduan Kelola Diskon',
            onPressed: _showDiscountInfo,
          ),
        ],
        bottom: TabBar(
          controller: _tabController,
          tabs: const [
            Tab(text: 'Active'),
            Tab(text: 'Expired'),
            Tab(text: 'Inactive'),
          ],
        ),
      ),
      body: SafeArea(
        child: discountsAsync.when(
          data: (discounts) {
            final activeDiscounts = discounts
                .where((d) => d.isActive && !d.isExpired)
                .toList();
            final expiredDiscounts = discounts
                .where((d) => d.isExpired)
                .toList();
            final inactiveDiscounts = discounts
                .where((d) => !d.isActive && !d.isExpired)
                .toList();

            return TabBarView(
              controller: _tabController,
              children: [
                _buildDiscountList(
                  context,
                  activeDiscounts,
                  'No active discounts',
                ),
                _buildDiscountList(
                  context,
                  expiredDiscounts,
                  'No expired discounts',
                ),
                _buildDiscountList(
                  context,
                  inactiveDiscounts,
                  'No inactive discounts',
                ),
              ],
            );
          },
          loading: () => const Center(child: CircularProgressIndicator()),
          // CANONICAL page-level error (PageErrorState): safe localized copy
          // only. This screen never offered a retry action for the load
          // failure, so no onRetry is passed — behaviour unchanged.
          error: (error, stack) => const PageErrorState(),
        ),
      ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: () => context.push(RoutePaths.sellerDiscountCreate),
        icon: const Icon(Icons.add),
        label: const Text('Create Discount'),
      ),
    );
  }

  Widget _buildDiscountList(
    BuildContext context,
    List<Discount> discounts,
    String emptyMessage,
  ) {
    if (discounts.isEmpty) {
      // Canonical empty state (shared EmptyState) — icon + title + hint,
      // not naked text like the old copy.
      return EmptyState(
        type: EmptyStateType.noData,
        icon: Icons.discount_outlined,
        title: emptyMessage,
        subtitle: 'Diskon akan muncul di sini.',
      );
    }

    return ListView.builder(
      padding: const EdgeInsets.all(AppMetrics.p16),
      itemCount: discounts.length,
      itemBuilder: (context, index) {
        final discount = discounts[index];
        return DiscountCard(
          discount: discount,
          onTap: () {
            // TODO: Navigate to detail screen
          },
          onEdit: () => _handleEdit(discount),
          onToggleActive: (isActive) => _handleToggleActive(discount, isActive),
          onDelete: () => _handleDelete(discount),
        );
      },
    );
  }

  void _handleEdit(Discount discount) {
    // Canonical edit route: the path identifies the discount, the loaded
    // entity travels as extra (owner-only management form).
    context.push(RoutePaths.sellerDiscountEditPath(discount.id), extra: discount);
  }

  Future<void> _handleToggleActive(Discount discount, bool isActive) async {
    // Show confirmation if deactivating used discount
    if (!isActive && discount.currentUsageCount > 0) {
      final confirm = await AppDialog.confirm(
        context: context,
        title: 'Deactivate Discount',
        message:
            'Diskon ini sudah digunakan ${discount.currentUsageCount} kali. '
            'Buyer yang sedang checkout dengan diskon ini tidak akan bisa submit order. '
            'Lanjutkan?',
        confirmLabel: 'Deactivate',
        cancelLabel: 'Cancel',
      );

      if (!confirm) return;
    }

    // Create updated discount with toggled isActive status
    final updatedDiscount = Discount(
      id: discount.id,
      code: discount.code,
      description: discount.description,
      type: discount.type,
      value: discount.value,
      minPurchase: discount.minPurchase,
      totalUsageLimit: discount.totalUsageLimit,
      appliesTo: discount.appliesTo,
      sellerId: discount.sellerId,
      validUntil: discount.validUntil,
      isActive: isActive, // Toggle status
      currentUsageCount: discount.currentUsageCount,
      createdAt: discount.createdAt,
      createdBy: discount.createdBy,
    );

    final authState = ref.read(authControllerProvider);
    final currentUser = authState is AuthStateAuthenticated
        ? authState.user
        : null;

    // Call update use case
    final updateUseCase = ref.read(updateDiscountUseCaseProvider);
    final result = await updateUseCase(updatedDiscount);

    if (!mounted) return;

    result.fold(
      (error) {
        // Show error message
        AppSnackBar.showError(
          context,
          error,
          duration: const Duration(seconds: 4),
        );
      },
      (updatedDiscount) {
        // Show success message
        AppSnackBar.showSuccess(
          context,
          isActive ? 'Diskon diaktifkan' : 'Diskon dinonaktifkan',
        );

        // Refresh list
        if (currentUser != null) {
          ref.invalidate(sellerDiscountsProvider(currentUser.id));
        }
      },
    );
  }

  Future<void> _handleDelete(Discount discount) async {
    // A used discount can never be deleted: the dialog is a notice, not a
    // decision. An unused one is a destructive confirmation.
    if (discount.currentUsageCount > 0) {
      await AppDialog.info(
        context: context,
        title: 'Delete Discount',
        message:
            'This discount has been used ${discount.currentUsageCount} times and cannot be deleted.',
        closeLabel: 'Cancel',
      );
      return;
    }

    final confirm = await AppDialog.confirm(
      context: context,
      title: 'Delete Discount',
      message:
          'Are you sure you want to delete discount "${discount.code}"? This action cannot be undone.',
      confirmLabel: 'Delete',
      cancelLabel: 'Cancel',
      intent: AppDialogIntent.destructive,
    );

    if (!confirm) return;

    final authState = ref.read(authControllerProvider);
    final currentUser = authState is AuthStateAuthenticated
        ? authState.user
        : null;

    // Call delete use case
    final deleteUseCase = ref.read(deleteDiscountUseCaseProvider);
    final result = await deleteUseCase(discount.id);

    if (!mounted) return;

    result.fold(
      (error) {
        // Show error message
        AppSnackBar.showError(
          context,
          error,
          duration: const Duration(seconds: 4),
        );
      },
      (_) {
        // Show success message
        AppSnackBar.showSuccess(context, 'Diskon dihapus');

        // Refresh list
        if (currentUser != null) {
          ref.invalidate(sellerDiscountsProvider(currentUser.id));
        }
      },
    );
  }

  /// Canonical Page Info / Help entry for this page.
  ///
  /// The surface is [AppDialog.info]; the page owns only the trigger and the
  /// information content (no local dialog authority).
  void _showDiscountInfo() {
    AppDialog.info(
      context: context,
      title: 'Discount Management Guide',
      content: _buildDiscountInfoContent(context),
      closeLabel: 'Got It',
    );
  }

  Widget _buildDiscountInfoContent(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        _buildDiscountFeatureSection(
          context: context,
          icon: Icons.edit_outlined,
          iconColor: Theme.of(context).colorScheme.onSurfaceVariant,
          title: 'Edit',
          description: 'Change discount information that has been created.',
          rules: [
            '• Discount never used: All fields can be changed',
            '• Discount used: Only some fields can be changed (status, description, extend period, add limit)',
          ],
        ),
        const SizedBox(height: 16),
        _buildDiscountFeatureSection(
          context: context,
          icon: Icons.visibility_off_outlined,
          iconColor: Theme.of(context).colorScheme.onSurfaceVariant,
          title: 'Deactivate / Activate',
          description: 'Change active/inactive status of discount.',
          rules: [
            '• Activate: Discount can be used by buyers',
            '• Deactivate: Buyers cannot use this discount',
            '• If discount has been used, warning will appear when deactivating',
          ],
        ),
        const SizedBox(height: 16),
        _buildDiscountFeatureSection(
          context: context,
          icon: Icons.delete_outline,
          iconColor: context.statusColors.error,
          title: 'Hapus',
          description: 'Permanently delete discount from system.',
          rules: [
            '• Can only delete discounts that have never been used',
            '• Used discounts cannot be deleted (for audit trail)',
            '• This action cannot be undone',
          ],
        ),
        const SizedBox(height: 16),
        Container(
          padding: const EdgeInsets.all(AppMetrics.p12),
          decoration: BoxDecoration(
            color: context.statusColors.warning.withValues(alpha: 0.1),
            border: Border.all(color: context.statusColors.warning),
            borderRadius: BorderRadius.circular(AppShape.r8),
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Icon(
                Icons.lightbulb_outline,
                color: context.statusColors.warning,
                size: AppIconSize.action,
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  'Tip: Use "Deactivate" to stop discount temporarily, and "Delete" to clean up incorrectly input or testing discounts.',
                  style: context.typeRoles.bodyDense.copyWith(
                    color: context.statusColors.warning,
                  ),
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildDiscountFeatureSection({
    required BuildContext context,
    required IconData icon,
    required Color iconColor,
    required String title,
    required String description,
    required List<String> rules,
  }) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Icon(icon, color: iconColor, size: AppIconSize.action),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                title,
                style: context.typeRoles.titleSection.copyWith(
                  fontWeight: FontWeight.bold,
                ),
                softWrap: true,
              ),
            ),
          ],
        ),
        const SizedBox(height: 8),
        Text(
          description,
          style: context.typeRoles.bodyDense.copyWith(
            color: Theme.of(context).colorScheme.onSurfaceVariant,
          ),
          softWrap: true,
        ),
        const SizedBox(height: 8),
        ...rules.map(
          (rule) => Padding(
            padding: const EdgeInsets.only(
              left: AppMetrics.p8,
              top: AppMetrics.p4,
            ),
            child: Text(
              rule,
              style: context.typeRoles.bodyDense.copyWith(
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
              softWrap: true,
            ),
          ),
        ),
      ],
    );
  }
}
