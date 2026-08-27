import 'package:geocoding/geocoding.dart';
import 'package:labuda/shared/services/places_autocomplete_service.dart';

/// Formatter untuk address dari placemarks dan place details
class AddressFormatter {
  /// Filter helper untuk skip Plus Codes dan data tidak berguna
  static bool isValidComponent(String? text) {
    if (text == null || text.isEmpty) return false;

    // Skip Plus Code format (contoh: Q43G+686, ABCD+123)
    final plusCodeRegex = RegExp(r'^[A-Z0-9]{4,8}\+[A-Z0-9]{2,3}$');
    if (plusCodeRegex.hasMatch(text.trim())) return false;

    // Skip pure numbers (postal codes, etc)
    if (RegExp(r'^\d+$').hasMatch(text.trim())) return false;

    // Skip jika hanya 1-2 karakter
    if (text.trim().length < 3) return false;

    return true;
  }

  /// Build address dari Placemark dengan format lengkap
  static String formatFromPlacemark(Placemark place) {
    // Build address dengan format lengkap dan readable
    final components = <String>[];

    // 1. Nama tempat spesifik (PALING PENTING)
    if (isValidComponent(place.name)) {
      components.add(place.name!);
    }

    // 2. Nama jalan (jika berbeda dari name)
    if (isValidComponent(place.street) && place.street != place.name) {
      components.add(place.street!);
    }

    // 3. Sub-area (kelurahan/desa)
    if (isValidComponent(place.subLocality) &&
        !components.contains(place.subLocality)) {
      components.add(place.subLocality!);
    }

    // 4. Kecamatan (jika ada)
    if (isValidComponent(place.subAdministrativeArea) &&
        !components.contains(place.subAdministrativeArea)) {
      components.add(place.subAdministrativeArea!);
    }

    // 5. Kota/kabupaten
    if (isValidComponent(place.locality) &&
        !components.contains(place.locality)) {
      components.add(place.locality!);
    }

    // 6. Provinsi (SELALU tambahkan)
    if (isValidComponent(place.administrativeArea) &&
        !components.contains(place.administrativeArea)) {
      components.add(place.administrativeArea!);
    }

    return components.join(', ');
  }

  /// Format address dari PlaceDetails (Places API New format)
  static String formatFromPlaceDetails(PlaceDetails details) {
    // Places API (New) uses formattedAddress directly
    // No need for components parsing since API already formats it nicely
    String fullAddress = cleanAddress(details.formattedAddress);

    // Fallback to name if formattedAddress is empty
    if (fullAddress.isEmpty && details.name != null) {
      fullAddress = details.name!;
    }

    return fullAddress;
  }

  /// Clean address string (remove duplicate parts)
  static String cleanAddress(String? address) {
    if (address == null || address.isEmpty) return '';

    // Remove common duplicates
    final cleaned = address
        .replaceAll('Indonesia, Indonesia', 'Indonesia')
        .replaceAll(RegExp(r',\s*,\s*'), ', ') // Remove double commas
        .trim();

    return cleaned;
  }
}
