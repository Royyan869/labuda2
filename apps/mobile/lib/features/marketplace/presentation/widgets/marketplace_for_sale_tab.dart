import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/for_sale.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/widgets/for_sale_card.dart';

/// For Sale tab content for Marketplace screen.
class MarketplaceForSaleTab extends ConsumerWidget {
  const MarketplaceForSaleTab({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final forSalesAsync = ref.watch(
      forSalesProvider(
        const ForSalesParams(status: ForSaleStatus.active, limit: 50),
      ),
    );

    return forSalesAsync.when(
      data: (forSales) {
        return RefreshIndicator(
          onRefresh: () async {
            await ref.read(
              forSalesProvider(
                const ForSalesParams(status: ForSaleStatus.active, limit: 50),
              ).future,
            );
          },
          child: CustomScrollView(
            physics: const AlwaysScrollableScrollPhysics(),
            slivers: [
              if (forSales.isEmpty)
                SliverFillRemaining(child: _buildEmptyState(context))
              else
                SliverPadding(
                  padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
                  sliver: SliverList(
                    delegate: SliverChildBuilderDelegate((context, index) {
                      final forSale = forSales[index];
                      return ForSaleCard(
                        forSale: forSale,
                        onTap: () => _navigateToForSaleDetail(context, forSale),
                      );
                    }, childCount: forSales.length),
                  ),
                ),
            ],
          ),
        );
      },
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, _) => const Center(child: Text('Data belum bisa dimuat.')),
    );
  }

  void _navigateToForSaleDetail(BuildContext context, ForSale forSale) {
    context.push('/for-sale/${forSale.forSaleId}');
  }

  Widget _buildEmptyState(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24.0),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.storefront_outlined,
              size: 64,
              color: isDark ? AppColors.darkGray600 : AppColors.neutralGray400,
            ),
            const SizedBox(height: 16),
            Text(
              'Belum ada koi untuk dijual',
              style: AppTypography.h6.copyWith(
                color: isDark
                    ? AppColors.neutralGray300
                    : AppColors.neutralGray700,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              'Cek lagi nanti ya!',
              style: AppTypography.bodyMedium.copyWith(
                color: isDark
                    ? AppColors.neutralGray500
                    : AppColors.neutralGray500,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
