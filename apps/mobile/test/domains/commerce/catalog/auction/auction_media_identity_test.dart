import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/entities/auction_media_identity.dart';

void main() {
  test(
    'normalizes transient signed query parameters from auction media URLs',
    () {
      expect(
        normalizeAuctionMediaReference(
          'https://cdn.example.com/auctions/cover.jpg?X-Amz-Signature=one&X-Amz-Expires=60',
        ),
        'auctions/cover.jpg',
      );
    },
  );

  test('preserves non-transient authority when query carries version data', () {
    expect(
      normalizeAuctionMediaReference(
        'https://cdn.example.com/auctions/cover.jpg?v=42&X-Amz-Signature=one',
      ),
      'auctions/cover.jpg?v=42',
    );
  });

  test('stable logical keys change when slot or object identity changes', () {
    final sameSlot = auctionMediaLogicalKey(
      auctionId: 'auction-1',
      mediaReference:
          'https://cdn.example.com/auctions/cover.jpg?X-Amz-Signature=one',
      position: 0,
    );
    final sameSlotRetry = auctionMediaLogicalKey(
      auctionId: 'auction-1',
      mediaReference:
          'https://cdn.example.com/auctions/cover.jpg?X-Amz-Signature=two',
      position: 0,
    );
    final otherSlot = auctionMediaLogicalKey(
      auctionId: 'auction-1',
      mediaReference:
          'https://cdn.example.com/auctions/cover.jpg?X-Amz-Signature=one',
      position: 1,
    );
    final otherObject = auctionMediaLogicalKey(
      auctionId: 'auction-1',
      mediaReference:
          'https://cdn.example.com/auctions/other.jpg?X-Amz-Signature=one',
      position: 0,
    );

    expect(sameSlot, sameSlotRetry);
    expect(sameSlot, isNot(otherSlot));
    expect(sameSlot, isNot(otherObject));
  });
}
