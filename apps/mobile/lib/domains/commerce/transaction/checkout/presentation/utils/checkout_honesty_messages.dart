/// Checkout Honesty Messages
///
/// UX Honesty constants for checkout failure messaging.
/// These messages reinforce the business truth that:
/// - First-come-first-served means checkout can fail due to availability
/// - This is normal business truth, not a system error
/// - Clear guidance on what to do next
///
/// **AUCTION WINNER FLOW:** Messages are framed as claiming a victory, not generic purchase
library;

class CheckoutHonestyMessages {
  // General availability failure
  static const String listingUnavailableTitle = 'Produk Tidak Tersedia';
  static const String listingUnavailableMessage =
      'Maaf, produk yang ingin Anda beli sudah tidak tersedia. '
      'Produk mungkin telah terjual atau dihapus oleh penjual.';

  static const String outOfStockTitle = 'Stok Habis';
  static const String outOfStockMessage =
      'Maaf, stok produk sudah habis. '
      'Stok dapat berubah sewaktu-waktu karena prinsip siapa cepat dia dapat.';

  // Source-specific failures
  static const String shortlistItemUnavailableTitle =
      'Item Tersimpan Tidak Tersedia';
  static const String shortlistItemUnavailableMessage =
      'Item yang Anda simpan sudah tidak tersedia. '
      'Silakan hapus item ini dari daftar tersimpan atau cari produk lain.';

  static const String negotiationUnavailableTitle = 'Negosiasi Tidak Valid';
  static const String negotiationUnavailableMessage =
      'Harga negosiasi masih valid, namun produk terkait sudah tidak tersedia. '
      'Silakan cari produk lain atau hubungi penjual.';

  static const String quoteUnavailableTitle = 'Penawaran Tidak Valid';
  static const String quoteUnavailableMessage =
      'Penawaran dari penjual masih ada, namun produk terkait sudah tidak tersedia. '
      'Silakan cari produk lain atau minta penawaran baru.';

  static const String auctionUnavailableTitle = 'Lelang Tidak Valid';
  static const String auctionUnavailableMessage =
      'Lelang ini sudah tidak tersedia untuk checkout. '
      'Mungkin sudah berakhir atau produk terjual.';

  // Auction winner-specific messages - framed as claiming victory
  static const String auctionWinnerExpiredTitle =
      'Waktu Klaim Kemenangan Habis';
  static const String auctionWinnerExpiredMessage =
      'Waktu untuk mengamankan kemenangan lelang Anda sudah habis. '
      'Silakan hubungi penjual untuk informasi lebih lanjut.';

  static const String auctionWinnerInvalidTitle = 'Kemenangan Tidak Valid';
  static const String auctionWinnerInvalidMessage =
      'Kemenangan lelang ini tidak dapat diproses. '
      'Status leang mungkin telah berubah.';

  // Pricing token failures
  static const String tokenExpiredTitle = 'Waktu Harga Habis';
  static const String tokenExpiredMessage =
      'Harga yang ditampilkan sudah kadaluarsa. '
      'Silakan refresh untuk mendapatkan harga terbaru.';

  static const String tokenInvalidTitle = 'Token Tidak Valid';
  static const String tokenInvalidMessage =
      'Token harga tidak valid. Silakan muat ulang halaman checkout.';

  // Recovery suggestions
  static const String suggestionBrowseProducts =
      'Silakan cari produk lain di marketplace.';
  static const String suggestionRefreshPricing =
      'Tap "Refresh Harga" untuk mencoba lagi.';
  static const String suggestionContactSeller =
      'Hubungi penjual untuk informasi lebih lanjut.';
  static const String suggestionRemoveInvalidItem =
      'Hapus item tidak valid dari daftar tersimpan.';
  static const String suggestionReturnToListing =
      'Kembali ke halaman produk untuk melihat status terbaru.';
  static const String suggestionTryAgain = 'Coba lagi dalam beberapa saat.';

  // First-come-first-served explanation
  static const String firstComeFirstServedExplanation =
      'Sistem kami menggunakan prinsip siapa cepat dia dapat. '
      'Barang tetap tersedia untuk semua pembeli sampai pesanan berhasil dibuat. '
      'Ini berarti ada kemungkinan barang sudah terjual saat Anda melakukan checkout.';

  static const String availabilityChangedExplanation =
      'Ketersediaan barang dapat berubah sewaktu-waktu. '
      'Kami mengecek ketersediaan saat Anda melakukan checkout, '
      'bukan saat barang disimpan.';

  // Shipping-related errors
  static const String shippingAddressInvalidTitle = 'Alamat Tidak Valid';
  static const String shippingAddressInvalidMessage =
      'Mohon periksa kembali alamat pengiriman Anda.';
}

/// Checkout Error Classification
class CheckoutErrorType {
  static const String listingUnavailable = 'LISTING_UNAVAILABLE';
  static const String outOfStock = 'OUT_OF_STOCK';
  static const String pricingTokenExpired = 'PRICING_TOKEN_EXPIRED';
  static const String pricingTokenInvalid = 'PRICING_TOKEN_INVALID';
  static const String negotiationUnavailable = 'NEGOTIATION_UNAVAILABLE';
  static const String quoteUnavailable = 'QUOTE_UNAVAILABLE';
  static const String auctionUnavailable = 'AUCTION_UNAVAILABLE';
  static const String auctionWinnerExpired = 'AUCTION_WINNER_EXPIRED';
  static const String auctionWinnerInvalid = 'AUCTION_WINNER_INVALID';
  static const String shippingInvalid = 'SHIPPING_INVALID';
  static const String genericError = 'GENERIC_ERROR';

  /// Get user-friendly title for error code
  static String getTitleForErrorCode(String errorCode) {
    switch (errorCode) {
      case listingUnavailable:
        return CheckoutHonestyMessages.listingUnavailableTitle;
      case outOfStock:
        return CheckoutHonestyMessages.outOfStockTitle;
      case pricingTokenExpired:
        return CheckoutHonestyMessages.tokenExpiredTitle;
      case pricingTokenInvalid:
        return CheckoutHonestyMessages.tokenInvalidTitle;
      case negotiationUnavailable:
        return CheckoutHonestyMessages.negotiationUnavailableTitle;
      case quoteUnavailable:
        return CheckoutHonestyMessages.quoteUnavailableTitle;
      case auctionUnavailable:
        return CheckoutHonestyMessages.auctionUnavailableTitle;
      case auctionWinnerExpired:
        return CheckoutHonestyMessages.auctionWinnerExpiredTitle;
      case auctionWinnerInvalid:
        return CheckoutHonestyMessages.auctionWinnerInvalidTitle;
      case shippingInvalid:
        return CheckoutHonestyMessages.shippingAddressInvalidTitle;
      default:
        return 'Terjadi Kesalahan';
    }
  }

  /// Get user-friendly message for error code
  static String getMessageForErrorCode(String errorCode) {
    switch (errorCode) {
      case listingUnavailable:
        return CheckoutHonestyMessages.listingUnavailableMessage;
      case outOfStock:
        return CheckoutHonestyMessages.outOfStockMessage;
      case pricingTokenExpired:
        return CheckoutHonestyMessages.tokenExpiredMessage;
      case pricingTokenInvalid:
        return CheckoutHonestyMessages.tokenInvalidMessage;
      case negotiationUnavailable:
        return CheckoutHonestyMessages.negotiationUnavailableMessage;
      case quoteUnavailable:
        return CheckoutHonestyMessages.quoteUnavailableMessage;
      case auctionUnavailable:
        return CheckoutHonestyMessages.auctionUnavailableMessage;
      case auctionWinnerExpired:
        return CheckoutHonestyMessages.auctionWinnerExpiredMessage;
      case auctionWinnerInvalid:
        return CheckoutHonestyMessages.auctionWinnerInvalidMessage;
      case shippingInvalid:
        return CheckoutHonestyMessages.shippingAddressInvalidMessage;
      default:
        return 'Terjadi kesalahan. Silakan coba lagi.';
    }
  }
}
