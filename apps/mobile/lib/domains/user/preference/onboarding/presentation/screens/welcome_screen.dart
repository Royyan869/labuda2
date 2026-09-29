import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

/// Welcome screen dengan professional branding dan smooth animations.
///
/// Features:
/// - Professional LABUDA branding
/// - Adaptive theme support (dark/light)
/// - Smooth page transitions
/// - Elegant call-to-action buttons
class WelcomeScreen extends ConsumerStatefulWidget {
  const WelcomeScreen({super.key});

  @override
  ConsumerState<WelcomeScreen> createState() => _WelcomeScreenState();
}

class _WelcomeScreenState extends ConsumerState<WelcomeScreen>
    with TickerProviderStateMixin {
  late AnimationController _fadeController;
  late AnimationController _slideController;
  late Animation<double> _fadeAnimation;
  late Animation<Offset> _slideAnimation;
  DateTime? _lastBackPressed;

  @override
  void initState() {
    super.initState();

    // Setup animations
    _fadeController = AnimationController(
      duration: AppMotion.longest,
      vsync: this,
    );

    _slideController = AnimationController(
      duration: AppMotion.ambient,
      vsync: this,
    );

    _fadeAnimation = Tween<double>(begin: 0.0, end: 1.0).animate(
      CurvedAnimation(parent: _fadeController, curve: Curves.easeInOut),
    );

    _slideAnimation =
        Tween<Offset>(begin: const Offset(0, 0.3), end: Offset.zero).animate(
          CurvedAnimation(parent: _slideController, curve: Curves.easeOutCubic),
        );

    // Start animations
    _fadeController.forward();
    Future.delayed(AppMotion.settled, () {
      _slideController.forward();
    });
  }

  @override
  void dispose() {
    _fadeController.dispose();
    _slideController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) {
        if (didPop) return;

        final now = DateTime.now();
        final backButtonHasNotBeenPressedOrSnackBarHasBeenClosed =
            _lastBackPressed == null ||
            now.difference(_lastBackPressed!) > const Duration(seconds: 2);

        if (backButtonHasNotBeenPressedOrSnackBarHasBeenClosed) {
          _lastBackPressed = now;
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(
              content: Text('Press back again to exit'),
              duration: Duration(seconds: 2),
            ),
          );
        } else {
          // Exit app properly
          SystemNavigator.pop();
        }
      },
      child: Scaffold(
        backgroundColor: scheme.surfaceContainerLowest,
        body: Container(
          decoration: BoxDecoration(
            gradient: LinearGradient(
              begin: Alignment.topCenter,
              end: Alignment.bottomCenter,
              colors: [scheme.surfaceContainerLowest, scheme.surface],
            ),
          ),
          child: SafeArea(
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p24),
              child: Column(
                children: [
                  // Top action buttons - home icon kiri, theme toggle kanan
                  Padding(
                    padding: const EdgeInsets.only(top: AppMetrics.p8),
                    child: Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [_buildHomeIcon(), _buildThemeToggle()],
                    ),
                  ),

                  // Main content - scrollable
                  Expanded(
                    child: SingleChildScrollView(
                      child: Column(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: [
                          const SizedBox(height: 24),

                          // Logo and branding
                          FadeTransition(
                            opacity: _fadeAnimation,
                            child: SlideTransition(
                              position: _slideAnimation,
                              child: _buildLogo(),
                            ),
                          ),

                          const SizedBox(height: 32),

                          // Title and subtitle
                          FadeTransition(
                            opacity: _fadeAnimation,
                            child: SlideTransition(
                              position: _slideAnimation,
                              child: _buildTitleSection(),
                            ),
                          ),

                          const SizedBox(height: 40),

                          // Action buttons
                          FadeTransition(
                            opacity: _fadeAnimation,
                            child: SlideTransition(
                              position: _slideAnimation,
                              child: _buildActionButtons(),
                            ),
                          ),

                          const SizedBox(height: 32),

                          // Footer
                          FadeTransition(
                            opacity: _fadeAnimation,
                            child: _buildFooter(),
                          ),

                          const SizedBox(height: 16),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildHomeIcon() {
    return IconButton(
      onPressed: _navigateToHome,
      icon: Icon(
        Icons.home_outlined,
        color: Theme.of(context).colorScheme.onSurfaceVariant,
      ),
      tooltip: 'Home',
    );
  }

  /// Theme toggle delegates to the canonical shared picker sheet
  /// ([showThemeSelectionSheet]) — the local copy is purged.
  Widget _buildThemeToggle() {
    return Consumer(
      builder: (context, ref, child) {
        final themeState = ref.watch(themeControllerProvider);
        final scheme = Theme.of(context).colorScheme;

        return IconButton(
          onPressed: () => showThemeSelectionSheet(context, ref),
          icon: Icon(
            themeState.themeMode.icon,
            color: scheme.onSurfaceVariant,
          ),
          tooltip: 'Change Theme',
        );
      },
    );
  }

  // _showThemeBottomSheet: PURGED — canonical shared showThemeSelectionSheet.

  Widget _buildLogo() {
    final scheme = Theme.of(context).colorScheme;
    return Column(
      children: [
        // LABUDA app logo
        Container(
          width: 120,
          height: 120,
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(AppShape.r24),
            boxShadow: [
              BoxShadow(
                color: scheme.shadow.withValues(alpha: 0.25),
                blurRadius: 16,
                offset: const Offset(0, 6),
              ),
            ],
          ),
          child: ClipRRect(
            borderRadius: BorderRadius.circular(AppShape.r24),
            child: Image.asset(
              'assets/images/app_logo.png',
              width: 120,
              height: 120,
              fit: BoxFit.cover,
            ),
          ),
        ),

        const SizedBox(height: 24),

        // LABUDA text logo
        Text(
          'LABUDA',
          style: Theme.of(context).textTheme.headlineLarge?.copyWith(
            fontWeight: FontWeight.bold,
            letterSpacing: 2.0,
            color: Theme.of(context).colorScheme.onSurface,
          ),
        ),
      ],
    );
  }

  Widget _buildTitleSection() {
    return Column(
      children: [
        Text(
          'Indonesian Koi\nCommunity',
          textAlign: TextAlign.center,
          style: Theme.of(context).textTheme.headlineMedium?.copyWith(
            fontWeight: FontWeight.w600,
            height: 1.2,
            color: Theme.of(context).colorScheme.onSurface,
          ),
        ),

        const SizedBox(height: 16),

        Text(
          'Social commerce platform for\nkoi enthusiasts in Indonesia',
          textAlign: TextAlign.center,
          style: Theme.of(context).textTheme.bodyLarge?.copyWith(
            color: Theme.of(context).colorScheme.onSurfaceVariant,
            height: 1.5,
          ),
        ),
      ],
    );
  }

  Widget _buildActionButtons() {
    return Column(
      children: [
        // Sign Up button
        SizedBox(
          width: double.infinity,
          height: 52,
          child: OutlinedButton(
            onPressed: _navigateToSignUp,
            child: const Text(
              'Join Now',
              style: TextStyle(fontSize: AppType.s16, fontWeight: FontWeight.w600),
            ),
          ),
        ),

        const SizedBox(height: 16),

        // Sign In button
        SizedBox(
          width: double.infinity,
          height: 52,
          child: OutlinedButton(
            onPressed: _navigateToSignIn,
            style: OutlinedButton.styleFrom(
              side: BorderSide(
                color: Theme.of(context).colorScheme.outline,
                width: 1.5,
              ),
            ),
            child: const Text(
              'Already have an account? Sign In',
              style: TextStyle(fontSize: AppType.s16, fontWeight: FontWeight.w500),
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildFooter() {
    return Text(
      'From koi lovers, for koi lovers',
      style: Theme.of(context).textTheme.bodySmall?.copyWith(
        color: Theme.of(context).colorScheme.onSurfaceVariant,
        fontStyle: FontStyle.italic,
      ),
    );
  }

  void _navigateToHome() {
    ref.navigation.navigateToHome();
  }

  void _navigateToSignUp() {
    ref.navigation.navigateToSignUp();
  }

  void _navigateToSignIn() {
    ref.navigation.navigateToSignIn();
  }
}
