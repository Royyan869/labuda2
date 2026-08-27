import 'dart:convert';
import 'dart:io';

/// Script untuk convert SQL wilayah ke JSON files
/// Source: wilayah/wilayah/db/wilayah.sql
/// Output: assets/data/ dengan struktur optimized
void main() async {
  print('🚀 Starting Wilayah JSON Generation...\n');

  final generator = WilayahJsonGenerator();
  await generator.generate();

  print('\n✅ All done!');
}

class WilayahJsonGenerator {
  final Map<String, Province> provinces = {};
  final Map<String, List<City>> citiesByProvince = {};
  final Map<String, List<District>> districtsByProvince = {};
  final Map<String, List<Village>> villagesByProvince = {};

  Future<void> generate() async {
    // 1. Parse SQL file
    print('📖 Reading SQL file...');
    await parseSqlFile();

    // 2. Create output directories
    print('📁 Creating output directories...');
    await createOutputDirectories();

    // 3. Generate JSON files
    print('📝 Generating JSON files...');
    await generateJsonFiles();

    // 4. Print statistics
    printStatistics();
  }

  Future<void> parseSqlFile() async {
    final file = File('wilayah/wilayah/db/wilayah.sql');
    if (!file.existsSync()) {
      throw Exception('SQL file not found: ${file.path}');
    }

    final lines = await file.readAsLines();
    int recordCount = 0;

    for (final line in lines) {
      // Parse INSERT VALUES lines
      if (line.trim().startsWith('(\'')) {
        final match = RegExp(r"\('([^']+)','([^']+)'\)").firstMatch(line);
        if (match != null) {
          final kode = match.group(1)!;
          final nama = match.group(2)!;

          processRecord(kode, nama);
          recordCount++;

          if (recordCount % 10000 == 0) {
            print('  Processed $recordCount records...');
          }
        }
      }
    }

    print('  ✓ Parsed $recordCount records total');
  }

  void processRecord(String kode, String nama) {
    final parts = kode.split('.');

    if (parts.length == 1 && parts[0].length == 2) {
      // Province: "11" -> Aceh
      final id = parts[0];
      provinces[id] = Province(id: id, name: nama);
    } else if (parts.length == 2) {
      // City: "11.01" -> Kabupaten Aceh Selatan
      final provinceId = parts[0];
      final cityId = kode.replaceAll('.', '');
      final city = City(
        id: cityId,
        name: nama,
        provinceId: provinceId,
        type: _getCityType(nama),
      );

      citiesByProvince.putIfAbsent(provinceId, () => []).add(city);
    } else if (parts.length == 3) {
      // District: "11.01.01" -> Bakongan
      final provinceId = parts[0];
      final districtId = kode.replaceAll('.', '');
      final cityId = '${parts[0]}${parts[1]}';

      final district = District(id: districtId, name: nama, cityId: cityId);

      districtsByProvince.putIfAbsent(provinceId, () => []).add(district);
    } else if (parts.length == 4) {
      // Village: "11.01.01.2001" -> Keude Bakongan
      final provinceId = parts[0];
      final villageId = kode.replaceAll('.', '');
      final districtId = '${parts[0]}${parts[1]}${parts[2]}';

      final village = Village(
        id: villageId,
        name: nama,
        districtId: districtId,
      );

      villagesByProvince.putIfAbsent(provinceId, () => []).add(village);
    }
  }

  String _getCityType(String nama) {
    if (nama.toLowerCase().startsWith('kota ')) return 'kota';
    if (nama.toLowerCase().startsWith('kabupaten ')) return 'kabupaten';
    return 'kota'; // default
  }

  Future<void> createOutputDirectories() async {
    final dirs = [
      'assets/data',
      'assets/data/cities',
      'assets/data/districts',
      'assets/data/villages',
    ];

    for (final dir in dirs) {
      await Directory(dir).create(recursive: true);
    }
  }

  Future<void> generateJsonFiles() async {
    // 1. Generate provinces.json
    print('  📄 Generating provinces.json...');
    final provincesList = provinces.values.toList()
      ..sort((a, b) => a.name.compareTo(b.name));

    final provincesJson = provincesList
        .map((p) => {'id': p.id, 'name': p.name})
        .toList();

    await _writeJsonFile('assets/data/provinces.json', provincesJson);

    // 2. Generate cities per province
    print('  📄 Generating cities files...');
    for (final provinceId in citiesByProvince.keys) {
      final cities = citiesByProvince[provinceId]!
        ..sort((a, b) => a.name.compareTo(b.name));

      final citiesJson = cities
          .map(
            (c) => {
              'id': c.id,
              'name': c.name,
              'provinceId': c.provinceId,
              'type': c.type,
            },
          )
          .toList();

      await _writeJsonFile('assets/data/cities/$provinceId.json', citiesJson);
    }

    // 3. Generate all cities (for search)
    print('  📄 Generating cities.json (all cities)...');
    final allCities = <City>[];
    for (var cities in citiesByProvince.values) {
      allCities.addAll(cities);
    }
    allCities.sort((a, b) => a.name.compareTo(b.name));

    final allCitiesJson = allCities
        .map(
          (c) => {
            'id': c.id,
            'name': c.name,
            'provinceId': c.provinceId,
            'type': c.type,
          },
        )
        .toList();

    await _writeJsonFile('assets/data/cities.json', allCitiesJson);

    // 4. Generate districts per province
    print('  📄 Generating districts files...');
    for (final provinceId in districtsByProvince.keys) {
      final districts = districtsByProvince[provinceId]!
        ..sort((a, b) => a.name.compareTo(b.name));

      final districtsJson = districts
          .map((d) => {'id': d.id, 'name': d.name, 'cityId': d.cityId})
          .toList();

      await _writeJsonFile(
        'assets/data/districts/$provinceId.json',
        districtsJson,
      );
    }

    // 5. Generate villages per province
    print('  📄 Generating villages files...');
    for (final provinceId in villagesByProvince.keys) {
      final villages = villagesByProvince[provinceId]!
        ..sort((a, b) => a.name.compareTo(b.name));

      final villagesJson = villages
          .map((v) => {'id': v.id, 'name': v.name, 'districtId': v.districtId})
          .toList();

      await _writeJsonFile(
        'assets/data/villages/$provinceId.json',
        villagesJson,
      );
    }
  }

  Future<void> _writeJsonFile(String path, dynamic data) async {
    final file = File(path);
    final encoder = JsonEncoder.withIndent('  ');
    await file.writeAsString(encoder.convert(data));
  }

  void printStatistics() {
    print('\n📊 Generation Statistics:');
    print('  Provinces: ${provinces.length}');

    int totalCities = 0;
    for (var cities in citiesByProvince.values) {
      totalCities += cities.length;
    }
    print('  Cities: $totalCities');

    int totalDistricts = 0;
    for (var districts in districtsByProvince.values) {
      totalDistricts += districts.length;
    }
    print('  Districts: $totalDistricts');

    int totalVillages = 0;
    for (var villages in villagesByProvince.values) {
      totalVillages += villages.length;
    }
    print('  Villages: $totalVillages');

    print('\n📦 Files Generated:');
    print('  assets/data/provinces.json');
    print('  assets/data/cities.json (all cities)');
    print(
      '  assets/data/cities/{provinceId}.json (${citiesByProvince.length} files)',
    );
    print(
      '  assets/data/districts/{provinceId}.json (${districtsByProvince.length} files)',
    );
    print(
      '  assets/data/villages/{provinceId}.json (${villagesByProvince.length} files)',
    );
  }
}

// Simple models for internal use
class Province {
  final String id;
  final String name;
  Province({required this.id, required this.name});
}

class City {
  final String id;
  final String name;
  final String provinceId;
  final String type;
  City({
    required this.id,
    required this.name,
    required this.provinceId,
    required this.type,
  });
}

class District {
  final String id;
  final String name;
  final String cityId;
  District({required this.id, required this.name, required this.cityId});
}

class Village {
  final String id;
  final String name;
  final String districtId;
  Village({required this.id, required this.name, required this.districtId});
}
