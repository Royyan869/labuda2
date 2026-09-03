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

/// Request untuk membuat shipping option baru
class CreateShippingSetupRequest {
  final String name;
  final ShippingType type;
  final String? internalNote;
  final List<CreateShippingCoverageRequest>? coverages;

  const CreateShippingSetupRequest({
    required this.name,
    required this.type,
    this.internalNote,
    this.coverages,
  });

  CreateShippingSetupRequest copyWith({
    String? name,
    ShippingType? type,
    String? internalNote,
    List<CreateShippingCoverageRequest>? coverages,
  }) {
    return CreateShippingSetupRequest(
      name: name ?? this.name,
      type: type ?? this.type,
      internalNote: internalNote ?? this.internalNote,
      coverages: coverages ?? this.coverages,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'name': name,
      'transport_type': type.name,
      if (internalNote != null) 'internal_note': internalNote,
      if (coverages != null)
        'coverages': coverages!.map((c) => c.toJson()).toList(),
    };
  }
}

/// Request untuk update shipping option
class UpdateShippingSetupRequest {
  final String? name;
  final bool? isActive;

  const UpdateShippingSetupRequest({
    this.name,
    this.isActive,
  });

  Map<String, dynamic> toJson() {
    return {
      if (name != null) 'name': name,
      if (isActive != null) 'is_active': isActive,
    };
  }
}

/// Request untuk add coverage
class AddCoverageRequest {
  final String provinceCode;
  final String provinceName;
  final double rate;
  final bool isAvailable;

  const AddCoverageRequest({
    required this.provinceCode,
    required this.provinceName,
    required this.rate,
    this.isAvailable = true,
  });

  Map<String, dynamic> toJson() {
    return {
      'province_code': provinceCode,
      'province_name': provinceName,
      'rate': rate.toInt(),
      'is_available': isAvailable,
    };
  }
}

/// Request untuk update coverage
class UpdateCoverageRequest {
  final String? provinceName;
  final double? provinceRate;
  final bool? isAvailable;

  const UpdateCoverageRequest({
    this.provinceName,
    this.provinceRate,
    this.isAvailable,
  });

  Map<String, dynamic> toJson() {
    return {
      if (provinceName != null) 'province_name': provinceName,
      if (provinceRate != null) 'rate': provinceRate!.toInt(),
      if (isAvailable != null) 'is_available': isAvailable,
    };
  }
}

/// Request untuk check delivery availability
class CheckDeliveryRequest {
  final String productId;
  final String provinceId;
  final String cityId;
  final String cityName;

  const CheckDeliveryRequest({
    required this.productId,
    required this.provinceId,
    required this.cityId,
    required this.cityName,
  });
}

/// Response untuk check delivery availability
class DeliveryOption {
  final String shippingSetupId;
  final String displayName;
  final String type;
  final double rate;
  final String? notes;
  final String source;

  const DeliveryOption({
    required this.shippingSetupId,
    required this.displayName,
    required this.type,
    required this.rate,
    this.notes,
    required this.source,
  });

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is DeliveryOption &&
        other.shippingSetupId == shippingSetupId &&
        other.displayName == displayName &&
        other.type == type &&
        other.rate == rate &&
        other.notes == notes &&
        other.source == source;
  }

  @override
  int get hashCode =>
      Object.hash(shippingSetupId, displayName, type, rate, source);
}

// =====================================
// Full Update / City Rule Requests
// =====================================

class CreateShippingCityRuleRequest {
  final String cityId;
  final String cityName;
  final int? overrideTariff;
  final bool excluded;

  const CreateShippingCityRuleRequest({
    required this.cityId,
    required this.cityName,
    this.overrideTariff,
    required this.excluded,
  });

  Map<String, dynamic> toJson() {
    return {
      'city_id': cityId,
      'city_name': cityName,
      if (overrideTariff != null) 'override_tariff': overrideTariff,
      'excluded': excluded,
    };
  }
}

class UpdateShippingCoverageRequest {
  final String provinceId;
  final String provinceName;
  final int tariff;
  final List<dynamic> cityRules;

  const UpdateShippingCoverageRequest({
    required this.provinceId,
    required this.provinceName,
    required this.tariff,
    required this.cityRules,
  });
}

class CreateShippingCoverageRequest {
  final String provinceId;
  final String provinceName;
  final int tariff;
  final List<CreateShippingCityRuleRequest> cityRules;

  const CreateShippingCoverageRequest({
    required this.provinceId,
    required this.provinceName,
    required this.tariff,
    this.cityRules = const [],
  });

  Map<String, dynamic> toJson() {
    return {
      'province_id': provinceId,
      'province_name': provinceName,
      'tariff': tariff,
      'city_rules': cityRules.map((r) => r.toJson()).toList(),
    };
  }
}

class UpdateShippingSetupFullRequest {
  final String name;
  final ShippingType transportType;
  final String? internalNote;
  final List<dynamic>? coverages;

  const UpdateShippingSetupFullRequest({
    required this.name,
    required this.transportType,
    this.internalNote,
    this.coverages,
  });
}
