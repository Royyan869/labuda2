import 'dart:async';
import 'dart:convert';
import 'package:http/http.dart' as http;

/// Custom Places Autocomplete Service
///
/// Menggunakan Places API (New) - bukan legacy API
/// Documentation: https://developers.google.com/maps/documentation/places/web-service/place-autocomplete
class PlacesAutocompleteService {
  final String apiKey;
  Timer? _debounce;

  PlacesAutocompleteService(this.apiKey);

  /// Search places dengan autocomplete menggunakan Places API (New)
  Future<List<PlacePrediction>> searchPlaces(
    String query, {
    String? countryCode = 'id',
  }) async {
    if (query.isEmpty) return [];

    try {
      // Places API (New) menggunakan POST request dengan JSON body
      final url = Uri.parse(
        'https://places.googleapis.com/v1/places:autocomplete',
      );

      final body = {
        'input': query,
        'languageCode': 'id',
        if (countryCode != null) 'includedRegionCodes': [countryCode],
      };

      final response = await http.post(
        url,
        headers: {'Content-Type': 'application/json', 'X-Goog-Api-Key': apiKey},
        body: json.encode(body),
      );

      if (response.statusCode == 200) {
        final data = json.decode(response.body);

        if (data['suggestions'] != null) {
          final suggestions = data['suggestions'] as List;

          final predictions = suggestions
              .where((item) => item['placePrediction'] != null)
              .map((item) => PlacePrediction.fromJson(item['placePrediction']))
              .toList();
          return predictions;
        }
        return [];
      } else {
        return [];
      }
    } catch (e) {
      return [];
    }
  }

  /// Get place details untuk mendapatkan coordinates menggunakan Places API (New)
  Future<PlaceDetails?> getPlaceDetails(String placeId) async {
    try {
      // Places API (New) format: places/{placeId}
      // PlaceId sudah include "places/" prefix dari API response
      final url = Uri.parse('https://places.googleapis.com/v1/$placeId');

      final response = await http.get(
        url,
        headers: {
          'Content-Type': 'application/json',
          'X-Goog-Api-Key': apiKey,
          'X-Goog-FieldMask': 'location,formattedAddress,displayName',
        },
      );

      if (response.statusCode == 200) {
        final data = json.decode(response.body);
        return PlaceDetails.fromJson(data);
      } else {
        return null;
      }
    } catch (e) {
      return null;
    }
  }

  /// Debounced search untuk avoid too many API calls
  void searchWithDebounce(
    String query,
    Function(List<PlacePrediction>) onResults, {
    Duration delay = const Duration(milliseconds: 500),
  }) {
    _debounce?.cancel();
    _debounce = Timer(delay, () async {
      final results = await searchPlaces(query);
      onResults(results);
    });
  }

  void dispose() {
    _debounce?.cancel();
  }
}

/// Place Prediction Model untuk Places API (New)
class PlacePrediction {
  final String placeId;
  final String description;
  final String? mainText;
  final String? secondaryText;

  PlacePrediction({
    required this.placeId,
    required this.description,
    this.mainText,
    this.secondaryText,
  });

  factory PlacePrediction.fromJson(Map<String, dynamic> json) {
    // Places API (New) format berbeda dari legacy API
    final mainText = json['structuredFormat']?['mainText']?['text'] as String?;
    final secondaryText =
        json['structuredFormat']?['secondaryText']?['text'] as String?;

    // Fallback ke text biasa jika structured format tidak ada
    final text = json['text']?['text'] as String? ?? '';

    // Places API (New) menggunakan field 'place' bukan 'placeId'
    final placeId =
        json['place'] as String? ?? json['placeId'] as String? ?? '';

    return PlacePrediction(
      placeId: placeId,
      description: text,
      mainText: mainText ?? text,
      secondaryText: secondaryText,
    );
  }
}

/// Place Details Model untuk Places API (New)
class PlaceDetails {
  final double latitude;
  final double longitude;
  final String formattedAddress;
  final String? name;

  PlaceDetails({
    required this.latitude,
    required this.longitude,
    required this.formattedAddress,
    this.name,
  });

  factory PlaceDetails.fromJson(Map<String, dynamic> json) {
    // Places API (New) format: location.latitude & location.longitude
    final location = json['location'];
    return PlaceDetails(
      latitude: (location['latitude'] as num).toDouble(),
      longitude: (location['longitude'] as num).toDouble(),
      formattedAddress: json['formattedAddress'] as String? ?? '',
      name: json['displayName']?['text'] as String?,
    );
  }
}
