import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_primitives.dart';
import 'package:labuda/domains/user/profile/profile.dart'
    show userDataProvider, profileStreamProvider;
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/governance/seller_tier_badge.dart';
import 'package:labuda/shared/models/seller_identity_data.dart';
import 'package:labuda/shared/shared.dart';

/// Canonical DETAIL seller card — ONE AUTHORITY for both sale channels.
///
/// ForSale and Auction detail surfaces render their seller block through this
/// widget, so identity order, redaction vocabulary, tier gating, avatar
/// treatment, tap behaviour and the section frame cannot drift between
/// channels. Channel screens pass their entity's facts and nothing else.
///
/// Owner truth:
///   - farm/store name = public seller identity, @username = public handle,
///     fullName is private/KYC and is NEVER read here.
///   - identity source priority: entity owner-truth scalars first (the detail
///     wire always carries them); `userDataProvider` is consulted ONLY when
///     the entity has no handle — never `user.fullName`.
///   - a buyer must be able to see WHERE the goods ship from: the listing's
///     buyer-facing origin (city, province of the sender address) is emitted
///     by the backend detail projection and rendered on this card. The card
///     never derives an origin from an address id, and never renders a street
///     address.
///   - missing truth is HIDDEN, never fabricated.
///
/// Axis boundary:
///   - user-identity axis degraded → full redaction label, neutral avatar,
///     tap disabled, chevron suppressed, tier badge suppressed.
///   - seller-trust axis degraded → identity stays, [SellerInactiveBadge]
///     renders, tier badge suppressed.
class CommerceDetailSellerCard extends ConsumerWidget {
  final String sellerId;
  final String? username;
  final String? storeName;
  final String? avatarUrl;

  /// Buyer-facing listing origin ("City, Province") from the detail wire.
  /// Detail payloads only; null hides the line.
  final String? originLine;
  final ContentLifecycle sellerUserLifecycle;
  final ContentLifecycle sellerTrustLifecycle;
  final String? tier;

  const CommerceDetailSellerCard({
    super.key,
    required this.sellerId,
    required this.username,
    required this.storeName,
    required this.avatarUrl,
    required this.sellerUserLifecycle,
    required this.sellerTrustLifecycle,
    this.originLine,
    this.tier,
  });

  bool get _degraded => sellerUserLifecycle.isDegraded;

  bool get _tierBadgeVisible =>
      !_degraded &&
      sellerTrustLifecycle == ContentLifecycle.active &&
      tier != null;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // E8.2 — user-axis gate fires first: the seller identity is redacted while
    // the listing itself stays visible (listing state is `status`, not the
    // seller's user lifecycle).
    if (_degraded) {
      return _frame(
        child: _row(
          context,
          avatar: ProfileAvatar(userId: '', size: 48),
          displayName: sellerUserLifecycle.publicRedactionLabel,
          usernameLine: null,
          italic: true,
          onTap: null,
        ),
      );
    }

    // ENTITY TRUTH FIRST: the canonical render never depends on a secondary
    // lookup. The user lookup below only covers a legacy payload without a
    // handle — its absence HIDES the block instead of fabricating identity.
    final entityUsername = _notBlank(username);
    if (entityUsername != null) {
      return _renderIdentity(
        context,
        ref,
        resolvedUsername: entityUsername,
        resolvedAvatar: _notBlank(avatarUrl),
      );
    }

    return ref
        .watch(userDataProvider(sellerId))
        .when(
          data: (user) => _renderIdentity(
            context,
            ref,
            resolvedUsername: _notBlank(user?.username),
            resolvedAvatar: _notBlank(avatarUrl) ?? _notBlank(user?.avatarUrl),
          ),
          // Non-identity loading hint — asserts no seller identity.
          loading: () => _frame(
            child: _row(
              context,
              avatar: ProfileAvatar(userId: sellerId, size: 48),
              displayName: 'Memuat...',
              usernameLine: null,
              italic: false,
              onTap: null,
            ),
          ),
          error: (_, _) => const SizedBox.shrink(),
        );
  }

  Widget _renderIdentity(
    BuildContext context,
    WidgetRef ref, {
    required String? resolvedUsername,
    required String? resolvedAvatar,
  }) {
    // Store truth for the dual avatar comes from the seller's profile stream
    // (canonical FarmInfo), never fabricated. `.asData` keeps the render safe
    // while the lookup is loading or failed.
    final farmInfo = ref
        .watch(profileStreamProvider(sellerId))
        .asData
        ?.value
        ?.farmInfo;

    // ONE identity authority: the card renders the canonical pairing — store
    // name primary, handle secondary — straight from the identity model.
    final identity = SellerIdentityData(
      userId: sellerId,
      username: resolvedUsername,
      storeName: storeName,
      avatarUrl: resolvedAvatar,
      storeImageUrl: farmInfo?.farmPhotoUrl,
      isSeller: true,
    );

    // Hide rather than fabricate when no truth is available.
    final displayName = identity.primaryLabel;
    if (displayName == null) return const SizedBox.shrink();

    return _frame(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          _row(
            context,
            avatar: SellerDualAvatar(
              identity: identity,
              size: 48,
              onTap: () => ref
                  .read(navigationHandlerProvider)
                  .navigateToUserProfile(sellerId),
            ),
            displayName: displayName,
            usernameLine: identity.secondaryLabel,
            originLine: _notBlank(originLine),
            italic: false,
            onTap: () => ref
                .read(navigationHandlerProvider)
                .navigateToUserProfile(sellerId),
          ),
          if (_tierBadgeVisible) ...[
            const SizedBox(height: 8),
            Padding(
              padding: const EdgeInsets.only(left: AppMetrics.p48),
              child: SellerTierBadge(tier: tier),
            ),
          ],
        ],
      ),
    );
  }

  Widget _frame({required Widget child}) {
    return CommerceDetailSectionCard(
      margin: const EdgeInsets.fromLTRB(
        AppMetrics.p16,
        AppMetrics.p0,
        AppMetrics.p16,
        AppMetrics.p16,
      ),
      child: child,
    );
  }

  Widget _row(
    BuildContext context, {
    required Widget avatar,
    required String displayName,
    required String? usernameLine,
    required bool italic,
    required VoidCallback? onTap,
    String? originLine,
  }) {
    final scheme = Theme.of(context).colorScheme;
    final content = Row(
      children: [
        avatar,
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                displayName,
                style: TextStyle(
                  fontSize: AppType.s16,
                  fontWeight: FontWeight.bold,
                  fontStyle: italic ? FontStyle.italic : FontStyle.normal,
                  color: italic ? scheme.onSurfaceVariant : scheme.onSurface,
                ),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
              if (usernameLine != null)
                Text(
                  usernameLine,
                  style: TextStyle(
                    fontSize: AppType.s12,
                    color: scheme.onSurfaceVariant,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              // Buyer-facing shipping origin — the listing's sender city and
              // province. Hidden when the wire carries no truth.
              if (originLine != null)
                Padding(
                  padding: const EdgeInsets.only(top: AppMetrics.p4),
                  child: Row(
                    children: [
                      Icon(
                        Icons.location_on_outlined,
                        size: AppType.s12,
                        color: scheme.onSurfaceVariant,
                      ),
                      const SizedBox(width: 4),
                      Expanded(
                        child: Text(
                          originLine,
                          style: TextStyle(
                            fontSize: AppType.s12,
                            color: scheme.onSurfaceVariant,
                          ),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                    ],
                  ),
                ),
            ],
          ),
        ),
        // Degraded / loading rows are structurally non-interactive: no
        // chevron, no InkWell — the tap-gate is the absence of an affordance.
        if (onTap != null)
          Icon(Icons.chevron_right, color: scheme.onSurfaceVariant),
      ],
    );

    if (onTap == null) return content;
    return InkWell(onTap: onTap, child: content);
  }

  String? _notBlank(String? value) {
    final trimmed = value?.trim();
    return trimmed == null || trimmed.isEmpty ? null : trimmed;
  }
}
