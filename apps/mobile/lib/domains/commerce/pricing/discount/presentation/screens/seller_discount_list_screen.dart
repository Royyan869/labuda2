import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';
import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/providers/discount_provider.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/screens/create_discount_screen.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/screens/edit_discount_screen.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/widgets/discount_card.dart';
import 'package:labuda/domains/commerce/pricing/discount/presentation/widgets/discount_management_info_tooltip.dart';

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
        appBar: AppBar(title: const Text('Manage Discounts')),
        body: const Center(child: Text('Please login first')),
      );
    }

    final discountsAsync = ref.watch(sellerDiscountsProvider(currentUser.id));

    return Scaffold(
      appBar: AppBar(
        title: const Text('Manage Discounts'),
        actions: const [DiscountManagementInfoTooltip()],
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
                _buildDiscountList(activeDiscounts, 'No active discounts'),
                _buildDiscountList(expiredDiscounts, 'No expired discounts'),
                _buildDiscountList(inactiveDiscounts, 'No inactive discounts'),
              ],
            );
          },
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (error, stack) =>
              const Center(child: Text('Data belum bisa dimuat.')),
        ),
      ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: () {
          Navigator.of(context).push(
            MaterialPageRoute(builder: (_) => const CreateDiscountScreen()),
          );
        },
        icon: const Icon(Icons.add),
        label: const Text('Create Discount'),
      ),
    );
  }

  Widget _buildDiscountList(List<Discount> discounts, String emptyMessage) {
    if (discounts.isEmpty) {
      return Center(
        child: Text(emptyMessage, style: const TextStyle(color: Colors.grey)),
      );
    }

    return ListView.builder(
      padding: const EdgeInsets.all(16),
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
    Navigator.of(context).push(
      MaterialPageRoute(builder: (_) => EditDiscountScreen(discount: discount)),
    );
  }

  Future<void> _handleToggleActive(Discount discount, bool isActive) async {
    // Show confirmation if deactivating used discount
    if (!isActive && discount.currentUsageCount > 0) {
      final confirm = await showDialog<bool>(
        context: context,
        builder: (context) => AlertDialog(
          title: const Text('Deactivate Discount'),
          content: Text(
            'Diskon ini sudah digunakan ${discount.currentUsageCount} kali. '
            'Buyer yang sedang checkout dengan diskon ini tidak akan bisa submit order. '
            'Lanjutkan?',
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context, false),
              child: const Text('Cancel'),
            ),
            ElevatedButton(
              onPressed: () => Navigator.pop(context, true),
              child: const Text('Deactivate'),
            ),
          ],
        ),
      );

      if (confirm != true) return;
    }

    // Create updated discount with toggled isActive status
    final updatedDiscount = Discount(
      id: discount.id,
      code: discount.code,
      description: discount.description,
      type: discount.type,
      value: discount.value,
      minPurchase: discount.minPurchase,
      maxDiscount: discount.maxDiscount,
      maxUsagePerUser: discount.maxUsagePerUser,
      totalUsageLimit: discount.totalUsageLimit,
      appliesTo: discount.appliesTo,
      targetMode: discount.targetMode,
      sellerId: discount.sellerId,
      applicableListingIds: discount.applicableListingIds,
      applicableAuctionIds: discount.applicableAuctionIds,
      validFrom: discount.validFrom,
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
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(error),
            backgroundColor: Colors.red,
            duration: const Duration(seconds: 4),
          ),
        );
      },
      (updatedDiscount) {
        // Show success message
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              isActive ? 'Discount activated' : 'Discount deactivated',
            ),
            duration: const Duration(seconds: 3),
          ),
        );

        // Refresh list
        if (currentUser != null) {
          ref.invalidate(sellerDiscountsProvider(currentUser.id));
        }
      },
    );
  }

  Future<void> _handleDelete(Discount discount) async {
    // Confirm deletion
    final confirm = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Delete Discount'),
        content: Text(
          discount.currentUsageCount > 0
              ? 'This discount has been used ${discount.currentUsageCount} times and cannot be deleted.'
              : 'Are you sure you want to delete discount "${discount.code}"? This action cannot be undone.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Cancel'),
          ),
          if (discount.currentUsageCount == 0)
            ElevatedButton(
              style: ElevatedButton.styleFrom(
                backgroundColor: Colors.red,
                foregroundColor: Colors.white,
              ),
              onPressed: () => Navigator.pop(context, true),
              child: const Text('Delete'),
            ),
        ],
      ),
    );

    if (confirm != true) return;

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
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(error),
            backgroundColor: Colors.red,
            duration: const Duration(seconds: 4),
          ),
        );
      },
      (_) {
        // Show success message
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Discount deleted successfully'),
            duration: Duration(seconds: 3),
          ),
        );

        // Refresh list
        if (currentUser != null) {
          ref.invalidate(sellerDiscountsProvider(currentUser.id));
        }
      },
    );
  }
}
