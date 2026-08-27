import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/models/seller_identity_data.dart';
import 'package:labuda/shared/widgets/seller_identity_view.dart';
import 'package:labuda/generated/app_localizations.dart';

/// Drawer header component
///
/// Shows either:
/// - Logo + auth buttons (when not logged in)
/// - User profile with avatar (when logged in)
class MainDrawerHeader extends ConsumerStatefulWidget {
  final bool isDark;
  final bool isLoggedIn;
  final bool showPlaceholder;
  final VoidCallback onSignIn;
  final VoidCallback onSignUp;

  final SellerIdentityData identity;

  final VoidCallback? onProfile;

  const MainDrawerHeader({
    super.key,
    required this.isDark,
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
    final l10n = AppLocalizations.of(context)!;

    return Container(
      width: double.infinity,
      decoration: BoxDecoration(
        gradient: widget.isLoggedIn
            ? LinearGradient(
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
                colors: [
                  AppColors.primaryRed,
                  AppColors.primaryRed.withValues(alpha: 0.85),
                ],
              )
            : null,
        color: widget.isLoggedIn
            ? null
            : (widget.isDark ? AppColors.darkGray700 : AppColors.neutralGray50),
      ),
      child: SafeArea(
        bottom: false,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(16, 16, 16, 16),
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
                        color: AppColors.neutralWhite.withValues(alpha: 0.14),
                        shape: BoxShape.circle,
                      ),
                      child: const Icon(
                        Icons.person_outline,
                        color: AppColors.neutralWhite,
                        size: 28,
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
                              color: AppColors.neutralWhite.withValues(
                                alpha: 0.22,
                              ),
                              borderRadius: BorderRadius.circular(999),
                            ),
                          ),
                          const SizedBox(height: 8),
                          Container(
                            height: 10,
                            width: 120,
                            decoration: BoxDecoration(
                              color: AppColors.neutralWhite.withValues(
                                alpha: 0.16,
                              ),
                              borderRadius: BorderRadius.circular(999),
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
                            style: TextStyle(
                              color: widget.isDark
                                  ? AppColors.neutralWhite
                                  : AppColors.neutralGray900,
                              fontSize: 18,
                              fontWeight: FontWeight.bold,
                            ),
                          ),
                          const SizedBox(height: 2),
                          Text(
                            l10n.koiCommunity,
                            style: TextStyle(
                              color: widget.isDark
                                  ? AppColors.neutralGray400
                                  : AppColors.neutralGray600,
                              fontSize: 12,
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
                          foregroundColor: AppColors.primaryRed,
                          side: const BorderSide(color: AppColors.primaryRed),
                          padding: const EdgeInsets.symmetric(vertical: 10),
                          shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(8),
                          ),
                        ),
                        child: const Text(
                          'Sign In',
                          style: TextStyle(fontSize: 13),
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
                          backgroundColor: AppColors.primaryRed,
                          foregroundColor: AppColors.neutralWhite,
                          padding: const EdgeInsets.symmetric(vertical: 10),
                          shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(8),
                          ),
                        ),
                        child: const Text(
                          'Sign Up',
                          style: TextStyle(fontSize: 13),
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
                  child: SellerIdentityView(
                    identity: widget.identity,
                    variant: SellerIdentityViewVariant.profile,
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
