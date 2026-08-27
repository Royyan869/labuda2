library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/external_product.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/external_product_review_status.dart';
import 'package:labuda/domains/commerce/pricing/promotion/presentation/providers/promotion_providers.dart';

class ExternalProductManagementScreen extends ConsumerStatefulWidget {
  const ExternalProductManagementScreen({super.key});

  @override
  ConsumerState<ExternalProductManagementScreen> createState() =>
      _ExternalProductManagementScreenState();
}

class _ExternalProductManagementScreenState
    extends ConsumerState<ExternalProductManagementScreen> {
  final _titleController = TextEditingController();
  final _urlController = TextEditingController();
  final _descriptionController = TextEditingController();

  @override
  void dispose() {
    _titleController.dispose();
    _urlController.dispose();
    _descriptionController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final productsAsync = ref.watch(myExternalProductsProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('External Products')),
      floatingActionButton: FloatingActionButton(
        onPressed: () => _showCreateDialog(context),
        child: const Icon(Icons.add),
      ),
      body: productsAsync.when(
        data: (result) {
          if (!result.isSuccess) {
            return Center(child: const Text('Data belum bisa dimuat.'));
          }

          final products = result.data ?? <ExternalProduct>[];
          if (products.isEmpty) {
            return const Center(
              child: Padding(
                padding: EdgeInsets.all(24),
                child: Text(
                  'No external products yet.\nTap + to create one.',
                  textAlign: TextAlign.center,
                ),
              ),
            );
          }

          return ListView.separated(
            padding: const EdgeInsets.all(16),
            itemCount: products.length,
            separatorBuilder: (_, _) => const SizedBox(height: 12),
            itemBuilder: (context, index) {
              final product = products[index];
              return _ExternalProductCard(
                product: product,
                onTap: () => _navigateToDetail(product.id),
              );
            },
          );
        },
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) =>
            const Center(child: Text('Data belum bisa dimuat.')),
      ),
    );
  }

  void _navigateToDetail(String productId) {
    context.push(
      RoutePaths.sellerExternalProducts.replaceAll(RegExp(r'$'), '/$productId'),
    );
  }

  Future<void> _showCreateDialog(BuildContext context) async {
    _titleController.clear();
    _urlController.clear();
    _descriptionController.clear();

    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('Create External Product'),
        content: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                controller: _titleController,
                decoration: const InputDecoration(
                  labelText: 'Title',
                  hintText: 'Product title',
                ),
              ),
              const SizedBox(height: 8),
              TextField(
                controller: _urlController,
                decoration: const InputDecoration(
                  labelText: 'External URL',
                  hintText: 'https://example.com/product',
                ),
              ),
              const SizedBox(height: 8),
              TextField(
                controller: _descriptionController,
                decoration: const InputDecoration(
                  labelText: 'Description (optional)',
                  hintText: 'Brief description',
                ),
                maxLines: 2,
              ),
            ],
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(false),
            child: const Text('Cancel'),
          ),
          ElevatedButton(
            onPressed: () => Navigator.of(dialogContext).pop(true),
            child: const Text('Create'),
          ),
        ],
      ),
    );

    if (confirmed != true || !context.mounted) return;

    final title = _titleController.text.trim();
    final url = _urlController.text.trim();
    if (title.isEmpty || url.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Title and URL are required'),
          backgroundColor: AppColors.primaryRed,
        ),
      );
      return;
    }

    final description = _descriptionController.text.trim();
    final controller = ref.read(promotionControllerProvider);
    final result = await controller.createExternalProductDraft(
      title: title,
      externalUrl: url,
      description: description.isEmpty ? null : description,
    );

    if (!context.mounted) return;

    if (result.isSuccess) {
      ref.invalidate(myExternalProductsProvider);
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('External product created')));
      _navigateToDetail(result.data!.id);
    } else {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: const Text('Gagal membuat produk. Coba lagi.'),
          backgroundColor: AppColors.primaryRed,
        ),
      );
    }
  }
}

class _ExternalProductCard extends StatelessWidget {
  final ExternalProduct product;
  final VoidCallback onTap;

  const _ExternalProductCard({required this.product, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(12),
      child: Container(
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: AppColors.neutralGray200),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    product.title,
                    style: const TextStyle(
                      fontWeight: FontWeight.w700,
                      fontSize: 16,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
                _ReviewStatusBadge(status: product.reviewStatus),
              ],
            ),
            const SizedBox(height: 6),
            Text(
              product.externalUrl,
              style: TextStyle(fontSize: 13, color: AppColors.neutralGray600),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
            if (product.hasRejectionReason) ...[
              const SizedBox(height: 6),
              Text(
                product.rejectionReason!,
                style: TextStyle(fontSize: 12, color: AppColors.primaryRed),
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
              ),
            ],
            if (product.publicVisible) ...[
              const SizedBox(height: 6),
              Row(
                children: [
                  Icon(
                    Icons.visibility,
                    size: 14,
                    color: AppColors.successGreen,
                  ),
                  const SizedBox(width: 4),
                  Text(
                    'Publicly visible',
                    style: TextStyle(
                      fontSize: 12,
                      color: AppColors.successGreen,
                    ),
                  ),
                ],
              ),
            ],
          ],
        ),
      ),
    );
  }
}

class _ReviewStatusBadge extends StatelessWidget {
  final ExternalProductReviewStatus status;

  const _ReviewStatusBadge({required this.status});

  @override
  Widget build(BuildContext context) {
    final (label, color) = switch (status) {
      ExternalProductReviewStatus.draft => ('Draft', AppColors.neutralGray600),
      ExternalProductReviewStatus.pendingReview => (
        'Menunggu Review',
        Colors.orange,
      ),
      ExternalProductReviewStatus.approved => (
        'Disetujui',
        AppColors.successGreen,
      ),
      ExternalProductReviewStatus.rejected => ('Ditolak', AppColors.primaryRed),
      ExternalProductReviewStatus.requestChanges => (
        'Perlu Perbaikan',
        Colors.orange,
      ),
      ExternalProductReviewStatus.hidden => (
        'Disembunyikan',
        AppColors.neutralGray600,
      ),
    };

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12),
        color: color.withValues(alpha: 0.1),
      ),
      child: Text(
        label,
        style: TextStyle(
          fontSize: 12,
          color: color,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }
}
