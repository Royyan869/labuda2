import 'package:flutter/material.dart';
import 'package:hishumi/shared/entities/post_location.dart';
import 'package:hishumi/shared/widgets/address_location_view.dart';
import 'package:hishumi/shared/widgets/app_snackbar.dart';
import 'package:url_launcher/url_launcher.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';

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
        AppSnackBar.showError(context, 'Tidak dapat membuka Google Maps');
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
          padding: const EdgeInsets.symmetric(
            vertical: AppMetrics.p4,
            horizontal: AppMetrics.p0,
          ),
          child: AddressLocationView(
            location: location.address,
            mode: AddressLocationMode.compact,
            icon: Icons.location_on,
            iconSize: AppIconSize.inlineGlyph,
            iconColor: scheme.primary,
            spacing: 4,
            mainAxisSize: MainAxisSize.min,
            trailing: Icon(
              Icons.open_in_new,
              size: AppIconSize.inlineGlyph,
              color: scheme.secondary,
            ),
            style: context.typeRoles.bodyDense.copyWith(
              color: scheme.secondary,
              decoration: TextDecoration.underline,
            ),
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
          border: Border.all(color: scheme.secondary.withValues(alpha: 0.3)),
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
                  AddressLocationText(
                    location: location.address,
                    mode: AddressLocationMode.detail,
                    maxLines: 2,
                    style: context.typeRoles.bodyDense.copyWith(
                      fontWeight: FontWeight.w500,
                      color: scheme.onSurface,
                    ),
                  ),
                  if (location.hasCoordinates) ...[
                    const SizedBox(height: 4),
                    Text(
                      '${location.latitude!.toStringAsFixed(6)}, ${location.longitude!.toStringAsFixed(6)}',
                      style: context.typeRoles.labelMicro.copyWith(
                        fontFamily: 'monospace',
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                  ],
                ],
              ),
            ),
            const SizedBox(width: 8),
            Icon(
              Icons.open_in_new,
              color: scheme.secondary,
              size: AppIconSize.action,
            ),
          ],
        ),
      ),
    );
  }
}
