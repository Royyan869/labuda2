import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/services/places_autocomplete_service.dart';

/// Header untuk Map Picker
class MapPickerHeader extends StatelessWidget {
  const MapPickerHeader({super.key});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        border: Border(
          bottom: BorderSide(
            color: isDark ? AppColors.neutralGray700 : AppColors.neutralGray200,
          ),
        ),
      ),
      child: Row(
        children: [
          Expanded(
            child: Text(
              'Pilih Lokasi',
              style: TextStyle(
                fontSize: 18,
                fontWeight: FontWeight.w600,
                color: isDark ? AppColors.neutralWhite : AppColors.neutralBlack,
              ),
            ),
          ),
          IconButton(
            icon: Icon(
              Icons.close,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
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
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        // Pin icon
        Container(
          decoration: BoxDecoration(
            color: AppColors.primaryRed,
            shape: BoxShape.circle,
            boxShadow: [
              BoxShadow(
                color: AppColors.primaryRed.withValues(alpha: 0.3),
                blurRadius: 8,
                offset: const Offset(0, 4),
              ),
            ],
          ),
          child: const Icon(Icons.place, color: Colors.white, size: 36),
        ),
        const SizedBox(height: 4),
        // Shadow untuk depth effect
        Container(
          width: 12,
          height: 6,
          decoration: BoxDecoration(
            color: AppColors.primaryRed.withValues(alpha: 0.2),
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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        // Search TextField
        Container(
          decoration: BoxDecoration(
            color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
            borderRadius: BorderRadius.circular(12),
            boxShadow: [
              BoxShadow(
                color: Colors.black.withValues(alpha: 0.1),
                blurRadius: 8,
                offset: const Offset(0, 2),
              ),
            ],
          ),
          child: TextField(
            controller: controller,
            decoration: InputDecoration(
              hintText: 'Cari lokasi...',
              hintStyle: TextStyle(
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray500,
              ),
              prefixIcon: Icon(
                Icons.search,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
              ),
              suffixIcon: controller.text.isNotEmpty
                  ? IconButton(
                      icon: const Icon(Icons.clear),
                      onPressed: onClear,
                    )
                  : isSearching
                  ? const Padding(
                      padding: EdgeInsets.all(14),
                      child: SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      ),
                    )
                  : null,
              border: InputBorder.none,
              contentPadding: const EdgeInsets.symmetric(
                horizontal: 16,
                vertical: 14,
              ),
            ),
            style: TextStyle(
              fontSize: 14,
              color: isDark ? AppColors.neutralWhite : AppColors.neutralBlack,
            ),
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
                color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
                borderRadius: BorderRadius.circular(12),
                boxShadow: [
                  BoxShadow(
                    color: Colors.black.withValues(alpha: 0.1),
                    blurRadius: 8,
                    offset: const Offset(0, 2),
                  ),
                ],
              ),
              child: ClipRRect(
                borderRadius: BorderRadius.circular(12),
                child: ListView.separated(
                  shrinkWrap: true,
                  padding: EdgeInsets.zero,
                  physics: const ClampingScrollPhysics(),
                  itemCount: searchResults.length,
                  separatorBuilder: (context, index) => Divider(
                    height: 1,
                    color: isDark
                        ? AppColors.neutralGray600
                        : AppColors.neutralGray200,
                  ),
                  itemBuilder: (context, index) {
                    final prediction = searchResults[index];
                    return _SearchResultItem(
                      prediction: prediction,
                      isDark: isDark,
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
  final bool isDark;
  final VoidCallback onTap;

  const _SearchResultItem({
    required this.prediction,
    required this.isDark,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          child: Row(
            children: [
              Icon(Icons.location_on, color: AppColors.primaryRed, size: 20),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      prediction.mainText ?? prediction.description,
                      style: TextStyle(
                        fontSize: 14,
                        fontWeight: FontWeight.w500,
                        color: isDark
                            ? AppColors.neutralWhite
                            : AppColors.neutralBlack,
                      ),
                    ),
                    if (prediction.secondaryText != null) ...[
                      const SizedBox(height: 2),
                      Text(
                        prediction.secondaryText!,
                        style: TextStyle(
                          fontSize: 12,
                          color: isDark
                              ? AppColors.neutralGray400
                              : AppColors.neutralGray600,
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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.1),
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
              Icon(Icons.place, color: AppColors.primaryRed, size: 20),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  'Lokasi yang Dipilih',
                  style: TextStyle(
                    fontSize: 12,
                    fontWeight: FontWeight.w600,
                    color: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray600,
                  ),
                ),
              ),
              // Default location warning badge
              if (isDefaultLocation && !isLoading)
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 8,
                    vertical: 4,
                  ),
                  decoration: BoxDecoration(
                    color: AppColors.primaryRed.withValues(alpha: 0.1),
                    borderRadius: BorderRadius.circular(6),
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(
                        Icons.warning_amber_rounded,
                        size: 12,
                        color: AppColors.primaryRed,
                      ),
                      const SizedBox(width: 4),
                      Text(
                        'Default Location',
                        style: TextStyle(
                          fontSize: 10,
                          fontWeight: FontWeight.w500,
                          color: AppColors.primaryRed,
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
                    color: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray600,
                  ),
                ),
                const SizedBox(width: 8),
                Text(
                  'Mendapatkan alamat...',
                  style: TextStyle(
                    fontSize: 14,
                    color: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray600,
                  ),
                ),
              ],
            )
          else
            Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  address ?? 'Pilih lokasi di peta',
                  style: TextStyle(
                    fontSize: 15,
                    fontWeight: FontWeight.w500,
                    height: 1.4,
                    color: isDark
                        ? AppColors.neutralWhite
                        : AppColors.neutralBlack,
                  ),
                  maxLines: 4,
                  overflow: TextOverflow.ellipsis,
                ),
                if (latitude != null && longitude != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 8),
                    child: Text(
                      '$latitude, $longitude',
                      style: TextStyle(
                        fontSize: 11,
                        fontFamily: 'monospace',
                        color: isDark
                            ? AppColors.neutralGray400
                            : AppColors.neutralGray600,
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

/// Confirm Button untuk Map Picker
class MapConfirmButton extends StatelessWidget {
  final bool canConfirm;
  final VoidCallback onConfirm;
  final double bottomPadding;

  const MapConfirmButton({
    super.key,
    required this.canConfirm,
    required this.onConfirm,
    required this.bottomPadding,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      padding: EdgeInsets.fromLTRB(16, 12, 16, 12 + bottomPadding),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        border: Border(
          top: BorderSide(
            color: isDark ? AppColors.neutralGray700 : AppColors.neutralGray200,
          ),
        ),
      ),
      child: SizedBox(
        width: double.infinity,
        child: FilledButton(
          onPressed: canConfirm ? onConfirm : null,
          style: FilledButton.styleFrom(
            backgroundColor: AppColors.primaryRed,
            padding: const EdgeInsets.symmetric(vertical: 14),
          ),
          child: const Text(
            'Pilih Lokasi Ini',
            style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
          ),
        ),
      ),
    );
  }
}
