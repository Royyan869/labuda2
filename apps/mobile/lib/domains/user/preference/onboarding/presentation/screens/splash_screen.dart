import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';

/// Splash screen - Visual Only (No Auth Logic)
///
/// Features:
/// - Elegant LABUDA branding
/// - Smooth loading animation
/// - Theme adaptive
///
/// **IMPORTANT:**
/// - NO polling - AuthController handles auth state
/// - NO manual navigation - Router redirect handles routing
/// - NO Firebase checks - AuthController authStateChanges() handles this
///
/// Router redirect logic based on AuthState:
/// - AuthStateInitial → /splash
/// - AuthStateAuthenticated → /home
/// - AuthStateUnauthenticated → /welcome
class SplashScreen extends ConsumerStatefulWidget {
  const SplashScreen({super.key});

  @override
  ConsumerState<SplashScreen> createState() => _SplashScreenState();
}

class _SplashScreenState extends ConsumerState<SplashScreen>
    with TickerProviderStateMixin {
  late AnimationController _logoController;
  late AnimationController _textController;
  late AnimationController _buttonController;

  late Animation<double> _logoFadeAnimation;
  late Animation<double> _logoScaleAnimation;
  late Animation<double> _textFadeAnimation;
  late Animation<double> _buttonFadeAnimation;

  @override
  void initState() {
    super.initState();

    _setupAnimations();
    _startAnimationSequence();
  }

  void _setupAnimations() {
    // Logo animations
    _logoController = AnimationController(
      duration: const Duration(milliseconds: 1500),
      vsync: this,
    );

    _logoFadeAnimation = Tween<double>(begin: 0.0, end: 1.0).animate(
      CurvedAnimation(
        parent: _logoController,
        curve: const Interval(0.0, 0.6, curve: Curves.easeOut),
      ),
    );

    _logoScaleAnimation = Tween<double>(begin: 0.5, end: 1.0).animate(
      CurvedAnimation(
        parent: _logoController,
        curve: const Interval(0.0, 0.8, curve: Curves.elasticOut),
      ),
    );

    // Text animation
    _textController = AnimationController(
      duration: const Duration(milliseconds: 800),
      vsync: this,
    );

    _textFadeAnimation = Tween<double>(begin: 0.0, end: 1.0).animate(
      CurvedAnimation(parent: _textController, curve: Curves.easeInOut),
    );

    // Loading animation
    _buttonController = AnimationController(
      duration: const Duration(milliseconds: 800),
      vsync: this,
    );

    _buttonFadeAnimation = Tween<double>(begin: 0.0, end: 1.0).animate(
      CurvedAnimation(parent: _buttonController, curve: Curves.easeInOut),
    );
  }

  void _startAnimationSequence() async {
    // Start logo animation
    _logoController.forward();

    // Start text animation after logo
    await Future.delayed(const Duration(milliseconds: 800));
    if (mounted) {
      _textController.forward();
    }

    // Start loading animation
    await Future.delayed(const Duration(milliseconds: 400));
    if (mounted) {
      _buttonController.forward();
    }

    // Navigation is handled by Router redirect based on AuthState
    // - AuthController emits state changes via authStateChanges()
    // - goRouterProvider watches authControllerProvider
    // - Router redirects automatically to /home or /welcome
  }

  @override
  void dispose() {
    _logoController.dispose();
    _textController.dispose();
    _buttonController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authControllerProvider);
    final authController = ref.watch(authControllerProvider.notifier);
    // STAGE 3B: retry-budget signal — true while the canonical automatic
    // backend-sync retry is still scheduled/in flight after a transient
    // backend-unavailable failure.
    final isRetryPending = authController.isBackendRetryPending;

    final isDark = Theme.of(context).brightness == Brightness.dark;

    // Canonical UX: BackendUnavailable while retry budget remains shows
    // an explicit "retrying" state (not generic "Memuat aplikasi..."),
    // so the user understands the app is actively reconnecting.
    // Terminal degraded is only shown once retry budget is exhausted.
    // BackendFailure (4xx) is immediate degraded (no retry).
    if (authState is AuthStateBackendUnavailable) {
      if (isRetryPending) {
        return _buildRetryingScaffold(context, isDark);
      }
      return _buildDegradedScaffold(
        context,
        isDark,
        authState,
        isRetryPending: isRetryPending,
      );
    }
    if (authState is AuthStateBackendFailure) {
      return _buildDegradedScaffold(
        context,
        isDark,
        authState,
        isRetryPending: isRetryPending,
      );
    }

    return Scaffold(
      body: Container(
        decoration: BoxDecoration(
          gradient: isDark
              ? const LinearGradient(
                  begin: Alignment.topLeft,
                  end: Alignment.bottomRight,
                  colors: [
                    AppColors.darkGray900,
                    AppColors.darkGray800,
                    AppColors.darkGray900,
                  ],
                  stops: [0.0, 0.5, 1.0],
                )
              : const LinearGradient(
                  begin: Alignment.topLeft,
                  end: Alignment.bottomRight,
                  colors: [
                    AppColors.neutralWhite,
                    AppColors.neutralGray50,
                    AppColors.neutralWhite,
                  ],
                  stops: [0.0, 0.5, 1.0],
                ),
        ),
        child: Center(
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              // Logo dengan animations
              AnimatedBuilder(
                animation: _logoController,
                builder: (context, child) {
                  return Transform.scale(
                    scale: _logoScaleAnimation.value,
                    child: Opacity(
                      opacity: _logoFadeAnimation.value,
                      child: _buildLogo(),
                    ),
                  );
                },
              ),

              const SizedBox(height: 32),

              // LABUDA text dengan fade in
              FadeTransition(
                opacity: _textFadeAnimation,
                child: _buildBrandText(),
              ),

              const SizedBox(height: 48),

              // Loading indicator
              FadeTransition(
                opacity: _buttonFadeAnimation,
                child: _buildLoadingIndicator(),
              ),
            ],
          ),
        ),
      ),
    );
  }

  /// PASS 2B: dedicated UI for AuthStateBackendUnavailable /
  /// AuthStateBackendFailure — replaces the ordinary spinner + the old
  /// silent bounce to /welcome. Explains that the SERVER (not the
  /// device's internet connection) cannot be reached, offers a manual
  /// retry that calls the existing AuthController.retryBackendSync() path,
  /// and a logout escape hatch. Safe to call signOut() here: this state is
  /// reached only during the pre-authenticated sync flow, so
  /// AuthController.signOut()'s AuthStateAuthenticated-only branches
  /// (backend logout call, FCM cleanup) are simply skipped.
  Widget _buildDegradedScaffold(
    BuildContext context,
    bool isDark,
    AuthState authState, {
    required bool isRetryPending,
  }) {
    final isUnavailable = authState is AuthStateBackendUnavailable;
    final message = isUnavailable
        ? 'Tidak bisa terhubung ke server Labuda. Pastikan backend sedang '
              'berjalan dan HP berada di jaringan yang sama.'
        : (authState as AuthStateBackendFailure).message;

    return Scaffold(
      backgroundColor: isDark ? AppColors.darkGray900 : AppColors.neutralWhite,
      body: SafeArea(
        child: Center(
          child: Padding(
            padding: const EdgeInsets.all(32),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Container(
                  width: 64,
                  height: 64,
                  decoration: BoxDecoration(
                    color: AppColors.primaryRed.withValues(alpha: 0.1),
                    shape: BoxShape.circle,
                  ),
                  child: Icon(
                    isUnavailable ? Icons.cloud_off : Icons.error_outline,
                    size: 32,
                    color: AppColors.primaryRed,
                  ),
                ),
                const SizedBox(height: 24),
                Text(
                  isUnavailable
                      ? 'Server Tidak Bisa Dijangkau'
                      : 'Gagal Memuat Data',
                  style: Theme.of(context).textTheme.titleLarge?.copyWith(
                    fontWeight: FontWeight.bold,
                    color: isDark
                        ? AppColors.neutralWhite
                        : AppColors.neutralGray900,
                  ),
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: 12),
                Text(
                  message,
                  style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                    color: isDark
                        ? AppColors.neutralGray300
                        : AppColors.neutralGray600,
                    height: 1.5,
                  ),
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: 32),
                // STAGE 3B: the manual "Coba Lagi" must not create a
                // parallel recovery cycle — it is disabled whenever the
                // canonical automatic retry is still pending. (The degraded
                // scaffold itself is only reached once the retry budget is
                // exhausted, so this guard also covers the window where a
                // fresh failure just scheduled a new automatic retry.)
                SizedBox(
                  width: double.infinity,
                  height: 48,
                  child: ElevatedButton.icon(
                    onPressed: isRetryPending
                        ? null
                        : () {
                            ref
                                .read(authControllerProvider.notifier)
                                .retryBackendSync();
                          },
                    icon: const Icon(Icons.refresh),
                    label: const Text('Coba Lagi'),
                    style: ElevatedButton.styleFrom(
                      backgroundColor: AppColors.primaryRed,
                      foregroundColor: AppColors.neutralWhite,
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(12),
                      ),
                    ),
                  ),
                ),
                const SizedBox(height: 12),
                TextButton(
                  onPressed: () {
                    ref.read(authControllerProvider.notifier).signOut();
                  },
                  child: Text(
                    'Keluar',
                    style: TextStyle(
                      color: isDark
                          ? AppColors.neutralGray400
                          : AppColors.neutralGray600,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  /// Retrying state: backend unreachable but automatic retry is in flight.
  /// Shows explicit "Menghubungkan..." instead of generic loading.
  Widget _buildRetryingScaffold(BuildContext context, bool isDark) {
    return Scaffold(
      body: Container(
        decoration: BoxDecoration(
          gradient: isDark
              ? const LinearGradient(
                  begin: Alignment.topLeft,
                  end: Alignment.bottomRight,
                  colors: [
                    AppColors.darkGray900,
                    AppColors.darkGray800,
                    AppColors.darkGray900,
                  ],
                  stops: [0.0, 0.5, 1.0],
                )
              : const LinearGradient(
                  begin: Alignment.topLeft,
                  end: Alignment.bottomRight,
                  colors: [
                    AppColors.neutralWhite,
                    AppColors.neutralGray50,
                    AppColors.neutralWhite,
                  ],
                  stops: [0.0, 0.5, 1.0],
                ),
        ),
        child: Center(
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              AnimatedBuilder(
                animation: _logoController,
                builder: (context, child) {
                  return Transform.scale(
                    scale: _logoScaleAnimation.value,
                    child: Opacity(
                      opacity: _logoFadeAnimation.value,
                      child: _buildLogo(),
                    ),
                  );
                },
              ),
              const SizedBox(height: 32),
              FadeTransition(
                opacity: _textFadeAnimation,
                child: _buildBrandText(),
              ),
              const SizedBox(height: 48),
              FadeTransition(
                opacity: _buttonFadeAnimation,
                child: Column(
                  children: [
                    SizedBox(
                      width: 32,
                      height: 32,
                      child: CircularProgressIndicator(
                        strokeWidth: 3,
                        valueColor: AlwaysStoppedAnimation<Color>(
                          AppColors.primaryRed,
                        ),
                        backgroundColor: isDark
                            ? AppColors.darkGray600
                            : AppColors.neutralGray200,
                      ),
                    ),
                    const SizedBox(height: 16),
                    Text(
                      'Menghubungkan ke server...',
                      style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                            color: isDark
                                ? AppColors.neutralGray300
                                : AppColors.neutralGray600,
                            fontWeight: FontWeight.w600,
                          ),
                    ),
                    const SizedBox(height: 8),
                    Text(
                      'Mencoba lagi secara otomatis',
                      style: Theme.of(context).textTheme.bodySmall?.copyWith(
                            color: isDark
                                ? AppColors.neutralGray500
                                : AppColors.neutralGray500,
                          ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildLogo() {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Container(
      width: 120,
      height: 120,
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(20),
        boxShadow: [
          BoxShadow(
            color: isDark
                ? Colors.black.withValues(alpha: 0.3)
                : Colors.grey.withValues(alpha: 0.2),
            blurRadius: 16,
            spreadRadius: 2,
            offset: const Offset(0, 4),
          ),
        ],
      ),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(20),
        child: Image.asset(
          'assets/images/app_logo.png',
          width: 120,
          height: 120,
          fit: BoxFit.cover,
        ),
      ),
    );
  }

  Widget _buildBrandText() {
    return Column(
      children: [
        Text(
          'LABUDA',
          style: Theme.of(context).textTheme.headlineLarge?.copyWith(
            fontWeight: FontWeight.bold,
            letterSpacing: 3.0,
            color: Theme.of(context).brightness == Brightness.dark
                ? AppColors.neutralWhite
                : AppColors.neutralGray900,
          ),
        ),
        const SizedBox(height: 8),
        Text(
          'Komunitas Koi Indonesia',
          style: Theme.of(context).textTheme.bodyMedium?.copyWith(
            letterSpacing: 1.0,
            color: Theme.of(context).brightness == Brightness.dark
                ? AppColors.neutralGray400
                : AppColors.neutralGray600,
          ),
        ),
      ],
    );
  }

  Widget _buildLoadingIndicator() {
    return Column(
      children: [
        SizedBox(
          width: 32,
          height: 32,
          child: CircularProgressIndicator(
            strokeWidth: 3,
            valueColor: AlwaysStoppedAnimation<Color>(
              Theme.of(context).brightness == Brightness.dark
                  ? AppColors.primaryRed
                  : AppColors.primaryRed,
            ),
            backgroundColor: Theme.of(context).brightness == Brightness.dark
                ? AppColors.darkGray600
                : AppColors.neutralGray200,
          ),
        ),
        const SizedBox(height: 16),
        Text(
          'Memuat aplikasi...',
          style: Theme.of(context).textTheme.bodySmall?.copyWith(
            color: Theme.of(context).brightness == Brightness.dark
                ? AppColors.neutralGray500
                : AppColors.neutralGray500,
          ),
        ),
      ],
    );
  }
}
