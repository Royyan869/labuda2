import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/domain.dart';

void main() {
  group('Shipping one-package request integer serialization', () {
    test('destination rate serializes as JSON integer', () {
      final req = ShippingDestinationRequest(
        provinceCode: '11',
        provinceName: 'Aceh',
        rate: 50000,
      );
      final json = req.toJson();
      expect(json['rate'], 50000);
      expect(json['rate'], isA<int>());
      expect(json['rate'], isNot(isA<double>()));
    });

    test('city qualification rate override serializes as JSON integer', () {
      final req = CityQualificationRequest(
        cityCode: '1101',
        cityName: 'Kabupaten Pidie Jaya',
        rateOverride: 100000,
      );
      final json = req.toJson();
      expect(json['rate'], 100000);
      expect(json['rate'], isA<int>());
      expect(json['rate'], isNot(isA<double>()));
      expect(json['is_available'], true);
    });

    test('excluded city qualification serializes correctly', () {
      final req = CityQualificationRequest(
        cityCode: '1102',
        cityName: 'Kota Banda Aceh',
        excluded: true,
      );
      final json = req.toJson();
      expect(json['is_available'], false);
      expect(json.containsKey('rate'), false);
    });

    test('multi-province package serializes every tariff as integer', () {
      final req = CreateShippingSetupRequest(
        name: 'KRT',
        type: ShippingType.custom,
        destinations: [
          ShippingDestinationRequest(
            provinceCode: '11',
            provinceName: 'Aceh',
            rate: 50000,
            cityQualifications: [
              CityQualificationRequest(
                cityCode: '1101',
                cityName: 'Kabupaten Pidie Jaya',
                rateOverride: 100000,
              ),
            ],
          ),
          ShippingDestinationRequest(
            provinceCode: '12',
            provinceName: 'Sumatera Utara',
            rate: 75000,
          ),
        ],
      );
      final json = req.toJson();

      final destinations = json['destinations'] as List;
      expect(destinations.length, 2);

      expect(destinations[0]['rate'], 50000);
      expect(destinations[0]['rate'], isA<int>());
      expect(destinations[0]['rate'], isNot(isA<double>()));

      expect(destinations[0]['city_qualifications'][0]['rate'], 100000);
      expect(
        destinations[0]['city_qualifications'][0]['rate'],
        isA<int>(),
      );

      expect(destinations[1]['rate'], 75000);
      expect(destinations[1]['rate'], isA<int>());
    });

    test('wire keys match the canonical backend contract', () {
      final req = CreateShippingSetupRequest(
        name: 'Bus Handoyo',
        type: ShippingType.bus,
        internalNote: 'kantong besar',
        destinations: [
          ShippingDestinationRequest(
            provinceCode: '31',
            provinceName: 'DKI Jakarta',
            rate: 150000,
          ),
        ],
      );
      final json = req.toJson();
      expect(json['name'], 'Bus Handoyo');
      expect(json['transport_type'], 'bus');
      expect(json['internal_purpose'], 'kantong besar');
      expect(json.containsKey('destinations'), true);

      final dest = (json['destinations'] as List).first
          as Map<String, dynamic>;
      expect(dest['province_code'], '31');
      expect(dest['province_name'], 'DKI Jakarta');
      expect(dest['is_available'], true);
    });

    test('null internal note serializes as empty seller-private string', () {
      final req = CreateShippingSetupRequest(
        name: 'Test',
        type: ShippingType.bus,
        destinations: const [
          ShippingDestinationRequest(
            provinceCode: '31',
            provinceName: 'DKI Jakarta',
            rate: 10000,
          ),
        ],
      );
      final json = req.toJson();
      expect(json['internal_purpose'], '');
      // Killed design: the old 'internal_note' key must never reappear.
      expect(json.containsKey('internal_note'), false);
    });
  });
}
