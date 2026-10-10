/// Seller Performance Screen
///
/// User-facing surface for a seller's trust / reputation and order-execution
/// reliability. It renders ONLY canonical data returned by
/// `GET /api/v1/seller/performance` (rolling 90-day Reputation + canonical
/// Rating). The backend is the sole aggregation authority — this screen never
/// re-aggregates orders, never calculates metrics, and never fabricates
/// fallback business data.
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/user/preference/seller/domain/entities/seller_performance.dart';
import 'package:hishumi/domains/user/preference/seller/seller_di.dart';

class SellerPerformanceScreen extends ConsumerWidget {
  const SellerPerformanceScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final authState = ref.watch(authControllerProvider);

    if (authState is! AuthStateAuthenticated) {
      return const Scaffold(body: Center(child: Text('Login Diperlukan')));
    }

    final sellerId = authState.user.id;
    final performanceAsync = ref.watch(sellerPerformanceProvider(sellerId));

    return Scaffold(
      appBar: AppBar(title: const Text('Performa Penjual')),
      body: RefreshIndicator(
        onRefresh: () async =>
            ref.invalidate(sellerPerformanceProvider(sellerId)),
        child: performanceAsync.when(
          data: (performance) => _PerformanceContent(performance: performance),
          loading: () => const _LoadingState(),
          error: (error, _) => _ErrorState(
            onRetry: () => ref.invalidate(sellerPerformanceProvider(sellerId)),
          ),
        ),
      ),
    );
  }
}

// =============================================================================
// STATES
// =============================================================================

/// Loading state — a real spinner. Never renders fake zero values.
class _LoadingState extends StatelessWidget {
  const _LoadingState();

  @override
  Widget build(BuildContext context) {
    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      children: const [
        SizedBox(height: 240),
        Center(child: CircularProgressIndicator()),
      ],
    );
  }
}

/// Error state — explicit failure with retry. Never silently substitutes
/// fabricated defaults.
class _ErrorState extends StatelessWidget {
  final VoidCallback onRetry;

  const _ErrorState({required this.onRetry});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: const EdgeInsets.all(AppMetrics.p24),
      children: [
        const SizedBox(height: 120),
        Icon(
          Icons.cloud_off_outlined,
          size: AppIconSize.display,
          color: scheme.onSurfaceVariant,
        ),
        const SizedBox(height: AppMetrics.p16),
        Text(
          'Gagal memuat performa',
          style: context.typeRoles.titleCompact.copyWith(
            fontWeight: FontWeight.w600,
          ),
          textAlign: TextAlign.center,
        ),
        const SizedBox(height: AppMetrics.p8),
        Text(
          'Terjadi kesalahan saat mengambil data performa Anda.',
          style: context.typeRoles.bodyDense.copyWith(
            color: scheme.onSurfaceVariant,
          ),
          textAlign: TextAlign.center,
        ),
        const SizedBox(height: AppMetrics.p24),
        Center(
          child: ElevatedButton.icon(
            onPressed: onRetry,
            icon: const Icon(Icons.refresh_outlined, size: AppIconSize.action),
            label: const Text('Coba Lagi'),
          ),
        ),
      ],
    );
  }
}

// =============================================================================
// CONTENT
// =============================================================================

class _PerformanceContent extends StatelessWidget {
  final SellerPerformance performance;

  const _PerformanceContent({required this.performance});

  @override
  Widget build(BuildContext context) {
    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: const EdgeInsets.all(AppMetrics.p16),
      children: [
        const _OverviewHeader(),
        const SizedBox(height: AppMetrics.p16),
        _TierCard(tier: performance.tier),
        const SizedBox(height: AppMetrics.p16),
        _FulfillmentSection(performance: performance),
        const SizedBox(height: AppMetrics.p16),
        _RatingSection(performance: performance),
      ],
    );
  }
}

class _OverviewHeader extends StatelessWidget {
  const _OverviewHeader();

  @override
  Widget build(BuildContext context) {
    return Text(
      'Ringkasan keandalan dan reputasi Anda sebagai penjual di HiShumi.',
      style: context.typeRoles.bodyDense.copyWith(
        color: Theme.of(context).colorScheme.onSurfaceVariant,
      ),
    );
  }
}

// =============================================================================
// TIER
// =============================================================================

class _TierCard extends StatelessWidget {
  final String tier;

  const _TierCard({required this.tier});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final presentation = _TierPresentation.fromWire(tier, scheme);
    return _SectionCard(
      child: Row(
        children: [
          Container(
            padding: const EdgeInsets.all(AppMetrics.p12),
            decoration: BoxDecoration(
              color: presentation.color.withValues(alpha: 0.12),
              shape: BoxShape.circle,
            ),
            child: Icon(
              presentation.icon,
              size: AppIconSize.header,
              color: presentation.color,
            ),
          ),
          const SizedBox(width: AppMetrics.p16),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Tingkat Penjual',
                  style: context.typeRoles.labelMicro.copyWith(
                    color: scheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: AppMetrics.p4),
                Text(
                  presentation.label,
                  style: context.typeRoles.titleSection.copyWith(
                    fontWeight: FontWeight.bold,
                    color: scheme.onSurface,
                  ),
                ),
                const SizedBox(height: AppMetrics.p4),
                Text(
                  presentation.description,
                  style: context.typeRoles.labelMicro.copyWith(
                    color: scheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// Presentation metadata for a wire tier value. This is a pure display mapping
/// of the canonical provider value — no tier calculation happens here. Colors
/// resolve from the theme scheme for light/dark adaptation.
class _TierPresentation {
  final String label;
  final String description;
  final IconData icon;
  final Color color;

  const _TierPresentation({
    required this.label,
    required this.description,
    required this.icon,
    required this.color,
  });

  factory _TierPresentation.fromWire(String tier, ColorScheme scheme) {
    switch (tier) {
      case 'elite':
        return _TierPresentation(
          label: 'Penjual Elite',
          description: 'Performa terbaik berdasarkan penilaian pembeli.',
          icon: Icons.workspace_premium_rounded,
          color: scheme.secondary,
        );
      case 'pro':
        return _TierPresentation(
          label: 'Penjual Pro',
          description: 'Penjual terpercaya dengan rekam jejak yang kuat.',
          icon: Icons.star_rounded,
          color: AppColors.primaryYellow,
        );
      case 'basic':
        return _TierPresentation(
          label: 'Penjual Dasar',
          description: 'Tingkat awal — tingkatkan dengan pesanan yang selesai.',
          icon: Icons.verified_outlined,
          color: scheme.onSurfaceVariant,
        );
      default:
        // Unknown wire value: show it verbatim rather than fabricating a tier.
        return _TierPresentation(
          label: tier.isEmpty ? '—' : tier,
          description: 'Tingkat penjual saat ini.',
          icon: Icons.verified_outlined,
          color: scheme.onSurfaceVariant,
        );
    }
  }
}

// =============================================================================
// FULFILLMENT
// =============================================================================

class _FulfillmentSection extends StatelessWidget {
  final SellerPerformance performance;

  const _FulfillmentSection({required this.performance});

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _SectionTitle(
          title: 'Keandalan Pesanan',
          subtitle: 'Data 90 hari terakhir',
        ),
        const SizedBox(height: AppMetrics.p12),
        _SectionCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                'Tingkat Pemenuhan',
                style: context.typeRoles.labelMicro.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
              ),
              const SizedBox(height: AppMetrics.p4),
              Text(
                _formatRate(performance.fulfillmentRate),
                style: context.typeRoles.titleProminent.copyWith(
                  fontWeight: FontWeight.bold,
                  color: context.statusColors.success,
                ),
              ),
              const SizedBox(height: AppMetrics.p12),
              ClipRRect(
                borderRadius: BorderRadius.circular(AppShape.r8),
                child: LinearProgressIndicator(
                  value: performance.fulfillmentRate.clamp(0.0, 1.0),
                  minHeight: 8,
                  backgroundColor: Theme.of(
                    context,
                  ).colorScheme.surfaceContainerHighest,
                  valueColor: AlwaysStoppedAnimation<Color>(
                    context.statusColors.success,
                  ),
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: AppMetrics.p12),
        Row(
          children: [
            Expanded(
              child: _MetricCard(
                label: 'Pesanan Selesai',
                value: '${performance.completedOrders}',
                icon: Icons.check_circle_outline,
                color: context.statusColors.success,
              ),
            ),
            const SizedBox(width: AppMetrics.p12),
            Expanded(
              child: _MetricCard(
                label: 'Tidak Dikirim',
                value: '${performance.cancelledTimeout}',
                icon: Icons.local_shipping_outlined,
                color: context.statusColors.warning,
              ),
            ),
          ],
        ),
        const SizedBox(height: AppMetrics.p8),
        Text(
          'Tidak Dikirim = pesanan yang dibatalkan karena melewati batas waktu pengiriman.',
          style: context.typeRoles.labelMicro.copyWith(
            color: Theme.of(context).colorScheme.onSurfaceVariant,
          ),
        ),
      ],
    );
  }
}

// =============================================================================
// RATING
// =============================================================================

class _RatingSection extends StatelessWidget {
  final SellerPerformance performance;

  const _RatingSection({required this.performance});

  @override
  Widget build(BuildContext context) {
    final reviewCount = performance.reviewCount;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _SectionTitle(
          title: 'Penilaian Pembeli',
          subtitle: reviewCount > 0 ? '$reviewCount ulasan' : 'Belum ada ulasan',
        ),
        const SizedBox(height: AppMetrics.p12),
        _SectionCard(
          child: reviewCount == 0
              ? _emptyRating(context)
              : _ratingSummary(context),
        ),
      ],
    );
  }

  Widget _emptyRating(BuildContext context) {
    return Text(
      'Belum ada ulasan dari pembeli.',
      style: context.typeRoles.bodyDense.copyWith(
        color: Theme.of(context).colorScheme.onSurfaceVariant,
      ),
    );
  }

  Widget _ratingSummary(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          crossAxisAlignment: CrossAxisAlignment.center,
          children: [
            Text(
              performance.averageRating.toStringAsFixed(1),
              style: context.typeRoles.titleProminent.copyWith(
                fontWeight: FontWeight.bold,
              ),
            ),
            const SizedBox(width: AppMetrics.p12),
            _StarRow(rating: performance.averageRating),
          ],
        ),
        const SizedBox(height: AppMetrics.p4),
        Text(
          'Dari ${performance.reviewCount} ulasan',
          style: context.typeRoles.labelMicro.copyWith(
            color: scheme.onSurfaceVariant,
          ),
        ),
        const SizedBox(height: AppMetrics.p16),
        ..._distributionRows(context),
      ],
    );
  }

  List<Widget> _distributionRows(BuildContext context) {
    // 5★ → 1★, each bar derived safely from the supplied counts.
    final counts = [
      performance.fiveStarCount,
      performance.fourStarCount,
      performance.threeStarCount,
      performance.twoStarCount,
      performance.oneStarCount,
    ];
    final total = counts.fold<int>(0, (sum, c) => sum + c);
    final rows = <Widget>[];
    for (var i = 0; i < counts.length; i++) {
      final stars = 5 - i;
      final count = counts[i];
      final fraction = total > 0 ? count / total : 0.0;
      rows.add(
        _DistributionRow(
          stars: stars,
          count: count,
          fraction: fraction,
        ),
      );
      if (i != counts.length - 1) {
        rows.add(const SizedBox(height: AppMetrics.p8));
      }
    }
    return rows;
  }
}

class _StarRow extends StatelessWidget {
  final double rating;

  const _StarRow({required this.rating});

  @override
  Widget build(BuildContext context) {
    final filled = rating.round();
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: List.generate(5, (index) {
        final position = index + 1;
        final isFilled = position <= filled;
        return Icon(
          isFilled ? Icons.star_rounded : Icons.star_border_rounded,
          size: AppIconSize.action,
          color: isFilled
              ? AppColors.primaryYellow
              : Theme.of(context).colorScheme.onSurfaceVariant,
        );
      }),
    );
  }
}

class _DistributionRow extends StatelessWidget {
  final int stars;
  final int count;
  final double fraction;

  const _DistributionRow({
    required this.stars,
    required this.count,
    required this.fraction,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Row(
      children: [
        SizedBox(
          width: 40,
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                '$stars',
                style: context.typeRoles.labelMicro.copyWith(
                  color: scheme.onSurfaceVariant,
                ),
              ),
              const SizedBox(width: 2),
              Icon(
                Icons.star_rounded,
                size: AppIconSize.inlineGlyph,
                color: AppColors.primaryYellow,
              ),
            ],
          ),
        ),
        const SizedBox(width: AppMetrics.p8),
        Expanded(
          child: ClipRRect(
            borderRadius: BorderRadius.circular(AppShape.r8),
            child: LinearProgressIndicator(
              value: fraction,
              minHeight: 8,
              backgroundColor: scheme.surfaceContainerHighest,
              valueColor: AlwaysStoppedAnimation<Color>(scheme.secondary),
            ),
          ),
        ),
        const SizedBox(width: AppMetrics.p8),
        SizedBox(
          width: 32,
          child: Text(
            '$count',
            textAlign: TextAlign.end,
            style: context.typeRoles.labelMicro.copyWith(
              color: scheme.onSurfaceVariant,
            ),
          ),
        ),
      ],
    );
  }
}

// =============================================================================
// SHARED PRIMITIVES
// =============================================================================

class _SectionTitle extends StatelessWidget {
  final String title;
  final String? subtitle;

  const _SectionTitle({required this.title, this.subtitle});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Row(
      crossAxisAlignment: CrossAxisAlignment.end,
      children: [
        Expanded(
          child: Text(
            title,
            style: context.typeRoles.titleSection.copyWith(
              fontWeight: FontWeight.bold,
            ),
          ),
        ),
        if (subtitle != null)
          Text(
            subtitle!,
            style: context.typeRoles.labelMicro.copyWith(
              color: scheme.onSurfaceVariant,
            ),
          ),
      ],
    );
  }
}

class _SectionCard extends StatelessWidget {
  final Widget child;

  const _SectionCard({required this.child});

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: Theme.of(context).colorScheme.outlineVariant),
      ),
      child: child,
    );
  }
}

class _MetricCard extends StatelessWidget {
  final String label;
  final String value;
  final IconData icon;
  final Color color;

  const _MetricCard({
    required this.label,
    required this.value,
    required this.icon,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    return _SectionCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(icon, size: AppIconSize.inlineGlyph, color: color),
              const SizedBox(width: AppMetrics.p4),
              Expanded(
                child: Text(
                  label,
                  style: context.typeRoles.labelMicro.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
            ],
          ),
          const SizedBox(height: AppMetrics.p8),
          Text(
            value,
            style: context.typeRoles.titleProminent.copyWith(
              fontWeight: FontWeight.bold,
            ),
          ),
        ],
      ),
    );
  }
}

/// Formats a 0.0–1.0 rate as a percentage string.
String _formatRate(double rate) => '${(rate * 100).toStringAsFixed(1)}%';
