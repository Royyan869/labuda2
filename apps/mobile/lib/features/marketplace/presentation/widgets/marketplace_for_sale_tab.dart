import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/for_sale.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/widgets/for_sale_card.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';
import 'package:labuda/shared/widgets/empty_state.dart';

/// For Sale tab content for Marketplace screen.
///
/// CANONICAL LAYOUT (owner-locked): public marketplace surfaces render through
/// the shared `CommerceMarketplaceGrid` (2 columns) with the shared empty /
/// error vocabulary. A surface may not own its own list or state widgets.
class MarketplaceForSaleTab extends ConsumerWidget {
  const MarketplaceForSaleTab({super.key});

  static const ForSalesParams _params = ForSalesParams(
    status: ForSaleStatus.active,
    limit: 50,
  );

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final forSalesAsync = ref.watch(forSalesProvider(_params));
    final forSales = forSalesAsync.asData?.value ?? const <ForSale>[];

    return RefreshIndicator(
      onRefresh: () => ref.refresh(forSalesProvider(_params).future),
      child: CustomScrollView(
        physics: const AlwaysScrollableScrollPhysics(),
        slivers: [
          CommerceMarketplaceGrid(
            itemCount: forSales.length,
            isLoading: forSalesAsync.isLoading,
            error: forSalesAsync.hasError ? forSalesAsync.error : null,
            itemBuilder: (context, index) {
              final forSale = forSales[index];
              return ForSaleCard(
                forSale: forSale,
                onTap: () => context.push('/for-sale/${forSale.forSaleId}'),
              );
            },
            emptyBuilder: (context) => const EmptyState(
              icon: Icons.storefront_outlined,
              title: 'Belum ada for sale',
              subtitle: 'Cek lagi nanti ya!',
            ),
            errorBuilder: (context, error, stackTrace) => EmptyState.error(
              title: 'Data belum bisa dimuat.',
              subtitle: 'Periksa koneksi kamu lalu coba lagi.',
              onRetry: () => ref.invalidate(forSalesProvider(_params)),
            ),
          ),
        ],
      ),
    );
  }
}
