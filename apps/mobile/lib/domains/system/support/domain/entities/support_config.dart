library;

/// Domain Entities for Support Configuration
/// Pure Dart - bebas dari Firebase, Flutter, dan external dependencies

import 'support_ticket.dart';

// ============================================
// CONFIGURATION ENTITIES
// ============================================

/// Support Identity - Generic brand identity untuk support chat
class SupportIdentity {
  static const String poolId = 'labuda_support_team';
  static const String displayName = 'HiShumi Support';
  static const String tagline = 'Typically replies in 30 minutes';

  // Avatar URL
  static const String avatarUrl =
      'https://firebasestorage.googleapis.com/v0/b/labuda-79de2.firebasestorage.app/o/assets%2Flabuda_logo.png?alt=media';

  // Support email
  static const String supportEmail = 'support@labuda.com';

  const SupportIdentity._(); // Private constructor
}

/// SLA (Service Level Agreement) Targets
class SLATargets {
  /// Target first response time: 30 minutes
  static const Duration firstResponseTime = Duration(minutes: 30);

  /// Target resolution time: 24 hours
  static const Duration resolutionTime = Duration(hours: 24);

  /// Urgent ticket first response: 10 minutes
  static const Duration urgentFirstResponse = Duration(minutes: 10);

  const SLATargets._();

  /// Check if first response time met SLA
  static bool isFirstResponseOnTime(DateTime created, DateTime responded) {
    return responded.difference(created) <= firstResponseTime;
  }

  /// Check if resolution time met SLA
  static bool isResolutionOnTime(DateTime created, DateTime resolved) {
    return resolved.difference(created) <= resolutionTime;
  }
}

/// Category presentation Configuration (icon + colour only).
///
/// Category *identity* is `SupportCategory.wireValue`; the category display
/// label is user-facing content owned by the canonical `AppLocalizations`
/// authority (see the `SupportCategoryLabel` extension in the presentation
/// layer). CategoryConfig intentionally carries no label/name string.
class CategoryConfig {
  final String icon;
  final String emoji;
  final int colorValue;

  const CategoryConfig({
    required this.icon,
    required this.emoji,
    required this.colorValue,
  });

  /// Presentation config keyed by the canonical category taxonomy. The keys are
  /// exactly the nine canonical values — no legacy category exists.
  static const Map<SupportCategory, CategoryConfig> configs = {
    SupportCategory.orderIssue: CategoryConfig(
      icon: '📦',
      emoji: '📦',
      colorValue: 0xFF3B82F6, // Blue
    ),
    SupportCategory.paymentIssue: CategoryConfig(
      icon: '💳',
      emoji: '💰',
      colorValue: 0xFF10B981, // Green
    ),
    SupportCategory.accountIssue: CategoryConfig(
      icon: '👤',
      emoji: '🔐',
      colorValue: 0xFFF59E0B, // Orange
    ),
    SupportCategory.listingIssue: CategoryConfig(
      icon: '🏷️',
      emoji: '🏷️',
      colorValue: 0xFF14B8A6, // Teal
    ),
    SupportCategory.shippingIssue: CategoryConfig(
      icon: '🚚',
      emoji: '🚚',
      colorValue: 0xFF0EA5E9, // Sky
    ),
    SupportCategory.refundRequest: CategoryConfig(
      icon: '💸',
      emoji: '💸',
      colorValue: 0xFF22C55E, // Green
    ),
    SupportCategory.dispute: CategoryConfig(
      icon: '⚖️',
      emoji: '⚖️',
      colorValue: 0xFFEF4444, // Red
    ),
    SupportCategory.technicalIssue: CategoryConfig(
      icon: '🔧',
      emoji: '⚙️',
      colorValue: 0xFF8B5CF6, // Purple
    ),
    SupportCategory.other: CategoryConfig(
      icon: '❓',
      emoji: '💬',
      colorValue: 0xFF6B7280, // Gray
    ),
  };

  static CategoryConfig get(SupportCategory category) {
    return configs[category] ??
        const CategoryConfig(icon: '❓', emoji: '💬', colorValue: 0xFF6B7280);
  }
}

/// Priority Configuration (icon + colour only).
///
/// Priority *identity* is the enum itself (`SupportPriority.name`, the
/// backend / API contract); the priority display label is user-facing content
/// owned by the canonical `AppLocalizations` authority (see the
/// `SupportPriorityLabel` extension in the presentation layer). PriorityConfig
/// intentionally carries no label/name string.
class PriorityConfig {
  final String icon;
  final int colorValue;

  const PriorityConfig({required this.icon, required this.colorValue});

  static const Map<SupportPriority, PriorityConfig> configs = {
    SupportPriority.urgent: PriorityConfig(
      icon: '🔴',
      colorValue: 0xFFEF4444, // Red
    ),
    SupportPriority.high: PriorityConfig(
      icon: '🟠',
      colorValue: 0xFFF97316, // Orange
    ),
    SupportPriority.medium: PriorityConfig(
      icon: '🟡',
      colorValue: 0xFFF59E0B, // Yellow
    ),
    SupportPriority.low: PriorityConfig(
      icon: '🟢',
      colorValue: 0xFF10B981, // Green
    ),
  };

  static PriorityConfig get(SupportPriority priority) {
    return configs[priority] ??
        const PriorityConfig(icon: '⚪', colorValue: 0xFF6B7280);
  }

  /// Get priority order untuk sorting (lower = higher priority)
  static int getOrder(SupportPriority priority) {
    switch (priority) {
      case SupportPriority.urgent:
        return 0;
      case SupportPriority.high:
        return 1;
      case SupportPriority.medium:
        return 2;
      case SupportPriority.low:
        return 3;
    }
  }
}

/// Status Configuration (icon + colour only).
///
/// Status *identity* is `SupportStatus.wireValue`; the status display label is
/// user-facing content owned by the canonical `AppLocalizations` authority (see
/// the `SupportStatusLabel` extension in the presentation layer). StatusConfig
/// intentionally carries no label/name string.
class StatusConfig {
  final String icon;
  final int colorValue;

  const StatusConfig({required this.icon, required this.colorValue});

  static const Map<SupportStatus, StatusConfig> configs = {
    SupportStatus.open: StatusConfig(
      icon: '🆕',
      colorValue: 0xFF3B82F6, // Blue
    ),
    SupportStatus.inProgress: StatusConfig(
      icon: '⏳',
      colorValue: 0xFFF59E0B, // Orange
    ),
    SupportStatus.waitingUser: StatusConfig(
      icon: '⏰',
      colorValue: 0xFF8B5CF6, // Purple
    ),
    SupportStatus.resolved: StatusConfig(
      icon: '✅',
      colorValue: 0xFF10B981, // Green
    ),
    SupportStatus.closed: StatusConfig(
      icon: '🔒',
      colorValue: 0xFF6B7280, // Gray
    ),
  };

  static StatusConfig get(SupportStatus status) {
    return configs[status] ??
        const StatusConfig(icon: '❓', colorValue: 0xFF6B7280);
  }
}

/// Greeting Templates - untuk auto-greeting system
class GreetingTemplates {
  /// Friendly style greetings
  static const List<String> friendly = [
    'Hai, saya Budi dari HiShumi. Ada yang bisa saya bantu? 😊',
    'Halo! Erna di sini dari HiShumi, siap membantu Anda!',
    'Hi, saya Sari dari HiShumi Support. Bagaimana saya bisa bantu hari ini?',
    'Hai! Saya Andi, siap untuk membantu Anda 🌱',
    'Halo, Dewi dari HiShumi Support. Ada kendala yang bisa saya bantu?',
    'Hi! Rudi di sini, ada yang ingin ditanyakan?',
    'Hai, saya Lina dari tim HiShumi. Mari saya bantu selesaikan masalah Anda 😊',
    'Halo! Saya Eko, bagian dari HiShumi Support. Silakan ceritakan kendala Anda!',
  ];

  /// Professional style greetings
  static const List<String> professional = [
    'Selamat {time}, saya Agus dari HiShumi Customer Support. Bagaimana saya dapat membantu Anda?',
    'Halo, Fitri dari tim HiShumi Support. Terima kasih telah menghubungi kami. Ada yang bisa saya bantu?',
    'Salam, saya Hendra dari HiShumi Support Team. Saya siap membantu menyelesaikan masalah Anda.',
    'Selamat {time}, Maya di sini dari HiShumi. Silakan sampaikan kendala yang Anda alami.',
    'Halo, saya Bambang dari HiShumi Customer Support. Kami siap membantu Anda.',
  ];

  /// Casual style greetings
  static const List<String> casual = [
    'Hei! Toni di sini dari HiShumi Support 👋 Ada yang bisa dibantu?',
    'Hi! Saya Dina dari support team. Gimana, ada masalah apa nih?',
    'Hai! Riko here, siap bantu kamu! Ada kendala apa? 😊',
    'Halo! Yuni dari HiShumi nih, ada yang perlu bantuan?',
    'Hai, saya Arif! Ada yang bisa aku bantu? 🌱',
    'Hi! Nina siap membantu. Ada pertanyaan atau kendala?',
  ];

  const GreetingTemplates._();

  /// Get greeting time (pagi, siang, sore, malam)
  static String getGreetingTime() {
    final hour = DateTime.now().hour;
    if (hour < 12) return 'pagi';
    if (hour < 15) return 'siang';
    if (hour < 18) return 'sore';
    return 'malam';
  }

  /// Replace placeholders in template
  static String format(String template, {required String name}) {
    return template
        .replaceAll('{name}', name)
        .replaceAll('{time}', getGreetingTime());
  }

  /// Get random greeting from style
  static String getRandom({
    required String style,
    required String adminFirstName,
  }) {
    List<String> templates;
    switch (style) {
      case 'professional':
        templates = professional;
        break;
      case 'casual':
        templates = casual;
        break;
      case 'friendly':
      default:
        templates = friendly;
        break;
    }

    // Simple random (in real app, use dart:math Random)
    final index = DateTime.now().millisecond % templates.length;
    final template = templates[index];
    return format(template, name: adminFirstName);
  }
}

/// Quick Reply Templates - untuk admin
class QuickReplies {
  static const Map<String, List<String>> templates = {
    'acknowledgment': [
      'Baik, saya cek dulu ya...',
      'Mohon tunggu sebentar, saya sedang mengecek...',
      'Terima kasih infonya, saya akan bantu cek sistemnya',
      'Saya mengerti masalahnya, izinkan saya cek lebih detail',
    ],
    'checking_order': [
      'Saya sudah cek order Anda. {details}',
      'Berdasarkan data yang saya lihat, {details}',
      'Order dengan ID {orderId} status nya adalah {status}',
    ],
    'checking_payment': [
      'Saya sudah cek transaksi pembayaran Anda...',
      'Berdasarkan history pembayaran, {details}',
      'Pembayaran Anda dengan nominal {amount} sudah kami terima',
    ],
    'resolved': [
      'Masalahnya sudah diselesaikan. Silakan dicek kembali ya!',
      'Sudah saya proses dan seharusnya sudah beres. Bisa dicoba lagi?',
      'Alhamdulillah sudah selesai. Kalau masih ada kendala, kabari saya lagi ya!',
    ],
    'follow_up': [
      'Ada yang lain yang bisa saya bantu?',
      'Apakah masih ada yang ingin ditanyakan?',
      'Semoga masalahnya sudah teratasi ya. Ada hal lain?',
    ],
    'closing': [
      'Terima kasih sudah menghubungi HiShumi! Semoga harimu menyenangkan 🌱',
      'Terima kasih! Jangan ragu hubungi kami lagi jika ada kendala ya.',
      'Senang bisa membantu! Have a great day! 😊',
    ],
    'escalation': [
      'Untuk kasus ini, saya perlu bantuan tim terkait. Mohon ditunggu ya.',
      'Saya akan eskalasi ke tim yang lebih spesialis handle kasus ini.',
    ],
  };

  const QuickReplies._();

  static List<String> get(String category) {
    return templates[category] ?? [];
  }
}

/// Support-related utility functions
class SupportUtils {
  const SupportUtils._();

  /// Get priority from keywords in message
  static SupportPriority detectPriority(String message) {
    final urgentKeywords = ['urgent', 'mendesak', 'segera', 'cepat', 'tolong'];
    final lowercaseMessage = message.toLowerCase();

    for (final keyword in urgentKeywords) {
      if (lowercaseMessage.contains(keyword)) {
        return SupportPriority.urgent;
      }
    }

    return SupportPriority.medium; // Default
  }

  /// Get category from keywords (basic auto-categorization)
  static SupportCategory? detectCategory(String message) {
    final lowercaseMessage = message.toLowerCase();

    // Refund keywords
    if (lowercaseMessage.contains('refund') ||
        lowercaseMessage.contains('pengembalian dana')) {
      return SupportCategory.refundRequest;
    }

    // Payment keywords
    if (lowercaseMessage.contains('bayar') ||
        lowercaseMessage.contains('payment') ||
        lowercaseMessage.contains('saldo')) {
      return SupportCategory.paymentIssue;
    }

    // Shipping keywords
    if (lowercaseMessage.contains('pengiriman') ||
        lowercaseMessage.contains('kirim') ||
        lowercaseMessage.contains('ongkir')) {
      return SupportCategory.shippingIssue;
    }

    // Order keywords
    if (lowercaseMessage.contains('pesanan') ||
        lowercaseMessage.contains('order')) {
      return SupportCategory.orderIssue;
    }

    // Technical keywords
    if (lowercaseMessage.contains('error') ||
        lowercaseMessage.contains('bug') ||
        lowercaseMessage.contains('crash') ||
        lowercaseMessage.contains('tidak bisa')) {
      return SupportCategory.technicalIssue;
    }

    // Account keywords
    if (lowercaseMessage.contains('akun') ||
        lowercaseMessage.contains('account') ||
        lowercaseMessage.contains('login') ||
        lowercaseMessage.contains('password')) {
      return SupportCategory.accountIssue;
    }

    return null; // Cannot auto-detect, user must select
  }

  /// Sort tickets by option
  static void sortTickets(List<SupportTicket> tickets, SortOption option) {
    switch (option) {
      case SortOption.newest:
        tickets.sort((a, b) => b.createdAt.compareTo(a.createdAt));
        break;
      case SortOption.priority:
        tickets.sort(
          (a, b) => PriorityConfig.getOrder(
            a.priority,
          ).compareTo(PriorityConfig.getOrder(b.priority)),
        );
        break;
      case SortOption.waitingLongest:
        tickets.sort((a, b) => a.createdAt.compareTo(b.createdAt));
        break;
    }
  }
}
