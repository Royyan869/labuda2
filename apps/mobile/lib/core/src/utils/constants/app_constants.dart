class AppConstants {
  AppConstants._();

  // App Information
  static const String appName = 'LABUDA';
  static const String appVersion = '1.0.0';
  static const String appDescription =
      'Social Commerce Platform untuk Komunitas Koi Indonesia';

  // Firebase Collections
  static const String usersCollection = 'users';
  static const String postsCollection = 'posts';
  static const String chatMessagesCollection = 'chat_messages';
  static const String chatThreadsCollection = 'chat_threads';
  static const String transactionsCollection = 'transactions';
  static const String reportsCollection = 'reports';
  static const String notificationsCollection = 'notifications';

  // User Roles
  static const String roleBuyer = 'buyer';
  static const String roleBasicSeller = 'basic_seller';
  static const String roleProSeller = 'pro_seller';
  static const String roleSupportAdmin = 'support_admin';
  static const String roleSuperAdmin = 'super_admin';

  // Media Limits
  static const int maxPhotosPerPost = 10;
  static const int maxVideosPerPost = 1;
  static const int maxVideoSizeMb = 100;
  static const int maxCommerceVideoDurationMs = 60000;
  static const int maxContentVideoDurationMs = 60000;
  static const int maxPhotoSizeMb = 10;

  // Text Limits
  static const int maxTitleLength = 100;
  static const int maxDescriptionLength = 2000;
  static const int maxCommentLength = 500;

  // Koi Specific
  // Note: Use KoiVarieties.all from core/constants/koi_varieties.dart
  // This getter maintained for backward compatibility
  @Deprecated('Use KoiVarieties.all instead')
  static List<String> get koiVarieties => [
    'Kohaku',
    'Sanke',
    'Showa',
    'Asagi',
    'Shusui',
    'Bekko',
    'Utsurimono',
    'Goshiki',
    'Kujaku',
    'Other',
  ];

  static const List<String> koiGenders = ['uncheck', 'male', 'female'];

  // Anti-Circumvention Keywords
  static const List<String> bannedKeywords = [
    'wa',
    'whatsapp',
    'ig',
    'instagram',
    'line',
    'telegram',
    'facebook',
    'fb',
    'chat',
    'dm',
  ];

  // Phone number detection pattern
  static const String phoneNumberPattern = r'08\d{8,}';

  // API Endpoints (will be configured per environment)
  static const String baseUrl = 'https://api.labuda.app';
  static const String awsS3BaseUrl = 'https://labuda-media.s3.amazonaws.com';

  // CDN Configuration
  static const String cloudFrontDomain = 'd358tu61i1wrtt.cloudfront.net';
  static const String cdnBaseUrl = 'https://$cloudFrontDomain';

  // Media URL Strategy
  static const bool useCloudFront = true; // Enabled for faster media loading

  // Development Settings
  static const bool isDevelopment =
      bool.fromEnvironment('dart.vm.product') == false;
  static const bool useDevCorsWorkaround = false; // Set true for local dev only
}
