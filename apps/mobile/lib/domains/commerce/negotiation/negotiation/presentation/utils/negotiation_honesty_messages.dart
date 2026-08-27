/// Negotiation Honesty Messages
///
/// UX Honesty constants for negotiation-related messaging.
/// These messages reinforce the business truth that:
/// - Negotiation provides a price path, NOT inventory hold
/// - Accepted negotiation doesn't reserve the item
/// - First-come-first-served applies until order is created
library;

class NegotiationHonestyMessages {
  // Primary honesty messages
  static const String negotiationDoesNotReserve =
      'Negosiasi yang disetujui memberikan harga spesial, '
      'TIDAK mengunci stok barang.';

  static const String pricePathOnly =
      'Kesepakatan harga adalah jalur pembelian dengan harga tertentu, '
      'bukan reservasi barang.';

  static const String firstComeFirstServed =
      'Barang tetap dapat dibeli oleh pembeli lain sampai pesanan Anda berhasil dibuat.';

  // Checkout warnings
  static const String listingUnavailableWarning =
      'Maaf, barang untuk negosiasi ini sudah tidak tersedia. '
      'Harga negosiasi masih valid, tapi barangnya sudah terjual atau dihapus.';

  static const String checkoutBlockedExplanation =
      'Negosiasi telah disetujui, namun barang terkait sudah tidak tersedia. '
      'Silakan cari barang lain atau hubungi penjual.';

  static const String createOrderFailedExplanation =
      'Pesanan gagal dibuat karena barang sudah tidak tersedia. '
      'Negosiasi harga Anda masih tersimpan di riwayat chat.';

  // Status messages
  static const String acceptedButListingUnavailable =
      'Negosiasi Disetujui - Barang Tidak Tersedia';

  static String get explanationForAcceptedWithoutReserve {
    return 'Negosiasi disetujui memberikan Anda hak untuk membeli '
        'dengan harga yang disepakati. Namun, barang tetap '
        'tersedia untuk pembeli lain sampai pesanan berhasil dibuat.';
  }
}

/// Negotiation Honesty Tooltips
class NegotiationHonestyTooltips {
  static const String acceptedDoesNotReserve =
      'Negosiasi yang disetujui TIDAK mengunci barang. '
      'Pembeli lain masih bisa membeli sampai pesanan Anda dibuat.';

  static const String checkoutUrgency =
      'Segera lakukan checkout setelah negosiasi disetujui '
      'untuk mengurangi risiko barang habis terjual.';
}
