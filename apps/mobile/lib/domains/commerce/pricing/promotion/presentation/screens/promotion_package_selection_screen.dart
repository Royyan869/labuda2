/// Promotion Package Selection Screen
///
/// Screen for users to select and purchase promotion packages
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/promotion_package.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/target_type.dart';
import 'package:labuda/domains/commerce/pricing/promotion/presentation/providers/promotion_providers.dart';
import 'package:labuda/shared/utils/app_formatters.dart';
import 'package:url_launcher/url_launcher.dart';

/// Promotion Package Selection Screen
class PromotionPackageSelectionScreen extends ConsumerStatefulWidget {
  final String? fixedPriceSaleId;
  final String? fixedPriceSaleTitle;
  final bool returnToActivationOnSuccess;
  final TargetType? preselectedTargetType;
  final String? preselectedTargetId;
  final String? preselectedTargetTitle;
  final String? preselectedOwnershipId;
  final String? reassignInstanceId;

  const PromotionPackageSelectionScreen({
    super.key,
    this.fixedPriceSaleId,
    this.fixedPriceSaleTitle,
    this.returnToActivationOnSuccess = false,
    this.preselectedTargetType,
    this.preselectedTargetId,
    this.preselectedTargetTitle,
    this.preselectedOwnershipId,
    this.reassignInstanceId,
  });

  @override
  ConsumerState<PromotionPackageSelectionScreen> createState() =>
      _PromotionPackageSelectionScreenState();
}

class _PromotionPackageSelectionScreenState
    extends ConsumerState<PromotionPackageSelectionScreen> {
  bool _isPurchasing = false;
  String? _selectedPackageId;

  @override
  Widget build(BuildContext context) {
    final packagesAsync = ref.watch(availablePackagesProvider);
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Scaffold(
      appBar: AppBar(
        title: const Text('Pilih Paket Promosi'),
        backgroundColor: isDark
            ? AppColors.darkGray800
            : AppColors.neutralWhite,
        foregroundColor: isDark
            ? AppColors.neutralWhite
            : AppColors.neutralGray900,
        elevation: 0,
        surfaceTintColor: Colors.transparent,
        scrolledUnderElevation: 0,
      ),
      body: packagesAsync.when(
        data: (result) {
          if (!result.isSuccess) {
            return _buildError(
              context,
              result.error ?? 'Failed to load packages',
            );
          }

          final packages = result.data ?? [];
          if (packages.isEmpty) {
            return _buildEmpty(context);
          }

          return Column(
            children: [
              // Header info
              Container(
                width: double.infinity,
                padding: const EdgeInsets.all(16),
                color: isDark ? AppColors.darkGray800 : AppColors.neutralGray50,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'Promosikan Fixed-Price Sale',
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.bold,
                        color: isDark
                            ? AppColors.neutralWhite
                            : AppColors.neutralGray900,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Text(
                      widget.fixedPriceSaleTitle ??
                          widget.preselectedTargetTitle ??
                          'Target promotion',
                      style: TextStyle(
                        fontSize: 14,
                        color: AppColors.neutralGray600,
                      ),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: 8),
                    Text(
                      'Pilih paket durasi promosi. Fixed-price sale akan langsung dipromosikan setelah pembayaran.',
                      style: TextStyle(
                        fontSize: 12,
                        color: AppColors.neutralGray600,
                      ),
                    ),
                  ],
                ),
              ),
              // Package list
              Expanded(
                child: ListView.builder(
                  padding: const EdgeInsets.all(16),
                  itemCount: packages.length,
                  itemBuilder: (context, index) {
                    final package = packages[index];
                    final isSelected = _selectedPackageId == package.id;
                    return _PackageCard(
                      package: package,
                      isSelected: isSelected,
                      onTap: () {
                        setState(() {
                          _selectedPackageId = package.id;
                        });
                      },
                      onPurchase: _isPurchasing
                          ? null
                          : () => _purchasePackage(context, package),
                    );
                  },
                ),
              ),
            ],
          );
        },
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => _buildError(context, error.toString()),
      ),
    );
  }

  Widget _buildEmpty(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.campaign_outlined,
            size: 64,
            color: AppColors.neutralGray400,
          ),
          const SizedBox(height: 16),
          Text(
            'Belum Ada Paket',
            style: TextStyle(
              fontSize: 20,
              fontWeight: FontWeight.bold,
              color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Paket promosi belum tersedia saat ini',
            style: TextStyle(fontSize: 14, color: AppColors.neutralGray600),
          ),
        ],
      ),
    );
  }

  Widget _buildError(BuildContext context, String error) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(
              Icons.error_outline,
              size: 48,
              color: AppColors.primaryRed,
            ),
            const SizedBox(height: 16),
            Text(
              'Terjadi Kesalahan',
              style: TextStyle(
                fontSize: 18,
                fontWeight: FontWeight.bold,
                color: AppColors.neutralGray900,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              error,
              style: TextStyle(fontSize: 14, color: AppColors.neutralGray600),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 24),
            ElevatedButton(
              onPressed: () {
                ref.invalidate(availablePackagesProvider);
              },
              child: const Text('Coba Lagi'),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _purchasePackage(
    BuildContext context,
    PromotionPackage package,
  ) async {
    if (_selectedPackageId == null) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Pilih paket terlebih dahulu'),
          backgroundColor: AppColors.primaryRed,
        ),
      );
      return;
    }

    setState(() {
      _isPurchasing = true;
    });

    final controller = ref.read(promotionControllerProvider);
    final result = await controller.purchasePackage(package.id);

    if (!mounted) return;

    setState(() {
      _isPurchasing = false;
    });

    if (result.isSuccess) {
      final billingId = result.data ?? '';
      final paymentResult = await controller.initiateBillingPayment(billingId);

      if (!context.mounted) return;

      String postMessage =
          'Payment processing; package will appear after confirmation.';
      if (paymentResult.isSuccess) {
        final paymentUrl = paymentResult.data?.paymentUrl ?? '';
        if (paymentUrl.isNotEmpty) {
          final uri = Uri.parse(paymentUrl);
          final opened = await launchUrl(
            uri,
            mode: LaunchMode.externalApplication,
          );
          if (!opened && context.mounted) {
            await showDialog<void>(
              context: context,
              builder: (context) => AlertDialog(
                title: const Text('Open Payment URL'),
                content: SelectableText(paymentUrl),
                actions: [
                  TextButton(
                    onPressed: () => Navigator.pop(context),
                    child: const Text('OK'),
                  ),
                ],
              ),
            );
          }
        }
      } else {
        postMessage =
            'Payment initiation failed. Please try again from My Promotions.';
      }

      if (!context.mounted) return;
      if (widget.returnToActivationOnSuccess) {
        Navigator.pop(
          context,
          PurchasePackageNavigationResult(
            goToActivation: true,
            message: postMessage,
            preselectedTargetType: widget.preselectedTargetType,
            preselectedTargetId: widget.preselectedTargetId,
            preselectedTargetTitle: widget.preselectedTargetTitle,
            preselectedOwnershipId: widget.preselectedOwnershipId,
            reassignInstanceId: widget.reassignInstanceId,
          ),
        );
        return;
      }
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(postMessage)));
      context.go(RoutePaths.sellerPromotions);
    } else {
      if (!mounted) return;
      ScaffoldMessenger.of(this.context).showSnackBar(
        SnackBar(
          content: Text('Gagal: ${result.error}'),
          backgroundColor: AppColors.primaryRed,
        ),
      );
    }
  }
}

class PurchasePackageNavigationResult {
  final bool goToActivation;
  final String? message;
  final TargetType? preselectedTargetType;
  final String? preselectedTargetId;
  final String? preselectedTargetTitle;
  final String? preselectedOwnershipId;
  final String? reassignInstanceId;

  const PurchasePackageNavigationResult({
    required this.goToActivation,
    this.message,
    this.preselectedTargetType,
    this.preselectedTargetId,
    this.preselectedTargetTitle,
    this.preselectedOwnershipId,
    this.reassignInstanceId,
  });
}

/// Package Card Widget
class _PackageCard extends StatelessWidget {
  final PromotionPackage package;
  final bool isSelected;
  final VoidCallback onTap;
  final VoidCallback? onPurchase;

  const _PackageCard({
    required this.package,
    required this.isSelected,
    required this.onTap,
    this.onPurchase,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      margin: const EdgeInsets.only(bottom: 12),
      decoration: BoxDecoration(
        color: isSelected
            ? AppColors.primaryRed.withValues(alpha: 0.1)
            : (isDark ? AppColors.darkGray800 : AppColors.neutralWhite),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isSelected
              ? AppColors.primaryRed
              : (isDark ? AppColors.darkGray700 : AppColors.neutralGray200),
          width: isSelected ? 2 : 1,
        ),
      ),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  // Radio indicator
                  Container(
                    width: 20,
                    height: 20,
                    decoration: BoxDecoration(
                      shape: BoxShape.circle,
                      border: Border.all(
                        color: isSelected
                            ? AppColors.primaryRed
                            : AppColors.neutralGray400,
                        width: 2,
                      ),
                    ),
                    child: isSelected
                        ? Center(
                            child: Container(
                              width: 10,
                              height: 10,
                              decoration: const BoxDecoration(
                                shape: BoxShape.circle,
                                color: AppColors.primaryRed,
                              ),
                            ),
                          )
                        : null,
                  ),
                  const SizedBox(width: 12),
                  // Package name
                  Expanded(
                    child: Text(
                      package.name,
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.bold,
                        color: isDark
                            ? AppColors.neutralWhite
                            : AppColors.neutralGray900,
                      ),
                    ),
                  ),
                  // Price
                  Text(
                    _formatPrice(package.priceAmount),
                    style: TextStyle(
                      fontSize: 18,
                      fontWeight: FontWeight.bold,
                      color: AppColors.primaryRed,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              // Details
              _buildDetailRow(
                context,
                Icons.schedule,
                'Durasi',
                _formatDuration(package.totalDurationHours),
              ),
              const SizedBox(height: 8),
              _buildDetailRow(
                context,
                Icons.timer_outlined,
                'Berlaku hingga',
                _formatDuration(package.validityWindowHours),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildDetailRow(
    BuildContext context,
    IconData icon,
    String label,
    String value,
  ) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Row(
      children: [
        Icon(icon, size: 16, color: AppColors.neutralGray600),
        const SizedBox(width: 8),
        Text(
          '$label: ',
          style: TextStyle(fontSize: 13, color: AppColors.neutralGray600),
        ),
        Text(
          value,
          style: TextStyle(
            fontSize: 13,
            fontWeight: FontWeight.w600,
            color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
          ),
        ),
      ],
    );
  }

  String _formatPrice(int priceAmount) {
    return AppFormatters.formatCurrencyInt(priceAmount);
  }

  String _formatDuration(int hours) {
    if (hours >= 24) {
      final days = (hours / 24).round();
      return '$days hari';
    }
    return '$hours jam';
  }
}
