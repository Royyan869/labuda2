import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/services/location_service.dart';

/// Widget untuk menampilkan accuracy indicator di map
/// Menampilkan circle radius sesuai GPS accuracy
class MapAccuracyIndicator extends StatelessWidget {
  final double? accuracy; // dalam meter
  final double zoomLevel;

  const MapAccuracyIndicator({
    super.key,
    required this.accuracy,
    this.zoomLevel = 16,
  });

  @override
  Widget build(BuildContext context) {
    if (accuracy == null) {
      return const SizedBox.shrink();
    }

    final level = _getAccuracyLevel(accuracy!);
    return Positioned(
      top: 16,
      left: 16,
      child: _AccuracyBadge(level: level, accuracy: accuracy!),
    );
  }

  AccuracyLevel _getAccuracyLevel(double acc) {
    if (acc <= 10) return AccuracyLevel.excellent;
    if (acc <= 25) return AccuracyLevel.good;
    if (acc <= 50) return AccuracyLevel.fair;
    return AccuracyLevel.poor;
  }
}

/// Badge untuk menampilkan accuracy level
class _AccuracyBadge extends StatelessWidget {
  final AccuracyLevel level;
  final double accuracy;

  const _AccuracyBadge({required this.level, required this.accuracy});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: isDark
            ? AppColors.darkGray700.withValues(alpha: 0.9)
            : AppColors.neutralWhite.withValues(alpha: 0.9),
        borderRadius: BorderRadius.circular(20),
        border: Border.all(
          color: level.color.withValues(alpha: 0.5),
          width: 1.5,
        ),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.1),
            blurRadius: 8,
            offset: const Offset(0, 2),
          ),
        ],
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          // Icon indikator
          Container(
            width: 8,
            height: 8,
            decoration: BoxDecoration(
              color: level.color,
              shape: BoxShape.circle,
            ),
          ),
          const SizedBox(width: 8),
          // Label
          Text(
            level.label,
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w600,
              color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
            ),
          ),
          const SizedBox(width: 4),
          // Accuracy value
          Text(
            '(${accuracy.toStringAsFixed(0)}m)',
            style: TextStyle(
              fontSize: 11,
              fontWeight: FontWeight.w500,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
          ),
        ],
      ),
    );
  }
}

/// Widget untuk menampilkan warning jika menggunakan default location
class DefaultLocationWarning extends StatelessWidget {
  final VoidCallback onRetry;

  const DefaultLocationWarning({super.key, required this.onRetry});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      margin: const EdgeInsets.all(16),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: AppColors.primaryRed.withValues(alpha: isDark ? 0.2 : 0.1),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: AppColors.primaryRed.withValues(alpha: 0.5),
          width: 1,
        ),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              Icon(
                Icons.warning_amber_rounded,
                color: AppColors.primaryRed,
                size: 20,
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  'Tidak Mendapatkan Lokasi GPS',
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: isDark
                        ? AppColors.neutralWhite
                        : AppColors.neutralGray900,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text(
            'Menggunakan lokasi default (Jakarta). Pastikan GPS aktif dan coba lagi.',
            style: TextStyle(
              fontSize: 12,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              Expanded(
                child: OutlinedButton(
                  onPressed: () => Navigator.pop(context),
                  style: OutlinedButton.styleFrom(
                    side: BorderSide(
                      color: isDark
                          ? AppColors.neutralGray600
                          : AppColors.neutralGray300,
                    ),
                    padding: const EdgeInsets.symmetric(vertical: 10),
                  ),
                  child: Text(
                    'Tutup',
                    style: TextStyle(
                      fontSize: 14,
                      color: isDark
                          ? AppColors.neutralWhite
                          : AppColors.neutralGray700,
                    ),
                  ),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: FilledButton(
                  onPressed: onRetry,
                  style: FilledButton.styleFrom(
                    backgroundColor: AppColors.primaryRed,
                    padding: const EdgeInsets.symmetric(vertical: 10),
                  ),
                  child: const Text(
                    'Coba Lagi',
                    style: TextStyle(fontSize: 14),
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

/// Widget accuracy indicator untuk LocationInfoCard
class LocationAccuracyIndicator extends StatelessWidget {
  final AccuracyLevel level;
  final String accuracyLabel;
  final bool isDefault;
  final bool isLastKnown;

  const LocationAccuracyIndicator({
    super.key,
    required this.level,
    required this.accuracyLabel,
    this.isDefault = false,
    this.isLastKnown = false,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    // Special case untuk default/last known
    if (isDefault || isLastKnown) {
      return Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
        decoration: BoxDecoration(
          color: isDefault
              ? AppColors.primaryRed.withValues(alpha: 0.1)
              : AppColors.neutralGray600.withValues(alpha: 0.2),
          borderRadius: BorderRadius.circular(6),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              isDefault ? Icons.warning_amber_rounded : Icons.history,
              size: 12,
              color: isDefault
                  ? AppColors.primaryRed
                  : (isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray600),
            ),
            const SizedBox(width: 4),
            Text(
              accuracyLabel,
              style: TextStyle(
                fontSize: 10,
                fontWeight: FontWeight.w500,
                color: isDefault
                    ? AppColors.primaryRed
                    : (isDark
                          ? AppColors.neutralGray400
                          : AppColors.neutralGray600),
              ),
            ),
          ],
        ),
      );
    }

    // Normal accuracy indicator
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
      decoration: BoxDecoration(
        color: level.color.withValues(alpha: 0.15),
        borderRadius: BorderRadius.circular(6),
        border: Border.all(color: level.color.withValues(alpha: 0.4), width: 1),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 6,
            height: 6,
            decoration: BoxDecoration(
              color: level.color,
              shape: BoxShape.circle,
            ),
          ),
          const SizedBox(width: 4),
          Text(
            accuracyLabel,
            style: TextStyle(
              fontSize: 10,
              fontWeight: FontWeight.w500,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray700,
            ),
          ),
        ],
      ),
    );
  }
}
