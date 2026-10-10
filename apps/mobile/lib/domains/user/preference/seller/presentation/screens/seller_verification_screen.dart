/// Seller Verification Screen
///
/// REAL implementation for seller identity verification submission.
/// This is required for sellers to withdraw funds.
library;

import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/widgets/app_dialog.dart';
import 'package:hishumi/shared/widgets/app_snackbar.dart';
import 'package:hishumi/shared/widgets/app_text_field.dart';
import 'package:hishumi/domains/user/identity/verification/verification.dart';
import 'package:hishumi/domains/user/profile/presentation/screens/ktp_camera_screen.dart';
import 'package:hishumi/domains/system/support/presentation/widgets/pre_chat_form_sheet.dart';

/// Seller Verification Screen
///
/// Allows sellers to submit KYC documents: KTP + selfie.
/// Both are required for withdrawal eligibility.
class SellerVerificationScreen extends ConsumerStatefulWidget {
  const SellerVerificationScreen({super.key});

  @override
  ConsumerState<SellerVerificationScreen> createState() =>
      _SellerVerificationScreenState();
}

class _SellerVerificationScreenState
    extends ConsumerState<SellerVerificationScreen> {
  final _formKey = GlobalKey<FormState>();
  final _fullNameController = TextEditingController();
  final _nikController = TextEditingController();

  File? _ktpImage;
  String? _ktpStorageKey;

  File? _selfieImage;
  String? _selfieStorageKey;

  bool _isUploading = false;

  @override
  void initState() {
    super.initState();
    // Load verification status
    Future.microtask(() {
      ref.read(sellerVerificationV2NotifierProvider.notifier).loadStatus();
      ref.read(sellerVerificationV2NotifierProvider.notifier).loadDocuments();
    });
  }

  @override
  void dispose() {
    _fullNameController.dispose();
    _nikController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authControllerProvider);

    if (authState is! AuthStateAuthenticated) {
      return _buildAuthRequired();
    }

    final verificationState = ref.watch(sellerVerificationV2NotifierProvider);
    final user = authState.user;

    return Scaffold(
      appBar: AppBar(
        // Chrome colour is theme data: this bar used to paint the BRAND
        // colour edge-to-edge with white ink — no other screen does that.
        title: const Text('Verifikasi Penjual'),
      ),
      // Canonical body-level bottom-inset authority (SAFE-AREA-20): the ONE
      // `SafeArea` consumes the live system bottom inset for the whole body.
      // The scroll view's explicit `p16` padding below is DESIGN spacing only
      // — an explicit `ScrollView.padding` never inherits MediaQuery padding.
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(AppMetrics.p16),
          child: Form(
            key: _formKey,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Verification Status Banner
                _buildVerificationStatusBanner(verificationState, user),

                const SizedBox(height: 24),

                // Instructions
                _buildInstructionsSection(),

                const SizedBox(height: 24),

                // Form: only show for states where the seller can (re)submit.
                // suspended / revoked / under_investigation / pending_review /
                // approved = no form.
                if (const {
                  SellerVerificationStatus.notSubmitted,
                  SellerVerificationStatus.rejected,
                  SellerVerificationStatus.needsResubmission,
                }.contains(verificationState.status)) ...[
                  _buildPersonalInfoSection(),
                  const SizedBox(height: 24),
                  _buildKtpUploadSection(),
                  const SizedBox(height: 24),
                  _buildSelfieUploadSection(),
                  const SizedBox(height: 32),
                  _buildSubmitButton(verificationState, user),
                ],

                // Documents List
                if (verificationState.documents.isNotEmpty) ...[
                  const SizedBox(height: 24),
                  _buildDocumentsSection(verificationState),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildAuthRequired() {
    return Scaffold(
      appBar: AppBar(title: const Text('Verifikasi Penjual')),
      body: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.lock_outline,
              size: AppIconSize.display,
              color: Theme.of(context).colorScheme.primary,
            ),
            const SizedBox(height: 16),
            Text(
              'Login Diperlukan',
              style: context.typeRoles.titleSection.copyWith(
                fontWeight: FontWeight.bold,
              ),
            ),
            const SizedBox(height: 8),
            const Text('Silakan login untuk verifikasi penjual'),
          ],
        ),
      ),
    );
  }

  Widget _buildVerificationStatusBanner(
    SellerVerificationV2State state,
    AuthUser user,
  ) {
    Color bgColor;
    Color textColor;
    IconData icon;
    String title;
    String message;

    switch (state.status) {
      case SellerVerificationStatus.approved:
        bgColor = context.statusColors.success.withValues(alpha: 0.1);
        textColor = context.statusColors.success;
        icon = Icons.verified;
        title = 'Terverifikasi';
        message =
            'Akun penjual Anda telah diverifikasi. Anda dapat melakukan penarikan dana.';
        break;
      case SellerVerificationStatus.pendingReview:
        bgColor = context.statusColors.warning.withValues(alpha: 0.1);
        textColor = context.statusColors.warning;
        icon = Icons.pending;
        title = 'Menunggu Verifikasi';
        message =
            'Dokumen Anda sedang ditinjau oleh tim kami. Proses ini biasanya memakan waktu 1-2 hari kerja.';
        break;
      case SellerVerificationStatus.needsResubmission:
        bgColor = context.statusColors.warning.withValues(alpha: 0.1);
        textColor = context.statusColors.warning;
        icon = Icons.edit_document;
        title = 'Perlu Pengajuan Ulang';
        message =
            'Admin meminta penyesuaian dokumen. Periksa catatan dan ajukan kembali.';
        break;
      case SellerVerificationStatus.rejected:
        bgColor = context.statusColors.error.withValues(alpha: 0.1);
        textColor = context.statusColors.error;
        icon = Icons.cancel;
        title = 'Verifikasi Ditolak';
        message =
            'Mohon periksa dokumen Anda dan ajukan kembali. Pastikan dokumen terbaca dengan jelas.';
        break;
      case SellerVerificationStatus.underInvestigation:
        // Status tone lives in ONE palette (context.statusColors); the raw
        // brand yellow was a second, off-palette amber.
        bgColor = context.statusColors.warning.withValues(alpha: 0.1);
        textColor = context.statusColors.warning;
        icon = Icons.manage_search;
        title = 'Dalam Investigasi';
        message =
            'Verifikasi Anda sedang dalam proses investigasi. Penjualan tetap aktif, namun penarikan dana ditangguhkan sementara.';
        break;
      case SellerVerificationStatus.suspended:
        bgColor = context.statusColors.error.withValues(alpha: 0.1);
        textColor = context.statusColors.error;
        icon = Icons.block;
        title = 'Verifikasi Ditangguhkan';
        message =
            'Verifikasi penjual Anda ditangguhkan oleh admin. Penjualan dan penarikan dana tidak tersedia. Hubungi dukungan untuk informasi lebih lanjut.';
        break;
      case SellerVerificationStatus.revoked:
        bgColor = context.statusColors.error.withValues(alpha: 0.1);
        textColor = context.statusColors.error;
        icon = Icons.gpp_bad;
        title = 'Verifikasi Dicabut';
        message =
            'Verifikasi penjual Anda telah dicabut secara permanen. Hubungi dukungan untuk informasi lebih lanjut.';
        break;
      case SellerVerificationStatus.notSubmitted:
        bgColor = Theme.of(context).colorScheme.surfaceContainer;
        textColor = Theme.of(context).colorScheme.onSurfaceVariant;
        icon = Icons.info_outline;
        title = 'Belum Diverifikasi';
        message =
            'Verifikasi diperlukan untuk dapat menarik dana dari penjualan.';
    }

    // Show help button for states where contacting support is actionable.
    final showHelpButton =
        state.status == SellerVerificationStatus.rejected ||
        state.status == SellerVerificationStatus.needsResubmission ||
        state.status == SellerVerificationStatus.suspended ||
        state.status == SellerVerificationStatus.revoked;

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: bgColor,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: textColor.withValues(alpha: 0.3)),
      ),
      child: Row(
        children: [
          Icon(icon, color: textColor, size: AppIconSize.emphasis),
          const SizedBox(width: 16),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: context.typeRoles.titleCompact.copyWith(
                    fontWeight: FontWeight.bold,
                    color: textColor,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  message,
                  // Body copy reads primary ink: an 80%-opacity status tone on
                  // a 10% tint of the same tone fails AA at 13px.
                  style: context.typeRoles.bodyDense.copyWith(
                    color: Theme.of(context).colorScheme.onSurface,
                  ),
                ),
                if ((state.status == SellerVerificationStatus.rejected ||
                        state.status ==
                            SellerVerificationStatus.needsResubmission) &&
                    (state.rejectionReason?.trim().isNotEmpty ?? false)) ...[
                  const SizedBox(height: 8),
                  Text(
                    'Alasan: ${state.rejectionReason}',
                    style: context.typeRoles.labelMicro.copyWith(
                      color: Theme.of(context).colorScheme.onSurfaceVariant,
                      fontStyle: FontStyle.italic,
                    ),
                  ),
                ],
                if (showHelpButton) ...[
                  const SizedBox(height: 12),
                  TextButton.icon(
                    onPressed: () => _openVerificationHelp(),
                    icon: const Icon(
                      Icons.support_agent,
                      size: AppIconSize.inlineGlyph,
                    ),
                    label: const Text('Dapatkan Bantuan'),
                    style: TextButton.styleFrom(
                      foregroundColor: textColor,
                      padding: const EdgeInsets.symmetric(
                        horizontal: AppMetrics.p8,
                        vertical: AppMetrics.p4,
                      ),
                      minimumSize: const Size(60, 32),
                    ),
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildInstructionsSection() {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(AppMetrics.p16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(
                  Icons.description_outlined,
                  color: Theme.of(context).colorScheme.primary,
                ),
                const SizedBox(width: 8),
                Text(
                  'Dokumen yang Diperlukan',
                  // Section header role (canonical map: section → titleMedium).
                  // This used to be s16 here and s18 two sections below.
                  style: Theme.of(context).textTheme.titleMedium,
                ),
              ],
            ),
            const SizedBox(height: 16),
            _buildInstructionItem(
              Icons.credit_card,
              'Foto KTP',
              'Pastikan semua informasi terbaca jelas',
            ),
            const SizedBox(height: 12),
            _buildInstructionItem(
              Icons.face,
              'Foto Selfie',
              'Selfie memegang KTP di depan wajah',
            ),
            const SizedBox(height: 12),
            _buildInstructionItem(
              Icons.badge,
              'Nama Lengkap',
              'Sesuai dengan KTP',
            ),
            const SizedBox(height: 12),
            _buildInstructionItem(
              Icons.numbers,
              'NIK',
              'Nomor identitas 16 digit',
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildInstructionItem(
    IconData icon,
    String title,
    String description,
  ) {
    return Row(
      children: [
        Container(
          padding: const EdgeInsets.all(AppMetrics.p8),
          decoration: BoxDecoration(
            color: Theme.of(context).colorScheme.primary.withValues(alpha: 0.1),
            shape: BoxShape.circle,
          ),
          child: Icon(
            icon,
            size: AppIconSize.action,
            color: Theme.of(context).colorScheme.primary,
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                title,
                style: TextStyle(
                  fontWeight: FontWeight.w600,
                  color: Theme.of(context).colorScheme.onSurface,
                ),
              ),
              Text(
                description,
                style: context.typeRoles.labelMicro.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildPersonalInfoSection() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Informasi Pribadi',
          style: Theme.of(context).textTheme.titleMedium,
        ),
        const SizedBox(height: 16),
        AppTextField(
          controller: _fullNameController,
          labelText: 'Nama Lengkap',
          hintText: 'Sesuai dengan KTP',
          prefixIcon: Icons.person,
          validator: (value) {
            if (value == null || value.isEmpty) {
              return 'Nama lengkap wajib diisi';
            }
            if (value.length < 3) {
              return 'Nama lengkap minimal 3 karakter';
            }
            return null;
          },
        ),
        const SizedBox(height: 16),
        AppTextField(
          controller: _nikController,
          labelText: 'NIK',
          hintText: '16 digit nomor identitas',
          prefixIcon: Icons.badge,
          keyboardType: TextInputType.number,
          maxLength: 16,
          validator: (value) {
            if (value == null || value.isEmpty) {
              return 'NIK wajib diisi';
            }
            if (value.length != 16) {
              return 'NIK harus 16 digit';
            }
            return null;
          },
        ),
      ],
    );
  }

  Widget _buildKtpUploadSection() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('Foto KTP', style: Theme.of(context).textTheme.titleMedium),
        const SizedBox(height: 8),
        InkWell(
          onTap: () => _captureKtp(),
          borderRadius: BorderRadius.circular(AppShape.r12),
          child: Container(
            height: AppContentSize.capture,
            decoration: BoxDecoration(
              color: Theme.of(context).colorScheme.surfaceContainer,
              borderRadius: BorderRadius.circular(AppShape.r12),
              border: Border.all(
                color: Theme.of(context).colorScheme.outlineVariant,
                width: _ktpImage != null ? 2 : 1,
              ),
            ),
            child: _ktpImage != null
                ? Stack(
                    children: [
                      ClipRRect(
                        borderRadius: BorderRadius.circular(AppShape.r10),
                        child: Image.file(
                          _ktpImage!,
                          width: double.infinity,
                          height: double.infinity,
                          fit: BoxFit.cover,
                        ),
                      ),
                      Positioned(
                        top: 8,
                        right: 8,
                        child: IconButton(
                          onPressed: () {
                            setState(() {
                              _ktpImage = null;
                              _ktpStorageKey = null;
                            });
                          },
                          icon: Icon(
                            Icons.close,
                            color: Theme.of(context).colorScheme.onPrimary,
                           semanticLabel: 'Hapus',
                           ),
                          style: IconButton.styleFrom(
                            backgroundColor: Theme.of(
                              context,
                            ).colorScheme.scrim.withValues(alpha: 0.54),
                          ),
                        ),
                      ),
                    ],
                  )
                : Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Icon(
                        Icons.camera_alt,
                        size: AppIconSize.display,
                        color: Theme.of(context).colorScheme.onSurfaceVariant,
                      ),
                      const SizedBox(height: 12),
                      Text(
                        'Tap untuk ambil foto KTP',
                        style: TextStyle(
                          color: Theme.of(context).colorScheme.onSurfaceVariant,
                        ),
                      ),
                      const SizedBox(height: 8),
                      Text(
                        'Pastikan terbaca dengan jelas',
                        style: context.typeRoles.labelMicro.copyWith(
                          color: Theme.of(context).colorScheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ),
          ),
        ),
      ],
    );
  }

  Widget _buildSelfieUploadSection() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Foto Selfie dengan KTP',
          style: Theme.of(context).textTheme.titleMedium,
        ),
        const SizedBox(height: 4),
        Text(
          'Pegang KTP di depan wajah, pastikan wajah dan KTP terlihat jelas',
          style: context.typeRoles.labelMicro.copyWith(
            color: Theme.of(context).colorScheme.onSurfaceVariant,
          ),
        ),
        const SizedBox(height: 8),
        InkWell(
          onTap: () => _captureSelfie(),
          borderRadius: BorderRadius.circular(AppShape.r12),
          child: Container(
            height: AppContentSize.capture,
            decoration: BoxDecoration(
              color: Theme.of(context).colorScheme.surfaceContainer,
              borderRadius: BorderRadius.circular(AppShape.r12),
              border: Border.all(
                color: Theme.of(context).colorScheme.outlineVariant,
                width: _selfieImage != null ? 2 : 1,
              ),
            ),
            child: _selfieImage != null
                ? Stack(
                    children: [
                      ClipRRect(
                        borderRadius: BorderRadius.circular(AppShape.r10),
                        child: Image.file(
                          _selfieImage!,
                          width: double.infinity,
                          height: double.infinity,
                          fit: BoxFit.cover,
                        ),
                      ),
                      Positioned(
                        top: 8,
                        right: 8,
                        child: IconButton(
                          onPressed: () {
                            setState(() {
                              _selfieImage = null;
                              _selfieStorageKey = null;
                            });
                          },
                          icon: Icon(
                            Icons.close,
                            color: Theme.of(context).colorScheme.onPrimary,
                           semanticLabel: 'Hapus',
                           ),
                          style: IconButton.styleFrom(
                            backgroundColor: Theme.of(
                              context,
                            ).colorScheme.scrim.withValues(alpha: 0.54),
                          ),
                        ),
                      ),
                    ],
                  )
                : Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Icon(
                        Icons.face,
                        size: AppIconSize.display,
                        color: Theme.of(context).colorScheme.onSurfaceVariant,
                      ),
                      const SizedBox(height: 12),
                      Text(
                        'Tap untuk ambil foto selfie',
                        style: TextStyle(
                          color: Theme.of(context).colorScheme.onSurfaceVariant,
                        ),
                      ),
                      const SizedBox(height: 8),
                      Text(
                        'Selfie memegang KTP di depan wajah',
                        style: context.typeRoles.labelMicro.copyWith(
                          color: Theme.of(context).colorScheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ),
          ),
        ),
      ],
    );
  }

  Widget _buildSubmitButton(SellerVerificationV2State state, AuthUser user) {
    final isFormValid = _formKey.currentState?.validate() ?? false;
    final hasKtp = _ktpImage != null;
    final hasSelfie = _selfieImage != null;
    // `!_isUploading` is essential: without it the button stays enabled during
    // the KTP/selfie upload phase and a second tap would re-enter the flow.
    final canSubmit =
        isFormValid && hasKtp && hasSelfie && !state.isLoading && !_isUploading;
    final scheme = Theme.of(context).colorScheme;

    return SizedBox(
      width: double.infinity,
      child: ElevatedButton(
        onPressed: canSubmit ? () => _submitVerification(user) : null,
        // Enabled/disabled fill and the label ink come from the button theme —
        // the screen must not re-decide them (same rule as the auction CTA).
        style: ElevatedButton.styleFrom(
          padding: const EdgeInsets.symmetric(vertical: AppMetrics.p16),
        ),
        child: _isUploading || state.isLoading
            ? Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  SizedBox(
                    width: 20,
                    height: 20,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      color: scheme.onPrimary,
                    ),
                  ),
                  const SizedBox(width: 12),
                  const Text('Memproses...'),
                ],
              )
            : const Text('Ajukan Verifikasi'),
      ),
    );
  }

  Widget _buildDocumentsSection(SellerVerificationV2State state) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Dokumen Terupload',
          style: Theme.of(context).textTheme.titleMedium,
        ),
        const SizedBox(height: 12),
        ...state.documents.map(
          (doc) => Card(
            child: ListTile(
              leading: Icon(
                Icons.description,
                color: doc['status'] == 'approved'
                    ? context.statusColors.success
                    : doc['status'] == 'rejected'
                    ? context.statusColors.error
                    : context.statusColors.warning,
              ),
              title: Text(_getDocumentTypeLabel(doc['document_type'])),
              subtitle: Text(_getDocumentStatusLabel(doc['status'])),
              trailing: null,
            ),
          ),
        ),
      ],
    );
  }

  String _getDocumentTypeLabel(String type) {
    switch (type.toLowerCase()) {
      case 'identity_ktp':
        return 'KTP';
      case 'identity_selfie':
        return 'Selfie dengan KTP';
      default:
        return type;
    }
  }

  String _getDocumentStatusLabel(String status) {
    switch (status.toLowerCase()) {
      case 'approved':
        return 'Disetujui';
      case 'rejected':
        return 'Ditolak';
      case 'pending':
      case 'pending_review':
        return 'Menunggu Review';
      default:
        return status;
    }
  }

  Future<void> _captureKtp() async {
    final result = await Navigator.push<String?>(
      context,
      MaterialPageRoute(builder: (context) => const KtpCameraScreen()),
    );

    if (result != null) {
      setState(() {
        _ktpImage = File(result);
        _ktpStorageKey = null;
      });
    }
  }

  Future<void> _captureSelfie() async {
    final result = await Navigator.push<String?>(
      context,
      MaterialPageRoute(builder: (context) => const KtpCameraScreen()),
    );

    if (result != null) {
      setState(() {
        _selfieImage = File(result);
        _selfieStorageKey = null;
      });
    }
  }

  Future<void> _submitVerification(AuthUser user) async {
    if (!_formKey.currentState!.validate()) return;
    if (_ktpImage == null) {
      AppSnackBar.showError(context, 'Silakan ambil foto KTP');
      return;
    }
    if (_selfieImage == null) {
      AppSnackBar.showError(context, 'Silakan ambil foto selfie dengan KTP');
      return;
    }

    // RE-ENTRY GUARD: an upload is already in flight — never start a second.
    if (_isUploading) return;

    // SUBMISSION SNAPSHOT: capture name/NIK BEFORE the document uploads so an
    // edit during upload cannot change the KYC that is actually submitted.
    final fullNameSnapshot = _fullNameController.text.trim();
    final nationalIdSnapshot = _nikController.text.trim();

    setState(() => _isUploading = true);

    try {
      final s3Service = ref.read(s3ServiceProvider);

      // Upload KTP via backend-issued presigned PUT URL — returns storage_key only.
      final ktpResult = await s3Service.uploadKYCDocument(
        _ktpImage!,
        'identity_ktp',
      );
      if (ktpResult.isError || ktpResult.data == null) {
        throw Exception(ktpResult.error ?? 'Gagal upload KTP');
      }
      _ktpStorageKey = ktpResult.data!;

      // Upload selfie via backend-issued presigned PUT URL — returns storage_key only.
      final selfieResult = await s3Service.uploadKYCDocument(
        _selfieImage!,
        'identity_selfie',
      );
      if (selfieResult.isError || selfieResult.data == null) {
        throw Exception(selfieResult.error ?? 'Gagal upload selfie');
      }
      _selfieStorageKey = selfieResult.data!;

      // Submit KYC (both documents atomic)
      final success = await ref
          .read(sellerVerificationV2NotifierProvider.notifier)
          .submitKYC(
            fullName: fullNameSnapshot,
            nationalId: nationalIdSnapshot,
            ktpStorageKey: _ktpStorageKey!,
            selfieStorageKey: _selfieStorageKey!,
          );

      if (!mounted) return;

      if (success) {
        AppSnackBar.showSuccess(context, 'Verifikasi berhasil dikirim');
        setState(() {
          _ktpImage = null;
          _ktpStorageKey = null;
          _selfieImage = null;
          _selfieStorageKey = null;
        });
      } else {
        await _handleVerificationFailure();
      }
    } on ApiException catch (e) {
      if (mounted) {
        await _handleApiException(e);
      }
    } catch (e) {
      if (mounted) {
        AppSnackBar.showError(context, 'Terjadi kesalahan. Coba lagi.');
      }
    } finally {
      if (mounted) {
        setState(() => _isUploading = false);
      }
    }
  }

  /// Honors the structured error code propagated from the notifier/repository.
  Future<void> _handleVerificationFailure() async {
    final state = ref.read(sellerVerificationV2NotifierProvider);
    switch (state.errorCode) {
      case 'EMAIL_VERIFICATION_REQUIRED':
        // Backend-rejection handler (defense-in-depth): the backend stays
        // the single authority for EMAIL_VERIFICATION_REQUIRED.
        if (!mounted) return;
        AppSnackBar.showError(
          context,
          'Verifikasi email kamu diperlukan sebelum mengajukan verifikasi penjual.',
        );
        return;
      case 'ACCOUNT_SUSPENDED':
        await AppDialog.info(
          context: context,
          title: 'Akun Ditangguhkan',
          message:
              'Akun Anda sedang ditangguhkan. Hubungi tim dukungan untuk informasi lebih lanjut.',
          closeLabel: 'Tutup',
        );
        return;
      case 'ACCOUNT_BANNED':
        await AppDialog.info(
          context: context,
          title: 'Akun Diblokir',
          message:
              'Akun Anda telah diblokir dan tidak dapat mengajukan verifikasi.',
          closeLabel: 'Tutup',
        );
        return;
    }
    if (!mounted) return;
    AppSnackBar.showError(
      context,
      state.errorMessage ?? 'Gagal mengirim verifikasi',
    );
  }

  Future<void> _handleApiException(ApiException e) async {
    switch (e.code) {
      case 'EMAIL_VERIFICATION_REQUIRED':
        // Backend-rejection handler (defense-in-depth): the backend stays
        // the single authority for EMAIL_VERIFICATION_REQUIRED.
        if (!mounted) return;
        AppSnackBar.showError(
          context,
          'Verifikasi email kamu diperlukan sebelum mengajukan verifikasi penjual.',
        );
        return;
      case 'ACCOUNT_SUSPENDED':
        await AppDialog.info(
          context: context,
          title: 'Akun Ditangguhkan',
          message:
              'Akun Anda sedang ditangguhkan. Hubungi tim dukungan untuk informasi lebih lanjut.',
          closeLabel: 'Tutup',
        );
        return;
      case 'ACCOUNT_BANNED':
        await AppDialog.info(
          context: context,
          title: 'Akun Diblokir',
          message:
              'Akun Anda telah diblokir dan tidak dapat mengajukan verifikasi.',
          closeLabel: 'Tutup',
        );
        return;
    }
    if (!mounted) return;
    AppSnackBar.showError(context, 'Terjadi kesalahan. Coba lagi.');
  }

  // CONTEXTUAL SUPPORT BRIDGE (Phase 2 Hardening)
  // Shows help options when verification is rejected.
  //
  // The SURFACE is the canonical [AppDialog.info]; the two navigation actions
  // (Lihat Panduan / Hubungi Support) are rendered as content actions and keep
  // their business behavior. No local dialog authority is introduced.
  void _openVerificationHelp() {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) return;

    AppDialog.info(
      context: context,
      title: 'Bantuan Verifikasi',
      closeLabel: 'Tutup',
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Verifikasi Anda ditolak. Pilih opsi di bawah:',
            style: context.typeRoles.bodyDense,
          ),
          const SizedBox(height: 16),
          Text(
            '• Pastikan KTP terbaca jelas',
            style: context.typeRoles.bodyDense,
          ),
          Text(
            '• Nama harus sesuai dengan KTP',
            style: context.typeRoles.bodyDense,
          ),
          Text('• NIK harus 16 digit', style: context.typeRoles.bodyDense),
          const SizedBox(height: 16),
          Builder(
            builder: (dialogContext) => Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                ElevatedButton(
                  onPressed: () {
                    Navigator.of(dialogContext).pop();
                    // Navigate to Help Center with verification category
                    context.push(RoutePaths.helpCategoryPath('verification'));
                  },
                  child: const Text('Lihat Panduan'),
                ),
                TextButton(
                  onPressed: () {
                    Navigator.of(dialogContext).pop();
                    // Open support chat with verification category
                    showPreChatFormRefactored(
                      context,
                      userId: authState.user.id,
                      userName: authState.user.username,
                      userAvatar: authState.user.avatarUrl,
                    );
                  },
                  child: const Text('Hubungi Support'),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
