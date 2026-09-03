import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:intl/intl.dart' as intl;

import 'app_localizations_en.dart';
import 'app_localizations_id.dart';

// ignore_for_file: type=lint

/// Callers can lookup localized strings with an instance of AppLocalizations
/// returned by `AppLocalizations.of(context)`.
///
/// Applications need to include `AppLocalizations.delegate()` in their app's
/// `localizationDelegates` list, and the locales they support in the app's
/// `supportedLocales` list. For example:
///
/// ```dart
/// import 'generated/app_localizations.dart';
///
/// return MaterialApp(
///   localizationsDelegates: AppLocalizations.localizationsDelegates,
///   supportedLocales: AppLocalizations.supportedLocales,
///   home: MyApplicationHome(),
/// );
/// ```
///
/// ## Update pubspec.yaml
///
/// Please make sure to update your pubspec.yaml to include the following
/// packages:
///
/// ```yaml
/// dependencies:
///   # Internationalization support.
///   flutter_localizations:
///     sdk: flutter
///   intl: any # Use the pinned version from flutter_localizations
///
///   # Rest of dependencies
/// ```
///
/// ## iOS Applications
///
/// iOS applications define key application metadata, including supported
/// locales, in an Info.plist file that is built into the application bundle.
/// To configure the locales supported by your app, you’ll need to edit this
/// file.
///
/// First, open your project’s ios/Runner.xcworkspace Xcode workspace file.
/// Then, in the Project Navigator, open the Info.plist file under the Runner
/// project’s Runner folder.
///
/// Next, select the Information Property List item, select Add Item from the
/// Editor menu, then select Localizations from the pop-up menu.
///
/// Select and expand the newly-created Localizations item then, for each
/// locale your application supports, add a new item and select the locale
/// you wish to add from the pop-up menu in the Value field. This list should
/// be consistent with the languages listed in the AppLocalizations.supportedLocales
/// property.
abstract class AppLocalizations {
  AppLocalizations(String locale)
    : localeName = intl.Intl.canonicalizedLocale(locale.toString());

  final String localeName;

  static AppLocalizations? of(BuildContext context) {
    return Localizations.of<AppLocalizations>(context, AppLocalizations);
  }

  static const LocalizationsDelegate<AppLocalizations> delegate =
      _AppLocalizationsDelegate();

  /// A list of this localizations delegate along with the default localizations
  /// delegates.
  ///
  /// Returns a list of localizations delegates containing this delegate along with
  /// GlobalMaterialLocalizations.delegate, GlobalCupertinoLocalizations.delegate,
  /// and GlobalWidgetsLocalizations.delegate.
  ///
  /// Additional delegates can be added by appending to this list in
  /// MaterialApp. This list does not have to be used at all if a custom list
  /// of delegates is preferred or required.
  static const List<LocalizationsDelegate<dynamic>> localizationsDelegates =
      <LocalizationsDelegate<dynamic>>[
        delegate,
        GlobalMaterialLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
      ];

  /// A list of this localizations delegate's supported locales.
  static const List<Locale> supportedLocales = <Locale>[
    Locale('en'),
    Locale('id'),
  ];

  /// The application name
  ///
  /// In en, this message translates to:
  /// **'LABUDA'**
  String get appName;

  /// The application description
  ///
  /// In en, this message translates to:
  /// **'Indonesian Koi Community'**
  String get appDescription;

  /// Login button text
  ///
  /// In en, this message translates to:
  /// **'Login'**
  String get login;

  /// Register button text
  ///
  /// In en, this message translates to:
  /// **'Register'**
  String get register;

  /// Logout button text
  ///
  /// In en, this message translates to:
  /// **'Logout'**
  String get logout;

  /// Logout success message
  ///
  /// In en, this message translates to:
  /// **'You have been logged out'**
  String get logoutSuccess;

  /// Login success message
  ///
  /// In en, this message translates to:
  /// **'Successfully logged in as'**
  String get loginSuccess;

  /// Registration coming soon message
  ///
  /// In en, this message translates to:
  /// **'Registration page coming soon'**
  String get registerComingSoon;

  /// Language change success message
  ///
  /// In en, this message translates to:
  /// **'Language changed successfully'**
  String get languageChanged;

  /// Language setting label
  ///
  /// In en, this message translates to:
  /// **'Language'**
  String get language;

  /// Indonesian language name
  ///
  /// In en, this message translates to:
  /// **'Bahasa Indonesia'**
  String get indonesian;

  /// English language name
  ///
  /// In en, this message translates to:
  /// **'English'**
  String get english;

  /// Koi community title
  ///
  /// In en, this message translates to:
  /// **'Indonesian Koi Community'**
  String get koiCommunity;

  /// Home tab label
  ///
  /// In en, this message translates to:
  /// **'Home'**
  String get home;

  /// Collections tab label
  ///
  /// In en, this message translates to:
  /// **'Collections'**
  String get products;

  /// Articles tab label
  ///
  /// In en, this message translates to:
  /// **'Articles'**
  String get articles;

  /// Settings tab label
  ///
  /// In en, this message translates to:
  /// **'Settings'**
  String get settings;

  /// Help and support menu item
  ///
  /// In en, this message translates to:
  /// **'Help & Support'**
  String get helpSupport;

  /// About LABUDA menu item
  ///
  /// In en, this message translates to:
  /// **'About LABUDA'**
  String get aboutLabuda;

  /// Coming soon text
  ///
  /// In en, this message translates to:
  /// **'coming soon'**
  String get comingSoon;

  /// Theme setting label
  ///
  /// In en, this message translates to:
  /// **'Theme'**
  String get theme;

  /// Light theme option
  ///
  /// In en, this message translates to:
  /// **'Light'**
  String get lightTheme;

  /// Dark theme option
  ///
  /// In en, this message translates to:
  /// **'Dark'**
  String get darkTheme;

  /// Profile label
  ///
  /// In en, this message translates to:
  /// **'Profile'**
  String get profile;

  /// Edit profile button
  ///
  /// In en, this message translates to:
  /// **'Edit Profile'**
  String get editProfile;

  /// Share button
  ///
  /// In en, this message translates to:
  /// **'Share'**
  String get share;

  /// Follow button
  ///
  /// In en, this message translates to:
  /// **'Follow'**
  String get follow;

  /// Chat button
  ///
  /// In en, this message translates to:
  /// **'Chat'**
  String get chat;

  /// Save button
  ///
  /// In en, this message translates to:
  /// **'Save'**
  String get save;

  /// Cancel button
  ///
  /// In en, this message translates to:
  /// **'Cancel'**
  String get cancel;

  /// Upgrade to seller title
  ///
  /// In en, this message translates to:
  /// **'Upgrade to Seller'**
  String get upgradeToSeller;

  /// Upgrade seller subtitle
  ///
  /// In en, this message translates to:
  /// **'Start selling your koi today!'**
  String get startSellingToday;

  /// Auction tab label
  ///
  /// In en, this message translates to:
  /// **'Auction'**
  String get auction;

  /// Collections tab label
  ///
  /// In en, this message translates to:
  /// **'Collections'**
  String get market;

  /// Messages label
  ///
  /// In en, this message translates to:
  /// **'Messages'**
  String get messages;

  /// Notifications title
  ///
  /// In en, this message translates to:
  /// **'Notifications'**
  String get notifications;

  /// Account settings section
  ///
  /// In en, this message translates to:
  /// **'Account Settings'**
  String get accountSettings;

  /// App preferences section
  ///
  /// In en, this message translates to:
  /// **'App Preferences'**
  String get appPreferences;

  /// Edit profile subtitle
  ///
  /// In en, this message translates to:
  /// **'Update your profile information'**
  String get updateProfileInfo;

  /// Address and payment menu
  ///
  /// In en, this message translates to:
  /// **'Address & Payment'**
  String get addressPayment;

  /// Address and payment subtitle
  ///
  /// In en, this message translates to:
  /// **'Manage shipping address and payment methods'**
  String get manageAddressPayment;

  /// Security menu
  ///
  /// In en, this message translates to:
  /// **'Security'**
  String get security;

  /// Security subtitle
  ///
  /// In en, this message translates to:
  /// **'Manage password, 2FA, and login sessions'**
  String get manageSecurity;

  /// Notification settings menu
  ///
  /// In en, this message translates to:
  /// **'Notification Settings'**
  String get notificationSettings;

  /// Notification settings subtitle
  ///
  /// In en, this message translates to:
  /// **'Manage email and push notifications'**
  String get manageNotifications;

  /// Privacy menu
  ///
  /// In en, this message translates to:
  /// **'Privacy'**
  String get privacy;

  /// Privacy subtitle
  ///
  /// In en, this message translates to:
  /// **'Manage profile visibility and data sharing'**
  String get managePrivacy;

  /// Help and support subtitle
  ///
  /// In en, this message translates to:
  /// **'Get help and contact support'**
  String get getHelp;

  /// Legal information menu
  ///
  /// In en, this message translates to:
  /// **'Legal Information'**
  String get legalInfo;

  /// Legal information subtitle
  ///
  /// In en, this message translates to:
  /// **'Terms of Service and Privacy Policy'**
  String get termsPrivacyPolicy;

  /// Notification preferences subtitle
  ///
  /// In en, this message translates to:
  /// **'Configure notification preferences'**
  String get configureNotificationPreferences;

  /// Privacy and security section
  ///
  /// In en, this message translates to:
  /// **'Privacy & Security'**
  String get privacySecurity;

  /// Public profile setting
  ///
  /// In en, this message translates to:
  /// **'Public Profile'**
  String get publicProfile;

  /// Public profile description
  ///
  /// In en, this message translates to:
  /// **'Make your profile visible to all users'**
  String get makeProfileVisible;

  /// Online status setting
  ///
  /// In en, this message translates to:
  /// **'Show Online Status'**
  String get showOnlineStatus;

  /// Online status description
  ///
  /// In en, this message translates to:
  /// **'Let others see when you\'re online'**
  String get letOthersSeeOnline;

  /// Messages setting
  ///
  /// In en, this message translates to:
  /// **'Allow Messages'**
  String get allowMessages;

  /// Messages description
  ///
  /// In en, this message translates to:
  /// **'Receive messages from other users'**
  String get receiveMessagesFromOthers;

  /// Blocked users setting
  ///
  /// In en, this message translates to:
  /// **'Blocked Users'**
  String get blockedUsers;

  /// Blocked users description
  ///
  /// In en, this message translates to:
  /// **'Manage blocked accounts'**
  String get manageBlockedAccounts;

  /// Business settings section
  ///
  /// In en, this message translates to:
  /// **'Business Settings'**
  String get businessSettings;

  /// Business profile setting
  ///
  /// In en, this message translates to:
  /// **'Business Profile'**
  String get businessProfile;

  /// Business profile description
  ///
  /// In en, this message translates to:
  /// **'Manage your business information'**
  String get manageBusinessInformation;

  /// Analytics setting
  ///
  /// In en, this message translates to:
  /// **'Analytics'**
  String get analytics;

  /// Analytics description
  ///
  /// In en, this message translates to:
  /// **'View your business analytics'**
  String get viewBusinessAnalytics;

  /// Payment settings
  ///
  /// In en, this message translates to:
  /// **'Payment Settings'**
  String get paymentSettings;

  /// Payment settings description
  ///
  /// In en, this message translates to:
  /// **'Manage payment methods and withdrawals'**
  String get managePaymentMethods;

  /// Support and legal section
  ///
  /// In en, this message translates to:
  /// **'Support & Legal'**
  String get supportLegal;

  /// Help and support title
  ///
  /// In en, this message translates to:
  /// **'Help & Support'**
  String get helpSupportTitle;

  /// Help support description
  ///
  /// In en, this message translates to:
  /// **'Get help and contact support'**
  String get getHelpContactSupport;

  /// Terms of service
  ///
  /// In en, this message translates to:
  /// **'Terms of Service'**
  String get termsOfService;

  /// Terms description
  ///
  /// In en, this message translates to:
  /// **'Read our terms and conditions'**
  String get readTermsConditions;

  /// Privacy policy
  ///
  /// In en, this message translates to:
  /// **'Privacy Policy'**
  String get privacyPolicy;

  /// Privacy policy description
  ///
  /// In en, this message translates to:
  /// **'Learn how we protect your data'**
  String get learnDataProtection;

  /// About LABUDA
  ///
  /// In en, this message translates to:
  /// **'About LABUDA'**
  String get aboutLABUDA;

  /// About app description
  ///
  /// In en, this message translates to:
  /// **'App version and information'**
  String get appVersionInformation;

  /// Account management section
  ///
  /// In en, this message translates to:
  /// **'Account Management'**
  String get accountManagement;

  /// Sign out
  ///
  /// In en, this message translates to:
  /// **'Sign Out'**
  String get signOut;

  /// Sign out description
  ///
  /// In en, this message translates to:
  /// **'Sign out of your account'**
  String get signOutAccount;

  /// Deactivate account
  ///
  /// In en, this message translates to:
  /// **'Deactivate Account'**
  String get deactivateAccount;

  /// Deactivate description
  ///
  /// In en, this message translates to:
  /// **'Temporarily deactivate your account'**
  String get temporarilyDeactivate;

  /// Guest role
  ///
  /// In en, this message translates to:
  /// **'Guest'**
  String get guest;

  /// User role
  ///
  /// In en, this message translates to:
  /// **'User'**
  String get user;

  /// Basic seller achievement badge (0-99 sales)
  ///
  /// In en, this message translates to:
  /// **'Basic Seller'**
  String get basicSellerBadge;

  /// Pro seller tier label
  ///
  /// In en, this message translates to:
  /// **'Pro Seller'**
  String get proSeller;

  /// Elite seller reputation tier badge
  ///
  /// In en, this message translates to:
  /// **'Elite Seller'**
  String get eliteSeller;

  /// Admin role
  ///
  /// In en, this message translates to:
  /// **'Admin'**
  String get admin;

  /// Moderator role
  ///
  /// In en, this message translates to:
  /// **'Moderator'**
  String get moderator;

  /// Tombstone shown when a chat message is hidden by moderation
  ///
  /// In en, this message translates to:
  /// **'Message hidden by moderator'**
  String get hiddenMessageByModerator;

  /// Buyer role
  ///
  /// In en, this message translates to:
  /// **'Buyer'**
  String get buyer;

  /// Current user type
  ///
  /// In en, this message translates to:
  /// **'Buyer'**
  String get currentUserType;

  /// Notification settings title
  ///
  /// In en, this message translates to:
  /// **'Notification Settings'**
  String get notificationSettingsTitle;

  /// Push notifications
  ///
  /// In en, this message translates to:
  /// **'Push Notifications'**
  String get pushNotifications;

  /// Push notifications description
  ///
  /// In en, this message translates to:
  /// **'Receive notifications on your device'**
  String get receiveNotificationsDevice;

  /// Email notifications
  ///
  /// In en, this message translates to:
  /// **'Email Notifications'**
  String get emailNotifications;

  /// Email notifications description
  ///
  /// In en, this message translates to:
  /// **'Receive notifications via email'**
  String get receiveNotificationsEmail;

  /// Marketing emails
  ///
  /// In en, this message translates to:
  /// **'Marketing Emails'**
  String get marketingEmails;

  /// Marketing emails description
  ///
  /// In en, this message translates to:
  /// **'Receive promotional and marketing emails'**
  String get receivePromotionalEmails;

  /// Save settings button
  ///
  /// In en, this message translates to:
  /// **'Save Settings'**
  String get saveSettings;

  /// Settings saved message
  ///
  /// In en, this message translates to:
  /// **'Notification settings saved'**
  String get notificationSettingsSaved;

  /// App description in about
  ///
  /// In en, this message translates to:
  /// **'LABUDA - Koi Social Commerce Platform'**
  String get koiSocialCommercePlatform;

  /// App version
  ///
  /// In en, this message translates to:
  /// **'Version 1.0.0'**
  String get version;

  /// Copyright text
  ///
  /// In en, this message translates to:
  /// **'© {year} LABUDA Team'**
  String copyrightLabudaTeam(Object year);

  /// LABUDA description
  ///
  /// In en, this message translates to:
  /// **'LABUDA is the first social commerce platform designed specifically for the Indonesian koi community.'**
  String get labudaDescription;

  /// Close button
  ///
  /// In en, this message translates to:
  /// **'Close'**
  String get close;

  /// Sign out confirmation
  ///
  /// In en, this message translates to:
  /// **'Are you sure you want to sign out of your account?'**
  String get signOutConfirm;

  /// Sign out success message
  ///
  /// In en, this message translates to:
  /// **'Signed out successfully'**
  String get signedOutSuccessfully;

  /// Deactivate account dialog title
  ///
  /// In en, this message translates to:
  /// **'Deactivate Account'**
  String get deactivateAccountTitle;

  /// Deactivate account description
  ///
  /// In en, this message translates to:
  /// **'Your account will be temporarily deactivated. You can reactivate it anytime by logging in again.'**
  String get deactivateAccountDescription;

  /// Deactivation reason label
  ///
  /// In en, this message translates to:
  /// **'Reason for deactivation:'**
  String get reasonForDeactivation;

  /// Select reason placeholder
  ///
  /// In en, this message translates to:
  /// **'Select a reason'**
  String get selectReason;

  /// Privacy concerns reason
  ///
  /// In en, this message translates to:
  /// **'Privacy concerns'**
  String get privacyConcerns;

  /// Not using app reason
  ///
  /// In en, this message translates to:
  /// **'Not using the app'**
  String get notUsingApp;

  /// Technical issues reason
  ///
  /// In en, this message translates to:
  /// **'Technical issues'**
  String get technicalIssues;

  /// Security concerns reason
  ///
  /// In en, this message translates to:
  /// **'Security concerns'**
  String get securityConcerns;

  /// Taking break reason
  ///
  /// In en, this message translates to:
  /// **'Taking a break'**
  String get takingBreak;

  /// Other reason
  ///
  /// In en, this message translates to:
  /// **'Other'**
  String get other;

  /// Additional notes label
  ///
  /// In en, this message translates to:
  /// **'Additional notes (optional)'**
  String get additionalNotesOptional;

  /// Additional notes placeholder
  ///
  /// In en, this message translates to:
  /// **'Please tell us more...'**
  String get pleaseTellUsMore;

  /// Deactivate button
  ///
  /// In en, this message translates to:
  /// **'Deactivate'**
  String get deactivate;

  /// User not authenticated error
  ///
  /// In en, this message translates to:
  /// **'User not authenticated'**
  String get userNotAuthenticated;

  /// Account deactivated success
  ///
  /// In en, this message translates to:
  /// **'Account deactivated successfully'**
  String get accountDeactivatedSuccessfully;

  /// Failed to deactivate account error
  ///
  /// In en, this message translates to:
  /// **'Failed to deactivate account'**
  String get failedToDeactivateAccount;

  /// Security screen title
  ///
  /// In en, this message translates to:
  /// **'Security'**
  String get securityTitle;

  /// Login required message
  ///
  /// In en, this message translates to:
  /// **'Please login to manage your account'**
  String get pleaseLoginToManage;

  /// Current account section
  ///
  /// In en, this message translates to:
  /// **'Current Account'**
  String get currentAccount;

  /// Email address label
  ///
  /// In en, this message translates to:
  /// **'Email Address'**
  String get emailAddress;

  /// Verified status
  ///
  /// In en, this message translates to:
  /// **'Verified'**
  String get verified;

  /// Unverified status
  ///
  /// In en, this message translates to:
  /// **'Unverified'**
  String get unverified;

  /// Resend verification button
  ///
  /// In en, this message translates to:
  /// **'Resend Verification Email'**
  String get resendVerificationEmail;

  /// Email management section
  ///
  /// In en, this message translates to:
  /// **'Email Management'**
  String get emailManagement;

  /// Change email title
  ///
  /// In en, this message translates to:
  /// **'Change Email Address'**
  String get changeEmailAddress;

  /// New email field
  ///
  /// In en, this message translates to:
  /// **'New Email Address'**
  String get newEmailAddress;

  /// New email placeholder
  ///
  /// In en, this message translates to:
  /// **'Enter your new email address'**
  String get enterNewEmailAddress;

  /// Current password field
  ///
  /// In en, this message translates to:
  /// **'Current Password'**
  String get currentPassword;

  /// Current password placeholder
  ///
  /// In en, this message translates to:
  /// **'Enter your current password to confirm'**
  String get enterCurrentPasswordToConfirm;

  /// Update email button
  ///
  /// In en, this message translates to:
  /// **'Update Email'**
  String get updateEmail;

  /// Email verification message
  ///
  /// In en, this message translates to:
  /// **'You will need to verify your new email address before you can use it.'**
  String get verifyNewEmailMessage;

  /// Password management section
  ///
  /// In en, this message translates to:
  /// **'Password Management'**
  String get passwordManagement;

  /// Change password title
  ///
  /// In en, this message translates to:
  /// **'Change Password'**
  String get changePassword;

  /// Current password placeholder
  ///
  /// In en, this message translates to:
  /// **'Enter your current password'**
  String get enterCurrentPassword;

  /// New password field
  ///
  /// In en, this message translates to:
  /// **'New Password'**
  String get newPassword;

  /// New password placeholder
  ///
  /// In en, this message translates to:
  /// **'Enter your new password'**
  String get enterNewPassword;

  /// Confirm password field
  ///
  /// In en, this message translates to:
  /// **'Confirm New Password'**
  String get confirmNewPassword;

  /// Confirm password placeholder
  ///
  /// In en, this message translates to:
  /// **'Confirm your new password'**
  String get confirmNewPasswordPlaceholder;

  /// Update password button
  ///
  /// In en, this message translates to:
  /// **'Update Password'**
  String get updatePassword;

  /// Strong password message
  ///
  /// In en, this message translates to:
  /// **'Choose a strong password with at least 8 characters, including uppercase, lowercase, and numbers.'**
  String get strongPasswordMessage;

  /// Advanced security section
  ///
  /// In en, this message translates to:
  /// **'Advanced Security'**
  String get advancedSecurity;

  /// Biometric login setting
  ///
  /// In en, this message translates to:
  /// **'Biometric Login'**
  String get biometricLogin;

  /// Biometric login description
  ///
  /// In en, this message translates to:
  /// **'Use fingerprint or face ID to login'**
  String get useFingerprintFaceId;

  /// 2FA description
  ///
  /// In en, this message translates to:
  /// **'Add extra security to your account'**
  String get addExtraSecurityAccount;

  /// Login sessions setting
  ///
  /// In en, this message translates to:
  /// **'Login Sessions'**
  String get loginSessions;

  /// No description provided for @noActiveSessions.
  ///
  /// In en, this message translates to:
  /// **'No active sessions found'**
  String get noActiveSessions;

  /// No description provided for @unknownDevice.
  ///
  /// In en, this message translates to:
  /// **'Unknown device'**
  String get unknownDevice;

  /// No description provided for @revokeSession.
  ///
  /// In en, this message translates to:
  /// **'Revoke'**
  String get revokeSession;

  /// No description provided for @revokeSessionTitle.
  ///
  /// In en, this message translates to:
  /// **'Revoke Session?'**
  String get revokeSessionTitle;

  /// No description provided for @revokeSessionMessage.
  ///
  /// In en, this message translates to:
  /// **'This device will be signed out. You\'ll need to sign in again on that device.'**
  String get revokeSessionMessage;

  /// No description provided for @signOutAllDevices.
  ///
  /// In en, this message translates to:
  /// **'Sign out all devices'**
  String get signOutAllDevices;

  /// No description provided for @signOutAllDevicesTitle.
  ///
  /// In en, this message translates to:
  /// **'Sign out all devices?'**
  String get signOutAllDevicesTitle;

  /// No description provided for @signOutAllDevicesMessage.
  ///
  /// In en, this message translates to:
  /// **'All active sessions will be terminated. You\'ll need to sign in again on all devices.'**
  String get signOutAllDevicesMessage;

  /// No description provided for @sessionRevokedSuccess.
  ///
  /// In en, this message translates to:
  /// **'Session revoked successfully'**
  String get sessionRevokedSuccess;

  /// No description provided for @allSessionsRevokedSuccess.
  ///
  /// In en, this message translates to:
  /// **'Signed out from all devices'**
  String get allSessionsRevokedSuccess;

  /// No description provided for @failedToLoadSessions.
  ///
  /// In en, this message translates to:
  /// **'Failed to load sessions. Please try again.'**
  String get failedToLoadSessions;

  /// No description provided for @lastActive.
  ///
  /// In en, this message translates to:
  /// **'Last active'**
  String get lastActive;

  /// Login sessions description
  ///
  /// In en, this message translates to:
  /// **'Manage your active sessions'**
  String get manageActiveSessions;

  /// Security features message
  ///
  /// In en, this message translates to:
  /// **'Enable additional security features to protect your account from unauthorized access.'**
  String get enableAdditionalSecurityMessage;

  /// Danger zone section
  ///
  /// In en, this message translates to:
  /// **'Danger Zone'**
  String get dangerZone;

  /// Delete account setting
  ///
  /// In en, this message translates to:
  /// **'Delete Account'**
  String get deleteAccount;

  /// Delete account description
  ///
  /// In en, this message translates to:
  /// **'Permanently delete your account'**
  String get permanentlyDeleteAccount;

  /// Permanent actions warning
  ///
  /// In en, this message translates to:
  /// **'These actions are permanent and cannot be undone. Please proceed with caution.'**
  String get permanentActionsWarning;

  /// Delete account confirmation
  ///
  /// In en, this message translates to:
  /// **'Are you sure you want to permanently delete your account? This action cannot be undone.'**
  String get deleteAccountConfirm;

  /// Delete button
  ///
  /// In en, this message translates to:
  /// **'Delete'**
  String get delete;

  /// Account deletion coming soon message
  ///
  /// In en, this message translates to:
  /// **'Account deletion coming soon'**
  String get accountDeletionComingSoon;

  /// Fill all fields error
  ///
  /// In en, this message translates to:
  /// **'Please fill all fields'**
  String get pleaseFillAllFields;

  /// Email change success message
  ///
  /// In en, this message translates to:
  /// **'Email change request sent! Check your new email for verification.'**
  String get emailChangeRequestSent;

  /// Failed to change email error
  ///
  /// In en, this message translates to:
  /// **'Failed to change email. Please try again.'**
  String get failedToChangeEmail;

  /// Generic error message
  ///
  /// In en, this message translates to:
  /// **'An error occurred. Please try again.'**
  String get anErrorOccurred;

  /// Password mismatch error
  ///
  /// In en, this message translates to:
  /// **'New passwords do not match'**
  String get newPasswordsDoNotMatch;

  /// Current password required error
  ///
  /// In en, this message translates to:
  /// **'Current password is required'**
  String get currentPasswordRequired;

  /// New password required error
  ///
  /// In en, this message translates to:
  /// **'New password is required'**
  String get newPasswordRequired;

  /// Confirm password required error
  ///
  /// In en, this message translates to:
  /// **'Please confirm your new password'**
  String get confirmPasswordRequired;

  /// Password update success
  ///
  /// In en, this message translates to:
  /// **'Password updated successfully!'**
  String get passwordUpdatedSuccessfully;

  /// Failed to change password error
  ///
  /// In en, this message translates to:
  /// **'Failed to change password. Please try again.'**
  String get failedToChangePassword;

  /// Verification email sent message
  ///
  /// In en, this message translates to:
  /// **'Verification email sent! Check your inbox.'**
  String get verificationEmailSent;

  /// Failed to send verification email error
  ///
  /// In en, this message translates to:
  /// **'Failed to send verification email. Please try again.'**
  String get failedToSendVerificationEmail;

  /// Address and payment screen title
  ///
  /// In en, this message translates to:
  /// **'Address & Payment'**
  String get addressPaymentTitle;

  /// Shipping address section
  ///
  /// In en, this message translates to:
  /// **'Shipping Address'**
  String get shippingAddress;

  /// Personal information section
  ///
  /// In en, this message translates to:
  /// **'Personal Information'**
  String get personalInformation;

  /// Payment methods section
  ///
  /// In en, this message translates to:
  /// **'Payment Methods'**
  String get paymentMethods;

  /// Bank account section
  ///
  /// In en, this message translates to:
  /// **'Bank Account Information'**
  String get bankAccountInformation;

  /// Primary shipping address title
  ///
  /// In en, this message translates to:
  /// **'Primary Shipping Address'**
  String get primaryShippingAddress;

  /// Street address field
  ///
  /// In en, this message translates to:
  /// **'Street Address'**
  String get streetAddress;

  /// Street address placeholder
  ///
  /// In en, this message translates to:
  /// **'Enter your street address'**
  String get enterStreetAddress;

  /// Street address required error
  ///
  /// In en, this message translates to:
  /// **'Street address is required'**
  String get streetAddressRequired;

  /// City field
  ///
  /// In en, this message translates to:
  /// **'City'**
  String get city;

  /// City placeholder
  ///
  /// In en, this message translates to:
  /// **'Enter city'**
  String get enterCity;

  /// City required error
  ///
  /// In en, this message translates to:
  /// **'City is required'**
  String get cityRequired;

  /// Province field
  ///
  /// In en, this message translates to:
  /// **'Province'**
  String get province;

  /// Province placeholder
  ///
  /// In en, this message translates to:
  /// **'Enter province'**
  String get enterProvince;

  /// Province required error
  ///
  /// In en, this message translates to:
  /// **'Province is required'**
  String get provinceRequired;

  /// Postal code field
  ///
  /// In en, this message translates to:
  /// **'Postal Code'**
  String get postalCode;

  /// Postal code placeholder
  ///
  /// In en, this message translates to:
  /// **'Enter postal code'**
  String get enterPostalCode;

  /// Postal code required error
  ///
  /// In en, this message translates to:
  /// **'Postal code is required'**
  String get postalCodeRequired;

  /// Invalid postal code error
  ///
  /// In en, this message translates to:
  /// **'Invalid postal code format'**
  String get invalidPostalCodeFormat;

  /// Country field
  ///
  /// In en, this message translates to:
  /// **'Country'**
  String get country;

  /// Country placeholder
  ///
  /// In en, this message translates to:
  /// **'Enter country'**
  String get enterCountry;

  /// Indonesia country
  ///
  /// In en, this message translates to:
  /// **'Indonesia'**
  String get indonesia;

  /// Use as billing address checkbox
  ///
  /// In en, this message translates to:
  /// **'Use as billing address'**
  String get useAsBillingAddress;

  /// Saved payment methods title
  ///
  /// In en, this message translates to:
  /// **'Saved Payment Methods'**
  String get savedPaymentMethods;

  /// GoPay payment method
  ///
  /// In en, this message translates to:
  /// **'GoPay'**
  String get gopay;

  /// GoPay payment description
  ///
  /// In en, this message translates to:
  /// **'Primary payment method • Ready for integration'**
  String get primaryPaymentMethodReady;

  /// Other e-wallets title
  ///
  /// In en, this message translates to:
  /// **'Other E-Wallets'**
  String get otherEWallets;

  /// Other e-wallets description
  ///
  /// In en, this message translates to:
  /// **'OVO, DANA, ShopeePay'**
  String get ovoDataShopeePay;

  /// Bank transfer title
  ///
  /// In en, this message translates to:
  /// **'Bank Transfer'**
  String get bankTransfer;

  /// Bank transfer description
  ///
  /// In en, this message translates to:
  /// **'BCA, Mandiri, BNI, BRI'**
  String get bcaMandiriBniBri;

  /// Credit/debit cards title
  ///
  /// In en, this message translates to:
  /// **'Credit/Debit Cards'**
  String get creditDebitCards;

  /// Credit cards description
  ///
  /// In en, this message translates to:
  /// **'Visa, Mastercard, JCB'**
  String get visaMastercardJcb;

  /// Secure payments message
  ///
  /// In en, this message translates to:
  /// **'Secure payments powered by GoPay. All transactions are encrypted and protected.'**
  String get securePaymentsMessage;

  /// Contact and identity title
  ///
  /// In en, this message translates to:
  /// **'Contact & Identity Information'**
  String get contactIdentityInformation;

  /// Phone number field
  ///
  /// In en, this message translates to:
  /// **'Phone Number'**
  String get phoneNumber;

  /// Phone number placeholder
  ///
  /// In en, this message translates to:
  /// **'Enter your phone number'**
  String get enterPhoneNumber;

  /// Invalid phone number error
  ///
  /// In en, this message translates to:
  /// **'Invalid phone number format'**
  String get invalidPhoneNumberFormat;

  /// Identity verification title
  ///
  /// In en, this message translates to:
  /// **'Identity Verification (Required)'**
  String get identityVerificationRequired;

  /// KTP number field
  ///
  /// In en, this message translates to:
  /// **'KTP Number'**
  String get ktpNumber;

  /// KTP number placeholder
  ///
  /// In en, this message translates to:
  /// **'Enter your KTP number (16 digits)'**
  String get enterKtpNumber;

  /// KTP number required error
  ///
  /// In en, this message translates to:
  /// **'KTP number is required'**
  String get ktpNumberRequired;

  /// KTP number format error
  ///
  /// In en, this message translates to:
  /// **'KTP number must be exactly 16 digits'**
  String get ktpNumberMust16Digits;

  /// Full name field
  ///
  /// In en, this message translates to:
  /// **'Full Name (as per KTP)'**
  String get fullNameAsPerKtp;

  /// Full name placeholder
  ///
  /// In en, this message translates to:
  /// **'Enter your full name as shown on KTP'**
  String get enterFullNameKtp;

  /// Full name required error
  ///
  /// In en, this message translates to:
  /// **'Full name is required'**
  String get fullNameRequired;

  /// Name too short error
  ///
  /// In en, this message translates to:
  /// **'Name too short'**
  String get nameTooShort;

  /// Name too long error
  ///
  /// In en, this message translates to:
  /// **'Name too long'**
  String get nameTooLong;

  /// Invalid name characters error
  ///
  /// In en, this message translates to:
  /// **'Invalid characters in name'**
  String get invalidCharactersInName;

  /// KTP photo title
  ///
  /// In en, this message translates to:
  /// **'KTP Photo'**
  String get ktpPhoto;

  /// Upload KTP photo button
  ///
  /// In en, this message translates to:
  /// **'Upload KTP Photo'**
  String get uploadKtpPhoto;

  /// Tap to select image text
  ///
  /// In en, this message translates to:
  /// **'Tap to select image'**
  String get tapToSelectImage;

  /// KTP verification message
  ///
  /// In en, this message translates to:
  /// **'KTP verification is required for account security and compliance. Upload a clear photo of your KTP. Your data is encrypted and secure.'**
  String get ktpVerificationMessage;

  /// Bank account title
  ///
  /// In en, this message translates to:
  /// **'Bank Account for Withdrawals'**
  String get bankAccountForWithdrawals;

  /// Manage button
  ///
  /// In en, this message translates to:
  /// **'Manage'**
  String get manage;

  /// Bank accounts description
  ///
  /// In en, this message translates to:
  /// **'Manage your bank accounts for receiving payments from sales. You can add multiple accounts and set one as primary.'**
  String get manageBankAccountsDescription;

  /// Account number field
  ///
  /// In en, this message translates to:
  /// **'Account Number'**
  String get accountNumber;

  /// Account number placeholder
  ///
  /// In en, this message translates to:
  /// **'Enter your bank account number'**
  String get enterBankAccountNumber;

  /// Account number required error
  ///
  /// In en, this message translates to:
  /// **'Account number is required'**
  String get accountNumberRequired;

  /// Invalid account number error
  ///
  /// In en, this message translates to:
  /// **'Invalid account number format'**
  String get invalidAccountNumberFormat;

  /// Account holder name field
  ///
  /// In en, this message translates to:
  /// **'Account Holder Name'**
  String get accountHolderName;

  /// Account holder name placeholder
  ///
  /// In en, this message translates to:
  /// **'Enter account holder name (as per bank records)'**
  String get enterAccountHolderName;

  /// Account holder name required error
  ///
  /// In en, this message translates to:
  /// **'Account holder name is required'**
  String get accountHolderNameRequired;

  /// Branch name field
  ///
  /// In en, this message translates to:
  /// **'Branch Name (Optional)'**
  String get branchNameOptional;

  /// Branch name placeholder
  ///
  /// In en, this message translates to:
  /// **'Enter bank branch name'**
  String get enterBankBranchName;

  /// Bank account withdrawal message
  ///
  /// In en, this message translates to:
  /// **'This bank account will be used for receiving withdrawals from sales. Make sure the account holder name matches your legal name.'**
  String get bankAccountWithdrawalMessage;

  /// Address payment saved success
  ///
  /// In en, this message translates to:
  /// **'Address and payment information saved successfully!'**
  String get addressPaymentSavedSuccessfully;

  /// Failed to save information error
  ///
  /// In en, this message translates to:
  /// **'Failed to save information. Please try again.'**
  String get failedToSaveInformation;

  /// GoPay integration title
  ///
  /// In en, this message translates to:
  /// **'GoPay Integration'**
  String get gopayIntegration;

  /// GoPay integration subtitle
  ///
  /// In en, this message translates to:
  /// **'Ready for seamless payments'**
  String get readyForSeamlessPayments;

  /// GoPay benefits title
  ///
  /// In en, this message translates to:
  /// **'Benefits of GoPay Integration:'**
  String get benefitsOfGopayIntegration;

  /// Instant payments feature
  ///
  /// In en, this message translates to:
  /// **'Instant Payments'**
  String get instantPayments;

  /// Instant payments description
  ///
  /// In en, this message translates to:
  /// **'Quick and secure transactions'**
  String get quickSecureTransactions;

  /// Bank level security feature
  ///
  /// In en, this message translates to:
  /// **'Bank-Level Security'**
  String get bankLevelSecurity;

  /// Bank level security description
  ///
  /// In en, this message translates to:
  /// **'Protected by GoPay\'s security system'**
  String get protectedByGopaySystem;

  /// Mobile first feature
  ///
  /// In en, this message translates to:
  /// **'Mobile First'**
  String get mobileFirst;

  /// Mobile first description
  ///
  /// In en, this message translates to:
  /// **'Optimized for mobile commerce'**
  String get optimizedForMobileCommerce;

  /// Auto receipts feature
  ///
  /// In en, this message translates to:
  /// **'Auto Receipts'**
  String get autoReceipts;

  /// Auto receipts description
  ///
  /// In en, this message translates to:
  /// **'Digital receipts for all transactions'**
  String get digitalReceiptsAllTransactions;

  /// Learn more button
  ///
  /// In en, this message translates to:
  /// **'Learn More'**
  String get learnMore;

  /// Setup GoPay button
  ///
  /// In en, this message translates to:
  /// **'Setup GoPay'**
  String get setupGopay;

  /// GoPay documentation coming soon
  ///
  /// In en, this message translates to:
  /// **'GoPay documentation coming soon'**
  String get gopayDocumentationComingSoon;

  /// GoPay integration coming soon
  ///
  /// In en, this message translates to:
  /// **'GoPay integration will be available soon'**
  String get gopayIntegrationAvailableSoon;

  /// KTP upload coming soon
  ///
  /// In en, this message translates to:
  /// **'KTP image upload feature coming soon'**
  String get ktpImageUploadComingSoon;

  /// Feature coming soon generic
  ///
  /// In en, this message translates to:
  /// **'feature coming soon'**
  String get featureComingSoon;

  /// Bank account management in progress
  ///
  /// In en, this message translates to:
  /// **'Bank Account Management - Implementation in progress'**
  String get bankAccountManagementInProgress;

  /// Upgrade to seller screen title
  ///
  /// In en, this message translates to:
  /// **'Upgrade to Seller'**
  String get upgradeToSellerTitle;

  /// Choose plan step title
  ///
  /// In en, this message translates to:
  /// **'Choose Your Plan'**
  String get chooseYourPlan;

  /// Business information step title
  ///
  /// In en, this message translates to:
  /// **'Business Information'**
  String get businessInformation;

  /// Review payment step title
  ///
  /// In en, this message translates to:
  /// **'Review & Payment'**
  String get reviewPayment;

  /// Step counter
  ///
  /// In en, this message translates to:
  /// **'Step {current} of {total}'**
  String stepOf(Object current, Object total);

  /// Choose plan description
  ///
  /// In en, this message translates to:
  /// **'Choose the plan that fits your business'**
  String get choosePlanThatFits;

  /// Basic seller plan title
  ///
  /// In en, this message translates to:
  /// **'Basic Seller'**
  String get basicSellerPlan;

  /// Basic seller price
  ///
  /// In en, this message translates to:
  /// **'Rp 99K'**
  String get basicSellerPrice;

  /// Per month text
  ///
  /// In en, this message translates to:
  /// **'/month'**
  String get perMonth;

  /// Basic seller feature 1
  ///
  /// In en, this message translates to:
  /// **'List up to 50 collections'**
  String get listUp50Products;

  /// Basic seller feature 2
  ///
  /// In en, this message translates to:
  /// **'Basic analytics dashboard'**
  String get basicAnalyticsDashboard;

  /// Basic seller feature 3
  ///
  /// In en, this message translates to:
  /// **'Standard customer support'**
  String get standardCustomerSupport;

  /// Basic seller feature 5
  ///
  /// In en, this message translates to:
  /// **'Basic store customization'**
  String get basicStoreCustomization;

  /// Pro seller plan title
  ///
  /// In en, this message translates to:
  /// **'Pro Seller'**
  String get proSellerPlan;

  /// Pro seller price
  ///
  /// In en, this message translates to:
  /// **'Rp 199K'**
  String get proSellerPrice;

  /// Pro seller feature 1
  ///
  /// In en, this message translates to:
  /// **'Unlimited collection listings'**
  String get unlimitedProductListings;

  /// Pro seller feature 2
  ///
  /// In en, this message translates to:
  /// **'Advanced analytics & insights'**
  String get advancedAnalyticsInsights;

  /// Pro seller feature 3
  ///
  /// In en, this message translates to:
  /// **'Priority customer support'**
  String get priorityCustomerSupport;

  /// Pro seller feature 5
  ///
  /// In en, this message translates to:
  /// **'Full store customization'**
  String get fullStoreCustomization;

  /// Pro seller feature 6
  ///
  /// In en, this message translates to:
  /// **'Featured listings'**
  String get featuredListings;

  /// Pro seller feature 7
  ///
  /// In en, this message translates to:
  /// **'Bulk upload tools'**
  String get bulkUploadTools;

  /// Pro seller feature 8
  ///
  /// In en, this message translates to:
  /// **'Marketing tools & promotions'**
  String get marketingToolsPromotions;

  /// Popular badge
  ///
  /// In en, this message translates to:
  /// **'POPULAR'**
  String get popular;

  /// Why become seller title
  ///
  /// In en, this message translates to:
  /// **'Why become a LABUDA Seller?'**
  String get whyBecomeLabudaSeller;

  /// Seller benefit 1
  ///
  /// In en, this message translates to:
  /// **'Reach 10,000+ active koi enthusiasts'**
  String get reach10kActiveKoiEnthusiasts;

  /// Seller benefit 2
  ///
  /// In en, this message translates to:
  /// **'Average seller earns Rp 5M+ per month'**
  String get averageSellerEarns5M;

  /// Seller benefit 3
  ///
  /// In en, this message translates to:
  /// **'Easy-to-use mobile selling tools'**
  String get easyMobileSellingTools;

  /// Seller benefit 4
  ///
  /// In en, this message translates to:
  /// **'Get featured on our homepage'**
  String get getFeaturedHomepage;

  /// Seller benefit 5
  ///
  /// In en, this message translates to:
  /// **'Join our professional seller community'**
  String get joinProfessionalSellerCommunity;

  /// Business info description
  ///
  /// In en, this message translates to:
  /// **'Tell us about your business'**
  String get tellUsAboutBusiness;

  /// Business name field
  ///
  /// In en, this message translates to:
  /// **'Business Name *'**
  String get businessNameRequired;

  /// Business name placeholder
  ///
  /// In en, this message translates to:
  /// **'e.g., Koi Farm Bandung'**
  String get businessNamePlaceholder;

  /// Business address field
  ///
  /// In en, this message translates to:
  /// **'Business Address *'**
  String get businessAddressRequired;

  /// Business address placeholder
  ///
  /// In en, this message translates to:
  /// **'Your business address'**
  String get businessAddressPlaceholder;

  /// Business phone field
  ///
  /// In en, this message translates to:
  /// **'Business Phone *'**
  String get businessPhoneRequired;

  /// Business phone placeholder
  ///
  /// In en, this message translates to:
  /// **'+62'**
  String get businessPhonePlaceholder;

  /// Business email field
  ///
  /// In en, this message translates to:
  /// **'Business Email *'**
  String get businessEmailRequired;

  /// Business email placeholder
  ///
  /// In en, this message translates to:
  /// **'business@example.com'**
  String get businessEmailPlaceholder;

  /// Business description field
  ///
  /// In en, this message translates to:
  /// **'Business Description'**
  String get businessDescription;

  /// Business description placeholder
  ///
  /// In en, this message translates to:
  /// **'Tell customers about your business...'**
  String get businessDescriptionPlaceholder;

  /// Review information description
  ///
  /// In en, this message translates to:
  /// **'Review your information'**
  String get reviewYourInformation;

  /// Selected plan title
  ///
  /// In en, this message translates to:
  /// **'Selected Plan'**
  String get selectedPlan;

  /// Plan basic seller text
  ///
  /// In en, this message translates to:
  /// **'Plan: Basic Seller'**
  String get planBasicSeller;

  /// Plan pro seller text
  ///
  /// In en, this message translates to:
  /// **'Plan: Pro Seller'**
  String get planProSeller;

  /// Basic price text
  ///
  /// In en, this message translates to:
  /// **'Price: Rp 99K/month'**
  String get priceRp99K;

  /// Pro price text
  ///
  /// In en, this message translates to:
  /// **'Price: Rp 199K/month'**
  String get priceRp199K;

  /// Business information summary title
  ///
  /// In en, this message translates to:
  /// **'Business Information'**
  String get businessInformationSummary;

  /// Name not provided text
  ///
  /// In en, this message translates to:
  /// **'Name: Not provided'**
  String get nameNotProvided;

  /// Phone not provided text
  ///
  /// In en, this message translates to:
  /// **'Phone: Not provided'**
  String get phoneNotProvided;

  /// Email not provided text
  ///
  /// In en, this message translates to:
  /// **'Email: Not provided'**
  String get emailNotProvided;

  /// Terms and agreements title
  ///
  /// In en, this message translates to:
  /// **'Terms & Agreements'**
  String get termsAgreements;

  /// Terms of service agreement
  ///
  /// In en, this message translates to:
  /// **'I agree to the Terms of Service'**
  String get agreeToTermsOfService;

  /// Data processing agreement
  ///
  /// In en, this message translates to:
  /// **'I agree to the Data Processing Policy'**
  String get agreeToDataProcessingPolicy;

  /// Payment method title
  ///
  /// In en, this message translates to:
  /// **'Payment Method'**
  String get paymentMethod;

  /// Payment integration future updates message
  ///
  /// In en, this message translates to:
  /// **'Payment integration will be implemented in future updates. For now, proceed to complete your seller registration.'**
  String get paymentIntegrationFutureUpdates;

  /// Back button
  ///
  /// In en, this message translates to:
  /// **'Back'**
  String get back;

  /// Continue button
  ///
  /// In en, this message translates to:
  /// **'Continue'**
  String get continueButton;

  /// Complete upgrade button
  ///
  /// In en, this message translates to:
  /// **'Complete Upgrade'**
  String get completeUpgrade;

  /// Basic seller upgrade success
  ///
  /// In en, this message translates to:
  /// **'Congratulations! You are now a Basic Seller!'**
  String get congratulationsBasicSeller;

  /// Pro seller upgrade success
  ///
  /// In en, this message translates to:
  /// **'Congratulations! You are now a Pro Seller!'**
  String get congratulationsProSeller;

  /// Upgrade failed error
  ///
  /// In en, this message translates to:
  /// **'Upgrade failed'**
  String get upgradeFailed;

  /// Upgrade failed unknown error
  ///
  /// In en, this message translates to:
  /// **'Upgrade failed: Unknown error'**
  String get upgradeFailedUnknownError;

  /// Coming soon feature message
  ///
  /// In en, this message translates to:
  /// **'{feature} coming soon'**
  String comingSoonFeature(Object feature);

  /// Quick help section title
  ///
  /// In en, this message translates to:
  /// **'Quick Help'**
  String get quickHelp;

  /// Browse by category section title
  ///
  /// In en, this message translates to:
  /// **'Browse by Category'**
  String get browseByCategory;

  /// Popular articles section title
  ///
  /// In en, this message translates to:
  /// **'Popular Articles'**
  String get popularArticles;

  /// Help center header title
  ///
  /// In en, this message translates to:
  /// **'How can we help you today?'**
  String get howCanWeHelp;

  /// Help center header description
  ///
  /// In en, this message translates to:
  /// **'Find answers quickly or get in touch with our support team.'**
  String get helpCenterDescription;

  /// Search help articles placeholder
  ///
  /// In en, this message translates to:
  /// **'Search help articles...'**
  String get searchHelpArticles;

  /// Orders category
  ///
  /// In en, this message translates to:
  /// **'Orders'**
  String get orders;

  /// Payments category
  ///
  /// In en, this message translates to:
  /// **'Payments'**
  String get payments;

  /// Selling category
  ///
  /// In en, this message translates to:
  /// **'Selling'**
  String get selling;

  /// Account category
  ///
  /// In en, this message translates to:
  /// **'Account'**
  String get account;

  /// Verification category
  ///
  /// In en, this message translates to:
  /// **'Verification'**
  String get verification;

  /// Technical category
  ///
  /// In en, this message translates to:
  /// **'Technical'**
  String get technical;

  /// Order help subtitle
  ///
  /// In en, this message translates to:
  /// **'Track, cancel, or refund orders'**
  String get orderHelpSubtitle;

  /// Payment help subtitle
  ///
  /// In en, this message translates to:
  /// **'Payment methods and failed transactions'**
  String get paymentHelpSubtitle;

  /// Selling help subtitle
  ///
  /// In en, this message translates to:
  /// **'Become a seller and manage your store'**
  String get sellingHelpSubtitle;

  /// Account help subtitle
  ///
  /// In en, this message translates to:
  /// **'Profile, password, and settings'**
  String get accountHelpSubtitle;

  /// Verification help subtitle
  ///
  /// In en, this message translates to:
  /// **'Seller verification and KTP'**
  String get verificationHelpSubtitle;

  /// Technical help subtitle
  ///
  /// In en, this message translates to:
  /// **'App issues and troubleshooting'**
  String get technicalHelpSubtitle;

  /// Still need help section title
  ///
  /// In en, this message translates to:
  /// **'Still need help?'**
  String get stillNeedHelp;

  /// Contact support description
  ///
  /// In en, this message translates to:
  /// **'Our support team is here to help you with any questions or issues.'**
  String get contactSupportDescription;

  /// Contact support button
  ///
  /// In en, this message translates to:
  /// **'Contact Support'**
  String get contactSupport;

  /// Help article screen title
  ///
  /// In en, this message translates to:
  /// **'Help Article'**
  String get helpArticle;

  /// Was this helpful feedback question
  ///
  /// In en, this message translates to:
  /// **'Was this helpful?'**
  String get wasThisHelpful;

  /// Yes button
  ///
  /// In en, this message translates to:
  /// **'Yes'**
  String get yes;

  /// No button
  ///
  /// In en, this message translates to:
  /// **'No'**
  String get no;

  /// Feedback thanks message
  ///
  /// In en, this message translates to:
  /// **'Thank you for your feedback!'**
  String get feedbackThanks;

  /// Article: How to pay
  ///
  /// In en, this message translates to:
  /// **'How to pay for my order?'**
  String get articleHowToPay;

  /// Article: How to pay content
  ///
  /// In en, this message translates to:
  /// **'To pay for your order:\n\n1. Go to your order from the Orders screen\n2. Tap the \'Pay Now\' button\n3. Select your payment method (GoPay, Bank Transfer, etc.)\n4. Follow the instructions to complete payment\n5. Your payment will be confirmed within 24 hours\n\nIf payment fails, you can retry from the order screen.'**
  String get articleHowToPayContent;

  /// Article: Track order
  ///
  /// In en, this message translates to:
  /// **'How to track my order?'**
  String get articleTrackOrder;

  /// Article: Track order content
  ///
  /// In en, this message translates to:
  /// **'To track your order:\n\n1. Go to \'My Orders\' from the profile menu\n2. Tap on the order you want to track\n3. You\'ll see the current status and tracking information\n\nOrder statuses:\n- Pending: Waiting for payment\n- Processing: Seller is preparing your order\n- Shipped: Order is on the way\n- Delivered: Order has arrived\n- Completed: Order is finished'**
  String get articleTrackOrderContent;

  /// Article: Request refund
  ///
  /// In en, this message translates to:
  /// **'How to request a refund?'**
  String get articleRequestRefund;

  /// Article: Request refund content
  ///
  /// In en, this message translates to:
  /// **'To request a refund:\n\n1. Open the order details\n2. Tap \'Request Refund\'\n3. Select a reason and describe the issue\n4. Upload unboxing video as proof (required)\n5. Submit your request\n\nThe seller will review your request. If rejected, you can escalate to admin for final decision.'**
  String get articleRequestRefundContent;

  /// Article: Become seller
  ///
  /// In en, this message translates to:
  /// **'How to become a seller?'**
  String get articleBecomeSeller;

  /// Article: Become seller content
  ///
  /// In en, this message translates to:
  /// **'To become a seller on LABUDA:\n\n1. Go to Settings → Upgrade to Seller\n2. Choose your plan (Basic or Pro)\n3. Fill in your business information\n4. Complete payment for the subscription\n5. Wait for verification approval\n\nOnce approved, you can start listing your koi for sale!'**
  String get articleBecomeSellerContent;

  /// Article: Cancel order
  ///
  /// In en, this message translates to:
  /// **'How to cancel an order?'**
  String get articleCancelOrder;

  /// Article: Cancel order content
  ///
  /// In en, this message translates to:
  /// **'Order cancellation depends on the status:\n\n- Pending payment: Auto-cancelled if not paid within 24 hours\n- Processing: Contact seller to cancel\n- Shipped: Cannot cancel, use refund process instead\n\nTo request cancellation, tap the order and select \'Contact Seller\' to discuss.'**
  String get articleCancelOrderContent;

  /// Article: Complete order
  ///
  /// In en, this message translates to:
  /// **'How to complete an order?'**
  String get articleConfirmDelivery;

  /// Article: Complete order content
  ///
  /// In en, this message translates to:
  /// **'To complete your order after receiving items:\n\n1. Open the order details\n2. Tap \'Terima Barang\' button\n3. Confirm that items match your order\n\nIf you don\'t confirm within 5 days of shipment, the order will auto-complete.'**
  String get articleConfirmDeliveryContent;

  /// Article: Payment failed
  ///
  /// In en, this message translates to:
  /// **'Payment failed, what to do?'**
  String get articlePaymentFailed;

  /// Article: Payment failed content
  ///
  /// In en, this message translates to:
  /// **'If your payment fails:\n\n1. Check your payment method balance\n2. Try a different payment method\n3. Ensure you have stable internet connection\n4. Retry payment from the order screen\n\nIf the issue persists, contact support with your order number.'**
  String get articlePaymentFailedContent;

  /// Article: Refund time
  ///
  /// In en, this message translates to:
  /// **'How long does refund take?'**
  String get articleRefundTime;

  /// Article: Refund time content
  ///
  /// In en, this message translates to:
  /// **'Refund processing time:\n\n1. Seller review: 1-3 days\n2. If approved: 3-7 business days for funds to return\n\nThe exact time depends on your payment method. GoPay refunds are usually faster than bank transfers.'**
  String get articleRefundTimeContent;

  /// Article: Create listing
  ///
  /// In en, this message translates to:
  /// **'How to create a listing?'**
  String get articleCreateListing;

  /// Article: Create listing content
  ///
  /// In en, this message translates to:
  /// **'To create a new listing:\n\n1. Tap the + button on the home screen\n2. Select \'Listing\'\n3. Add photos of your koi (multiple angles recommended)\n4. Fill in details (variety, size, price, location)\n5. Write a description\n6. Publish your listing\n\nYour listing will be visible to buyers immediately!'**
  String get articleCreateListingContent;

  /// Article: Shipping setup
  ///
  /// In en, this message translates to:
  /// **'How to set up shipping?'**
  String get articleShippingSetup;

  /// Article: Shipping setup content
  ///
  /// In en, this message translates to:
  /// **'To set up shipping:\n\n1. Go to Seller Dashboard\n2. Select a shipping partner (available options)\n3. Set your shipping zones and rates\n4. Add packaging instructions\n\nAlways use proper packaging with oxygen for live koi shipping!'**
  String get articleShippingSetupContent;

  /// Article: Edit profile
  ///
  /// In en, this message translates to:
  /// **'How to edit my profile?'**
  String get articleEditProfile;

  /// Article: Edit profile content
  ///
  /// In en, this message translates to:
  /// **'To edit your profile:\n\n1. Go to your profile screen\n2. Tap the edit icon\n3. Update your information:\n   - Profile photo\n   - Display name\n   - Bio\n   - Location\n4. Tap \'Save\' to apply changes'**
  String get articleEditProfileContent;

  /// Article: Change password
  ///
  /// In en, this message translates to:
  /// **'How to change my password?'**
  String get articleChangePassword;

  /// Article: Change password content
  ///
  /// In en, this message translates to:
  /// **'To change your password:\n\n1. Go to Settings → Security\n2. Tap \'Change Password\'\n3. Enter your current password\n4. Enter your new password (min 8 characters)\n5. Confirm the new password\n6. Tap \'Update Password\'\n\nYou\'ll be logged out from other devices after changing password.'**
  String get articleChangePasswordContent;

  /// Article: Seller verification
  ///
  /// In en, this message translates to:
  /// **'Seller verification requirements'**
  String get articleSellerVerification;

  /// Article: Seller verification content
  ///
  /// In en, this message translates to:
  /// **'Seller verification requires:\n\n1. Valid KTP (Indonesian ID card)\n2. Clear photo of KTP\n3. KTP number (16 digits)\n4. Name matching KTP\n5. Business address\n6. Active phone number\n\nVerification usually takes 1-3 business days.'**
  String get articleSellerVerificationContent;

  /// Article: App not working
  ///
  /// In en, this message translates to:
  /// **'App not working properly?'**
  String get articleAppNotWorking;

  /// Article: App not working content
  ///
  /// In en, this message translates to:
  /// **'If the app is not working:\n\n1. Check your internet connection\n2. Close and reopen the app\n3. Clear app cache (Settings → Clear Cache)\n4. Update to the latest app version\n5. Restart your phone\n\nIf issues persist, contact support with details of what\'s not working.'**
  String get articleAppNotWorkingContent;

  /// Article: Clear cache
  ///
  /// In en, this message translates to:
  /// **'How to clear app cache?'**
  String get articleClearCache;

  /// Article: Clear cache content
  ///
  /// In en, this message translates to:
  /// **'To clear app cache:\n\n1. Go to Settings\n2. Scroll to \'App Preferences\'\n3. Tap \'Clear Cache\'\n4. Confirm when prompted\n\nThis will free up storage but won\'t delete your data. You\'ll need to log in again.'**
  String get articleClearCacheContent;

  /// Label for Pro tier seller badge
  ///
  /// In en, this message translates to:
  /// **'Pro Seller'**
  String get sellerTierPro;

  /// Label for Elite tier seller badge
  ///
  /// In en, this message translates to:
  /// **'Elite Seller'**
  String get sellerTierElite;

  /// Title for suspended account screen
  ///
  /// In en, this message translates to:
  /// **'Account Suspended'**
  String get accountSuspendedTitle;

  /// Title for banned account screen
  ///
  /// In en, this message translates to:
  /// **'Account Banned'**
  String get accountBannedTitle;

  /// Message for suspended account
  ///
  /// In en, this message translates to:
  /// **'Your account has been temporarily suspended. During the suspension period, you cannot access app features.'**
  String get accountSuspendedMessage;

  /// Message for banned account
  ///
  /// In en, this message translates to:
  /// **'Your account has been permanently banned for violating our terms of service. You cannot access app features.'**
  String get accountBannedMessage;

  /// Support message for restricted accounts
  ///
  /// In en, this message translates to:
  /// **'If you believe this is a mistake, please contact our support team via email.'**
  String get accountRestrictedSupport;
}

class _AppLocalizationsDelegate
    extends LocalizationsDelegate<AppLocalizations> {
  const _AppLocalizationsDelegate();

  @override
  Future<AppLocalizations> load(Locale locale) {
    return SynchronousFuture<AppLocalizations>(lookupAppLocalizations(locale));
  }

  @override
  bool isSupported(Locale locale) =>
      <String>['en', 'id'].contains(locale.languageCode);

  @override
  bool shouldReload(_AppLocalizationsDelegate old) => false;
}

AppLocalizations lookupAppLocalizations(Locale locale) {
  // Lookup logic when only language code is specified.
  switch (locale.languageCode) {
    case 'en':
      return AppLocalizationsEn();
    case 'id':
      return AppLocalizationsId();
  }

  throw FlutterError(
    'AppLocalizations.delegate failed to load unsupported locale "$locale". This is likely '
    'an issue with the localizations generation tool. Please file an issue '
    'on GitHub with a reproducible sample app and the gen-l10n configuration '
    'that was used.',
  );
}
