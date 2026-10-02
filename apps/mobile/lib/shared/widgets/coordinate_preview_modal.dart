import 'package:flutter/material.dart';
import 'package:labuda/core/src/config/google_config.dart';
import 'package:labuda/shared/entities/post_location.dart';
import 'package:labuda/shared/shared.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

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
    final scheme = Theme.of(context).colorScheme;

    return Container(
      decoration: BoxDecoration(
        color: scheme.surface,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(AppShape.r20)),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          // Drag handle — ONE authority: `AppDragHandle` beside the bottom-sheet
          // base (the default spacing is this sheet's `top p12 + bottom p8`).
          const Center(child: AppDragHandle()),

          // Header
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16, vertical: AppMetrics.p8),
            child: Row(
              children: [
                Icon(Icons.location_on, color: scheme.primary, size: AppIconSize.header),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(
                    'Pinpoint Location',
                    style: TextStyle(
                      fontSize: AppType.s20,
                      fontWeight: FontWeight.bold,
                      color: scheme.onSurfaceVariant,
                    ),
                  ),
                ),
                IconButton(
                  onPressed: () => Navigator.pop(context),
                  icon: Icon(
                    Icons.close,
                    color: scheme.onSurfaceVariant,
                  ),
                  constraints: const BoxConstraints(),
                  padding: EdgeInsets.zero,
                ),
              ],
            ),
          ),

          // Static Map Preview
          Container(
            height: AppContentSize.preview,
            width: double.infinity,
            margin: const EdgeInsets.symmetric(horizontal: AppMetrics.p16),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(AppShape.r12),
              color: scheme.surfaceContainerHighest,
            ),
            clipBehavior: Clip.antiAlias,
            child: GoogleConfig.isConfigured
                ? AppImage(
                    imageUrl: _staticMapUrl,
                    fit: BoxFit.cover,
                    backgroundColor: scheme.surfaceContainerHighest,
                    errorWidget: _buildMapPlaceholder(scheme),
                  )
                : _buildMapPlaceholder(scheme),
          ),

          // Coordinates Display
          Padding(
            padding: const EdgeInsets.all(AppMetrics.p16),
            child: Container(
              padding: const EdgeInsets.all(AppMetrics.p12),
              decoration: BoxDecoration(
                color: scheme.surfaceContainerHighest,
                borderRadius: BorderRadius.circular(AppShape.r8),
                border: Border.all(color: scheme.outlineVariant),
              ),
              child: Row(
                children: [
                  Icon(Icons.pin_drop, size: AppIconSize.action, color: scheme.primary),
                  const SizedBox(width: 10),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          'Coordinates',
                          style: TextStyle(
                            fontSize: AppType.s12,
                            color: scheme.onSurfaceVariant,
                          ),
                        ),
                        const SizedBox(height: 2),
                        Text(
                          '${latitude.toStringAsFixed(6)}, ${longitude.toStringAsFixed(6)}',
                          style: TextStyle(
                            fontSize: AppType.s14,
                            fontFamily: 'monospace',
                            fontWeight: FontWeight.w500,
                            color: scheme.onSurfaceVariant,
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
              padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16),
              child: Text(
                address!,
                style: TextStyle(
                  fontSize: AppType.s14,
                  color: scheme.onSurfaceVariant,
                ),
                textAlign: TextAlign.center,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
              ),
            ),

          // Action Buttons
          Padding(
            padding: const EdgeInsets.all(AppMetrics.p16),
            child: Row(
              children: [
                // Edit Button (only if callback provided)
                if (onCoordinatesChanged != null) ...[
                  Expanded(
                    child: OutlinedButton.icon(
                      onPressed: () => _openMapPicker(context),
                      icon: const Icon(Icons.edit_location, size: AppIconSize.action),
                      label: const Text('Edit'),
                      style: OutlinedButton.styleFrom(
                        padding: const EdgeInsets.symmetric(vertical: AppMetrics.p12),
                      ),
                    ),
                  ),
                  const SizedBox(width: 12),
                ],

                // View in Maps Button
                Expanded(
                  child: ElevatedButton.icon(
                    onPressed: () => _openInGoogleMaps(context),
                    icon: const Icon(Icons.map_outlined, size: AppIconSize.action),
                    label: const Text('View Maps'),
                    style: ElevatedButton.styleFrom(
                      padding: const EdgeInsets.symmetric(vertical: AppMetrics.p12),
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

  Widget _buildMapPlaceholder(ColorScheme scheme) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.map_outlined,
            size: AppIconSize.display,
            color: scheme.onSurfaceVariant,
          ),
          const SizedBox(height: 8),
          Text(
            'Preview not available',
            style: TextStyle(
              fontSize: AppType.s12,
              color: scheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }
}
