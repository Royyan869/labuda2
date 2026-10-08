/// Seller Analytics Screen
///
/// 30-day read projection over canonical Product View + sale/auction data.
/// The backend is the sole aggregation authority; this screen only renders the
/// returned numbers (no client-side re-aggregation).
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_analytics_read.dart';
import 'package:labuda/domains/user/preference/seller/seller_di.dart';

class SellerAnalyticsScreen extends ConsumerWidget {
  const SellerAnalyticsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final authState = ref.watch(authControllerProvider);

    if (authState is! AuthStateAuthenticated) {
      return const Scaffold(body: Center(child: Text('Login Diperlukan')));
    }

    final sellerId = authState.user.id;
    final analyticsAsync = ref.watch(sellerAnalyticsProvider(sellerId));

    return Scaffold(
      appBar: AppBar(title: const Text('Analitik Penjual')),
      body: RefreshIndicator(
        onRefresh: () async => ref.invalidate(
          sellerAnalyticsProvider(sellerId),
        ),
        child: analyticsAsync.when(
          data: (analytics) => _buildContent(context, analytics),
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (e, _) => _buildError(context, e.toString()),
        ),
      ),
    );
  }

  Widget _buildContent(BuildContext context, SellerAnalytics analytics) {
    return ListView(
      padding: const EdgeInsets.all(AppMetrics.p16),
      children: [
        _SummarySection(summary: analytics.summary),
        const SizedBox(height: AppMetrics.p24),
        Text(
          'Produk',
          style: context.typeRoles.titleSection.copyWith(
            fontWeight: FontWeight.bold,
          ),
        ),
        const SizedBox(height: AppMetrics.p12),
        if (analytics.products.isEmpty)
          _buildEmpty(context)
        else
          ...analytics.products.map((p) => _ProductRow(product: p)),
      ],
    );
  }

  Widget _buildEmpty(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surfaceContainerLow,
        borderRadius: BorderRadius.circular(AppShape.r12),
      ),
      child: Text(
        'Belum ada produk.',
        style: context.typeRoles.bodyDense,
      ),
    );
  }

  Widget _buildError(BuildContext context, String message) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(AppMetrics.p24),
        child: Text(
          'Gagal memuat analitik: $message',
          style: context.typeRoles.bodyDense,
          textAlign: TextAlign.center,
        ),
      ),
    );
  }
}

class _SummarySection extends StatelessWidget {
  final SellerAnalyticsSummary summary;

  const _SummarySection({required this.summary});

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: _StatCard(label: 'Views', value: summary.totalViews30d),
        ),
        const SizedBox(width: AppMetrics.p12),
        Expanded(
          child: _StatCard(
            label: 'Produk Dilihat',
            value: summary.productsWithViews30d,
          ),
        ),
        const SizedBox(width: AppMetrics.p12),
        Expanded(
          child: _StatCard(
            label: 'Produk Terjual',
            value: summary.productsSold30d,
          ),
        ),
      ],
    );
  }
}

class _StatCard extends StatelessWidget {
  final String label;
  final int value;

  const _StatCard({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: Theme.of(context).colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            label,
            style: context.typeRoles.labelMicro.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
          const SizedBox(height: AppMetrics.p8),
          Text(
            '$value',
            style: context.typeRoles.titleProminent.copyWith(
              fontWeight: FontWeight.bold,
            ),
          ),
        ],
      ),
    );
  }
}

class _ProductRow extends StatelessWidget {
  final SellerAnalyticsProduct product;

  const _ProductRow({required this.product});

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: AppMetrics.p8),
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: Theme.of(context).colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  product.title,
                  style: context.typeRoles.bodyDense.copyWith(
                    fontWeight: FontWeight.w600,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              const SizedBox(width: AppMetrics.p8),
              _SoldBadge(sold: product.sold),
            ],
          ),
          const SizedBox(height: AppMetrics.p4),
          Text(
            '${product.isAuction ? 'Lelang' : 'For Sale'} · ${_stateLabel(product.state)}',
            style: context.typeRoles.labelMicro.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: AppMetrics.p8),
          Row(
            children: [
              _MetricChip(
                icon: Icons.visibility_outlined,
                label: '${product.views30d} views',
              ),
              if (product.isAuction) ...[
                const SizedBox(width: AppMetrics.p12),
                _MetricChip(
                  icon: Icons.gavel_outlined,
                  label: '${product.bidCount30d} bid',
                ),
              ],
            ],
          ),
        ],
      ),
    );
  }

  String _stateLabel(String state) {
    switch (state) {
      case 'active':
        return 'Aktif';
      case 'sold':
        return 'Terjual';
      case 'withdrawn':
        return 'Ditarik';
      case 'scheduled':
        return 'Terjadwal';
      case 'waiting_settlement':
        return 'Menunggu Penyelesaian';
      case 'ended':
        return 'Selesai';
      case 'cancelled':
        return 'Dibatalkan';
      case 'lapsed':
        return 'Kedaluwarsa';
      default:
        return state;
    }
  }
}

class _SoldBadge extends StatelessWidget {
  final bool sold;

  const _SoldBadge({required this.sold});

  @override
  Widget build(BuildContext context) {
    final color = sold
        ? context.statusColors.success
        : context.statusColors.warning;
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p8,
        vertical: AppMetrics.p4,
      ),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(AppShape.r8),
      ),
      child: Text(
        sold ? 'Terjual' : 'Belum terjual',
        style: context.typeRoles.labelMicro.copyWith(
          color: color,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }
}

class _MetricChip extends StatelessWidget {
  final IconData icon;
  final String label;

  const _MetricChip({required this.icon, required this.label});

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Icon(icon, size: AppIconSize.inlineGlyph, color: Theme.of(context).colorScheme.onSurfaceVariant),
        const SizedBox(width: AppMetrics.p4),
        Text(
          label,
          style: context.typeRoles.labelMicro.copyWith(
            color: Theme.of(context).colorScheme.onSurfaceVariant,
          ),
        ),
      ],
    );
  }
}
