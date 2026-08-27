/// Shipping Expectation Messages
///
/// UX Honesty constants for shipping-related messaging.
/// These messages reinforce the business truth that:
/// - Shipping is seller-managed, NOT platform-controlled logistics
/// - Shipping options and rates are configured by sellers
/// - Tracking and proof are provided by sellers, not live courier integration
library;

class ShippingHonestyMessages {
  // Primary honesty messages
  static const String sellerManagedShipping =
      'Pengiriman dikelola langsung oleh penjual.';

  static const String notPlatformLogistics =
      'Platform tidak mengontrol pengiriman secara langsung. '
      'Penjual yang mengatur opsi dan biaya pengiriman.';

  // Shipping options explanation
  static const String optionsBySeller =
      'Opsi dan biaya pengiriman ditentukan oleh penjual '
      'berdasarkan lokasi Anda.';

  static const String rateEstimated =
      'Biaya pengiriman adalah perkiraan dari penjual. '
      'Biaya aktual dapat berbeda tergantung kondisi pengiriman.';

  // Tracking explanation
  static const String trackingBySeller =
      'Nomor resi dan bukti pengiriman diberikan oleh penjual. '
      'Tracking real-time tersedia jika penjual menyediakannya.';

  static const String notLiveTracking =
      'Platform tidak memiliki integrasi live tracking dengan kurir. '
      'Silakan hubungi penjual untuk update pengiriman.';

  // Shipping proof
  static const String proofProvidedBySeller =
      'Bukti pengiriman (foto/resi) diupload oleh penjual '
      'sebagai bukti barang telah dikirim.';

  // Dispute and trust
  static const String platformFacilitatesTrust =
      'Platform memfasilitasi escrow dan dispute resolution '
      'untuk keamanan transaksi.';

  // Checkout section message
  static const String checkoutShippingInfo =
      'Opsi pengiriman dari penjual akan muncul setelah Anda memasukkan alamat. '
      'Biaya dihitung berdasarkan lokasi Anda.';

  // Order detail section message
  static const String orderDetailShippingInfo =
      'Penjual akan mengatur pengiriman setelah pembayaran dikonfirmasi. '
      'Anda akan menerima notifikasi saat barang dikirim.';
}

/// Shipping Honesty Tooltips
class ShippingHonestyTooltips {
  static const String shippingCostSource =
      'Ongkir berasal dari konfigurasi penjual, '
      'bukan perhitungan otomatis platform.';

  static const String shippingTime =
      'Estimasi waktu pengiriman adalah perkiraan dari penjual. '
      'Waktu aktual dapat berbeda.';

  static const String tracking =
      'Nomor resi akan muncul setelah penjual mengirim barang. '
      'Gunakan nomor tersebut untuk tracking di website kurir.';
}

/// Shipping Section Labels
class ShippingSectionLabels {
  static const String sellerManagedLabel = 'Pengiriman oleh Penjual';
  static const String optionsLabel = 'Pilihan Opsi Pengiriman';
  static const String costLabel = 'Biaya Pengiriman (ditentukan Penjual)';
  static const String trackingLabel = 'Tracking (dari Penjual)';
  static const String proofLabel = 'Bukti Pengiriman';

  static const String disclaimer =
      'Platform memfasilitasi transaksi aman dengan escrow. '
      'Pengiriman dikelola langsung oleh penjual masing-masing.';
}
