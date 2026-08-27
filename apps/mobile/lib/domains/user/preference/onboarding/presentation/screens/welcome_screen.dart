import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';

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
      duration: const Duration(milliseconds: 1500),
      vsync: this,
    );

    _slideController = AnimationController(
      duration: const Duration(milliseconds: 1200),
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
    Future.delayed(const Duration(milliseconds: 300), () {
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
    final isDark = Theme.of(context).brightness == Brightness.dark;

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
        body: Container(
          decoration: BoxDecoration(
            gradient: isDark
                ? const LinearGradient(
                    begin: Alignment.topCenter,
                    end: Alignment.bottomCenter,
                    colors: [AppColors.darkGray900, AppColors.darkGray800],
                  )
                : const LinearGradient(
                    begin: Alignment.topCenter,
                    end: Alignment.bottomCenter,
                    colors: [AppColors.neutralGray50, AppColors.neutralWhite],
                  ),
          ),
          child: SafeArea(
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 24.0),
              child: Column(
                children: [
                  // Top action buttons - home icon kiri, theme toggle kanan
                  Padding(
                    padding: const EdgeInsets.only(top: 8.0),
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
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final iconColor = isDark
        ? AppColors.neutralGray300
        : AppColors.neutralGray600;

    return IconButton(
      onPressed: () {
        // Use context.go with for-sale route for guest browsing
        // ${RoutePaths.forSales} is a public route that allows unauthenticated access
        context.go(RoutePaths.forSales);
      },
      icon: Icon(Icons.home_outlined, color: iconColor),
      tooltip: 'Explore as Guest',
    );
  }

  Widget _buildThemeToggle() {
    return Consumer(
      builder: (context, ref, child) {
        final themeState = ref.watch(themeControllerProvider);
        final isDark = themeState.isDarkMode(context);
        final iconColor = isDark
            ? AppColors.neutralGray300
            : AppColors.neutralGray600;

        return IconButton(
          onPressed: () => _showThemeBottomSheet(context, ref),
          icon: Icon(
            _getThemeIcon(themeState.themeMode, isDark),
            color: iconColor,
          ),
          tooltip: 'Change Theme',
        );
      },
    );
  }

  IconData _getThemeIcon(ThemeMode themeMode, bool isDark) {
    switch (themeMode) {
      case ThemeMode.light:
        return Icons.light_mode;
      case ThemeMode.dark:
        return Icons.dark_mode;
      case ThemeMode.system:
        return Icons.brightness_auto;
    }
  }

  String _getThemeDisplayName(ThemeMode themeMode) {
    switch (themeMode) {
      case ThemeMode.light:
        return 'Light';
      case ThemeMode.dark:
        return 'Dark';
      case ThemeMode.system:
        return 'System';
    }
  }

  void _showThemeBottomSheet(BuildContext context, WidgetRef ref) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final currentTheme = ref.read(themeControllerProvider).themeMode;

    showModalBottomSheet<void>(
      context: context,
      backgroundColor: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (BuildContext context) {
        return Container(
          padding: const EdgeInsets.all(20),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Handle bar
              Center(
                child: Container(
                  width: 40,
                  height: 4,
                  decoration: BoxDecoration(
                    color: isDark
                        ? AppColors.neutralGray600
                        : AppColors.neutralGray300,
                    borderRadius: BorderRadius.circular(2),
                  ),
                ),
              ),
              const SizedBox(height: 20),

              // Title
              Text(
                'Theme',
                style: TextStyle(
                  color: isDark
                      ? AppColors.neutralGray200
                      : AppColors.neutralGray900,
                  fontSize: 20,
                  fontWeight: FontWeight.bold,
                ),
              ),
              const SizedBox(height: 20),

              // Theme options
              ...ThemeMode.values.map((themeMode) {
                final isSelected = themeMode == currentTheme;
                return ListTile(
                  contentPadding: EdgeInsets.zero,
                  leading: Icon(
                    _getThemeIcon(themeMode, isDark),
                    color: isSelected
                        ? AppColors.primaryRed
                        : (isDark
                              ? AppColors.neutralGray400
                              : AppColors.neutralGray600),
                    size: 24,
                  ),
                  title: Text(
                    _getThemeDisplayName(themeMode),
                    style: TextStyle(
                      color: isDark
                          ? AppColors.neutralGray200
                          : AppColors.neutralGray900,
                      fontSize: 16,
                      fontWeight: isSelected
                          ? FontWeight.w600
                          : FontWeight.w500,
                    ),
                  ),
                  subtitle: themeMode == ThemeMode.system
                      ? Text(
                          'Follow system setting',
                          style: TextStyle(
                            color: isDark
                                ? AppColors.neutralGray500
                                : AppColors.neutralGray500,
                            fontSize: 12,
                          ),
                        )
                      : null,
                  trailing: isSelected
                      ? Icon(
                          Icons.check_circle,
                          color: AppColors.primaryRed,
                          size: 20,
                        )
                      : null,
                  onTap: () {
                    if (themeMode != currentTheme) {
                      ref
                          .read(themeControllerProvider.notifier)
                          .setThemeMode(themeMode);
                    }
                    Navigator.of(context).pop();
                  },
                );
              }),

              const SizedBox(height: 20),
            ],
          ),
        );
      },
    );
  }

  Widget _buildLogo() {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Column(
      children: [
        // LABUDA app logo
        Container(
          width: 120,
          height: 120,
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(24),
            boxShadow: [
              BoxShadow(
                color: isDark
                    ? Colors.black.withValues(alpha: 0.3)
                    : Colors.grey.withValues(alpha: 0.2),
                blurRadius: 16,
                offset: const Offset(0, 6),
              ),
            ],
          ),
          child: ClipRRect(
            borderRadius: BorderRadius.circular(24),
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
            color: Theme.of(context).brightness == Brightness.dark
                ? AppColors.neutralWhite
                : AppColors.neutralGray900,
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
            color: Theme.of(context).brightness == Brightness.dark
                ? AppColors.neutralGray100
                : AppColors.neutralGray800,
          ),
        ),

        const SizedBox(height: 16),

        Text(
          'Social commerce platform for\nkoi enthusiasts in Indonesia',
          textAlign: TextAlign.center,
          style: Theme.of(context).textTheme.bodyLarge?.copyWith(
            color: Theme.of(context).brightness == Brightness.dark
                ? AppColors.neutralGray400
                : AppColors.neutralGray600,
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
            style: OutlinedButton.styleFrom(
              foregroundColor: AppColors.primaryRed,
              side: const BorderSide(color: AppColors.primaryRed, width: 2),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(12),
              ),
            ),
            child: const Text(
              'Join Now',
              style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
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
              foregroundColor: Theme.of(context).brightness == Brightness.dark
                  ? AppColors.neutralWhite
                  : AppColors.neutralGray800,
              side: BorderSide(
                color: Theme.of(context).brightness == Brightness.dark
                    ? AppColors.darkGray600
                    : AppColors.neutralGray300,
                width: 1.5,
              ),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(12),
              ),
            ),
            child: const Text(
              'Already have an account? Sign In',
              style: TextStyle(fontSize: 16, fontWeight: FontWeight.w500),
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
        color: Theme.of(context).brightness == Brightness.dark
            ? AppColors.neutralGray500
            : AppColors.neutralGray500,
        fontStyle: FontStyle.italic,
      ),
    );
  }

  void _navigateToSignUp() {
    ref.navigation.navigateToSignUp();
  }

  void _navigateToSignIn() {
    ref.navigation.navigateToSignIn();
  }
}
