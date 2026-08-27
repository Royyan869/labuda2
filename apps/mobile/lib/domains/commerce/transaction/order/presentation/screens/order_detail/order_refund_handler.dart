import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/transaction/order/order.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';

/// Handles refund operations for orders
class OrderRefundHandler {
  /// Show refund request dialog and process the refund
  static void showRefundDialog({
    required BuildContext context,
    required WidgetRef ref,
    required String orderId,
    required double orderSubtotal,
    required String buyerId,
    required String sellerId,
  }) {
    showDialog(
      context: context,
      builder: (ctx) => RefundRequestDialog(
        orderId: orderId,
        onCancel: () => Navigator.of(ctx).pop(),
        onSubmit: (reason, desc, unboxingVideo, evidencePhotos) async {
          try {
            // Use S3Service from core provider instead of ServiceLocator
            final s3Service = ref.read(s3ServiceProvider);
            final allEvidenceUrls = <String>[];

            // Upload unboxing video (REQUIRED) using S3
            if (unboxingVideo != null) {
              final result = await s3Service.uploadVideo(
                File(unboxingVideo.path),
              );
              if (result.isSuccess && result.data != null) {
                allEvidenceUrls.add(result.data!);
              } else {
                throw Exception('Video upload failed: ${result.error}');
              }
            }

            // Upload evidence photos (OPTIONAL) using S3
            if (evidencePhotos.isNotEmpty) {
              for (final photo in evidencePhotos) {
                final result = await s3Service.uploadImage(File(photo.path));
                if (result.isSuccess && result.data != null) {
                  allEvidenceUrls.add(result.data!);
                }
              }
            }

            // Create refund request with correct parameters
            final params = CreateRefundParams(
              orderId: orderId,
              reason: reason, // Pass RefundReason enum directly
              description: desc ?? '',
              evidence: allEvidenceUrls.isNotEmpty ? allEvidenceUrls : null,
            );

            // Submit refund request using createRefund
            final repository = ref.read(refundRepositoryProvider);
            await repository.createRefund(params);

            if (ctx.mounted) {
              Navigator.of(ctx).pop(); // Close dialog
              AppSnackBar.showSuccess(
                ctx,
                'Refund request submitted successfully',
              );
            }
          } catch (e) {
            if (ctx.mounted) {
              AppSnackBar.showError(
                ctx,
                'Failed to submit refund: ${e.toString()}',
              );
            }
          }
        },
      ),
    );
  }
}
