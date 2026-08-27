import 'package:equatable/equatable.dart';
import 'package:labuda/shared/helpers/user_identity_formatter.dart';

/// Canonical seller identity payload used across shared commerce surfaces.
///
/// The raw values stay immutable and unformatted; presentation layers derive
/// the handle via [UserIdentityFormatter.formatHandle] and keep store names
/// untouched.
class SellerIdentityData extends Equatable {
  final String userId;
  final String? username;
  final String? storeName;
  final String? displayHandleLabel;
  final String? publicOriginLine;
  final String? avatarUrl;
  final String? storeImageUrl;
  final String? storeImageReloadToken;
  final bool isSeller;

  const SellerIdentityData({
    required this.userId,
    this.username,
    this.storeName,
    this.displayHandleLabel,
    this.publicOriginLine,
    this.avatarUrl,
    this.storeImageUrl,
    this.storeImageReloadToken,
    this.isSeller = true,
  });

  factory SellerIdentityData.fromJson(
    Map<String, dynamic> json, {
    required String userId,
    bool isSeller = true,
  }) {
    return SellerIdentityData(
      userId: userId,
      username: _readString(json['username']),
      storeName: _readString(json['store_name']),
      displayHandleLabel: _readString(json['display_handle_label']),
      publicOriginLine: _readString(json['public_origin_line']),
      avatarUrl: _readString(json['avatar_url']),
      storeImageUrl: _readString(json['store_image_url']),
      storeImageReloadToken: _readString(json['store_image_reload_token']),
      isSeller: isSeller,
    );
  }

  String? get normalizedUsername =>
      UserIdentityFormatter.normalizeUsername(username);

  String? get handle => UserIdentityFormatter.formatHandle(username);

  String? get displayHandle =>
      _clean(displayHandleLabel) ?? handle;

  String? get normalizedStoreName => _clean(storeName);

  String? get normalizedAvatarUrl => _clean(avatarUrl);

  String? get normalizedStoreImageUrl => _clean(storeImageUrl);

  bool get hasStoreName => normalizedStoreName != null;

  bool get hasIdentity => handle != null || hasStoreName;

  SellerIdentityData copyWith({
    String? userId,
    String? username,
    String? storeName,
    String? displayHandleLabel,
    String? publicOriginLine,
    String? avatarUrl,
    String? storeImageUrl,
    String? storeImageReloadToken,
    bool? isSeller,
  }) {
    return SellerIdentityData(
      userId: userId ?? this.userId,
      username: username ?? this.username,
      storeName: storeName ?? this.storeName,
      displayHandleLabel: displayHandleLabel ?? this.displayHandleLabel,
      publicOriginLine: publicOriginLine ?? this.publicOriginLine,
      avatarUrl: avatarUrl ?? this.avatarUrl,
      storeImageUrl: storeImageUrl ?? this.storeImageUrl,
      storeImageReloadToken:
          storeImageReloadToken ?? this.storeImageReloadToken,
      isSeller: isSeller ?? this.isSeller,
    );
  }

  @override
  List<Object?> get props => [
    userId,
    username,
    storeName,
    displayHandleLabel,
    publicOriginLine,
    avatarUrl,
    storeImageUrl,
    storeImageReloadToken,
    isSeller,
  ];

  static String? _readString(dynamic value) {
    if (value is String) {
      final trimmed = value.trim();
      return trimmed.isEmpty ? null : trimmed;
    }
    return null;
  }

  static String? _clean(String? value) {
    final trimmed = value?.trim();
    if (trimmed == null || trimmed.isEmpty) return null;
    return trimmed;
  }
}
