/// Wilayah Models
///
/// Model untuk data wilayah Indonesia (provinsi, kabupaten/kota, kecamatan)
/// Digunakan untuk dropdown alamat dan shipping calculation
library;

class Province {
  final String id;
  final String name;

  const Province({required this.id, required this.name});

  factory Province.fromJson(Map<String, dynamic> json) {
    return Province(id: json['id'] as String, name: json['name'] as String);
  }

  Map<String, dynamic> toJson() {
    return {'id': id, 'name': name};
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is Province && runtimeType == other.runtimeType && id == other.id;

  @override
  int get hashCode => id.hashCode;

  @override
  String toString() => 'Province(id: $id, name: $name)';
}

class City {
  final String id;
  final String name;
  final String provinceId;

  const City({required this.id, required this.name, required this.provinceId});

  factory City.fromJson(Map<String, dynamic> json) {
    return City(
      id: json['id'] as String,
      name: json['name'] as String,
      provinceId: json['provinceId'] ?? json['province_id'] as String,
    );
  }

  Map<String, dynamic> toJson() {
    return {'id': id, 'name': name, 'province_id': provinceId};
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is City && runtimeType == other.runtimeType && id == other.id;

  @override
  int get hashCode => id.hashCode;

  @override
  String toString() => 'City(id: $id, name: $name, provinceId: $provinceId)';
}

class District {
  final String id;
  final String name;
  final String cityId;

  const District({required this.id, required this.name, required this.cityId});

  factory District.fromJson(Map<String, dynamic> json) {
    return District(
      id: json['id'] as String,
      name: json['name'] as String,
      cityId: json['cityId'] ?? json['city_id'] as String,
    );
  }

  Map<String, dynamic> toJson() {
    return {'id': id, 'name': name, 'city_id': cityId};
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is District && runtimeType == other.runtimeType && id == other.id;

  @override
  int get hashCode => id.hashCode;

  @override
  String toString() => 'District(id: $id, name: $name, cityId: $cityId)';
}

class Village {
  final String id;
  final String name;
  final String districtId;

  const Village({
    required this.id,
    required this.name,
    required this.districtId,
  });

  factory Village.fromJson(Map<String, dynamic> json) {
    return Village(
      id: json['id'] as String,
      name: json['name'] as String,
      districtId: json['districtId'] ?? json['district_id'] as String,
    );
  }

  Map<String, dynamic> toJson() {
    return {'id': id, 'name': name, 'district_id': districtId};
  }

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is Village && runtimeType == other.runtimeType && id == other.id;

  @override
  int get hashCode => id.hashCode;

  @override
  String toString() => 'Village(id: $id, name: $name, districtId: $districtId)';
}
