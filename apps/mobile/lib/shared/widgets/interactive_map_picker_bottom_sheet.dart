import 'dart:async';
import 'package:flutter/material.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart' as gmaps;
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/entities/post_location.dart';
import 'package:labuda/shared/services/places_autocomplete_service.dart';
import 'package:labuda/shared/services/location_service.dart';
import 'package:labuda/shared/services/logger_service.dart';
import 'package:labuda/shared/widgets/map_picker/map_picker_widgets.dart';
import 'package:labuda/shared/widgets/map_picker/map_picker_handlers.dart';

/// Interactive Map Picker dengan draggable pin (WhatsApp-style)
///
/// Features:
/// - Interactive Google Map dengan center pin
/// - Search bar untuk cari tempat (Google Places)
/// - Drag map untuk ubah posisi pin
/// - Live coordinate display
/// - Reverse geocoding untuk dapat address
class InteractiveMapPickerBottomSheet extends StatefulWidget {
  final PostLocation? initialLocation;
  final String? googleApiKey;

  const InteractiveMapPickerBottomSheet({
    super.key,
    this.initialLocation,
    this.googleApiKey,
  });

  /// Show bottom sheet dan return selected location
  static Future<PostLocation?> show({
    required BuildContext context,
    PostLocation? initialLocation,
    String? googleApiKey,
  }) async {
    return showModalBottomSheet<PostLocation>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      isDismissible: true,
      enableDrag: false,
      builder: (context) => InteractiveMapPickerBottomSheet(
        initialLocation: initialLocation,
        googleApiKey: googleApiKey,
      ),
    );
  }

  @override
  State<InteractiveMapPickerBottomSheet> createState() =>
      _InteractiveMapPickerBottomSheetState();
}

class _InteractiveMapPickerBottomSheetState
    extends State<InteractiveMapPickerBottomSheet>
    with MapPickerHandlers {
  gmaps.GoogleMapController? _mapController;
  final TextEditingController _searchController = TextEditingController();
  PlacesAutocompleteService? _placesService;
  LocationService? _locationService;
  List<PlacePrediction> _searchResults = [];
  bool _isSearching = false;
  bool _isLoadingInitialLocation = true;

  // Default location: Jakarta, Indonesia
  static const gmaps.LatLng _defaultLocation = gmaps.LatLng(-6.2088, 106.8456);

  gmaps.LatLng? _selectedLocation;
  String? _selectedAddress;
  bool _isLoadingAddress = false;
  bool _addressFromSearch = false;

  // Location status tracking
  bool _isDefaultLocation = false;

  @override
  void initState() {
    super.initState();

    // Initialize services
    if (widget.googleApiKey != null && widget.googleApiKey!.isNotEmpty) {
      _placesService = PlacesAutocompleteService(widget.googleApiKey!);
    }
    _locationService = LocationService(logger: LoggerService.instance);

    // Listen to search input
    _searchController.addListener(onSearchChanged);

    // Initialize location asynchronously
    _initializeLocation();
  }

  @override
  void dispose() {
    _mapController?.dispose();
    _searchController.dispose();
    super.dispose();
  }

  Future<void> _initializeLocation() async {
    setState(() => _isLoadingInitialLocation = true);

    try {
      // Priority 1: Use initialLocation if provided
      if (widget.initialLocation?.hasCoordinates == true) {
        _selectedLocation = gmaps.LatLng(
          widget.initialLocation!.latitude!,
          widget.initialLocation!.longitude!,
        );
        _selectedAddress = widget.initialLocation!.address;
        _isDefaultLocation = false;
        if (mounted) {
          setState(() => _isLoadingInitialLocation = false);
        }
        // Animate camera ke initial location setelah map ready
        _animateToLocationAfterDelay();
        return;
      }

      // Priority 2: Get current location
      final locationWithAccuracy = await _locationService!
          .getInitialLocationForMap();

      if (locationWithAccuracy != null && mounted) {
        _selectedLocation = gmaps.LatLng(
          locationWithAccuracy.latitude,
          locationWithAccuracy.longitude,
        );
        _isDefaultLocation = locationWithAccuracy.isDefault;

        setState(() => _isLoadingInitialLocation = false);

        // Animate camera ke lokasi yang didapat
        await _animateToSelectedLocation();
        await getAddressFromLatLng(_selectedLocation!);
      }
    } catch (e) {
      if (mounted) {
        _selectedLocation = _defaultLocation;
        _isDefaultLocation = true;
        setState(() => _isLoadingInitialLocation = false);
        await getAddressFromLatLng(_defaultLocation);
      }
    }
  }

  /// Animate camera ke selected location
  /// Perlu delay sebentar karena map controller belum siap
  Future<void> _animateToSelectedLocation() async {
    if (_selectedLocation == null) return;

    // Tunggu sebentar untuk map controller siap
    await Future.delayed(const Duration(milliseconds: 300));

    if (_mapController != null && mounted) {
      await _mapController!.animateCamera(
        gmaps.CameraUpdate.newLatLngZoom(_selectedLocation!, 16),
      );
    }
  }

  /// Animate ke location setelah delay (untuk initialLocation case)
  void _animateToLocationAfterDelay() {
    Future.delayed(const Duration(milliseconds: 500), () {
      if (_mapController != null && _selectedLocation != null && mounted) {
        _mapController!.animateCamera(
          gmaps.CameraUpdate.newLatLngZoom(_selectedLocation!, 16),
        );
      }
    });
  }

  void _onConfirm() {
    if (_selectedLocation != null) {
      Navigator.pop(
        context,
        PostLocation(
          address: _selectedAddress ?? '',
          latitude: _selectedLocation!.latitude,
          longitude: _selectedLocation!.longitude,
        ),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final mediaQuery = MediaQuery.of(context);
    final bottomPadding = mediaQuery.padding.bottom;

    return Container(
      height: mediaQuery.size.height * 0.9,
      decoration: BoxDecoration(
        color: theme.brightness == Brightness.dark
            ? AppColors.darkGray800
            : AppColors.neutralWhite,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(16)),
      ),
      child: Column(
        children: [
          // Header
          const MapPickerHeader(),

          // Map with search overlay
          Expanded(
            child: Stack(
              children: [
                // Google Map
                _buildMap(),

                // Loading indicator
                if (_isLoadingInitialLocation)
                  const Center(child: _InitialLoadingIndicator())
                else ...[
                  // Center Pin (fixed in center)
                  const Positioned.fill(
                    child: Center(child: IgnorePointer(child: MapCenterPin())),
                  ),

                  // Search Bar (floating)
                  Positioned(
                    top: 16,
                    left: 16,
                    right: 16,
                    child: MapSearchBar(
                      controller: _searchController,
                      isSearching: _isSearching,
                      searchResults: _searchResults,
                      onClear: () {
                        _searchController.clear();
                        setSearchResults([]);
                      },
                      onPlaceSelected: onSearchPlaceSelected,
                    ),
                  ),

                  // Default location warning
                  if (_isDefaultLocation)
                    Positioned(
                      top: 80,
                      left: 16,
                      right: 16,
                      child: _DefaultLocationBanner(
                        onRetry: _initializeLocation,
                      ),
                    ),

                  // My Location button
                  Positioned(
                    right: 16,
                    bottom: 100,
                    child: _buildCurrentLocationButton(context),
                  ),

                  // Location info card
                  Positioned(
                    bottom: 16,
                    left: 16,
                    right: 16,
                    child: MapLocationInfoCard(
                      address: _selectedAddress,
                      latitude: _selectedLocation?.latitude.toStringAsFixed(6),
                      longitude: _selectedLocation?.longitude.toStringAsFixed(
                        6,
                      ),
                      isLoading: _isLoadingAddress,
                      isDefaultLocation: _isDefaultLocation,
                    ),
                  ),
                ],
              ],
            ),
          ),

          // Confirm button
          MapConfirmButton(
            canConfirm: _selectedLocation != null && !_isLoadingAddress,
            onConfirm: _onConfirm,
            bottomPadding: bottomPadding,
          ),
        ],
      ),
    );
  }

  Widget _buildMap() {
    return gmaps.GoogleMap(
      initialCameraPosition: gmaps.CameraPosition(
        target: _selectedLocation ?? _defaultLocation,
        zoom: 16,
      ),
      onMapCreated: (controller) => _mapController = controller,
      onCameraMove: onCameraMove,
      onCameraIdle: onCameraIdle,
      onCameraMoveStarted: () {
        // Hanya reset jika sudah lebih dari 500ms sejak search
        Future.delayed(const Duration(milliseconds: 500), () {
          if (mounted) {
            setAddressFromSearch(false);
          }
        });
      },
      myLocationButtonEnabled: false,
      zoomControlsEnabled: false,
      mapToolbarEnabled: false,
      compassEnabled: false,
      mapType: gmaps.MapType.normal,
    );
  }

  Widget _buildCurrentLocationButton(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return FloatingActionButton(
      mini: true,
      backgroundColor: isDark ? AppColors.darkGray700 : AppColors.neutralWhite,
      onPressed: recenterToCurrentLocation,
      child: Icon(Icons.my_location, color: AppColors.primaryRed, size: 20),
    );
  }

  // MapPickerHandlers implementation
  @override
  gmaps.GoogleMapController? get mapController => _mapController;

  @override
  TextEditingController get searchController => _searchController;

  @override
  PlacesAutocompleteService? get placesService => _placesService;

  @override
  LocationService? get locationService => _locationService;

  @override
  List<PlacePrediction> get searchResults => _searchResults;

  @override
  bool get isSearching => _isSearching;

  @override
  bool get addressFromSearch => _addressFromSearch;

  @override
  gmaps.LatLng? get selectedLocation => _selectedLocation;

  @override
  String? get googleApiKey => widget.googleApiKey;

  @override
  void setSearchResults(List<PlacePrediction> results) {
    setState(() => _searchResults = results);
  }

  @override
  void setSearching(bool searching) {
    setState(() => _isSearching = searching);
  }

  @override
  void setSelectedLocation(gmaps.LatLng? location) {
    setState(() => _selectedLocation = location);
  }

  @override
  void setSelectedAddress(String? address) {
    setState(() => _selectedAddress = address);
  }

  @override
  void setAddressFromSearch(bool value) {
    setState(() => _addressFromSearch = value);
  }

  @override
  void setIsLoadingAddress(bool loading) {
    setState(() => _isLoadingAddress = loading);
  }

  @override
  void setIsDefaultLocation(bool value) {
    setState(() => _isDefaultLocation = value);
  }
}

/// Initial Loading Indicator
class _InitialLoadingIndicator extends StatelessWidget {
  const _InitialLoadingIndicator();

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      padding: const EdgeInsets.all(20),
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
        mainAxisSize: MainAxisSize.min,
        children: [
          const CircularProgressIndicator(color: AppColors.primaryRed),
          const SizedBox(height: 12),
          Text(
            'Mendapatkan lokasi...',
            style: TextStyle(
              fontSize: 14,
              color: isDark ? AppColors.neutralWhite : AppColors.neutralBlack,
            ),
          ),
        ],
      ),
    );
  }
}

/// Default Location Banner - warning saat menggunakan default location
class _DefaultLocationBanner extends StatelessWidget {
  final VoidCallback onRetry;

  const _DefaultLocationBanner({required this.onRetry});

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: AppColors.primaryRed.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: AppColors.primaryRed.withValues(alpha: 0.4),
          width: 1,
        ),
      ),
      child: Row(
        children: [
          Icon(
            Icons.warning_amber_rounded,
            color: AppColors.primaryRed,
            size: 18,
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'GPS Tidak Terdeteksi',
                  style: TextStyle(
                    fontSize: 12,
                    fontWeight: FontWeight.w600,
                    color: isDark
                        ? AppColors.neutralWhite
                        : AppColors.neutralGray900,
                  ),
                ),
                Text(
                  'Menggunakan lokasi default (Jakarta)',
                  style: TextStyle(
                    fontSize: 11,
                    color: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray600,
                  ),
                ),
              ],
            ),
          ),
          TextButton(
            onPressed: onRetry,
            style: TextButton.styleFrom(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
              tapTargetSize: MaterialTapTargetSize.shrinkWrap,
            ),
            child: const Text('Coba Lagi', style: TextStyle(fontSize: 12)),
          ),
        ],
      ),
    );
  }
}
