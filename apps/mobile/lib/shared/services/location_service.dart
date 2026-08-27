import 'dart:io' show Platform;
import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:geolocator/geolocator.dart';
import 'package:geocoding/geocoding.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:labuda/core/core.dart';

/// Service untuk handle GPS location dan reverse geocoding
class LocationService {
  final ILoggerService _logger;

  // SharedPreferences keys untuk last known location
  static const String _keyLastLatitude = 'last_known_latitude';
  static const String _keyLastLongitude = 'last_known_longitude';

  const LocationService({required ILoggerService logger}) : _logger = logger;

  /// Save last known location ke SharedPreferences
  Future<void> _saveLastKnownLocation(double latitude, double longitude) async {
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.setDouble(_keyLastLatitude, latitude);
      await prefs.setDouble(_keyLastLongitude, longitude);
      await _logger.info(
        'Last known location saved',
        extra: {'latitude': latitude, 'longitude': longitude},
      );
    } catch (e) {
      await _logger.error(
        'Error saving last known location',
        extra: {'error': e},
      );
    }
  }

  /// Load last known location dari SharedPreferences
  /// Returns null jika belum pernah save
  Future<({double latitude, double longitude})?> getLastKnownLocation() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      final latitude = prefs.getDouble(_keyLastLatitude);
      final longitude = prefs.getDouble(_keyLastLongitude);

      if (latitude != null && longitude != null) {
        await _logger.info(
          'Last known location loaded',
          extra: {'latitude': latitude, 'longitude': longitude},
        );
        return (latitude: latitude, longitude: longitude);
      }

      return null;
    } catch (e) {
      await _logger.error(
        'Error loading last known location',
        extra: {'error': e},
      );
      return null;
    }
  }

  /// Get initial location untuk map
  /// Priority:
  /// 1. Current location (jika permission granted)
  /// 2. Last known location (dari SharedPreferences)
  /// 3. Default location (Jakarta)
  ///
  /// Returns: LocationWithAccuracy atau null jika gagal total
  Future<LocationWithAccuracy?> getInitialLocationForMap({
    Duration timeout = const Duration(seconds: 5),
  }) async {
    await _logger.info('Getting initial location for map');

    // Try get current location first
    final positionResult = await getCurrentPosition();
    if (positionResult.isSuccess) {
      final position = positionResult.data!;
      return LocationWithAccuracy(
        latitude: position.latitude,
        longitude: position.longitude,
        accuracy: position.accuracy,
        isDefault: false,
      );
    }

    // Fallback to last known location
    final lastKnown = await getLastKnownLocation();
    if (lastKnown != null) {
      await _logger.info('Using last known location');
      return LocationWithAccuracy(
        latitude: lastKnown.latitude,
        longitude: lastKnown.longitude,
        accuracy: null, // Unknown accuracy untuk last known
        isDefault: false,
        isLastKnown: true,
      );
    }

    // Final fallback to Jakarta default
    await _logger.info('Using default location (Jakarta)');
    return LocationWithAccuracy(
      latitude: -6.2088,
      longitude: 106.8456,
      accuracy: null,
      isDefault: true,
    );
  }

  /// Check if platform supports GPS
  bool isPlatformSupported() {
    if (kIsWeb) {
      return true; // Web supports through browser geolocation API
    }
    if (Platform.isAndroid || Platform.isIOS) {
      return true; // Mobile platforms have GPS
    }
    return false; // Desktop platforms (Windows, macOS, Linux) don't have GPS
  }

  /// Check location permissions dan get current position
  Future<Result<Position>> getCurrentPosition() async {
    try {
      await _logger.info('Getting current position - START');

      // Check if location services are enabled
      bool serviceEnabled = await Geolocator.isLocationServiceEnabled();
      await _logger.info(
        'Location service enabled',
        extra: {'enabled': serviceEnabled},
      );
      if (!serviceEnabled) {
        await _logger.error('Location services disabled');
        return Result.error(
          'Location services tidak aktif. Aktifkan GPS terlebih dahulu.',
        );
      }

      // Check permissions
      LocationPermission permission = await Geolocator.checkPermission();
      await _logger.info(
        'Location permission',
        extra: {'permission': permission.toString()},
      );
      if (permission == LocationPermission.denied) {
        permission = await Geolocator.requestPermission();
        await _logger.info(
          'Location permission after request',
          extra: {'permission': permission.toString()},
        );
        if (permission == LocationPermission.denied) {
          return Result.error(
            'Izin lokasi ditolak. Aktifkan izin lokasi untuk menggunakan fitur ini.',
          );
        }
      }

      if (permission == LocationPermission.deniedForever) {
        return Result.error(
          'Izin lokasi ditolak permanen. Aktifkan di Settings untuk menggunakan fitur ini.',
        );
      }

      await _logger.info(
        'Calling Geolocator.getCurrentPosition with HIGH accuracy',
      );

      // Get current position dengan HIGH accuracy untuk pinpoint location
      Position position = await Geolocator.getCurrentPosition(
        locationSettings: const LocationSettings(
          accuracy: LocationAccuracy.high,
          timeLimit: Duration(seconds: 30),
          distanceFilter: 0, // Get semua update, jangan filter
        ),
      );

      await _logger.info(
        'Location obtained successfully',
        extra: {
          'latitude': position.latitude,
          'longitude': position.longitude,
          'accuracy': position.accuracy,
          'altitude': position.altitude,
          'timestamp': position.timestamp.toIso8601String(),
        },
      );

      // Save as last known location
      await _saveLastKnownLocation(position.latitude, position.longitude);

      return Result.success(position);
    } catch (e) {
      await _logger.error(
        'Failed to get current position',
        extra: {'error': e.toString(), 'errorType': e.runtimeType.toString()},
      );
      return Result.error('Gagal mendapatkan lokasi: ${e.toString()}');
    }
  }

  /// Convert coordinates ke address menggunakan reverse geocoding
  Future<Result<String>> getAddressFromCoordinates(
    double latitude,
    double longitude,
  ) async {
    try {
      List<Placemark> placemarks = await placemarkFromCoordinates(
        latitude,
        longitude,
      );

      if (placemarks.isEmpty) {
        return Result.error('Tidak dapat menemukan alamat untuk lokasi ini');
      }

      final placemark = placemarks.first;

      // Build address string prioritizing Indonesian format
      String address = '';

      if (placemark.subLocality?.isNotEmpty == true) {
        address += placemark.subLocality!;
      } else if (placemark.locality?.isNotEmpty == true) {
        address += placemark.locality!;
      }

      if (placemark.administrativeArea?.isNotEmpty == true) {
        if (address.isNotEmpty) address += ', ';
        address += placemark.administrativeArea!;
      }

      if (placemark.country?.isNotEmpty == true) {
        if (address.isNotEmpty) address += ', ';
        address += placemark.country!;
      }

      if (address.isEmpty) {
        address =
            '${latitude.toStringAsFixed(4)}, ${longitude.toStringAsFixed(4)}';
      }

      await _logger.info(
        'Address resolved successfully',
        extra: {
          'address': address,
          'placemark': {
            'locality': placemark.locality,
            'subLocality': placemark.subLocality,
            'administrativeArea': placemark.administrativeArea,
            'country': placemark.country,
          },
        },
      );

      return Result.success(address);
    } catch (e) {
      await _logger.error(
        'Failed to get address from coordinates',
        extra: {
          'error': e.toString(),
          'latitude': latitude,
          'longitude': longitude,
        },
      );
      return Result.error('Gagal mengambil alamat: ${e.toString()}');
    }
  }

  /// Get current location dengan address - supports all platforms
  Future<Result<LocationInfo>> getCurrentLocation() async {
    try {
      // Check platform support first
      if (!isPlatformSupported()) {
        // Return mock location for desktop development
        await _logger.info('Using mock location for desktop platform');
        return Result.success(
          LocationInfo(
            address: 'Jakarta, Indonesia (Mock Location)',
            latitude: -6.2088,
            longitude: 106.8456,
          ),
        );
      }

      final positionResult = await getCurrentPosition();
      if (positionResult.isError) {
        return Result.error(positionResult.error!);
      }

      final position = positionResult.data!;
      final addressResult = await getAddressFromCoordinates(
        position.latitude,
        position.longitude,
      );

      if (addressResult.isError) {
        // Return coordinates as fallback
        final fallbackAddress =
            '${position.latitude.toStringAsFixed(4)}, ${position.longitude.toStringAsFixed(4)}';
        return Result.success(
          LocationInfo(
            address: fallbackAddress,
            latitude: position.latitude,
            longitude: position.longitude,
          ),
        );
      }

      return Result.success(
        LocationInfo(
          address: addressResult.data!,
          latitude: position.latitude,
          longitude: position.longitude,
        ),
      );
    } catch (e) {
      await _logger.error(
        'Failed to get current location',
        extra: {'error': e.toString()},
      );
      return Result.error('Gagal mendapatkan lokasi saat ini: ${e.toString()}');
    }
  }

  /// Get mock locations for testing/development
  static List<LocationInfo> getMockLocations() {
    return [
      LocationInfo(
        address: 'Jakarta Selatan, Jakarta, Indonesia',
        latitude: -6.2615,
        longitude: 106.8106,
      ),
      LocationInfo(
        address: 'Bandung, Jawa Barat, Indonesia',
        latitude: -6.9175,
        longitude: 107.6191,
      ),
      LocationInfo(
        address: 'Surabaya, Jawa Timur, Indonesia',
        latitude: -7.2575,
        longitude: 112.7521,
      ),
      LocationInfo(
        address: 'Yogyakarta, DIY, Indonesia',
        latitude: -7.7956,
        longitude: 110.3695,
      ),
      LocationInfo(
        address: 'Bali, Indonesia',
        latitude: -8.3405,
        longitude: 115.0920,
      ),
    ];
  }
}

/// Model untuk informasi lokasi
class LocationInfo {
  final String address;
  final double latitude;
  final double longitude;

  const LocationInfo({
    required this.address,
    required this.latitude,
    required this.longitude,
  });

  @override
  String toString() => address;
}

/// Model untuk lokasi dengan accuracy info
/// Digunakan untuk map picker agar bisa menampilkan accuracy ke user
class LocationWithAccuracy {
  final double latitude;
  final double longitude;
  final double? accuracy; // dalam meter, null jika unknown
  final bool isDefault; // true jika menggunakan default location (Jakarta)
  final bool isLastKnown; // true jika menggunakan last known location

  const LocationWithAccuracy({
    required this.latitude,
    required this.longitude,
    this.accuracy,
    this.isDefault = false,
    this.isLastKnown = false,
  });

  /// Get accuracy level untuk UI display
  AccuracyLevel get accuracyLevel {
    if (accuracy == null) return AccuracyLevel.unknown;
    if (accuracy! <= 10) return AccuracyLevel.excellent;
    if (accuracy! <= 25) return AccuracyLevel.good;
    if (accuracy! <= 50) return AccuracyLevel.fair;
    return AccuracyLevel.poor;
  }

  /// Get accuracy label untuk display
  String get accuracyLabel {
    if (accuracy == null) {
      if (isDefault) return 'Default Location';
      if (isLastKnown) return 'Last Known';
      return 'Unknown';
    }
    return '${accuracy!.toStringAsFixed(0)}m';
  }

  /// Check jika accuracy baik enough untuk pinpoint
  bool get isAccurate => accuracy != null && accuracy! <= 50;
}

/// Level akurasi GPS
enum AccuracyLevel {
  excellent, // ≤10m - sangat akurat
  good, // ≤25m - akurat
  fair, // ≤50m - cukup akurat
  poor, // >50m - kurang akurat
  unknown, // tidak diketahui
}

/// Extension untuk AccuracyLevel
extension AccuracyLevelExtension on AccuracyLevel {
  String get label {
    switch (this) {
      case AccuracyLevel.excellent:
        return 'Sangat Akurat';
      case AccuracyLevel.good:
        return 'Akurat';
      case AccuracyLevel.fair:
        return 'Cukup Akurat';
      case AccuracyLevel.poor:
        return 'Kurang Akurat';
      case AccuracyLevel.unknown:
        return 'Tidak Diketahui';
    }
  }

  String get description {
    switch (this) {
      case AccuracyLevel.excellent:
        return 'GPS sangat akurat (±10m)';
      case AccuracyLevel.good:
        return 'GPS akurat (±25m)';
      case AccuracyLevel.fair:
        return 'GPS cukup akurat (±50m)';
      case AccuracyLevel.poor:
        return 'GPS kurang akurat (±50m+)';
      case AccuracyLevel.unknown:
        return 'Akurasi tidak diketahui';
    }
  }

  Color get color {
    switch (this) {
      case AccuracyLevel.excellent:
        return const Color(0xFF22C55E); // Green
      case AccuracyLevel.good:
        return const Color(0xFF84CC16); // Light Green
      case AccuracyLevel.fair:
        return const Color(0xFFF59E0B); // Orange
      case AccuracyLevel.poor:
        return const Color(0xFFEF4444); // Red
      case AccuracyLevel.unknown:
        return const Color(0xFF6B7280); // Gray
    }
  }
}
