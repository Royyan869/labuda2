import 'dart:async';
import 'package:flutter/material.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart' as gmaps;
import 'package:hishumi/shared/entities/post_location.dart';
import 'package:hishumi/shared/services/places_autocomplete_service.dart';
import 'package:hishumi/shared/services/location_service.dart';
import 'package:hishumi/shared/services/logger_service.dart';
import 'package:hishumi/shared/widgets/app_bottom_sheet_base.dart';
import 'package:hishumi/shared/widgets/map_picker/map_picker_widgets.dart';
import 'package:hishumi/shared/widgets/map_picker/map_picker_handlers.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';

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
    // GENUINE BESPOKE EXCEPTION — the interactive map needs its own gesture
    // model (pan/pinch the map), which conflicts with the sheet's drag. It still
    // obeys the canonical presentation contract: modal, scroll-controlled,
    // surface/shape/elevation from `bottomSheetTheme`, usable height from the
    // canonical ceiling (`AppBottomSheetBase.availableHeight`), keyboard lift
    // owned by `BottomActionBar`, drag disabled, and no local surface/radius.
    return showModalBottomSheet<PostLocation>(
      context: context,
      isScrollControlled: true,
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
    await Future.delayed(AppMotion.settled);

    if (_mapController != null && mounted) {
      await _mapController!.animateCamera(
        gmaps.CameraUpdate.newLatLngZoom(_selectedLocation!, 16),
      );
    }
  }

  /// Animate ke location setelah delay (untuk initialLocation case)
  void _animateToLocationAfterDelay() {
    Future.delayed(AppMotion.slow, () {
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
    // Finite height from the canonical usable-height ceiling
    // ([AppBottomSheetBase.availableHeight]: window minus the keyboard minus
    // the system top inset), keeping the 0.9 product factor. A fraction of the
    // raw screen height would go stale the moment the search field opens the
    // keyboard, leaving the sheet taller than the space above it. Keyboard
    // lift inside the body stays owned by `BottomActionBar`; no second
    // inset math lives here.
    // Surface + top-r20 shape come from `bottomSheetTheme`.
    return SizedBox(
      height: AppBottomSheetBase.availableHeight(context) * 0.9,
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

          // Confirm button — chrome owned by BottomActionBar.
          MapConfirmButton(
            canConfirm: _selectedLocation != null && !_isLoadingAddress,
            onConfirm: _onConfirm,
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
        Future.delayed(AppMotion.slow, () {
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
    final scheme = Theme.of(context).colorScheme;

    return FloatingActionButton(
      mini: true,
      backgroundColor: scheme.surfaceContainerHigh,
      onPressed: recenterToCurrentLocation,
      child: Icon(
        Icons.my_location,
        color: scheme.primary,
        size: AppIconSize.action,
      ),
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
    final scheme = Theme.of(context).colorScheme;

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p24),
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
        mainAxisSize: MainAxisSize.min,
        children: [
          CircularProgressIndicator(color: scheme.primary),
          const SizedBox(height: 12),
          Text(
            'Mendapatkan lokasi...',
            style: context.typeRoles.bodyDense.copyWith(
              color: scheme.onSurface,
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
    final scheme = Theme.of(context).colorScheme;

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: scheme.error.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(
          color: scheme.error.withValues(alpha: 0.4),
          width: 1,
        ),
      ),
      child: Row(
        children: [
          Icon(
            Icons.warning_amber_rounded,
            color: scheme.error,
            size: AppIconSize.action,
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'GPS Tidak Terdeteksi',
                  style: context.typeRoles.labelMicro.copyWith(
                    fontWeight: FontWeight.w600,
                    color: scheme.onSurface,
                  ),
                ),
                Text(
                  'Menggunakan lokasi default (Jakarta)',
                  style: context.typeRoles.labelMicro.copyWith(
                    color: scheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ),
          TextButton(
            onPressed: onRetry,
            style: TextButton.styleFrom(
              padding: const EdgeInsets.symmetric(
                horizontal: AppMetrics.p12,
                vertical: AppMetrics.p8,
              ),
              tapTargetSize: MaterialTapTargetSize.shrinkWrap,
            ),
            child: Text(
              'Coba Lagi',
              style: Theme.of(context).textTheme.labelLarge,
            ),
          ),
        ],
      ),
    );
  }
}
