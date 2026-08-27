import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/shared/data/dto/commerce_media_request_dto.dart';

void main() {
  test('CommerceMediaRequestDto round-trips position in JSON', () {
    final dto = CommerceMediaRequestDto.video(
      url: 'https://cdn.example.com/video-a.mp4',
      width: 1920,
      height: 1080,
      duration: 14400,
      thumbnailUrl: 'https://cdn.example.com/video-a.jpg',
    ).copyWith(position: 2);

    final json = dto.toJson();

    expect(json['position'], 2);
    final roundTrip = CommerceMediaRequestDto.fromJson(json);
    expect(roundTrip.position, 2);
    expect(roundTrip.thumbnailUrl, 'https://cdn.example.com/video-a.jpg');
  });
}
