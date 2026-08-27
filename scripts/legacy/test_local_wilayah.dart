import 'dart:io';
import 'dart:convert';

/// Quick test untuk verify JSON files bisa di-parse
void main() async {
  print('🧪 Testing Local Wilayah JSON Files\n');

  await testProvinces();
  await testCities();
  await testDistricts();
  await testVillages();

  print('\n✅ All tests passed!');
}

Future<void> testProvinces() async {
  print('Testing provinces.json...');
  final file = File('assets/data/provinces.json');
  final content = await file.readAsString();
  final List<dynamic> data = json.decode(content);

  print('  ✓ Found ${data.length} provinces');
  print('  ✓ Sample: ${data.first}');
}

Future<void> testCities() async {
  print('\nTesting cities files...');

  // Test Jakarta (31)
  final file = File('assets/data/cities/31.json');
  final content = await file.readAsString();
  final List<dynamic> data = json.decode(content);

  print('  ✓ Found ${data.length} cities for Jakarta');
  print('  ✓ Sample: ${data.first}');

  // Test all cities file
  final allFile = File('assets/data/cities.json');
  final allContent = await allFile.readAsString();
  final List<dynamic> allData = json.decode(allContent);
  print('  ✓ Total cities: ${allData.length}');
}

Future<void> testDistricts() async {
  print('\nTesting districts files...');

  // Test Jakarta (31)
  final file = File('assets/data/districts/31.json');
  final content = await file.readAsString();
  final List<dynamic> data = json.decode(content);

  print('  ✓ Found ${data.length} districts for Jakarta');
  print('  ✓ Sample: ${data.first}');
}

Future<void> testVillages() async {
  print('\nTesting villages files...');

  // Test Jakarta (31)
  final file = File('assets/data/villages/31.json');
  final content = await file.readAsString();
  final List<dynamic> data = json.decode(content);

  print('  ✓ Found ${data.length} villages for Jakarta');
  print('  ✓ Sample: ${data.first}');
}
