library;

import 'package:flutter/material.dart';
import 'package:labuda/core/common/types/preparation_time.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/entities/for_sale.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_primitives.dart';

class CommerceCommonProductDetailsData {
  final String? variety;
  final double? sizeCm;
  final int? ageMonths;
  final String? gender;
  final String? breeder;
  final String? bloodline;
  final List<String> certificates;
  final PreparationTime? preparationTime;
  final String? preparationNote;
  final String? description;

  const CommerceCommonProductDetailsData({
    this.variety,
    this.sizeCm,
    this.ageMonths,
    this.gender,
    this.breeder,
    this.bloodline,
    this.certificates = const [],
    this.preparationTime,
    this.preparationNote,
    this.description,
  });

  factory CommerceCommonProductDetailsData.fromListing(ForSale listing) {
    return CommerceCommonProductDetailsData(
      variety: listing.variety,
      sizeCm: listing.sizeCm,
      ageMonths: listing.ageMonths,
      gender: listing.gender,
      breeder: listing.breeder,
      bloodline: listing.bloodline,
      certificates: const [],
      preparationTime: listing.preparationTime,
      preparationNote: listing.preparationNote,
      description: listing.description,
    );
  }

  factory CommerceCommonProductDetailsData.fromAuction(Auction auction) {
    return CommerceCommonProductDetailsData(
      variety: auction.koiDetails.variety,
      sizeCm: auction.koiDetails.sizeInCm,
      ageMonths: auction.koiDetails.ageInMonths,
      gender: auction.koiDetails.gender,
      breeder: auction.koiDetails.breeder,
      bloodline: auction.koiDetails.bloodline,
      certificates: auction.koiDetails.certificates,
      preparationTime: null,
      preparationNote: null,
      description: auction.description,
    );
  }

  bool get hasPreparationInfo =>
      preparationTime != null || _isNotBlank(preparationNote);
}

class CommerceCommonProductDetailSection extends StatelessWidget {
  final String title;
  final CommerceCommonProductDetailsData data;

  const CommerceCommonProductDetailSection({
    super.key,
    required this.title,
    required this.data,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final variety = _isNotBlank(data.variety) ? data.variety! : null;
    final size = _formatSize(data.sizeCm);
    final age = _formatAge(data.ageMonths);
    final gender = _formatGender(data.gender);
    final certificates = _formatCertificates(data.certificates);
    final rows = <Widget>[
      if (variety != null)
        _buildRow(context, label: 'Varietas', value: variety),
      if (size != null) _buildRow(context, label: 'Ukuran', value: size),
      if (age != null) _buildRow(context, label: 'Usia', value: age),
      if (gender != null) _buildRow(context, label: 'Kelamin', value: gender),
      if (_isNotBlank(data.breeder))
        _buildRow(
          context,
          label: 'Breeder',
          value: data.breeder!,
          layout: CommerceDetailValueLayout.vertical,
        ),
      if (_isNotBlank(data.bloodline))
        _buildRow(
          context,
          label: 'Bloodline',
          value: data.bloodline!,
          layout: CommerceDetailValueLayout.vertical,
        ),
      if (certificates != null)
        _buildRow(
          context,
          label: 'Sertifikat',
          value: certificates,
          layout: CommerceDetailValueLayout.vertical,
        ),
    ];

    final hasDescription = _isNotBlank(data.description);
    final hasRows = rows.isNotEmpty;

    if (!data.hasPreparationInfo && !hasRows && !hasDescription) {
      return const SizedBox.shrink();
    }

    return CommerceDetailSectionCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.w700,
            ),
          ),
          if (data.hasPreparationInfo) ...[
            const SizedBox(height: 12),
            _PreparationInfoBanner(data: data),
          ],
          if (hasRows) ...[
            const SizedBox(height: 12),
            ...rows,
            if (certificates != null) ...[
              const SizedBox(height: 4),
              Text(
                'Berdasarkan pernyataan seller',
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                  fontStyle: FontStyle.italic,
                ),
              ),
            ],
          ],
          if (hasDescription) ...[
            const SizedBox(height: 12),
            Text(
              'Deskripsi',
              style: theme.textTheme.titleSmall?.copyWith(
                fontWeight: FontWeight.w700,
                color: theme.colorScheme.onSurface,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              data.description!,
              style: theme.textTheme.bodyMedium?.copyWith(
                color: theme.colorScheme.onSurface,
              ),
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildRow(
    BuildContext context, {
    required String label,
    required String value,
    CommerceDetailValueLayout layout = CommerceDetailValueLayout.auto,
  }) {
    return CommerceDetailLabelValue(
      label: label,
      value: value,
      layout: layout,
    );
  }
}

class _PreparationInfoBanner extends StatelessWidget {
  final CommerceCommonProductDetailsData data;

  const _PreparationInfoBanner({required this.data});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final preparationTime = data.preparationTime;
    final resolvedTitle = preparationTime == null
        ? 'Waktu persiapan'
        : (preparationTime.isImmediate
              ? 'Siap kirim langsung'
              : 'Estimasi siap kirim: ${preparationTime.displayName.toLowerCase()}');
    final resolvedDescription = preparationTime == null
        ? (data.preparationNote ?? '')
        : preparationTime.description;
    final resolvedIcon = preparationTime == null
        ? Icons.schedule_outlined
        : (preparationTime.isImmediate
              ? Icons.local_shipping_outlined
              : Icons.schedule_outlined);

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerLow,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Icon(
                resolvedIcon,
                size: 16,
                color: theme.colorScheme.onSurfaceVariant,
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  resolvedTitle,
                  style: theme.textTheme.bodyMedium?.copyWith(
                    fontWeight: FontWeight.w600,
                    color: theme.colorScheme.onSurface,
                  ),
                ),
              ),
            ],
          ),
          if (resolvedDescription.isNotEmpty) ...[
            const SizedBox(height: 6),
            Text(
              resolvedDescription,
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
                height: 1.4,
              ),
            ),
          ],
          const SizedBox(height: 4),
          Text(
            'Estimasi maksimal, penjual bisa kirim lebih cepat',
            style: theme.textTheme.bodySmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
              fontStyle: FontStyle.italic,
            ),
          ),
          if (_isNotBlank(data.preparationNote) && preparationTime != null) ...[
            const SizedBox(height: 8),
            Text(
              data.preparationNote!,
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ],
        ],
      ),
    );
  }
}

String? _formatSize(double? sizeCm) {
  if (sizeCm == null || sizeCm <= 0) {
    return null;
  }
  return '${sizeCm.toStringAsFixed(sizeCm % 1 == 0 ? 0 : 1)} cm';
}

String? _formatAge(int? ageMonths) {
  if (ageMonths == null || ageMonths <= 0) {
    return null;
  }
  return '$ageMonths bulan';
}

String? _formatGender(String? value) {
  if (!_isNotBlank(value)) {
    return null;
  }

  final normalized = value!.trim().toLowerCase();
  switch (normalized) {
    case 'male':
      return 'Jantan';
    case 'female':
      return 'Betina';
    case 'unknown':
      return null;
    default:
      return _titleCase(normalized);
  }
}

String? _formatCertificates(List<String> certificates) {
  final labels = certificates
      .map(_formatCertificate)
      .whereType<String>()
      .toList(growable: false);
  if (labels.isEmpty) {
    return null;
  }
  return labels.join(', ');
}

String? _formatCertificate(String value) {
  switch (value.trim().toLowerCase()) {
    case 'breeder':
      return 'Breeder';
    case 'contest':
      return 'Kontes';
    case 'ownership':
      return 'Kepemilikan';
    case 'health':
      return 'Kesehatan';
    default:
      return null;
  }
}
String _titleCase(String value) {
  return value
      .split(RegExp(r'\s+'))
      .where((part) => part.isNotEmpty)
      .map((part) => part[0].toUpperCase() + part.substring(1))
      .join(' ');
}

bool _isNotBlank(String? value) => value != null && value.trim().isNotEmpty;
