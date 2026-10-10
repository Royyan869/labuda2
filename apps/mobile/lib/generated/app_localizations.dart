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
  /// **'HiShumi'**
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

  /// Messages setting
  ///
  /// In en, this message translates to:
  /// **'Allow Messages'**
  String get allowMessages;

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

  /// About HiShumi
  ///
  /// In en, this message translates to:
  /// **'About HiShumi'**
  String get aboutHiShumi;

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
  /// **'HiShumi - Koi Social Commerce Platform'**
  String get koiSocialCommercePlatform;

  /// App version
  ///
  /// In en, this message translates to:
  /// **'Version 1.0.0'**
  String get version;

  /// Copyright text
  ///
  /// In en, this message translates to:
  /// **'Â© {year} Labuda Team'**
  String copyrightLabudaTeam(Object year);

  /// HiShumi description
  ///
  /// In en, this message translates to:
  /// **'HiShumi is the first social commerce platform designed specifically for the Indonesian koi community.'**
  String get hishumiDescription;

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
  /// **'Primary payment method â€¢ Ready for integration'**
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
  /// **'Unlimited For Sale'**
  String get unlimitedForSale;

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
  /// **'Featured For Sale'**
  String get featuredForSale;

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
  /// **'To pay for your order:\n\n1. Go to your order from the Orders screen\n2. Tap the \'Pay Now\' button\n3. Select your payment method (GoPay, Bank Transfer, etc.)\n4. Follow the instructions to complete payment\n\nComplete the payment before the deadline shown on the order screen. If the deadline passes, the order is cancelled automatically.\n\nYour order status updates as soon as the payment is confirmed. If payment fails, you can retry from the order screen.'**
  String get articleHowToPayContent;

  /// Article: Track order
  ///
  /// In en, this message translates to:
  /// **'How to track my order?'**
  String get articleTrackOrder;

  /// Article: Track order content
  ///
  /// In en, this message translates to:
  /// **'To track your order:\n\n1. Go to \'My Orders\' from the profile menu\n2. Tap on the order you want to track\n3. You\'ll see the current status, shipping info, and tracking number\n\nOrder statuses:\n- Waiting for Payment: Your order has been created, please complete your payment\n- Being Prepared: Payment received, the seller is preparing your order\n- In Delivery: The order has been shipped\n- Completed: The order is finished\n\nIf the seller does not ship within the preparation time shown on the order, the order can be cancelled.'**
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
  /// **'To become a seller on HiShumi:\n\n1. Go to Settings â†’ Upgrade to Seller\n2. Choose your plan (Basic or Pro)\n3. Fill in your business information\n4. Complete payment for the subscription\n5. Wait for verification approval\n\nOnce approved, you can start creating For Sale for your koi!'**
  String get articleBecomeSellerContent;

  /// Article: Cancel order
  ///
  /// In en, this message translates to:
  /// **'How to cancel an order?'**
  String get articleCancelOrder;

  /// Article: Cancel order content
  ///
  /// In en, this message translates to:
  /// **'Order cancellation depends on the status:\n\n- Waiting for Payment: Cancel from the order details, or let it expire automatically if the payment deadline passes\n- Being Prepared or In Delivery: The order can\'t be cancelled directly, use the refund process instead\n- Seller overdue to ship: You can cancel the order from the order details\n- Completed: Use the refund process if there is a problem\n\nTo discuss cancellation, open the order and select \'Chat Seller\'.'**
  String get articleCancelOrderContent;

  /// Article: Complete order
  ///
  /// In en, this message translates to:
  /// **'How to complete an order?'**
  String get articleConfirmDelivery;

  /// Article: Complete order content
  ///
  /// In en, this message translates to:
  /// **'To complete your order after receiving the items:\n\n1. Open the order details\n2. Tap the \'Confirm Receipt\' button\n3. Confirm that the items match your order\n\nYou have 5 days from the moment the seller marks the order as shipped to confirm receipt or open a dispute. If you don\'t confirm in time, the order completes automatically.\n\nIf your order hasn\'t arrived yet and the 5 days are almost over, you can use \'Extend Confirmation\' once to extend the deadline by 3 days.'**
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
  /// **'Refund processing depends on the case:\n\n1. Submit the refund request with the required evidence\n2. The seller reviews the request. If it is rejected, you can escalate to admin for the final decision\n3. If the refund is approved, the funds are returned to your original payment method\n\nThe review time and the time for the funds to arrive depend on the case and your payment provider. You can follow the status from the order details.'**
  String get articleRefundTimeContent;

  /// Article: Create For Sale
  ///
  /// In en, this message translates to:
  /// **'How to create a For Sale?'**
  String get articleCreateForSale;

  /// Article: Create For Sale content
  ///
  /// In en, this message translates to:
  /// **'To create a new For Sale:\n\n1. Tap the + button on the home screen\n2. Select \'For Sale\'\n3. Add photos of your koi (multiple angles recommended)\n4. Fill in details (variety, size, price, location)\n5. Write a description\n6. Publish your For Sale\n\nYour For Sale will be visible to buyers immediately!'**
  String get articleCreateForSaleContent;

  /// Article: Shipping setup
  ///
  /// In en, this message translates to:
  /// **'How to set up shipping?'**
  String get articleShippingSetup;

  /// Article: Shipping setup content
  ///
  /// In en, this message translates to:
  /// **'To set up shipping:\n\n1. Go to Settings â†’ Pengiriman, or Seller Dashboard â†’ Atur Pengiriman\n2. Add a shipping option (train, bus, travel, plane, or custom)\n3. Set the province coverage with the rate you charge for each province\n4. Toggle the option active to make it available for your For Sales\n5. When creating a For Sale, choose which of your options apply to that For Sale\n\nShipping is seller-managed: you decide the options, rates, and courier. For irregular cases (large fish, special handling), send a shipping quote to the buyer in chat as a fallback.\n\nAlways use proper packaging with oxygen for live koi shipping!'**
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
  /// **'To change your password:\n\n1. Go to Settings â†’ Security\n2. Tap \'Change Password\'\n3. Enter your current password\n4. Enter your new password (min 8 characters)\n5. Confirm the new password\n6. Tap \'Update Password\'\n\nYou\'ll be logged out from other devices after changing password.'**
  String get articleChangePasswordContent;

  /// Article: Seller verification
  ///
  /// In en, this message translates to:
  /// **'Seller verification requirements'**
  String get articleSellerVerification;

  /// Article: Seller verification content
  ///
  /// In en, this message translates to:
  /// **'Seller verification requires:\n\n1. Valid KTP (Indonesian ID card)\n2. Clear photo of KTP\n3. KTP number (16 digits)\n4. Name matching KTP\n5. Business address\n6. Active phone number\n\nVerification usually takes 1-2 business days.'**
  String get articleSellerVerificationContent;

  /// Article: App not working
  ///
  /// In en, this message translates to:
  /// **'App not working properly?'**
  String get articleAppNotWorking;

  /// Article: App not working content
  ///
  /// In en, this message translates to:
  /// **'If the app is not working properly:\n\n1. Check your internet connection\n2. Close and reopen the app\n3. Make sure you are using the latest app version\n4. Restart your phone\n\nIf the problem continues, contact support with details of what\'s not working.'**
  String get articleAppNotWorkingContent;

  /// Article: App slow or not loading
  ///
  /// In en, this message translates to:
  /// **'App is slow or not loading?'**
  String get articleAppSlowOrNotLoading;

  /// Article: App slow or not loading content
  ///
  /// In en, this message translates to:
  /// **'If the app feels slow or a screen is not loading:\n\n1. Check your internet connection â€” try Wi-Fi or mobile data\n2. Close the app completely and open it again\n3. Make sure you are using the latest app version\n4. Restart your phone\n\nIf the problem continues, contact support and mention which screen is affected.'**
  String get articleAppSlowOrNotLoadingContent;

  /// Article: Withdrawal failed
  ///
  /// In en, this message translates to:
  /// **'Withdrawal failed, what to do?'**
  String get articleWithdrawalFailed;

  /// Article: Withdrawal failed content
  ///
  /// In en, this message translates to:
  /// **'If your withdrawal fails:\n\n1. Check that your bank account details are correct\n2. Make sure your seller verification (KTP) is complete\n3. Check that the amount meets the minimum withdrawal shown on the Earnings screen\n\nWithdrawals are reviewed by admin first. Once approved, the funds are transferred to your registered bank account within 1-3 business days.\n\nNext steps:\nâ€¢ Open the Earnings screen and check the withdrawal status\nâ€¢ If the withdrawal failed, check your bank details and submit a new request\nâ€¢ Contact support if the funds were deducted from your balance but not received'**
  String get articleWithdrawalFailedContent;

  /// Article: For Sale not visible
  ///
  /// In en, this message translates to:
  /// **'Why is my For Sale not visible?'**
  String get articleForSaleNotVisible;

  /// Article: For Sale not visible content
  ///
  /// In en, this message translates to:
  /// **'Your For Sale may not be visible to buyers because:\n\n1. It is sold â€” the stock has been sold out, so it is no longer offered\n2. It is withdrawn â€” the For Sale has been removed from sale\n3. Its details are incomplete â€” make sure the photos, price, and shipping options are filled in\n\nA new For Sale is shown to buyers as soon as you publish it â€” there is no approval stage to wait for.\n\nNext steps:\nâ€¢ Go to My For Sales and check the status\nâ€¢ Complete any missing details and save again\n\nIf the For Sale is still active but not showing to buyers, contact support.'**
  String get articleForSaleNotVisibleContent;

  /// Article: Seller payment pending
  ///
  /// In en, this message translates to:
  /// **'Payment from order not received?'**
  String get articleSellerPaymentPending;

  /// Article: Seller payment pending content
  ///
  /// In en, this message translates to:
  /// **'Order payments reach the seller in these stages:\n\n1. Payment completed â†’ the funds are held in escrow while the order is running\n2. Order shipped â†’ the funds stay in escrow until the buyer confirms receipt\n3. Order completed â†’ the funds are released to your income and can be withdrawn immediately\n\nCheck these screens:\nâ€¢ Order status in the Seller Dashboard\nâ€¢ Earnings screen for your available balance\n\nOnce the buyer taps \'Confirm Receipt\' (or the order completes automatically), the order amount enters your income and there is no waiting period before you can withdraw it.\n\nIf a completed order is not showing in your earnings, contact support with the order number.'**
  String get articleSellerPaymentPendingContent;

  /// Article: Shipment and delivery issues
  ///
  /// In en, this message translates to:
  /// **'Track shipment and delivery issues'**
  String get articleOrderShipmentHelp;

  /// Article: Shipment and delivery content
  ///
  /// In en, this message translates to:
  /// **'For shipment issues:\n\n1. Open the order details\n2. Check the tracking number in the shipping info\n3. Track the package with the courier\'s website or app\n\nCommon issues:\nâ€¢ Tracking not updating â€” it can take some time before the courier records the first scan\nâ€¢ Delivery delayed â€” contact the seller through the order chat for an update\nâ€¢ Wrong address â€” message the seller immediately\n\nIf the package has not arrived:\nâ€¢ Check the order status and delivery confirmation\nâ€¢ Contact the seller through the order chat\nâ€¢ If the delivery window has passed, open a dispute from the order\n\nStill having issues? Contact support with your order number.'**
  String get articleOrderShipmentHelpContent;

  /// Article: Item not received
  ///
  /// In en, this message translates to:
  /// **'Item paid but not received?'**
  String get articleItemNotReceived;

  /// Article: Item not received content
  ///
  /// In en, this message translates to:
  /// **'If you paid but haven\'t received your item:\n\nStep 1: Check the order status\nâ€¢ Being Prepared: the seller is preparing your order\nâ€¢ In Delivery: check the tracking number on the order\n\nStep 2: Contact the seller\nâ€¢ Use the \'Chat Seller\' button on the order\nâ€¢ Ask for a shipping update or the tracking number\n\nStep 3: Use the protection window\nâ€¢ You have 5 days from the moment the seller ships to confirm receipt or open a dispute\nâ€¢ If you need more time, use \'Extend Confirmation\' once to add 3 days\nâ€¢ If the seller does not ship in time, the order can be cancelled\n\nNext actions:\n1. Chat with the seller first (fastest resolution)\n2. If there is no response, contact support with your order details'**
  String get articleItemNotReceivedContent;

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

  /// Title of the canonical page-level load error state
  ///
  /// In en, this message translates to:
  /// **'Something Went Wrong'**
  String get pageErrorTitle;

  /// Safe user-facing message of the canonical page-level load error state; never contains technical error detail
  ///
  /// In en, this message translates to:
  /// **'We couldn\'t load the data. Please try again.'**
  String get pageErrorMessage;

  /// Retry action label used by the canonical page-level error state
  ///
  /// In en, this message translates to:
  /// **'Try Again'**
  String get retryAction;

  /// Generic collection-empty title: the request succeeded and the collection has zero items
  ///
  /// In en, this message translates to:
  /// **'No Data Yet'**
  String get emptyCollectionTitle;

  /// Generic collection-empty supporting message
  ///
  /// In en, this message translates to:
  /// **'There are no items to show.'**
  String get emptyCollectionMessage;

  /// Search/filter-empty title: the collection exists but the active query or filter matched nothing
  ///
  /// In en, this message translates to:
  /// **'No Results Found'**
  String get emptySearchTitle;

  /// Search/filter-empty supporting message
  ///
  /// In en, this message translates to:
  /// **'Try different keywords or filters.'**
  String get emptySearchMessage;

  /// Primary action that clears the active search query or filter on an empty state
  ///
  /// In en, this message translates to:
  /// **'Reset'**
  String get resetFilterAction;

  /// Reusable first-use action that takes the user to the marketplace
  ///
  /// In en, this message translates to:
  /// **'Explore Marketplace'**
  String get exploreMarketplaceAction;

  /// First-use action that starts creating a For Sale listing
  ///
  /// In en, this message translates to:
  /// **'Create Listing'**
  String get createForSaleAction;

  /// First-use action that starts a new chat
  ///
  /// In en, this message translates to:
  /// **'Start Chat'**
  String get startChatAction;

  /// First-use action that creates a support ticket
  ///
  /// In en, this message translates to:
  /// **'Create Ticket'**
  String get createTicketAction;

  /// First-use action that adds a shipping address
  ///
  /// In en, this message translates to:
  /// **'Add Address'**
  String get addAddressAction;

  /// Collection-empty title for public For Sale surfaces
  ///
  /// In en, this message translates to:
  /// **'No listings yet'**
  String get emptyForSaleTitle;

  /// Collection-empty title for auction surfaces
  ///
  /// In en, this message translates to:
  /// **'No auctions yet'**
  String get emptyAuctionTitle;

  /// Collection-empty message for public marketplace tabs
  ///
  /// In en, this message translates to:
  /// **'Check back later!'**
  String get emptyCheckBackMessage;

  /// Collection-empty message on a seller's public store tab
  ///
  /// In en, this message translates to:
  /// **'This seller has no active listings.'**
  String get emptySellerForSaleMessage;

  /// Collection-empty message on a seller's public auction tab
  ///
  /// In en, this message translates to:
  /// **'This seller has no active auctions.'**
  String get emptySellerAuctionMessage;

  /// Collection-empty title on the seller's own For Sale management screen
  ///
  /// In en, this message translates to:
  /// **'No Listings Yet'**
  String get myForSalesTitle;

  /// First-use message on the seller's own For Sale screen
  ///
  /// In en, this message translates to:
  /// **'Create your first listing to start selling.'**
  String get firstUseForSaleMessage;

  /// Collection-empty title on the saved items screen
  ///
  /// In en, this message translates to:
  /// **'No saved items yet'**
  String get emptySavedTitle;

  /// Collection-empty message on the saved items screen
  ///
  /// In en, this message translates to:
  /// **'For Sale and auction items you save will appear here.'**
  String get emptySavedMessage;

  /// Collection-empty title for the buyer order list
  ///
  /// In en, this message translates to:
  /// **'No Orders Yet'**
  String get emptyOrdersTitle;

  /// Collection-empty message for the buyer order list
  ///
  /// In en, this message translates to:
  /// **'Start shopping from the best Koi collection'**
  String get emptyOrdersMessage;

  /// Status-filter-empty title for the buyer order list. Neutral on purpose: a status tab being empty must not imply the buyer has never ordered (orders may exist under other statuses).
  ///
  /// In en, this message translates to:
  /// **'No Orders'**
  String get emptyOrdersByStatusTitle;

  /// Status-filter-empty message for the buyer order list
  ///
  /// In en, this message translates to:
  /// **'There are no orders with this status yet'**
  String get emptyOrdersByStatusMessage;

  /// Collection-empty title for the seller incoming-orders list
  ///
  /// In en, this message translates to:
  /// **'No Incoming Orders'**
  String get emptyIncomingOrdersTitle;

  /// Collection-empty message for the seller incoming-orders list
  ///
  /// In en, this message translates to:
  /// **'Orders from buyers will appear here'**
  String get emptyIncomingOrdersMessage;

  /// Collection-empty title on the chat list
  ///
  /// In en, this message translates to:
  /// **'No Chats Yet'**
  String get emptyChatsTitle;

  /// Collection-empty message on the chat list
  ///
  /// In en, this message translates to:
  /// **'Contact sellers to ask about products'**
  String get emptyChatsMessage;

  /// Search-empty title on the chat list when the active search matched nothing
  ///
  /// In en, this message translates to:
  /// **'No Chats Found'**
  String get emptyChatSearchTitle;

  /// Collection-empty title on the followers list
  ///
  /// In en, this message translates to:
  /// **'No followers yet'**
  String get emptyFollowersTitle;

  /// Collection-empty title on the following list
  ///
  /// In en, this message translates to:
  /// **'Not following anyone yet'**
  String get emptyFollowingTitle;

  /// Collection-empty title on the my-reports screen
  ///
  /// In en, this message translates to:
  /// **'No reports yet'**
  String get emptyReportsTitle;

  /// Collection-empty message on the my-reports screen
  ///
  /// In en, this message translates to:
  /// **'You haven\'t submitted any reports yet. Use the report action on content to report it.'**
  String get emptyReportsMessage;

  /// Collection-empty title on the support tickets screen
  ///
  /// In en, this message translates to:
  /// **'No support tickets yet'**
  String get emptySupportTicketsTitle;

  /// Collection-empty message on the support tickets screen
  ///
  /// In en, this message translates to:
  /// **'Create a ticket to get help from our support team'**
  String get emptySupportTicketsMessage;

  /// Support ticket category label (category identity: order_issue)
  ///
  /// In en, this message translates to:
  /// **'Order Problems'**
  String get supportCategoryOrderIssue;

  /// Support ticket category label (category identity: payment_issue)
  ///
  /// In en, this message translates to:
  /// **'Payment Issues'**
  String get supportCategoryPaymentIssue;

  /// Support ticket category label (category identity: account_issue)
  ///
  /// In en, this message translates to:
  /// **'Account Help'**
  String get supportCategoryAccountIssue;

  /// Support ticket category label (category identity: listing_issue)
  ///
  /// In en, this message translates to:
  /// **'Listing Problems'**
  String get supportCategoryListingIssue;

  /// Support ticket category label (category identity: shipping_issue)
  ///
  /// In en, this message translates to:
  /// **'Shipping Issues'**
  String get supportCategoryShippingIssue;

  /// Support ticket category label (category identity: refund_request)
  ///
  /// In en, this message translates to:
  /// **'Refund Request'**
  String get supportCategoryRefundRequest;

  /// Support ticket category label (category identity: dispute)
  ///
  /// In en, this message translates to:
  /// **'Dispute'**
  String get supportCategoryDispute;

  /// Support ticket category label (category identity: technical_issue)
  ///
  /// In en, this message translates to:
  /// **'Technical Help'**
  String get supportCategoryTechnicalIssue;

  /// Support ticket category label (category identity: other)
  ///
  /// In en, this message translates to:
  /// **'Other'**
  String get supportCategoryOther;

  /// Support ticket status label (status identity: open)
  ///
  /// In en, this message translates to:
  /// **'Open'**
  String get supportStatusOpen;

  /// Support ticket status label (status identity: in_progress)
  ///
  /// In en, this message translates to:
  /// **'In Progress'**
  String get supportStatusInProgress;

  /// Support ticket status label (status identity: waiting_user)
  ///
  /// In en, this message translates to:
  /// **'Waiting User'**
  String get supportStatusWaitingUser;

  /// Support ticket status label (status identity: resolved)
  ///
  /// In en, this message translates to:
  /// **'Resolved'**
  String get supportStatusResolved;

  /// Support ticket status label (status identity: closed)
  ///
  /// In en, this message translates to:
  /// **'Closed'**
  String get supportStatusClosed;

  /// Support ticket priority label (priority identity: low)
  ///
  /// In en, this message translates to:
  /// **'Low'**
  String get supportPriorityLow;

  /// Support ticket priority label (priority identity: medium)
  ///
  /// In en, this message translates to:
  /// **'Medium'**
  String get supportPriorityMedium;

  /// Support ticket priority label (priority identity: high)
  ///
  /// In en, this message translates to:
  /// **'High'**
  String get supportPriorityHigh;

  /// Support ticket priority label (priority identity: urgent)
  ///
  /// In en, this message translates to:
  /// **'Urgent'**
  String get supportPriorityUrgent;

  /// Support ticket activity label (event identity: ticket_created)
  ///
  /// In en, this message translates to:
  /// **'Ticket Created'**
  String get supportEventTicketCreated;

  /// Support ticket activity label (event identity: ticket_claimed)
  ///
  /// In en, this message translates to:
  /// **'Ticket Claimed'**
  String get supportEventTicketClaimed;

  /// Support ticket activity label (event identity: ticket_waiting_user)
  ///
  /// In en, this message translates to:
  /// **'Waiting for Your Reply'**
  String get supportEventTicketWaitingUser;

  /// Support ticket activity label (event identity: status_changed)
  ///
  /// In en, this message translates to:
  /// **'Status Changed'**
  String get supportEventStatusChanged;

  /// Support ticket activity label (event identity: priority_changed)
  ///
  /// In en, this message translates to:
  /// **'Priority Changed'**
  String get supportEventPriorityChanged;

  /// Support ticket activity label (event identity: category_changed)
  ///
  /// In en, this message translates to:
  /// **'Category Changed'**
  String get supportEventCategoryChanged;

  /// Support ticket activity label (event identity: ticket_resolved)
  ///
  /// In en, this message translates to:
  /// **'Ticket Resolved'**
  String get supportEventTicketResolved;

  /// Support ticket activity label (event identity: ticket_closed)
  ///
  /// In en, this message translates to:
  /// **'Ticket Closed'**
  String get supportEventTicketClosed;

  /// Support ticket activity label (event identity: ticket_reopened)
  ///
  /// In en, this message translates to:
  /// **'Ticket Reopened'**
  String get supportEventTicketReopened;

  /// Support ticket activity label (event identity: admin_assigned)
  ///
  /// In en, this message translates to:
  /// **'Support Agent Assigned'**
  String get supportEventAdminAssigned;

  /// Support ticket activity label (event identity: admin_unassigned)
  ///
  /// In en, this message translates to:
  /// **'Support Agent Unassigned'**
  String get supportEventAdminUnassigned;

  /// Support ticket activity label (event identity: ticket_escalated)
  ///
  /// In en, this message translates to:
  /// **'Escalated to Dispute'**
  String get supportEventTicketEscalated;

  /// Support ticket activity label for an unrecognised event type
  ///
  /// In en, this message translates to:
  /// **'Ticket Activity'**
  String get supportEventUnknown;

  /// Support ticket activity detail: a localized state transition between two canonical labelled values
  ///
  /// In en, this message translates to:
  /// **'From {from} to {to}'**
  String supportEventTransition(String from, String to);

  /// Section title for the read-only Support ticket activity timeline
  ///
  /// In en, this message translates to:
  /// **'Activity Timeline'**
  String get supportTimelineTitle;

  /// Section empty state for the Support ticket activity timeline
  ///
  /// In en, this message translates to:
  /// **'No activity recorded yet'**
  String get supportTimelineEmpty;

  /// Collection-empty title on the address book
  ///
  /// In en, this message translates to:
  /// **'No address yet'**
  String get emptyAddressTitle;

  /// Collection-empty message on the address book
  ///
  /// In en, this message translates to:
  /// **'Add an address to shop and to ship from'**
  String get emptyAddressMessage;

  /// Collection-empty title on another user's profile content tab
  ///
  /// In en, this message translates to:
  /// **'No content yet'**
  String get emptyProfileContentTitle;

  /// Collection-empty message on another user's profile content tab
  ///
  /// In en, this message translates to:
  /// **'This user hasn\'t shared any content'**
  String get emptyProfileContentMessage;

  /// Order action CTA for backend label_key action.mark_shipped
  ///
  /// In en, this message translates to:
  /// **'Ship Order'**
  String get orderActionMarkShipped;

  /// Order action CTA for backend label_key action.confirm_receipt
  ///
  /// In en, this message translates to:
  /// **'Confirm Receipt'**
  String get orderActionConfirmReceipt;

  /// Order action CTA for backend label_key action.provide_evidence
  ///
  /// In en, this message translates to:
  /// **'Provide Evidence'**
  String get orderActionProvideEvidence;

  /// Order action CTA for backend label_key action.cancel_order_overdue
  ///
  /// In en, this message translates to:
  /// **'Cancel Order'**
  String get orderActionCancelOrderOverdue;

  /// Order action CTA for backend label_key action.pay_now
  ///
  /// In en, this message translates to:
  /// **'Pay Now'**
  String get orderActionPayNow;

  /// Order action CTA for backend label_key action.payment_continue
  ///
  /// In en, this message translates to:
  /// **'Continue Payment'**
  String get orderActionPaymentContinue;

  /// Order action CTA for backend label_key action.payment_check_status
  ///
  /// In en, this message translates to:
  /// **'Check Payment Status'**
  String get orderActionPaymentCheckStatus;

  /// Order action CTA for backend label_key action.pay_again
  ///
  /// In en, this message translates to:
  /// **'Retry Payment'**
  String get orderActionPayAgain;

  /// Order action CTA for backend label_key action.cancel_order
  ///
  /// In en, this message translates to:
  /// **'Cancel'**
  String get orderActionCancelOrder;

  /// Order action CTA for backend label_key action.extend_confirmation
  ///
  /// In en, this message translates to:
  /// **'Extend Confirmation'**
  String get orderActionExtendConfirmation;

  /// Order action CTA for backend label_key action.request_refund
  ///
  /// In en, this message translates to:
  /// **'Request Refund'**
  String get orderActionRequestRefund;

  /// Order action CTA for backend label_key action.open_dispute
  ///
  /// In en, this message translates to:
  /// **'Open Dispute'**
  String get orderActionOpenDispute;

  /// Order action CTA for backend label_key action.update_tracking
  ///
  /// In en, this message translates to:
  /// **'Update Tracking'**
  String get orderActionUpdateTracking;

  /// Order action CTA for backend label_key action.chat_seller
  ///
  /// In en, this message translates to:
  /// **'Chat Seller'**
  String get orderActionChatSeller;

  /// Order action CTA for backend label_key action.contact_support
  ///
  /// In en, this message translates to:
  /// **'Contact Support'**
  String get orderActionContactSupport;

  /// First-use empty title on the home feed
  ///
  /// In en, this message translates to:
  /// **'🎯 Kamu ingin apa hari ini?'**
  String get homeFirstUseTitle;

  /// Security screen: Change Password is unavailable because the account has no HiShumi password credential (Google-only)
  ///
  /// In en, this message translates to:
  /// **'Password managed by Google'**
  String get passwordManagedByGoogleTitle;

  /// Security screen: explanation body shown instead of the Change Password form for accounts without a password credential
  ///
  /// In en, this message translates to:
  /// **'You signed in with Google and this account doesn\'t have a HiShumi password. To change your password, manage it in your Google account.'**
  String get passwordManagedByGoogleBody;
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
