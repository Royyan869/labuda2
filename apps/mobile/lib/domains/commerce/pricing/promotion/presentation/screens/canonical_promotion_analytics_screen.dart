/// Canonical Promotion Analytics Screen
///
/// Displays canonical promotion delivery analytics for seller-owned promotion
/// contracts. Consumes GET /api/v1/promotions/contracts/:id/analytics
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/promotion/data/dto/canonical_promotion_analytics_dto.dart';
import 'package:labuda/domains/commerce/pricing/promotion/presentation/providers/canonical_promotion_analytics_providers.dart';

/// Screen that displays canonical promotion delivery analytics.
///
/// This screen is analytics-ONLY: it shows the truthful canonical metrics
/// (included_count / impression_count / click_count) for one canonical
/// promotion contract identified by its canonical contract ID. It is NOT a
/// promotion detail screen — there is no promotion lifecycle/management
/// surface here.
///
/// Authorization is handled by the backend - only the contract owner can
/// view analytics. A foreign/missing contract ID returns the same error (no
/// existence oracle).
class CanonicalPromotionAnalyticsScreen extends ConsumerWidget {
  /// The canonical promotion contract ID to fetch analytics for.
  final String contractId;

  const CanonicalPromotionAnalyticsScreen({
    super.key,
    required this.contractId,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final analyticsAsync = ref.watch(
      canonicalPromotionDeliveryAnalyticsProvider(contractId),
    );

    return Scaffold(
      appBar: AppBar(
        title: const Text('Promotion Analytics'),
      ),
      body: analyticsAsync.when(
        data: (result) {
          if (result.isSuccess) {
            return _buildAnalyticsContent(context, result.data!);
          } else {
            return _buildErrorState(
              context,
              ref,
              result.error ?? 'Failed to load analytics',
            );
          }
        },
        loading: () => _buildLoadingState(),
        error: (error, _) =>
            _buildErrorState(context, ref, error.toString()),
      ),
    );
  }

  Widget _buildLoadingState() {
    return const Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          CircularProgressIndicator(),
          SizedBox(height: 16),
          Text('Loading analytics...'),
        ],
      ),
    );
  }

  Widget _buildErrorState(BuildContext context, WidgetRef ref, String message) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(
              Icons.error_outline,
              size: 64,
              color: AppColors.primaryRed,
            ),
            const SizedBox(height: 16),
            Text(
              'Failed to Load Analytics',
              style: TextStyle(
                fontSize: 20,
                fontWeight: FontWeight.bold,
                color: AppColors.neutralGray900,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              message,
              textAlign: TextAlign.center,
              style: TextStyle(
                fontSize: 14,
                color: AppColors.neutralGray600,
              ),
            ),
            const SizedBox(height: 24),
            ElevatedButton(
              onPressed: () => ref.invalidate(
                canonicalPromotionDeliveryAnalyticsProvider(contractId),
              ),
              child: const Text('Retry'),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildAnalyticsContent(BuildContext context, CanonicalPromotionAnalyticsDto analytics) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header
          Text(
            'Delivery Metrics',
            style: TextStyle(
              fontSize: 20,
              fontWeight: FontWeight.bold,
              color: AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Truthful canonical measurement from delivery events',
            style: TextStyle(
              fontSize: 14,
              color: AppColors.neutralGray600,
            ),
          ),
          const SizedBox(height: 24),

          // Metrics Cards
          _MetricsCard(
            title: 'Included',
            value: analytics.includedCount,
            description: 'Card placed in feed responses',
            icon: Icons.visibility_outlined,
            color: AppColors.primaryBlue,
          ),
          const SizedBox(height: 16),
          _MetricsCard(
            title: 'Impressions',
            value: analytics.impressionCount,
            description: 'Client acknowledged card exposure',
            icon: Icons.check_circle_outline,
            color: AppColors.successGreen,
          ),
          const SizedBox(height: 16),
          _MetricsCard(
            title: 'Clicks',
            value: analytics.clickCount,
            description: 'Client tapped the promoted card',
            icon: Icons.touch_app_outlined,
            color: AppColors.primaryRed,
          ),
          const SizedBox(height: 24),

          // Info Section
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              color: AppColors.neutralGray50,
              borderRadius: BorderRadius.circular(12),
              border: Border.all(color: AppColors.neutralGray200),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'About These Metrics',
                  style: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.w600,
                    color: AppColors.neutralGray900,
                  ),
                ),
                const SizedBox(height: 8),
                Text(
                  'These metrics are projected directly from canonical delivery events. '
                  'They represent truthful measurements of your promotion\'s delivery performance.',
                  style: TextStyle(
                    fontSize: 13,
                    color: AppColors.neutralGray600,
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

/// Card widget for displaying a single metric.
class _MetricsCard extends StatelessWidget {
  final String title;
  final int value;
  final String description;
  final IconData icon;
  final Color color;

  const _MetricsCard({
    required this.title,
    required this.value,
    required this.description,
    required this.icon,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: AppColors.neutralGray200),
        boxShadow: [
          BoxShadow(
            color: AppColors.neutralGray200.withValues(alpha: 0.5),
            blurRadius: 8,
            offset: const Offset(0, 2),
          ),
        ],
      ),
      child: Row(
        children: [
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: color.withValues(alpha: 0.1),
              shape: BoxShape.circle,
            ),
            child: Icon(icon, color: color, size: 24),
          ),
          const SizedBox(width: 16),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: AppColors.neutralGray600,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  value.toString(),
                  style: TextStyle(
                    fontSize: 28,
                    fontWeight: FontWeight.bold,
                    color: AppColors.neutralGray900,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  description,
                  style: TextStyle(
                    fontSize: 12,
                    color: AppColors.neutralGray500,
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