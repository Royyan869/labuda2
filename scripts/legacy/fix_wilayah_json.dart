import 'dart:convert';
import 'dart:io';

void main() {
  print('Fixing wilayah JSON format...\n');

  // Fix districts
  fixDistricts();

  // Fix villages
  fixVillages();

  print('\nDone!');
}

void fixDistricts() {
  final file = File('assets/data/districts_sample.json');
  final content = file.readAsStringSync();
  final List<dynamic> districts = json.decode(content);

  final fixed = districts.map((d) {
    final id = (d['id'] as String).replaceAll('.', '');
    final cityId = (d['city_id'] as String).replaceAll('.', '');

    return {
      'id': id,
      'name': d['name'],
      'cityId': cityId,
    };
  }).toList();

  file.writeAsStringSync(JsonEncoder.withIndent('  ').convert(fixed));
  print('✓ Fixed ${fixed.length} districts');
}

void fixVillages() {
  final file = File('assets/data/villages_sample.json');
  final content = file.readAsStringSync();
  final List<dynamic> villages = json.decode(content);

  final fixed = villages.map((v) {
    final id = (v['id'] as String).replaceAll('.', '');
    final districtId = (v['district_id'] as String).replaceAll('.', '');

    return {
      'id': id,
      'name': v['name'],
      'districtId': districtId,
      'type': v['type'] ?? 'desa',
    };
  }).toList();

  file.writeAsStringSync(JsonEncoder.withIndent('  ').convert(fixed));
  print('✓ Fixed ${fixed.length} villages');
}