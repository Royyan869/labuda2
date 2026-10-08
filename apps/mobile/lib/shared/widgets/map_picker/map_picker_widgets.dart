import 'package:flutter/material.dart';
import 'package:labuda/shared/services/places_autocomplete_service.dart';
import 'package:labuda/shared/widgets/address_location_view.dart';
import 'package:labuda/shared/widgets/bottom_action_bar.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Header untuk Map Picker
class MapPickerHeader extends StatelessWidget {
  const MapPickerHeader({super.key});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: scheme.surface,
        border: Border(bottom: BorderSide(color: scheme.outlineVariant)),
      ),
      child: Row(
        children: [
          Expanded(
            child: Text(
              'Pilih Lokasi',
              style: context.typeRoles.titleSection.copyWith(
                fontWeight: FontWeight.w600,
                color: scheme.onSurface,
              ),
            ),
          ),
          IconButton(
            icon: Icon(Icons.close, color: scheme.onSurfaceVariant, semanticLabel: 'Tutup'),
            onPressed: () => Navigator.pop(context),
          ),
        ],
      ),
    );
  }
}

/// Center Pin untuk Map (WhatsApp-style)
class MapCenterPin extends StatelessWidget {
  const MapCenterPin({super.key});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        // Pin icon
        Container(
          decoration: BoxDecoration(
            color: scheme.primary,
            shape: BoxShape.circle,
            boxShadow: [
              BoxShadow(
                color: scheme.primary.withValues(alpha: 0.3),
                blurRadius: 8,
                offset: const Offset(0, 4),
              ),
            ],
          ),
          child: Icon(
            Icons.place,
            color: scheme.onPrimary,
            size: AppIconSize.emphasis,
          ),
        ),
        const SizedBox(height: 4),
        // Shadow untuk depth effect
        Container(
          width: 12,
          height: 6,
          decoration: BoxDecoration(
            color: scheme.primary.withValues(alpha: 0.2),
            shape: BoxShape.circle,
          ),
        ),
      ],
    );
  }
}

/// Search Bar dengan Results untuk Map Picker
class MapSearchBar extends StatelessWidget {
  final TextEditingController controller;
  final bool isSearching;
  final List<PlacePrediction> searchResults;
  final VoidCallback onClear;
  final Function(PlacePrediction) onPlaceSelected;

  const MapSearchBar({
    super.key,
    required this.controller,
    required this.isSearching,
    required this.searchResults,
    required this.onClear,
    required this.onPlaceSelected,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        // Search TextField
        Container(
          decoration: BoxDecoration(
            color: scheme.surfaceContainerHigh,
            borderRadius: BorderRadius.circular(AppShape.r12),
            boxShadow: [
              BoxShadow(
                color: scheme.shadow.withValues(alpha: 0.1),
                blurRadius: 8,
                offset: const Offset(0, 2),
              ),
            ],
          ),
          child: TextField(
            controller: controller,
            decoration: InputDecoration(
              hintText: 'Cari lokasi...',
              hintStyle: TextStyle(color: scheme.onSurfaceVariant),
              prefixIcon: Icon(Icons.search, color: scheme.onSurfaceVariant),
              suffixIcon: controller.text.isNotEmpty
                  ? IconButton(
                      icon: const Icon(Icons.clear, semanticLabel: 'Bersihkan'),
                      onPressed: onClear,
                    )
                  : isSearching
                  ? const Padding(
                      padding: EdgeInsets.all(AppMetrics.p16),
                      child: SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      ),
                    )
                  : null,
              border: InputBorder.none,
              contentPadding: const EdgeInsets.symmetric(
                horizontal: AppMetrics.p16,
                vertical: AppMetrics.p16,
              ),
            ),
            style: Theme.of(
              context,
            ).textTheme.bodyLarge?.copyWith(color: scheme.onSurface),
          ),
        ),

        // Search Results
        if (searchResults.isNotEmpty) ...[
          const SizedBox(height: 8),
          Material(
            color: Colors.transparent,
            child: Container(
              constraints: const BoxConstraints(maxHeight: 300),
              decoration: BoxDecoration(
                color: scheme.surfaceContainerHigh,
                borderRadius: BorderRadius.circular(AppShape.r12),
                boxShadow: [
                  BoxShadow(
                    color: scheme.shadow.withValues(alpha: 0.1),
                    blurRadius: 8,
                    offset: const Offset(0, 2),
                  ),
                ],
              ),
              child: ClipRRect(
                borderRadius: BorderRadius.circular(AppShape.r12),
                child: ListView.separated(
                  shrinkWrap: true,
                  padding: EdgeInsets.zero,
                  physics: const ClampingScrollPhysics(),
                  itemCount: searchResults.length,
                  separatorBuilder: (context, index) =>
                      Divider(height: 1, color: scheme.outlineVariant),
                  itemBuilder: (context, index) {
                    final prediction = searchResults[index];
                    return _SearchResultItem(
                      prediction: prediction,
                      onTap: () => onPlaceSelected(prediction),
                    );
                  },
                ),
              ),
            ),
          ),
        ],
      ],
    );
  }
}

/// Search Result Item
class _SearchResultItem extends StatelessWidget {
  final PlacePrediction prediction;
  final VoidCallback onTap;

  const _SearchResultItem({required this.prediction, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.symmetric(
            horizontal: AppMetrics.p16,
            vertical: AppMetrics.p12,
          ),
          child: Row(
            children: [
              Icon(
                Icons.location_on,
                color: scheme.primary,
                size: AppIconSize.action,
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      prediction.mainText ?? prediction.description,
                      style: context.typeRoles.bodyDense.copyWith(
                        fontWeight: FontWeight.w500,
                        color: scheme.onSurface,
                      ),
                    ),
                    if (prediction.secondaryText != null) ...[
                      const SizedBox(height: 2),
                      Text(
                        prediction.secondaryText!,
                        style: context.typeRoles.labelMicro.copyWith(
                          color: scheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Location Info Card untuk Map Picker
class MapLocationInfoCard extends StatelessWidget {
  final String? address;
  final String? latitude;
  final String? longitude;
  final bool isLoading;
  final bool isDefaultLocation; // true jika menggunakan default location

  const MapLocationInfoCard({
    super.key,
    required this.address,
    required this.latitude,
    required this.longitude,
    required this.isLoading,
    this.isDefaultLocation = false,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHigh,
        borderRadius: BorderRadius.circular(AppShape.r12),
        boxShadow: [
          BoxShadow(
            color: scheme.shadow.withValues(alpha: 0.1),
            blurRadius: 8,
            offset: const Offset(0, 2),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              Icon(
                Icons.place,
                color: scheme.primary,
                size: AppIconSize.action,
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  'Lokasi yang Dipilih',
                  style: context.typeRoles.labelMicro.copyWith(
                    fontWeight: FontWeight.w600,
                    color: scheme.onSurfaceVariant,
                  ),
                ),
              ),
              // Default location warning badge
              if (isDefaultLocation && !isLoading)
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: AppMetrics.p8,
                    vertical: AppMetrics.p4,
                  ),
                  decoration: BoxDecoration(
                    color: scheme.primary.withValues(alpha: 0.1),
                    borderRadius: BorderRadius.circular(AppShape.r6),
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(
                        Icons.warning_amber_rounded,
                        size: AppIconSize.inlineGlyph,
                        color: scheme.primary,
                      ),
                      const SizedBox(width: 4),
                      Text(
                        'Default Location',
                        style: context.typeRoles.labelMicro.copyWith(
                          fontWeight: FontWeight.w500,
                          color: scheme.primary,
                        ),
                      ),
                    ],
                  ),
                ),
            ],
          ),
          const SizedBox(height: 8),
          if (isLoading)
            Row(
              children: [
                SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: scheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(width: 8),
                Text(
                  'Mendapatkan alamat...',
                  style: context.typeRoles.bodyDense.copyWith(
                    color: scheme.onSurfaceVariant,
                  ),
                ),
              ],
            )
          else
            Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                AddressLocationText(
                  location: address ?? 'Pilih lokasi di peta',
                  mode: AddressLocationMode.detail,
                  maxLines: 4,
                  style: context.typeRoles.titleCompact.copyWith(
                    fontWeight: FontWeight.w500,
                    height: 1.4,
                    color: scheme.onSurface,
                  ),
                ),
                if (latitude != null && longitude != null)
                  Padding(
                    padding: const EdgeInsets.only(top: AppMetrics.p8),
                    child: Text(
                      '$latitude, $longitude',
                      style: context.typeRoles.labelMicro.copyWith(
                        fontFamily: 'monospace',
                        color: scheme.onSurfaceVariant,
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

/// Confirm Button untuk Map Picker — content only; chrome (surface,
/// separator, Safe Area, keyboard inset, button height, disabled language) is
/// owned by [BottomActionBar].
class MapConfirmButton extends StatelessWidget {
  final bool canConfirm;
  final VoidCallback onConfirm;

  const MapConfirmButton({
    super.key,
    required this.canConfirm,
    required this.onConfirm,
  });

  @override
  Widget build(BuildContext context) {
    return BottomActionBar(
      primary: BottomBarAction(
        label: 'Pilih Lokasi Ini',
        onPressed: canConfirm ? onConfirm : null,
      ),
    );
  }
}
