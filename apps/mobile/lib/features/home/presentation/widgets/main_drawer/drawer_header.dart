import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/models/seller_identity_data.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Drawer header component
///
/// Shows either:
/// - Logo + auth buttons (when not logged in)
/// - User profile with avatar (when logged in)
class MainDrawerHeader extends ConsumerStatefulWidget {
  final bool isLoggedIn;
  final bool showPlaceholder;
  final VoidCallback onSignIn;
  final VoidCallback onSignUp;

  final SellerIdentityData identity;

  final VoidCallback? onProfile;

  const MainDrawerHeader({
    super.key,
    required this.isLoggedIn,
    required this.showPlaceholder,
    required this.onSignIn,
    required this.onSignUp,
    required this.identity,
    this.onProfile,
  });

  @override
  ConsumerState<MainDrawerHeader> createState() => _MainDrawerHeaderState();
}

class _MainDrawerHeaderState extends ConsumerState<MainDrawerHeader> {
  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final l10n = AppLocalizations.of(context)!;

    return Container(
      width: double.infinity,
      decoration: BoxDecoration(
        gradient: widget.isLoggedIn
            ? LinearGradient(
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
                colors: [
                  scheme.primary,
                  scheme.primary.withValues(alpha: 0.85),
                ],
              )
            : null,
        color: widget.isLoggedIn
            ? null
            : scheme.surfaceContainerHigh,
      ),
      child: SafeArea(
        bottom: false,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(AppMetrics.p16, AppMetrics.p16, AppMetrics.p16, AppMetrics.p16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              // Not logged in: Show logo + brand
              if (widget.showPlaceholder) ...[
                Row(
                  children: [
                    Container(
                      width: 56,
                      height: 56,
                      decoration: BoxDecoration(
                        color: scheme.surfaceContainerHighest,
                        shape: BoxShape.circle,
                      ),
                      child: Icon(
                        Icons.person_outline,
                        color: scheme.onSurfaceVariant,
                        size: AppIconSize.emphasis,
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Container(
                            height: 14,
                            width: double.infinity,
                            decoration: BoxDecoration(
                              color: scheme.surfaceContainerHighest,
                              borderRadius: BorderRadius.circular(AppShape.pill),
                            ),
                          ),
                          const SizedBox(height: 8),
                          Container(
                            height: 10,
                            width: 120,
                            decoration: BoxDecoration(
                              color: scheme.surfaceContainerHighest,
                              borderRadius: BorderRadius.circular(AppShape.pill),
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ] else if (!widget.isLoggedIn) ...[
                Row(
                  children: [
                    const AppLogo(size: 40),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: [
                          Text(
                            'LABUDA',
                            style: context.typeRoles.titleSection.copyWith(
                              color: scheme.onSurface,
                              fontWeight: FontWeight.bold,
                            ),
                          ),
                          const SizedBox(height: 2),
                          Text(
                            l10n.koiCommunity,
                            style: context.typeRoles.labelMicro.copyWith(
                              color: scheme.onSurfaceVariant,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
                // Auth buttons
                const SizedBox(height: 12),
                Row(
                  children: [
                    Expanded(
                      child: OutlinedButton(
                        onPressed: () {
                          Navigator.pop(context);
                          widget.onSignIn();
                        },
                        style: OutlinedButton.styleFrom(
                          padding: const EdgeInsets.symmetric(vertical: AppMetrics.p12),
                        ),
                        child: Text(
                          'Sign In',
                          style: Theme.of(context).textTheme.labelLarge,
                        ),
                      ),
                    ),
                    const SizedBox(width: 10),
                    Expanded(
                      child: ElevatedButton(
                        onPressed: () {
                          Navigator.pop(context);
                          widget.onSignUp();
                        },
                        style: ElevatedButton.styleFrom(
                          padding: const EdgeInsets.symmetric(vertical: AppMetrics.p12),
                        ),
                        child: Text(
                          'Sign Up',
                          style: Theme.of(context).textTheme.labelLarge,
                        ),
                      ),
                    ),
                  ],
                ),
              ],

              // Logged in: Show user profile
              if (widget.isLoggedIn) ...[
                GestureDetector(
                  onTap: () {
                    Navigator.pop(context);
                    widget.onProfile?.call();
                  },
                  behavior: HitTestBehavior.opaque,
                  child: SellerIdentityView(identity: widget.identity),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
