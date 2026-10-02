import 'package:flutter/material.dart';
import 'package:labuda/shared/entities/post_location.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Clickable location widget yang bisa buka Google Maps
///
/// Usage:
/// ```dart
/// ClickableLocationWidget(
///   location: PostLocation(
///     address: "Stadion Gelora Bung Karno",
///     latitude: -6.2088,
///     longitude: 106.8456,
///   ),
/// )
/// ```
class ClickableLocationWidget extends StatelessWidget {
  final PostLocation location;
  final bool compact; // Compact mode untuk inline display

  const ClickableLocationWidget({
    super.key,
    required this.location,
    this.compact = false,
  });

  Future<void> _openInMaps(BuildContext context) async {
    if (!location.hasCoordinates) {
      // Jika tidak ada coordinates, coba search berdasarkan address
      final query = Uri.encodeComponent(location.address);
      final url = Uri.parse(
        'https://www.google.com/maps/search/?api=1&query=$query',
      );

      if (await canLaunchUrl(url)) {
        await launchUrl(url, mode: LaunchMode.externalApplication);
      }
      return;
    }

    // Jika ada coordinates, buka dengan koordinat
    final lat = location.latitude!;
    final lng = location.longitude!;

    // Google Maps URL dengan coordinates
    // Format: https://www.google.com/maps/search/?api=1&query=lat,lng
    final url = Uri.parse(
      'https://www.google.com/maps/search/?api=1&query=$lat,$lng',
    );

    try {
      if (await canLaunchUrl(url)) {
        await launchUrl(url, mode: LaunchMode.externalApplication);
      }
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Cannot open Google Maps'),
            duration: Duration(seconds: 4),
          ),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    if (compact) {
      // Compact mode - inline dengan icon
      return InkWell(
        onTap: () => _openInMaps(context),
        borderRadius: BorderRadius.circular(AppShape.r4),
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: AppMetrics.p4, horizontal: AppMetrics.p0),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.location_on, size: AppIconSize.inlineGlyph, color: scheme.primary),
              const SizedBox(width: 4),
              Flexible(
                child: Text(
                  location.address,
                  style: TextStyle(
                    fontSize: AppType.s14,
                    color: scheme.secondary,
                    decoration: TextDecoration.underline,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              const SizedBox(width: 4),
              Icon(Icons.open_in_new, size: AppIconSize.inlineGlyph, color: scheme.secondary),
            ],
          ),
        ),
      );
    }

    // Full mode - card dengan detail
    return InkWell(
      onTap: () => _openInMaps(context),
      borderRadius: BorderRadius.circular(AppShape.r8),
      child: Container(
        padding: const EdgeInsets.all(AppMetrics.p12),
        decoration: BoxDecoration(
          color: scheme.secondary.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(AppShape.r8),
          border: Border.all(
            color: scheme.secondary.withValues(alpha: 0.3),
          ),
        ),
        child: Row(
          children: [
            Container(
              padding: const EdgeInsets.all(AppMetrics.p8),
              decoration: BoxDecoration(
                color: scheme.primary.withValues(alpha: 0.12),
                borderRadius: BorderRadius.circular(AppShape.r8),
              ),
              child: Icon(
                Icons.location_on,
                color: scheme.primary,
                size: AppIconSize.action,
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    location.address,
                    style: TextStyle(
                      fontSize: AppType.s14,
                      fontWeight: FontWeight.w500,
                      color: scheme.onSurface,
                    ),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                  if (location.hasCoordinates) ...[
                    const SizedBox(height: 4),
                    Text(
                      '${location.latitude!.toStringAsFixed(6)}, ${location.longitude!.toStringAsFixed(6)}',
                      style: TextStyle(
                        fontSize: AppType.s12,
                        fontFamily: 'monospace',
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                  ],
                ],
              ),
            ),
            const SizedBox(width: 8),
            Icon(Icons.open_in_new, color: scheme.secondary, size: AppIconSize.action),
          ],
        ),
      ),
    );
  }
}
