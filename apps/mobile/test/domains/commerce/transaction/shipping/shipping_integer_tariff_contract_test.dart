import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/domain.dart';

void main() {
  group('ShippingOption request integer serialization', () {
    test('province tariff serializes as JSON integer', () {
      final req = CreateShippingCoverageRequest(
        provinceId: '11',
        provinceName: 'Aceh',
        tariff: 50000,
      );
      final json = req.toJson();
      expect(json['tariff'], 50000);
      expect(json['tariff'], isA<int>());
      expect(json['tariff'], isNot(isA<double>()));
    });

    test('city override tariff serializes as JSON integer', () {
      final req = CreateShippingCityRuleRequest(
        cityId: '1101',
        cityName: 'Kabupaten Pidie Jaya',
        overrideTariff: 100000,
        excluded: false,
      );
      final json = req.toJson();
      expect(json['override_tariff'], 100000);
      expect(json['override_tariff'], isA<int>());
      expect(json['override_tariff'], isNot(isA<double>()));
    });

    test('excluded city rule serializes correctly with no override_tariff', () {
      final req = CreateShippingCityRuleRequest(
        cityId: '1102',
        cityName: 'Kota Banda Aceh',
        excluded: true,
      );
      final json = req.toJson();
      expect(json['excluded'], true);
      expect(json.containsKey('override_tariff'), false);
    });

    test('multi-province request serializes every tariff as integer', () {
      final req = CreateShippingOptionRequest(
        name: 'KRT',
        type: ShippingType.custom,
        coverages: [
          CreateShippingCoverageRequest(
            provinceId: '11',
            provinceName: 'Aceh',
            tariff: 50000,
            cityRules: [
              CreateShippingCityRuleRequest(
                cityId: '1101',
                cityName: 'Kabupaten Pidie Jaya',
                overrideTariff: 100000,
                excluded: false,
              ),
            ],
          ),
          CreateShippingCoverageRequest(
            provinceId: '12',
            provinceName: 'Sumatera Utara',
            tariff: 75000,
          ),
        ],
      );
      final json = req.toJson();

      final coverages = json['coverages'] as List;
      expect(coverages.length, 2);

      expect(coverages[0]['tariff'], 50000);
      expect(coverages[0]['tariff'], isA<int>());
      expect(coverages[0]['tariff'], isNot(isA<double>()));

      expect(coverages[0]['city_rules'][0]['override_tariff'], 100000);
      expect(coverages[0]['city_rules'][0]['override_tariff'], isA<int>());

      expect(coverages[1]['tariff'], 75000);
      expect(coverages[1]['tariff'], isA<int>());
    });

    test('AddCoverageRequest rate serializes as integer', () {
      final req = AddCoverageRequest(
        provinceCode: '11',
        provinceName: 'Aceh',
        rate: 50000,
      );
      final json = req.toJson();
      expect(json['rate'], 50000);
      expect(json['rate'], isA<int>());
    });

    test('UpdateCoverageRequest provinceRate serializes as integer', () {
      final req = UpdateCoverageRequest(provinceRate: 75000);
      final json = req.toJson();
      expect(json['rate'], 75000);
      expect(json['rate'], isA<int>());
    });

    test('coverages empty by default in CreateShippingOptionRequest', () {
      final req = CreateShippingOptionRequest(
        name: 'Test',
        type: ShippingType.custom,
        coverages: const [],
      );
      final json = req.toJson();
      expect(json['coverages'], isEmpty);
    });

    test('internal note is omitted from JSON when null', () {
      final req = CreateShippingOptionRequest(
        name: 'Test',
        type: ShippingType.bus,
        coverages: const [],
      );
      final json = req.toJson();
      expect(json.containsKey('internal_note'), false);
    });
  });
}
