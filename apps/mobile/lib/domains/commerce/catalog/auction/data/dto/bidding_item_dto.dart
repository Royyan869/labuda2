class BiddingItemDto {
  final String auctionId;
  final String title;
  final int yourLastBid;
  final int currentBid;
  final String status;
  final DateTime endAt;

  const BiddingItemDto({
    required this.auctionId,
    required this.title,
    required this.yourLastBid,
    required this.currentBid,
    required this.status,
    required this.endAt,
  });

  factory BiddingItemDto.fromJson(Map<String, dynamic> json) {
    return BiddingItemDto(
      auctionId: json['auction_id'] as String,
      title: json['title'] as String? ?? '',
      yourLastBid: (json['your_last_bid'] as num?)?.toInt() ?? 0,
      currentBid: (json['current_bid'] as num?)?.toInt() ?? 0,
      status: json['status'] as String? ?? '',
      endAt: DateTime.parse(json['end_at'] as String),
    );
  }
}
