import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/src/config/google_config.dart';
import 'package:labuda/shared/entities/post_location.dart';
import 'package:labuda/shared/shared.dart';
import 'package:url_launcher/url_launcher.dart';

/// Modal untuk preview koordinat dengan opsi Edit dan Lihat Maps
///
/// Features:
/// - Static map preview dari Google Maps
/// - Koordinat display
/// - Tombol "Edit" untuk membuka map picker
/// - Tombol "Lihat Maps" untuk membuka Google Maps eksternal
class CoordinatePreviewModal extends StatelessWidget {
  final double latitude;
  final double longitude;
  final String? address;

  /// Callback ketika user tap Edit dan memilih koordinat baru
  /// Jika null, tombol Edit tidak akan ditampilkan
  final Function(double lat, double lng)? onCoordinatesChanged;

  const CoordinatePreviewModal({
    super.key,
    required this.latitude,
    required this.longitude,
    this.address,
    this.onCoordinatesChanged,
  });

  /// Show modal sebagai bottom sheet
  static Future<void> show(
    BuildContext context, {
    required double latitude,
    required double longitude,
    String? address,
    Function(double lat, double lng)? onCoordinatesChanged,
  }) {
    return showModalBottomSheet(
      context: context,
      backgroundColor: Colors.transparent,
      isScrollControlled: true,
      builder: (context) => CoordinatePreviewModal(
        latitude: latitude,
        longitude: longitude,
        address: address,
        onCoordinatesChanged: onCoordinatesChanged,
      ),
    );
  }

  String get _staticMapUrl {
    final apiKey = GoogleConfig.apiKey;
    final marker = '$latitude,$longitude';
    return 'https://maps.googleapis.com/maps/api/staticmap'
        '?center=$marker'
        '&zoom=16'
        '&size=600x300'
        '&maptype=roadmap'
        '&markers=color:red%7C$marker'
        '&key=$apiKey';
  }

  Future<void> _openInGoogleMaps(BuildContext context) async {
    final url = Uri.parse(
      'https://www.google.com/maps/search/?api=1&query=$latitude,$longitude',
    );

    if (await canLaunchUrl(url)) {
      await launchUrl(url, mode: LaunchMode.externalApplication);
    } else {
      if (context.mounted) {
        AppSnackBar.showError(context, 'Cannot open Google Maps');
      }
    }
  }

  Future<void> _openMapPicker(BuildContext context) async {
    Navigator.pop(context); // Close this modal first

    final location = await InteractiveMapPickerBottomSheet.show(
      context: context,
      initialLocation: PostLocation(
        address: address ?? '',
        latitude: latitude,
        longitude: longitude,
      ),
      googleApiKey: GoogleConfig.apiKey,
    );

    if (location != null && location.hasCoordinates) {
      onCoordinatesChanged?.call(location.latitude!, location.longitude!);
    }
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(20)),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          // Drag handle
          Center(
            child: Container(
              margin: const EdgeInsets.only(top: 12, bottom: 8),
              width: 40,
              height: 4,
              decoration: BoxDecoration(
                color: isDark
                    ? AppColors.neutralGray600
                    : AppColors.neutralGray300,
                borderRadius: BorderRadius.circular(2),
              ),
            ),
          ),

          // Header
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
            child: Row(
              children: [
                Icon(Icons.location_on, color: AppColors.primaryRed, size: 24),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(
                    'Pinpoint Location',
                    style: TextStyle(
                      fontSize: 18,
                      fontWeight: FontWeight.bold,
                      color: isDark
                          ? AppColors.neutralWhite
                          : AppColors.neutralGray900,
                    ),
                  ),
                ),
                IconButton(
                  onPressed: () => Navigator.pop(context),
                  icon: Icon(
                    Icons.close,
                    color: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray600,
                  ),
                  constraints: const BoxConstraints(),
                  padding: EdgeInsets.zero,
                ),
              ],
            ),
          ),

          // Static Map Preview
          Container(
            height: 180,
            width: double.infinity,
            margin: const EdgeInsets.symmetric(horizontal: 16),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(12),
              color: isDark ? AppColors.darkGray700 : AppColors.neutralGray100,
            ),
            clipBehavior: Clip.antiAlias,
            child: GoogleConfig.isConfigured
                ? Image.network(
                    _staticMapUrl,
                    fit: BoxFit.cover,
                    loadingBuilder: (context, child, loadingProgress) {
                      if (loadingProgress == null) return child;
                      return Center(
                        child: CircularProgressIndicator(
                          value: loadingProgress.expectedTotalBytes != null
                              ? loadingProgress.cumulativeBytesLoaded /
                                    loadingProgress.expectedTotalBytes!
                              : null,
                        ),
                      );
                    },
                    errorBuilder: (context, error, stackTrace) {
                      return _buildMapPlaceholder(isDark);
                    },
                  )
                : _buildMapPlaceholder(isDark),
          ),

          // Coordinates Display
          Padding(
            padding: const EdgeInsets.all(16),
            child: Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: isDark ? AppColors.darkGray700 : AppColors.neutralGray50,
                borderRadius: BorderRadius.circular(8),
                border: Border.all(
                  color: isDark
                      ? AppColors.darkGray600
                      : AppColors.neutralGray200,
                ),
              ),
              child: Row(
                children: [
                  Icon(Icons.pin_drop, size: 18, color: AppColors.primaryRed),
                  const SizedBox(width: 10),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          'Coordinates',
                          style: TextStyle(
                            fontSize: 12,
                            color: isDark
                                ? AppColors.neutralGray400
                                : AppColors.neutralGray600,
                          ),
                        ),
                        const SizedBox(height: 2),
                        Text(
                          '${latitude.toStringAsFixed(6)}, ${longitude.toStringAsFixed(6)}',
                          style: TextStyle(
                            fontSize: 14,
                            fontFamily: 'monospace',
                            fontWeight: FontWeight.w500,
                            color: isDark
                                ? AppColors.neutralGray200
                                : AppColors.neutralGray800,
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),

          // Address if available
          if (address != null && address!.isNotEmpty)
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16),
              child: Text(
                address!,
                style: TextStyle(
                  fontSize: 13,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
                textAlign: TextAlign.center,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
              ),
            ),

          // Action Buttons
          Padding(
            padding: const EdgeInsets.all(16),
            child: Row(
              children: [
                // Edit Button (only if callback provided)
                if (onCoordinatesChanged != null) ...[
                  Expanded(
                    child: OutlinedButton.icon(
                      onPressed: () => _openMapPicker(context),
                      icon: const Icon(Icons.edit_location, size: 18),
                      label: const Text('Edit'),
                      style: OutlinedButton.styleFrom(
                        foregroundColor: AppColors.primaryRed,
                        side: BorderSide(color: AppColors.primaryRed),
                        padding: const EdgeInsets.symmetric(vertical: 12),
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(8),
                        ),
                      ),
                    ),
                  ),
                  const SizedBox(width: 12),
                ],

                // View in Maps Button
                Expanded(
                  child: ElevatedButton.icon(
                    onPressed: () => _openInGoogleMaps(context),
                    icon: const Icon(Icons.map_outlined, size: 18),
                    label: const Text('View Maps'),
                    style: ElevatedButton.styleFrom(
                      backgroundColor: AppColors.primaryRed,
                      foregroundColor: AppColors.neutralWhite,
                      padding: const EdgeInsets.symmetric(vertical: 12),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(8),
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),

          // Bottom safe area
          SizedBox(height: MediaQuery.of(context).padding.bottom),
        ],
      ),
    );
  }

  Widget _buildMapPlaceholder(bool isDark) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.map_outlined,
            size: 48,
            color: isDark ? AppColors.neutralGray500 : AppColors.neutralGray400,
          ),
          const SizedBox(height: 8),
          Text(
            'Preview not available',
            style: TextStyle(
              fontSize: 12,
              color: isDark
                  ? AppColors.neutralGray500
                  : AppColors.neutralGray500,
            ),
          ),
        ],
      ),
    );
  }
}
