/// Checkout Honesty Messages
///
/// UX Honesty constants for checkout failure messaging.
/// These messages reinforce the business truth that:
/// - First-come-first-served means checkout can fail due to availability
/// - This is normal business truth, not a system error
/// - Clear guidance on what to do next
///
/// **AUCTION WINNER FLOW:** Messages are framed as claiming a victory, not generic purchase
///
/// Every constant here is referenced by the checkout screen. The former
/// `CheckoutErrorType` classifier (error-code → copy dispatch) and the constants
/// no screen ever rendered were purged: error-code dispatch lives in
/// `checkout_screen_impl.dart` (`_showOrderError` + the notifier error listener),
/// so a second, unused classification table was a duplicate authority.
library;

class CheckoutHonestyMessages {
  // General availability failure
  static const String forSaleUnavailableTitle = 'Produk Tidak Tersedia';
  static const String forSaleUnavailableMessage =
      'Maaf, produk yang ingin Anda beli sudah tidak tersedia. '
      'Produk mungkin telah terjual atau dihapus oleh penjual.';

  static const String outOfStockTitle = 'Stok Habis';
  static const String outOfStockMessage =
      'Maaf, stok produk sudah habis. '
      'Stok dapat berubah sewaktu-waktu karena prinsip siapa cepat dia dapat.';

  // Source-specific failures
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

  // First-come-first-served explanation
  static const String firstComeFirstServedExplanation =
      'Sistem kami menggunakan prinsip siapa cepat dia dapat. '
      'Barang tetap tersedia untuk semua pembeli sampai pesanan berhasil dibuat. '
      'Ini berarti ada kemungkinan barang sudah terjual saat Anda melakukan checkout.';

  // Shipping-related errors
  static const String shippingAddressInvalidTitle = 'Alamat Tidak Valid';
  static const String shippingAddressInvalidMessage =
      'Mohon periksa kembali alamat pengiriman Anda.';
}
