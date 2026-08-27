/// SQL to JSON Converter for Wilayah Data
///
/// Script untuk convert data wilayah dari SQL ke JSON format
/// Run dengan: dart tools/sql_to_json_converter.dart
library;

import 'dart:io';
import 'dart:convert';

Future<void> main() async {
  print('Starting SQL to JSON conversion...');

  // Read SQL file
  final sqlFile = File('wilayah/wilayah/db/wilayah.sql');

  if (!await sqlFile.exists()) {
    print('ERROR: SQL file not found at wilayah/wilayah/db/wilayah.sql');
    return;
  }

  final lines = await sqlFile.readAsLines();
  print('Read ${lines.length} lines from SQL file');

  // Prepare collections
  final provinces = <Map<String, dynamic>>[];
  final cities = <Map<String, dynamic>>[];
  final districts = <Map<String, dynamic>>[];
  final villages = <Map<String, dynamic>>[];

  // Parse SQL INSERT statements
  int lineCount = 0;
  for (final line in lines) {
    lineCount++;

    // Match pattern: ('code','name'),
    if (line.contains("('") && line.contains("'),")) {
      // Extract data using regex
      final regex = RegExp(r"\('([^']+)','([^']+)'\),?");
      final match = regex.firstMatch(line);

      if (match != null) {
        final id = match.group(1)!;
        final name = match.group(2)!;

        // Determine type based on ID format
        final parts = id.split('.');

        if (parts.length == 1 && parts[0].length == 2) {
          // Province (e.g., '11')
          provinces.add({
            'id': id,
            'name': name,
          });
        } else if (parts.length == 2) {
          // City/Regency (e.g., '11.01')
          cities.add({
            'id': id,
            'name': name,
            'province_id': parts[0],
          });
        } else if (parts.length == 3) {
          // District (e.g., '11.01.01')
          districts.add({
            'id': id,
            'name': name,
            'city_id': '${parts[0]}.${parts[1]}',
          });
        } else if (parts.length == 4) {
          // Village (e.g., '11.01.01.2001')
          villages.add({
            'id': id,
            'name': name,
            'district_id': '${parts[0]}.${parts[1]}.${parts[2]}',
          });
        }
      }
    }

    // Progress indicator
    if (lineCount % 10000 == 0) {
      print('Processed $lineCount lines...');
    }
  }

  // Create output directory
  final outputDir = Directory('assets/data/full');
  if (!await outputDir.exists()) {
    await outputDir.create(recursive: true);
  }

  // Write JSON files
  print('\nWriting JSON files...');

  await writeJsonFile('assets/data/full/provinces.json', provinces);
  print('✅ Provinces: ${provinces.length} items');

  await writeJsonFile('assets/data/full/cities.json', cities);
  print('✅ Cities: ${cities.length} items');

  await writeJsonFile('assets/data/full/districts.json', districts);
  print('✅ Districts: ${districts.length} items');

  await writeJsonFile('assets/data/full/villages.json', villages);
  print('✅ Villages: ${villages.length} items');

  print('\n🎉 Conversion completed successfully!');
  print('Files saved in: assets/data/full/');

  // Summary
  print('\n📊 Data Summary:');
  print('Total Provinces: ${provinces.length}');
  print('Total Cities/Regencies: ${cities.length}');
  print('Total Districts: ${districts.length}');
  print('Total Villages: ${villages.length}');
  print('Total Records: ${provinces.length + cities.length + districts.length + villages.length}');
}

Future<void> writeJsonFile(String path, List<Map<String, dynamic>> data) async {
  final file = File(path);
  final encoder = JsonEncoder.withIndent('  '); // Pretty print
  await file.writeAsString(encoder.convert(data));
}