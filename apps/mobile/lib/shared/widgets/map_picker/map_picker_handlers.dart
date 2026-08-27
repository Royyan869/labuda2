import 'package:flutter/material.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart' as gmaps;
import 'package:geocoding/geocoding.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/services/places_autocomplete_service.dart';
import 'package:labuda/shared/services/location_service.dart';
import 'package:labuda/shared/widgets/map_picker/address_formatter.dart';

/// Handler mixin untuk Map Picker logic
mixin MapPickerHandlers<T extends StatefulWidget> on State<T> {
  /// Getters untuk state (harus di-override)
  gmaps.GoogleMapController? get mapController;
  TextEditingController get searchController;
  PlacesAutocompleteService? get placesService;
  LocationService? get locationService;
  List<PlacePrediction> get searchResults;
  bool get isSearching;
  bool get addressFromSearch;
  String? get googleApiKey;
  gmaps.LatLng? get selectedLocation;

  /// Setters untuk state (harus di-override)
  void setSearchResults(List<PlacePrediction> results);
  void setSearching(bool searching);
  void setSelectedLocation(gmaps.LatLng? location);
  void setSelectedAddress(String? address);
  void setAddressFromSearch(bool value);
  void setIsLoadingAddress(bool loading);

  /// Default location setter (optional untuk tracking)
  void setIsDefaultLocation(bool value) {}

  /// Handle search input change
  void onSearchChanged() {
    final query = searchController.text.trim();

    // Clear results if empty
    if (query.isEmpty) {
      setSearchResults([]);
      setSearching(false);
      return;
    }

    // Minimum 3 characters untuk search
    if (query.length < 3) {
      setSearchResults([]);
      setSearching(false);
      return;
    }

    setSearching(true);

    placesService?.searchWithDebounce(query, (results) {
      if (mounted) {
        setSearchResults(results);
        setSearching(false);
      }
    });
  }

  /// Handle reverse geocoding
  Future<void> getAddressFromLatLng(gmaps.LatLng position) async {
    setIsLoadingAddress(true);

    try {
      final placemarks = await placemarkFromCoordinates(
        position.latitude,
        position.longitude,
      );

      if (placemarks.isNotEmpty && mounted) {
        final place = placemarks.first;
        final address = AddressFormatter.formatFromPlacemark(place);

        setSelectedAddress(
          address.isNotEmpty
              ? address
              : 'Lat: ${position.latitude.toStringAsFixed(6)}, Lng: ${position.longitude.toStringAsFixed(6)}',
        );
        setIsLoadingAddress(false);
      }
    } catch (e) {
      if (mounted) {
        setSelectedAddress(
          'Lat: ${position.latitude.toStringAsFixed(6)}, '
          'Lng: ${position.longitude.toStringAsFixed(6)}',
        );
        setIsLoadingAddress(false);
      }
    }
  }

  /// Handle search place selected
  Future<void> onSearchPlaceSelected(PlacePrediction prediction) async {
    if (googleApiKey == null || googleApiKey!.isEmpty) return;

    // Get place details
    final details = await placesService?.getPlaceDetails(prediction.placeId);
    if (details == null || mounted == false) return;

    final location = gmaps.LatLng(details.latitude, details.longitude);

    final address = AddressFormatter.formatFromPlaceDetails(details);

    setSelectedLocation(location);
    setSelectedAddress(address);
    searchController.clear();
    setSearchResults([]);
    setAddressFromSearch(true);

    mapController?.animateCamera(
      gmaps.CameraUpdate.newLatLngZoom(location, 16),
    );
  }

  /// Handle camera move
  void onCameraMove(gmaps.CameraPosition position) {
    setSelectedLocation(position.target);
  }

  /// Handle camera idle (reverse geocode)
  void onCameraIdle() {
    if (addressFromSearch) {
      // Skip reverse geocoding jika address dari search
      // Reset flag setelah delay
      Future.delayed(const Duration(milliseconds: 500), () {
        if (mounted) {
          setAddressFromSearch(false);
        }
      });
      return;
    }

    // Get current camera position through selectedLocation state
    if (selectedLocation != null) {
      getAddressFromLatLng(selectedLocation!);
    }
  }

  /// Handle re-center to current location
  Future<void> recenterToCurrentLocation() async {
    if (locationService == null) return;

    try {
      // Gunakan getInitialLocationForMap untuk dapat location
      final locationWithAccuracy = await locationService!
          .getInitialLocationForMap();

      if (locationWithAccuracy != null && mounted) {
        final location = gmaps.LatLng(
          locationWithAccuracy.latitude,
          locationWithAccuracy.longitude,
        );

        setSelectedLocation(location);
        setAddressFromSearch(false);
        setIsDefaultLocation(locationWithAccuracy.isDefault);

        mapController?.animateCamera(
          gmaps.CameraUpdate.newLatLngZoom(location, 16),
        );

        await getAddressFromLatLng(location);

        // Show warning jika menggunakan default location
        if (locationWithAccuracy.isDefault && mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(
              content: Text(
                'GPS tidak terdeteksi. Menggunakan lokasi default.',
              ),
              duration: Duration(seconds: 3),
              backgroundColor: AppColors.primaryRed,
            ),
          );
        }
      } else if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Gagal mendapatkan lokasi'),
            duration: Duration(seconds: 4),
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Gagal mendapatkan lokasi saat ini'),
            duration: Duration(seconds: 4),
          ),
        );
      }
    }
  }
}
