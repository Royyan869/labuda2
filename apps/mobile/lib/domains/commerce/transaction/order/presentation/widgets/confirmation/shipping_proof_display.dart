import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/domains/commerce/transaction/shipping/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/utils/shipping_honesty_messages.dart';
import 'package:labuda/shared/widgets/app_image.dart';
import 'package:url_launcher/url_launcher.dart';

/// Shipping Proof Display Widget
///
/// Displays shipping proof uploaded by seller:
/// - Photo gallery (horizontal scroll)
/// - Video thumbnails
/// - Shipping reference (resi/phone/other - honest labeling)
/// - Courier phone (if available, tap to call)
/// - Shipping note (if provided by seller)
///
/// SHIPPING CONFIRMATION TRUTH:
/// - Reference label is honest: "No. HP / WA" for phone, "Nomor Resi" for tracking
/// - No fake tracking UI
/// - Shows seller note for additional context
///
/// This is a "dumb" widget - only renders the provided data.
///
/// OWNERSHIP: Uses canonical ShippingProof from shipping domain
/// (features/shipping/domain/repositories/shipping_repository.dart)
/// Note: API provides photo URLs only, no blurhash support
class ShippingProofDisplay extends ConsumerWidget {
  final ShippingProof proof;

  const ShippingProofDisplay({super.key, required this.proof});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: isDark
            ? core.AppColors.darkGray800
            : core.AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(
          color: isDark
              ? core.AppColors.neutralGray700
              : core.AppColors.neutralGray200,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header
          Row(
            children: [
              Icon(
                Icons.verified_outlined,
                color: core.AppColors.primaryGreen,
                size: 18,
              ),
              const SizedBox(width: 8),
              Text(
                'Bukti Pengiriman',
                style: TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w600,
                  color: isDark
                      ? core.AppColors.neutralGray200
                      : core.AppColors.neutralGray800,
                ),
              ),
            ],
          ),
          // UX Honesty: Shipping is seller-managed
          Padding(
            padding: const EdgeInsets.only(top: 4),
            child: Row(
              children: [
                Icon(
                  Icons.info_outline,
                  size: 10,
                  color: core.AppColors.neutralGray500,
                ),
                const SizedBox(width: 4),
                Expanded(
                  child: Text(
                    ShippingHonestyMessages.proofProvidedBySeller,
                    style: TextStyle(
                      fontSize: 9,
                      color: isDark
                          ? core.AppColors.neutralGray500
                          : core.AppColors.neutralGray600,
                      fontStyle: FontStyle.italic,
                    ),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 12),

          // Photos
          if (proof.photos.isNotEmpty) ...[
            _buildPhotosSection(proof.photos, isDark),
            const SizedBox(height: 12),
          ],

          // Videos
          if (proof.videos.isNotEmpty) ...[
            _buildVideosSection(proof.videos, isDark),
            const SizedBox(height: 12),
          ],

          // Shipping Reference (honest label based on type)
          if (proof.effectiveReference != null &&
              proof.effectiveReference!.isNotEmpty) ...[
            _buildShippingReferenceRow(isDark: isDark),
            const SizedBox(height: 8),
          ],

          // Courier phone
          if (proof.courierPhone != null && proof.courierPhone!.isNotEmpty) ...[
            _buildPhoneRow(
              icon: Icons.phone_outlined,
              label: 'No. HP Kurir',
              phone: proof.formattedCourierPhone ?? proof.courierPhone!,
              isDark: isDark,
            ),
            const SizedBox(height: 8),
          ],

          // Shipping note from seller
          if (proof.shippingNote != null && proof.shippingNote!.isNotEmpty) ...[
            _buildNoteSection(proof.shippingNote!, isDark),
          ],
        ],
      ),
    );
  }

  Widget _buildPhotosSection(List<String> photos, bool isDark) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Foto (${photos.length})',
          style: TextStyle(
            fontSize: 11,
            fontWeight: FontWeight.w500,
            color: isDark
                ? core.AppColors.neutralGray400
                : core.AppColors.neutralGray600,
          ),
        ),
        const SizedBox(height: 6),
        SizedBox(
          height: 80,
          child: ListView.builder(
            scrollDirection: Axis.horizontal,
            itemCount: photos.length,
            itemBuilder: (context, index) {
              return Padding(
                padding: EdgeInsets.only(
                  right: index < photos.length - 1 ? 8 : 0,
                ),
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(6),
                  child: AppImage(
                    imageUrl: photos[index],
                    width: 80,
                    height: 80,
                    fit: BoxFit.cover,
                  ),
                ),
              );
            },
          ),
        ),
      ],
    );
  }

  Widget _buildVideosSection(List<String> videos, bool isDark) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Video (${videos.length})',
          style: TextStyle(
            fontSize: 11,
            fontWeight: FontWeight.w500,
            color: isDark
                ? core.AppColors.neutralGray400
                : core.AppColors.neutralGray600,
          ),
        ),
        const SizedBox(height: 6),
        ...videos.map(
          (videoUrl) => Padding(
            padding: const EdgeInsets.only(bottom: 6),
            child: Container(
              padding: const EdgeInsets.all(8),
              decoration: BoxDecoration(
                color: core.AppColors.primaryBlue.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(6),
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.play_circle_outline,
                    color: core.AppColors.primaryBlue,
                    size: 20,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      'Video Pengiriman',
                      style: TextStyle(
                        fontSize: 11,
                        color: core.AppColors.primaryBlue,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildInfoRow({
    required IconData icon,
    required String label,
    required String value,
    required bool isDark,
  }) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(
          icon,
          size: 16,
          color: isDark
              ? core.AppColors.neutralGray400
              : core.AppColors.neutralGray600,
        ),
        const SizedBox(width: 8),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                label,
                style: TextStyle(
                  fontSize: 10,
                  color: isDark
                      ? core.AppColors.neutralGray400
                      : core.AppColors.neutralGray600,
                ),
              ),
              Text(
                value,
                style: TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.w500,
                  color: isDark
                      ? core.AppColors.neutralGray200
                      : core.AppColors.neutralGray800,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildPhoneRow({
    required IconData icon,
    required String label,
    required String phone,
    required bool isDark,
  }) {
    return InkWell(
      onTap: () => _callPhone(phone),
      borderRadius: BorderRadius.circular(6),
      child: Container(
        padding: const EdgeInsets.all(8),
        decoration: BoxDecoration(
          color: core.AppColors.primaryGreen.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(6),
        ),
        child: Row(
          children: [
            Icon(icon, size: 16, color: core.AppColors.primaryGreen),
            const SizedBox(width: 8),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    label,
                    style: TextStyle(
                      fontSize: 10,
                      color: core.AppColors.primaryGreen,
                    ),
                  ),
                  Text(
                    phone,
                    style: TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.w500,
                      color: core.AppColors.primaryGreen,
                    ),
                  ),
                ],
              ),
            ),
            Icon(Icons.call, size: 16, color: core.AppColors.primaryGreen),
          ],
        ),
      ),
    );
  }

  /// Build shipping reference row with honest label based on reference type
  Widget _buildShippingReferenceRow({required bool isDark}) {
    final referenceType = proof.effectiveReferenceType;
    final reference = proof.effectiveReference!;

    // Phone-type reference: show as tappable phone number
    if (referenceType == 'phone') {
      return _buildPhoneRow(
        icon: Icons.phone_outlined,
        label: proof.referenceLabel,
        phone: reference,
        isDark: isDark,
      );
    }

    // Tracking or other: show as info row
    return _buildInfoRow(
      icon: referenceType == 'tracking'
          ? Icons.receipt_long_outlined
          : Icons.description_outlined,
      label: proof.referenceLabel,
      value: proof.effectiveReference ?? reference.toUpperCase(),
      isDark: isDark,
    );
  }

  /// Build shipping note section
  Widget _buildNoteSection(String note, bool isDark) {
    return Container(
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        color: isDark
            ? core.AppColors.neutralGray800
            : core.AppColors.neutralGray100,
        borderRadius: BorderRadius.circular(6),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                Icons.note_alt_outlined,
                size: 14,
                color: isDark
                    ? core.AppColors.neutralGray400
                    : core.AppColors.neutralGray600,
              ),
              const SizedBox(width: 6),
              Text(
                'Catatan Penjual',
                style: TextStyle(
                  fontSize: 11,
                  fontWeight: FontWeight.w500,
                  color: isDark
                      ? core.AppColors.neutralGray300
                      : core.AppColors.neutralGray700,
                ),
              ),
            ],
          ),
          const SizedBox(height: 4),
          Text(
            note,
            style: TextStyle(
              fontSize: 12,
              color: isDark
                  ? core.AppColors.neutralGray200
                  : core.AppColors.neutralGray800,
            ),
          ),
        ],
      ),
    );
  }

  Future<void> _callPhone(String phone) async {
    final uri = Uri.parse('tel:$phone');
    if (await canLaunchUrl(uri)) {
      await launchUrl(uri);
    }
  }
}
