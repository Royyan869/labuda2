/// Seller Verification Screen
///
/// REAL implementation for seller identity verification submission.
/// This is required for sellers to withdraw funds.
library;

import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/verification/verification.dart';
import 'package:labuda/domains/user/profile/presentation/screens/ktp_camera_screen.dart';
import 'package:labuda/domains/system/support/presentation/screens/help_center_screen.dart';
import 'package:labuda/domains/system/support/presentation/widgets/pre_chat_form_sheet.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/widgets/blocked_action_gate.dart';

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
        title: const Text('Verifikasi Penjual'),
        backgroundColor: AppColors.primaryRed,
        foregroundColor: Colors.white,
      ),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
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
    );
  }

  Widget _buildAuthRequired() {
    return Scaffold(
      body: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(
              Icons.lock_outline,
              size: 64,
              color: AppColors.primaryRed,
            ),
            const SizedBox(height: 16),
            const Text(
              'Login Diperlukan',
              style: TextStyle(fontSize: 20, fontWeight: FontWeight.bold),
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
        bgColor = AppColors.successGreen.withValues(alpha: 0.1);
        textColor = AppColors.successGreen;
        icon = Icons.verified;
        title = 'Terverifikasi';
        message =
            'Akun penjual Anda telah diverifikasi. Anda dapat melakukan penarikan dana.';
        break;
      case SellerVerificationStatus.pendingReview:
        bgColor = Colors.orange.withValues(alpha: 0.1);
        textColor = Colors.orange;
        icon = Icons.pending;
        title = 'Menunggu Verifikasi';
        message =
            'Dokumen Anda sedang ditinjau oleh tim kami. Proses ini biasanya memakan waktu 1-2 hari kerja.';
        break;
      case SellerVerificationStatus.needsResubmission:
        bgColor = Colors.orange.withValues(alpha: 0.1);
        textColor = Colors.orange;
        icon = Icons.edit_document;
        title = 'Perlu Pengajuan Ulang';
        message =
            'Admin meminta penyesuaian dokumen. Periksa catatan dan ajukan kembali.';
        break;
      case SellerVerificationStatus.rejected:
        bgColor = AppColors.error.withValues(alpha: 0.1);
        textColor = AppColors.error;
        icon = Icons.cancel;
        title = 'Verifikasi Ditolak';
        message =
            'Mohon periksa dokumen Anda dan ajukan kembali. Pastikan dokumen terbaca dengan jelas.';
        break;
      case SellerVerificationStatus.underInvestigation:
        bgColor = Colors.amber.withValues(alpha: 0.1);
        textColor = Colors.amber.shade800;
        icon = Icons.manage_search;
        title = 'Dalam Investigasi';
        message =
            'Verifikasi Anda sedang dalam proses investigasi. Penjualan tetap aktif, namun penarikan dana ditangguhkan sementara.';
        break;
      case SellerVerificationStatus.suspended:
        bgColor = AppColors.error.withValues(alpha: 0.1);
        textColor = AppColors.error;
        icon = Icons.block;
        title = 'Verifikasi Ditangguhkan';
        message =
            'Verifikasi penjual Anda ditangguhkan oleh admin. Penjualan dan penarikan dana tidak tersedia. Hubungi dukungan untuk informasi lebih lanjut.';
        break;
      case SellerVerificationStatus.revoked:
        bgColor = AppColors.error.withValues(alpha: 0.1);
        textColor = AppColors.error;
        icon = Icons.gpp_bad;
        title = 'Verifikasi Dicabut';
        message =
            'Verifikasi penjual Anda telah dicabut secara permanen. Hubungi dukungan untuk informasi lebih lanjut.';
        break;
      case SellerVerificationStatus.notSubmitted:
        bgColor = AppColors.neutralGray100;
        textColor = AppColors.neutralGray700;
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
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: bgColor,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: textColor.withValues(alpha: 0.3)),
      ),
      child: Row(
        children: [
          Icon(icon, color: textColor, size: 32),
          const SizedBox(width: 16),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.bold,
                    color: textColor,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  message,
                  style: TextStyle(
                    fontSize: 13,
                    color: textColor.withValues(alpha: 0.8),
                  ),
                ),
                if ((state.status == SellerVerificationStatus.rejected ||
                        state.status ==
                            SellerVerificationStatus.needsResubmission) &&
                    (state.rejectionReason?.trim().isNotEmpty ?? false)) ...[
                  const SizedBox(height: 8),
                  Text(
                    'Alasan: ${state.rejectionReason}',
                    style: TextStyle(
                      fontSize: 12,
                      color: textColor.withValues(alpha: 0.85),
                      fontStyle: FontStyle.italic,
                    ),
                  ),
                ],
                if (showHelpButton) ...[
                  const SizedBox(height: 12),
                  TextButton.icon(
                    onPressed: () => _showVerificationHelpDialog(),
                    icon: const Icon(Icons.support_agent, size: 16),
                    label: const Text('Dapatkan Bantuan'),
                    style: TextButton.styleFrom(
                      foregroundColor: textColor,
                      padding: const EdgeInsets.symmetric(
                        horizontal: 8,
                        vertical: 4,
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
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(Icons.description_outlined, color: AppColors.primaryRed),
                const SizedBox(width: 8),
                Text(
                  'Dokumen yang Diperlukan',
                  style: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.bold,
                    color: AppColors.neutralGray800,
                  ),
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
          padding: const EdgeInsets.all(8),
          decoration: BoxDecoration(
            color: AppColors.primaryRed.withValues(alpha: 0.1),
            shape: BoxShape.circle,
          ),
          child: Icon(icon, size: 20, color: AppColors.primaryRed),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                title,
                style: const TextStyle(
                  fontWeight: FontWeight.w600,
                  color: AppColors.neutralGray800,
                ),
              ),
              Text(
                description,
                style: const TextStyle(
                  fontSize: 12,
                  color: AppColors.neutralGray600,
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
          style: TextStyle(
            fontSize: 18,
            fontWeight: FontWeight.bold,
            color: AppColors.neutralGray800,
          ),
        ),
        const SizedBox(height: 16),
        TextFormField(
          controller: _fullNameController,
          decoration: const InputDecoration(
            labelText: 'Nama Lengkap',
            hintText: 'Sesuai dengan KTP',
            prefixIcon: Icon(Icons.person),
            border: OutlineInputBorder(),
          ),
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
        TextFormField(
          controller: _nikController,
          decoration: const InputDecoration(
            labelText: 'NIK',
            hintText: '16 digit nomor identitas',
            prefixIcon: Icon(Icons.badge),
            border: OutlineInputBorder(),
          ),
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
        Text(
          'Foto KTP',
          style: TextStyle(
            fontSize: 16,
            fontWeight: FontWeight.bold,
            color: AppColors.neutralGray800,
          ),
        ),
        const SizedBox(height: 8),
        InkWell(
          onTap: () => _captureKtp(),
          borderRadius: BorderRadius.circular(12),
          child: Container(
            height: 200,
            decoration: BoxDecoration(
              color: AppColors.neutralGray100,
              borderRadius: BorderRadius.circular(12),
              border: Border.all(
                color: AppColors.neutralGray300,
                width: _ktpImage != null ? 2 : 1,
              ),
            ),
            child: _ktpImage != null
                ? Stack(
                    children: [
                      ClipRRect(
                        borderRadius: BorderRadius.circular(10),
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
                          icon: const Icon(Icons.close, color: Colors.white),
                          style: IconButton.styleFrom(
                            backgroundColor: Colors.black54,
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
                        size: 48,
                        color: AppColors.neutralGray400,
                      ),
                      const SizedBox(height: 12),
                      Text(
                        'Tap untuk ambil foto KTP',
                        style: TextStyle(color: AppColors.neutralGray600),
                      ),
                      const SizedBox(height: 8),
                      Text(
                        'Pastikan terbaca dengan jelas',
                        style: TextStyle(
                          fontSize: 12,
                          color: AppColors.neutralGray400,
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
          style: TextStyle(
            fontSize: 16,
            fontWeight: FontWeight.bold,
            color: AppColors.neutralGray800,
          ),
        ),
        const SizedBox(height: 4),
        Text(
          'Pegang KTP di depan wajah, pastikan wajah dan KTP terlihat jelas',
          style: TextStyle(fontSize: 12, color: AppColors.neutralGray600),
        ),
        const SizedBox(height: 8),
        InkWell(
          onTap: () => _captureSelfie(),
          borderRadius: BorderRadius.circular(12),
          child: Container(
            height: 200,
            decoration: BoxDecoration(
              color: AppColors.neutralGray100,
              borderRadius: BorderRadius.circular(12),
              border: Border.all(
                color: AppColors.neutralGray300,
                width: _selfieImage != null ? 2 : 1,
              ),
            ),
            child: _selfieImage != null
                ? Stack(
                    children: [
                      ClipRRect(
                        borderRadius: BorderRadius.circular(10),
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
                          icon: const Icon(Icons.close, color: Colors.white),
                          style: IconButton.styleFrom(
                            backgroundColor: Colors.black54,
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
                        size: 48,
                        color: AppColors.neutralGray400,
                      ),
                      const SizedBox(height: 12),
                      Text(
                        'Tap untuk ambil foto selfie',
                        style: TextStyle(color: AppColors.neutralGray600),
                      ),
                      const SizedBox(height: 8),
                      Text(
                        'Selfie memegang KTP di depan wajah',
                        style: TextStyle(
                          fontSize: 12,
                          color: AppColors.neutralGray400,
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
    final canSubmit = isFormValid && hasKtp && hasSelfie && !state.isLoading;

    return SizedBox(
      width: double.infinity,
      child: ElevatedButton(
        onPressed: canSubmit ? () => _submitVerification(user) : null,
        style: ElevatedButton.styleFrom(
          backgroundColor: canSubmit
              ? AppColors.primaryRed
              : AppColors.neutralGray400,
          foregroundColor: Colors.white,
          padding: const EdgeInsets.symmetric(vertical: 16),
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
        ),
        child: _isUploading || state.isLoading
            ? const Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  SizedBox(
                    width: 20,
                    height: 20,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      color: Colors.white,
                    ),
                  ),
                  SizedBox(width: 12),
                  Text('Memproses...'),
                ],
              )
            : const Text(
                'Ajukan Verifikasi',
                style: TextStyle(fontSize: 16, fontWeight: FontWeight.bold),
              ),
      ),
    );
  }

  Widget _buildDocumentsSection(SellerVerificationV2State state) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Dokumen Terupload',
          style: TextStyle(
            fontSize: 16,
            fontWeight: FontWeight.bold,
            color: AppColors.neutralGray800,
          ),
        ),
        const SizedBox(height: 12),
        ...state.documents.map(
          (doc) => Card(
            child: ListTile(
              leading: Icon(
                Icons.description,
                color: doc['status'] == 'approved'
                    ? AppColors.successGreen
                    : doc['status'] == 'rejected'
                    ? AppColors.error
                    : Colors.orange,
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
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('Silakan ambil foto KTP')));
      return;
    }
    if (_selfieImage == null) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Silakan ambil foto selfie dengan KTP')),
      );
      return;
    }

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
            fullName: _fullNameController.text.trim(),
            nationalId: _nikController.text.trim(),
            ktpStorageKey: _ktpStorageKey!,
            selfieStorageKey: _selfieStorageKey!,
          );

      if (!mounted) return;

      if (success) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Verifikasi berhasil dikirim'),
            backgroundColor: AppColors.successGreen,
          ),
        );
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
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: const Text('Terjadi kesalahan. Coba lagi.'),
            backgroundColor: AppColors.error,
          ),
        );
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
        await showBlockedActionGate(
          context,
          actionDescription: 'mengajukan verifikasi penjual',
        );
        return;
      case 'ACCOUNT_SUSPENDED':
        await _showAccountBlockedDialog(
          title: 'Akun Ditangguhkan',
          message:
              'Akun Anda sedang ditangguhkan. Hubungi tim dukungan untuk informasi lebih lanjut.',
        );
        return;
      case 'ACCOUNT_BANNED':
        await _showAccountBlockedDialog(
          title: 'Akun Diblokir',
          message:
              'Akun Anda telah diblokir dan tidak dapat mengajukan verifikasi.',
        );
        return;
    }
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(state.errorMessage ?? 'Gagal mengirim verifikasi'),
        backgroundColor: AppColors.error,
      ),
    );
  }

  Future<void> _handleApiException(ApiException e) async {
    switch (e.code) {
      case 'EMAIL_VERIFICATION_REQUIRED':
        await showBlockedActionGate(
          context,
          actionDescription: 'mengajukan verifikasi penjual',
        );
        return;
      case 'ACCOUNT_SUSPENDED':
        await _showAccountBlockedDialog(
          title: 'Akun Ditangguhkan',
          message:
              'Akun Anda sedang ditangguhkan. Hubungi tim dukungan untuk informasi lebih lanjut.',
        );
        return;
      case 'ACCOUNT_BANNED':
        await _showAccountBlockedDialog(
          title: 'Akun Diblokir',
          message:
              'Akun Anda telah diblokir dan tidak dapat mengajukan verifikasi.',
        );
        return;
    }
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: const Text('Terjadi kesalahan. Coba lagi.'),
        backgroundColor: AppColors.error,
      ),
    );
  }

  Future<void> _showAccountBlockedDialog({
    required String title,
    required String message,
  }) {
    return showDialog<void>(
      context: context,
      builder: (dialogCtx) => AlertDialog(
        title: Text(title),
        content: Text(message),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogCtx).pop(),
            child: const Text('Tutup'),
          ),
        ],
      ),
    );
  }

  // CONTEXTUAL SUPPORT BRIDGE (Phase 2 Hardening)
  // Shows help options when verification is rejected
  void _showVerificationHelpDialog() {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) return;

    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Bantuan Verifikasi'),
        content: const Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              'Verifikasi Anda ditolak. Pilih opsi di bawah:',
              style: TextStyle(fontSize: 14),
            ),
            SizedBox(height: 16),
            Text(
              '• Pastikan KTP terbaca jelas',
              style: TextStyle(fontSize: 13),
            ),
            Text(
              '• Nama harus sesuai dengan KTP',
              style: TextStyle(fontSize: 13),
            ),
            Text('• NIK harus 16 digit', style: TextStyle(fontSize: 13)),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Tutup'),
          ),
          ElevatedButton(
            onPressed: () {
              Navigator.pop(context);
              // Navigate to Help Center with verification category
              Navigator.push(
                context,
                MaterialPageRoute(
                  builder: (context) => const HelpCenterScreen(),
                ),
              );
            },
            style: ElevatedButton.styleFrom(
              backgroundColor: AppColors.primaryRed,
            ),
            child: const Text('Lihat Panduan'),
          ),
          TextButton(
            onPressed: () {
              Navigator.pop(context);
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
    );
  }
}
