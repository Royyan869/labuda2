import 'package:equatable/equatable.dart';

// =====================================
// Enums
// =====================================

/// Enum untuk jenis opsi pengiriman
enum ShippingType {
  train('Train', '🚂'),
  bus('Bus', '🚌'),
  travel('Travel', '🚐'),
  plane('Plane', '✈️'),
  custom('Custom', '📦');

  final String label;
  final String emoji;
  const ShippingType(this.label, this.emoji);

  String get snakeCaseName => name;

  static ShippingType? fromString(String value) {
    try {
      return ShippingType.values.firstWhere(
        (e) => e.name.toLowerCase() == value.toLowerCase(),
      );
    } catch (_) {
      return null;
    }
  }
}

// =====================================
// Value Objects
// =====================================

/// City-level shipping rate (override dari province)
class CityShippingRate extends Equatable {
  final String cityId;
  final String cityName;
  final double rate;
  final String? notes;
  final bool? excluded;

  const CityShippingRate({
    required this.cityId,
    required this.cityName,
    required this.rate,
    this.notes,
    this.excluded,
  });

  CityShippingRate copyWith({
    String? cityId,
    String? cityName,
    double? rate,
    String? notes,
    bool? excluded,
  }) {
    return CityShippingRate(
      cityId: cityId ?? this.cityId,
      cityName: cityName ?? this.cityName,
      rate: rate ?? this.rate,
      notes: notes ?? this.notes,
      excluded: excluded ?? this.excluded,
    );
  }

  @override
  List<Object?> get props => [
    cityId,
    cityName,
    rate,
    notes,
    excluded,
  ];
}

/// Result dari query rate untuk kota tertentu
class ShippingRateResult {
  final double rate;
  final String? notes;
  final String source; // 'province' atau 'city'

  const ShippingRateResult({
    required this.rate,
    this.notes,
    required this.source,
  });

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is ShippingRateResult &&
        other.rate == rate &&
        other.notes == notes &&
        other.source == source;
  }

  @override
  int get hashCode => Object.hash(rate, notes, source);
}

// =====================================
// Entities
// =====================================

/// Coverage Area (Province-level dengan optional City overrides)
class ShippingCoverage extends Equatable {
  final String provinceId;
  final String provinceName;

  /// Rate untuk seluruh provinsi (nullable - jika null berarti provinsi ini tidak dilayani)
  final double? provinceRate;
  final String? provinceNotes;
  final bool isAvailable;

  /// City-level overrides (hanya untuk kota yang di-override)
  final List<CityShippingRate> cityOverrides;

  const ShippingCoverage({
    required this.provinceId,
    required this.provinceName,
    this.provinceRate,
    this.provinceNotes,
    this.isAvailable = true,
    this.cityOverrides = const [],
  });

  /// Check apakah kota tertentu dilayani
  /// Priority: City override > Province rate > Not covered
  ShippingRateResult? getRateForCity(String cityId, String cityName) {
    // 1. Check city override first (highest priority)
    final cityOverride = cityOverrides
        .where((c) => c.cityId == cityId)
        .firstOrNull;

    if (cityOverride != null) {
      return ShippingRateResult(
        rate: cityOverride.rate,
        notes: cityOverride.notes,
        source: 'city',
      );
    }

    // 2. Fallback ke province rate
    if (provinceRate != null) {
      return ShippingRateResult(
        rate: provinceRate!,
        notes: provinceNotes,
        source: 'province',
      );
    }

    // 3. Not covered
    return null;
  }

  /// Get total jumlah kota yang dilayani di provinsi ini
  int get totalCitiesCovered {
    if (provinceRate != null) {
      return 15; // Estimasi rata-rata kota per provinsi
    }
    return cityOverrides.length;
  }

  ShippingCoverage copyWith({
    String? provinceId,
    String? provinceName,
    double? provinceRate,
    String? provinceNotes,
    bool? isAvailable,
    List<CityShippingRate>? cityOverrides,
  }) {
    return ShippingCoverage(
      provinceId: provinceId ?? this.provinceId,
      provinceName: provinceName ?? this.provinceName,
      provinceRate: provinceRate ?? this.provinceRate,
      provinceNotes: provinceNotes ?? this.provinceNotes,
      isAvailable: isAvailable ?? this.isAvailable,
      cityOverrides: cityOverrides ?? this.cityOverrides,
    );
  }

  @override
  List<Object?> get props => [
    provinceId,
    provinceName,
    provinceRate,
    provinceNotes,
    isAvailable,
    cityOverrides,
  ];
}

/// Main entity untuk Shipping Option
/// Satu seller bisa punya banyak shipping options (kereta, travel, pesawat, custom)
class ShippingSetup extends Equatable {
  final String id;
  final String name;
  final String? sellerId;
  final ShippingType type;

  /// ID dari farm address (untuk referensi lokasi asal)
  final String? farmAddressId;

  /// Coverage areas dengan rates
  final List<ShippingCoverage> coverageAreas;

  /// Status aktif/non-aktif
  final bool isActive;

  /// Catatan internal (tidak ditampilkan ke pembeli)
  final String? internalNote;

  final DateTime createdAt;
  final DateTime updatedAt;

  const ShippingSetup({
    required this.id,
    required this.name,
    this.sellerId,
    required this.type,
    this.farmAddressId,
    required this.coverageAreas,
    this.isActive = true,
    this.internalNote,
    required this.createdAt,
    required this.updatedAt,
  });

  /// Get display name
  String get displayName {
    return name;
  }

  /// Get short name (untuk tampilan compact)
  String get shortName => name;

  /// Get emoji icon
  String get emoji => type.emoji;

  /// Total jumlah provinsi yang dilayani
  int get totalProvincesCovered {
    return coverageAreas
        .where((c) => c.provinceRate != null || c.cityOverrides.isNotEmpty)
        .length;
  }

  /// Total jumlah kota yang dilayani (estimasi)
  int get totalCitiesCovered {
    int count = 0;
    for (final coverage in coverageAreas) {
      count += coverage.totalCitiesCovered;
    }
    return count;
  }

  /// Check apakah bisa deliver ke provinsi/kota tertentu
  ShippingRateResult? canDeliverTo({
    required String provinceId,
    required String cityId,
    required String cityName,
  }) {
    final coverage = coverageAreas
        .where((c) => c.provinceId == provinceId)
        .firstOrNull;

    if (coverage == null) return null;

    return coverage.getRateForCity(cityId, cityName);
  }

  ShippingSetup copyWith({
    String? id,
    String? name,
    String? sellerId,
    ShippingType? type,
    String? farmAddressId,
    List<ShippingCoverage>? coverageAreas,
    bool? isActive,
    String? internalNote,
    DateTime? createdAt,
    DateTime? updatedAt,
  }) {
    return ShippingSetup(
      id: id ?? this.id,
      name: name ?? this.name,
      sellerId: sellerId ?? this.sellerId,
      type: type ?? this.type,
      farmAddressId: farmAddressId ?? this.farmAddressId,
      coverageAreas: coverageAreas ?? this.coverageAreas,
      isActive: isActive ?? this.isActive,
      internalNote: internalNote ?? this.internalNote,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
    );
  }

  @override
  List<Object?> get props => [
    id,
    name,
    sellerId,
    type,
    farmAddressId,
    coverageAreas,
    isActive,
    internalNote,
    createdAt,
    updatedAt,
  ];
}

// =====================================
// Request/Response Objects
// =====================================

/// Request untuk membuat shipping option sebagai SATU PAKET (kontrak
/// canonical): identitas (nama, jenis, catatan privat seller) + destinasi
/// (provinsi dengan tarif all-in ongkir+packing + kualifikasi kota).
/// Backend menolak paket tanpa minimal satu destinasi.
class CreateShippingSetupRequest {
  final String name;
  final ShippingType type;

  /// Catatan privat seller ("kantong besar", "untuk 1 ekor", ...).
  /// Tidak pernah tampil di sisi buyer.
  final String? internalNote;

  /// Minimal satu destinasi provinsi dengan tarif. Gerbang bisnis:
  /// opsi tanpa destinasi tidak boleh tersimpan.
  final List<ShippingDestinationRequest> destinations;

  const CreateShippingSetupRequest({
    required this.name,
    required this.type,
    this.internalNote,
    required this.destinations,
  });

  Map<String, dynamic> toJson() {
    return {
      'name': name,
      'transport_type': type.name,
      'internal_purpose': internalNote ?? '',
      'destinations': destinations.map((d) => d.toJson()).toList(),
    };
  }
}

/// Satu destinasi provinsi dalam paket shipping.
class ShippingDestinationRequest {
  final String provinceCode;
  final String provinceName;

  /// Tarif all-in (ongkir + packing) untuk seluruh kota di provinsi ini.
  final int rate;
  final bool isAvailable;
  final List<CityQualificationRequest> cityQualifications;

  const ShippingDestinationRequest({
    required this.provinceCode,
    required this.provinceName,
    required this.rate,
    this.isAvailable = true,
    this.cityQualifications = const [],
  });

  Map<String, dynamic> toJson() {
    return {
      'province_code': provinceCode,
      'province_name': provinceName,
      'rate': rate,
      'is_available': isAvailable,
      'city_qualifications': cityQualifications.map((c) => c.toJson()).toList(),
    };
  }
}

/// Kualifikasi per kota: beda tarif, atau dinonaktifkan (excluded).
/// Nilai null / excluded=false tanpa override = ikut tarif provinsi.
class CityQualificationRequest {
  final String cityCode;
  final String cityName;

  /// Tarif override kota (null = ikut tarif provinsi).
  final int? rateOverride;

  /// Kota tidak dilayani (dinonaktifkan meski provinsinya dilayani).
  final bool excluded;

  const CityQualificationRequest({
    required this.cityCode,
    required this.cityName,
    this.rateOverride,
    this.excluded = false,
  });

  Map<String, dynamic> toJson() {
    return {
      'city_code': cityCode,
      'city_name': cityName,
      if (!excluded && rateOverride != null) 'rate': rateOverride,
      'is_available': !excluded,
    };
  }
}

/// Request untuk update shipping option sebagai SATU PAKET (full replace,
/// satu transaksi di backend). Destinasi opsional di wire supaya edit
/// identitas saja tidak memaksa kirim ulang destinasi.
class UpdateShippingSetupRequest {
  final String name;
  final ShippingType type;
  final String? internalNote;
  final bool? isActive;
  final List<ShippingDestinationRequest>? destinations;

  const UpdateShippingSetupRequest({
    required this.name,
    required this.type,
    this.internalNote,
    this.isActive,
    this.destinations,
  });

  Map<String, dynamic> toJson() {
    return {
      'name': name,
      'transport_type': type.name,
      'internal_purpose': internalNote ?? '',
      if (isActive != null) 'is_active': isActive,
      if (destinations != null)
        'destinations': destinations!.map((d) => d.toJson()).toList(),
    };
  }
}

/// Request untuk check delivery availability
///
/// `provinceId` / `cityId` carry the 2-digit / 4-digit BPS codes — the same
/// value space as the address `Province.id` / `City.id` — and are serialized to
/// the canonical backend wire keys `province_code` / `city_code`.
class CheckDeliveryRequest {
  final String productId;
  final String provinceId;
  final String cityId;

  const CheckDeliveryRequest({
    required this.productId,
    required this.provinceId,
    required this.cityId,
  });
}

/// Response untuk check delivery availability
///
/// Domain shape of one deliverable option, projected from the canonical wire
/// option (`shipping_option_id` / `name` / `transport_type` / `rate`).
class DeliveryOption {
  final String shippingSetupId;
  final String displayName;
  final String type;
  final double rate;

  const DeliveryOption({
    required this.shippingSetupId,
    required this.displayName,
    required this.type,
    required this.rate,
  });

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is DeliveryOption &&
        other.shippingSetupId == shippingSetupId &&
        other.displayName == displayName &&
        other.type == type &&
        other.rate == rate;
  }

  @override
  int get hashCode => Object.hash(shippingSetupId, displayName, type, rate);
}
