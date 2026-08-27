import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/object/presentation/widgets/object_preview_card.dart';

/// Universal Attachment Widget - Routes to specific attachment widget based on type
/// Single source of truth for attachment rendering across Chat, Comment, and Post
///
/// **SIMPLIFIED:** Now accepts ShareReference or Attachment directly instead of
/// MessageAttachment wrapper abstraction for cleaner architecture.
///
/// **SHIPPING QUOTE FIX:** Added activeQuoteOfferId parameter to mark the active quote.
class AttachmentWidget extends ConsumerWidget {
  final ShareReference? objectReference;
  final Attachment? attachment;
  final bool isFromCurrentUser;
  final VoidCallback? onTap;
  final VoidCallback? onNegotiate;
  final VoidCallback? onPurchase;
  final String? currentUserId;
  final String? contextId; // chatId, commentId, or contentId

  /// **SHIPPING QUOTE FIX:** The offerId of the currently active shipping quote.
  /// When this matches a ShippingQuoteAttachment's offerId, it's marked as "Penawaran Aktif".
  /// Other quotes are marked as "Penawaran Tidak Berlaku" (expired/superseded).
  final String? activeQuoteOfferId;

  const AttachmentWidget({
    super.key,
    this.objectReference,
    this.attachment,
    this.isFromCurrentUser = false,
    this.onTap,
    this.onNegotiate,
    this.onPurchase,
    this.currentUserId,
    this.contextId,
    this.activeQuoteOfferId, // **SHIPPING QUOTE FIX**
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // ============================================================================
    // SIMPLIFIED ATTACHMENT HANDLING
    // ============================================================================
    // Direct handling of ShareReference and Attachment types without wrapper.
    // ============================================================================

    // Handle ShareReference (object references)
    if (objectReference != null) {
      return ObjectPreviewCard(
        reference: objectReference!,
        onTap: onTap,
        showTypeBadge: true,
      );
    }

    // Handle Attachment types
    if (attachment != null) {
      return _buildAttachment(context, attachment!);
    }

    return const SizedBox.shrink();
  }

  /// Build attachment widgets
  Widget _buildAttachment(BuildContext context, Attachment attachment) {
    // Workflow Payloads → Keep custom rendering (domain-specific logic)
    if (attachment is LocationAttachment) {
      return _buildLocationAttachment(context, attachment);
    } else if (attachment is NegotiationOfferAttachment) {
      return _buildNegotiationOfferAttachment(context, attachment);
    } else if (attachment is NegotiationProposalAttachment) {
      return _buildNegotiationProposalAttachment(context, attachment);
    } else if (attachment is NegotiationResultAttachment) {
      return _buildNegotiationResultAttachment(context, attachment);
    } else if (attachment is ShippingQuoteAttachment) {
      return _buildShippingQuoteAttachment(
        context,
        attachment,
        isActiveQuote: attachment.offerId == activeQuoteOfferId,
      );
    }

    // Fallback for unknown attachment types
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: isDark
            ? AppColors.darkGray700.withValues(alpha: 0.5)
            : AppColors.neutralGray200.withValues(alpha: 0.5),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Text(
        'Unsupported attachment: ${attachment.runtimeType}',
        style: TextStyle(
          fontSize: 12,
          fontStyle: FontStyle.italic,
          color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
        ),
      ),
    );
  }

  // =============================================================================
  // WORKFLOW PAYLOAD ATTACHMENTS - Domain-specific business logic
  // =============================================================================
  // These attachments contain workflow/business state and require custom rendering.
  // They are NOT object references - they are domain-specific payloads.
  // =============================================================================

  /// Negotiation Offer Attachment - Active negotiation in chat
  ///
  /// **EXECUTION WAVE CV1:** Enhanced with:
  /// - Clearer status indication
  /// - Better action button labeling
  /// - Next-step guidance
  Widget _buildNegotiationOfferAttachment(
    BuildContext context,
    NegotiationOfferAttachment offer,
  ) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final canUserAct =
        currentUserId != null && offer.canUserAct(currentUserId!);
    final isPending = offer.isActive;
    final statusColor = _getNegotiationStatusColor(offer.status);
    final statusLabel = _getNegotiationStatusLabel(offer.status);

    return Container(
      constraints: const BoxConstraints(maxWidth: 280),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: statusColor.withValues(alpha: 0.5),
          width: 1.5,
        ),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header with pulse animation for pending
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
            decoration: BoxDecoration(
              color: statusColor.withValues(alpha: 0.1),
              borderRadius: const BorderRadius.vertical(
                top: Radius.circular(11),
              ),
            ),
            child: Row(
              children: [
                // **EXECUTION WAVE CV1:** Pulse animation for active negotiations
                if (isPending)
                  SizedBox(
                    width: 18,
                    height: 18,
                    child: Stack(
                      children: [
                        Icon(
                          Icons.handshake_outlined,
                          size: 18,
                          color: statusColor,
                        ),
                        Positioned(
                          right: 0,
                          bottom: 0,
                          child: Container(
                            width: 8,
                            height: 8,
                            decoration: BoxDecoration(
                              color: statusColor,
                              shape: BoxShape.circle,
                            ),
                          ),
                        ),
                      ],
                    ),
                  )
                else
                  Icon(Icons.handshake_outlined, size: 18, color: statusColor),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    'Negosiasi ${isPending ? 'Aktif' : statusLabel}',
                    style: TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w600,
                      color: statusColor,
                    ),
                  ),
                ),
                if (isPending)
                  Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 6,
                      vertical: 2,
                    ),
                    decoration: BoxDecoration(
                      color: statusColor.withValues(alpha: 0.2),
                      borderRadius: BorderRadius.circular(10),
                    ),
                    child: Text(
                      'Ronde ${offer.round}',
                      style: TextStyle(
                        fontSize: 10,
                        fontWeight: FontWeight.w600,
                        color: statusColor,
                      ),
                    ),
                  ),
              ],
            ),
          ),

          // Content
          Padding(
            padding: const EdgeInsets.all(12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Product Name
                Text(
                  offer.listingName,
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w500,
                    color: isDark
                        ? AppColors.neutralWhite
                        : AppColors.neutralGray900,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),

                const SizedBox(height: 10),

                // Price Row
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          'Harga Asli',
                          style: TextStyle(
                            fontSize: 11,
                            color: AppColors.neutralGray500,
                          ),
                        ),
                        Text(
                          _formatCurrency(offer.originalPrice),
                          style: TextStyle(
                            fontSize: 12,
                            color: AppColors.neutralGray500,
                            decoration: TextDecoration.lineThrough,
                          ),
                        ),
                      ],
                    ),
                    Column(
                      crossAxisAlignment: CrossAxisAlignment.end,
                      children: [
                        Text(
                          'Penawaran',
                          style: TextStyle(
                            fontSize: 11,
                            color: AppColors.neutralGray500,
                          ),
                        ),
                        Text(
                          _formatCurrency(offer.currentOfferPrice),
                          style: TextStyle(
                            fontSize: 16,
                            fontWeight: FontWeight.bold,
                            color: statusColor,
                          ),
                        ),
                      ],
                    ),
                  ],
                ),

                // Discount Badge
                if (offer.discountPercentage > 0) ...[
                  const SizedBox(height: 8),
                  Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 4,
                    ),
                    decoration: BoxDecoration(
                      color: AppColors.successGreen.withValues(alpha: 0.1),
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: Text(
                      'Diskon ${offer.discountPercentage.toInt()}%',
                      style: TextStyle(
                        fontSize: 11,
                        fontWeight: FontWeight.w500,
                        color: AppColors.successGreen,
                      ),
                    ),
                  ),
                ],

                // **EXECUTION WAVE CV1:** Next-step hint based on who can act
                if (canUserAct && isPending) ...[
                  const SizedBox(height: 8),
                  Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 4,
                    ),
                    decoration: BoxDecoration(
                      color: AppColors.coinPrimary.withValues(alpha: 0.1),
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          Icons.info_outline,
                          size: 10,
                          color: AppColors.coinPrimary,
                        ),
                        const SizedBox(width: 4),
                        Text(
                          'Barang belum dikunci sampai disetujui',
                          style: TextStyle(
                            fontSize: 9,
                            color: AppColors.coinPrimary,
                            fontWeight: FontWeight.w500,
                          ),
                        ),
                      ],
                    ),
                  ),
                ] else if (!canUserAct && isPending) ...[
                  const SizedBox(height: 8),
                  Text(
                    'Menunggu respons ${offer.lastOfferBy == 'buyer' ? 'penjual' : 'pembeli'}...',
                    style: TextStyle(
                      fontSize: 10,
                      color: AppColors.neutralGray500,
                      fontStyle: FontStyle.italic,
                    ),
                  ),
                ],

                // Last Offer By
                if (!canUserAct || !isPending) ...[
                  const SizedBox(height: 6),
                  Row(
                    children: [
                      Icon(
                        Icons.person_outline,
                        size: 11,
                        color: AppColors.neutralGray500,
                      ),
                      const SizedBox(width: 4),
                      Text(
                        'Terakhir dari ${offer.lastOfferBy == 'buyer' ? 'Pembeli' : 'Penjual'}',
                        style: TextStyle(
                          fontSize: 10,
                          color: AppColors.neutralGray500,
                        ),
                      ),
                    ],
                  ),
                ],

                // **EXECUTION WAVE CV1:** Action Buttons with clearer labels
                if (canUserAct && isPending) ...[
                  const SizedBox(height: 12),
                  Row(
                    children: [
                      Expanded(
                        child: OutlinedButton(
                          onPressed: onNegotiate,
                          style: OutlinedButton.styleFrom(
                            foregroundColor: AppColors.statusError,
                            side: const BorderSide(
                              color: AppColors.statusError,
                            ),
                            padding: const EdgeInsets.symmetric(
                              horizontal: 8,
                              vertical: 8,
                            ),
                          ),
                          child: const Text(
                            'Tolak',
                            style: TextStyle(
                              fontSize: 12,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ),
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        flex: 2,
                        child: ElevatedButton(
                          onPressed: onPurchase,
                          style: ElevatedButton.styleFrom(
                            backgroundColor: AppColors.successGreen,
                            foregroundColor: Colors.white,
                            padding: const EdgeInsets.symmetric(
                              horizontal: 8,
                              vertical: 8,
                            ),
                          ),
                          child: const Text(
                            'Setuju & Lanjut',
                            style: TextStyle(
                              fontSize: 12,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ),
                      ),
                    ],
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }

  /// Negotiation Result Attachment - Shows final negotiation result
  ///
  /// **EXECUTION WAVE CV1:** Enhanced with:
  /// - More prominent CTA for accepted negotiations
  /// - Urgency hint for checkout
  /// - Clearer next-step guidance
  /// Negotiation Proposal Attachment - Live backend proposal (initial / counter)
  ///
  /// Renders a minimal truthful card mirroring backend payload only.
  /// No accept/reject buttons (action wiring deferred until UX is finalized).
  Widget _buildNegotiationProposalAttachment(
    BuildContext context,
    NegotiationProposalAttachment proposal,
  ) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final statusColor = AppColors.coinPrimary;
    final headerLabel = proposal.isInitialProposal
        ? 'Penawaran Awal'
        : 'Penawaran Balasan • Ronde ${proposal.proposalSequence}';

    return Container(
      constraints: const BoxConstraints(maxWidth: 280),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: statusColor.withValues(alpha: 0.4),
          width: 1.5,
        ),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
            decoration: BoxDecoration(
              color: statusColor.withValues(alpha: 0.1),
              borderRadius: const BorderRadius.vertical(
                top: Radius.circular(11),
              ),
            ),
            child: Row(
              children: [
                Icon(Icons.handshake_outlined, size: 18, color: statusColor),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    headerLabel,
                    style: TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w600,
                      color: statusColor,
                    ),
                  ),
                ),
              ],
            ),
          ),
          Padding(
            padding: const EdgeInsets.all(12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Harga Penawaran',
                  style: TextStyle(
                    fontSize: 11,
                    color: AppColors.neutralGray500,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  _formatCurrency(proposal.price.toDouble()),
                  style: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.w700,
                    color: isDark
                        ? AppColors.neutralWhite
                        : AppColors.neutralGray900,
                  ),
                ),
                if (proposal.note != null && proposal.note!.isNotEmpty) ...[
                  const SizedBox(height: 8),
                  Text(
                    proposal.note!,
                    style: TextStyle(
                      fontSize: 12,
                      color: isDark
                          ? AppColors.neutralGray300
                          : AppColors.neutralGray700,
                    ),
                    maxLines: 3,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildNegotiationResultAttachment(
    BuildContext context,
    NegotiationResultAttachment result,
  ) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final isAccepted = result.status == 'accepted';
    final statusColor = isAccepted
        ? AppColors.successGreen
        : AppColors.statusError;
    final statusLabel = isAccepted
        ? 'Disetujui • Belum dipesan'
        : _getNegotiationStatusLabel(result.status);

    return Container(
      constraints: const BoxConstraints(maxWidth: 280),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: statusColor.withValues(alpha: 0.5),
          width: 1.5,
        ),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
            decoration: BoxDecoration(
              color: statusColor.withValues(alpha: 0.1),
              borderRadius: const BorderRadius.vertical(
                top: Radius.circular(11),
              ),
            ),
            child: Row(
              children: [
                Icon(
                  isAccepted ? Icons.check_circle : Icons.cancel,
                  size: 18,
                  color: statusColor,
                ),
                const SizedBox(width: 8),
                Text(
                  'Negosiasi $statusLabel',
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w600,
                    color: statusColor,
                  ),
                ),
              ],
            ),
          ),

          // Content
          Padding(
            padding: const EdgeInsets.all(12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Product Name
                Text(
                  result.listingName,
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w500,
                    color: isDark
                        ? AppColors.neutralWhite
                        : AppColors.neutralGray900,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),

                const SizedBox(height: 10),

                // Price Display
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Text(
                      'Harga Asli',
                      style: TextStyle(
                        fontSize: 12,
                        color: AppColors.neutralGray500,
                      ),
                    ),
                    Text(
                      _formatCurrency(result.originalPrice),
                      style: TextStyle(
                        fontSize: 12,
                        color: AppColors.neutralGray500,
                        decoration: result.agreedPrice != null
                            ? TextDecoration.lineThrough
                            : null,
                      ),
                    ),
                  ],
                ),

                if (result.agreedPrice != null) ...[
                  const SizedBox(height: 4),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text(
                        'Harga Disetujui',
                        style: TextStyle(
                          fontSize: 12,
                          fontWeight: FontWeight.w500,
                          color: isDark
                              ? AppColors.neutralWhite
                              : AppColors.neutralGray900,
                        ),
                      ),
                      Text(
                        _formatCurrency(result.agreedPrice!),
                        style: TextStyle(
                          fontSize: 15,
                          fontWeight: FontWeight.bold,
                          color: statusColor,
                        ),
                      ),
                    ],
                  ),
                  // Show savings if applicable
                  if (result.agreedPrice! < result.originalPrice) ...[
                    const SizedBox(height: 4),
                    Text(
                      'Hemat ${_formatCurrency(result.originalPrice - result.agreedPrice!)}',
                      style: TextStyle(
                        fontSize: 10,
                        color: AppColors.successGreen,
                        fontWeight: FontWeight.w500,
                      ),
                    ),
                  ],
                ],

                // Round Info
                const SizedBox(height: 8),
                Text(
                  'Selesai dalam ${result.totalRounds} ronde',
                  style: TextStyle(
                    fontSize: 11,
                    color: AppColors.neutralGray500,
                  ),
                ),

                // **EXECUTION WAVE CV1:** Enhanced Purchase Button with urgency
                if (isAccepted && result.canPurchase && onPurchase != null) ...[
                  const SizedBox(height: 12),
                  // **EXECUTION WAVE CV1:** Urgency hint
                  Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 8,
                      vertical: 6,
                    ),
                    margin: const EdgeInsets.only(bottom: 8),
                    decoration: BoxDecoration(
                      color: AppColors.primaryRed.withValues(alpha: 0.1),
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          Icons.access_time,
                          size: 11,
                          color: AppColors.primaryRed,
                        ),
                        const SizedBox(width: 4),
                        Text(
                          'Segera checkout sebelum barang terjual',
                          style: TextStyle(
                            fontSize: 10,
                            color: AppColors.primaryRed,
                            fontWeight: FontWeight.w500,
                          ),
                        ),
                      ],
                    ),
                  ),
                  SizedBox(
                    width: double.infinity,
                    child: ElevatedButton.icon(
                      onPressed: onPurchase,
                      icon: const Icon(Icons.point_of_sale, size: 16),
                      label: const Text('Lanjut ke Checkout'),
                      style: ElevatedButton.styleFrom(
                        backgroundColor: AppColors.primaryRed,
                        foregroundColor: Colors.white,
                        padding: const EdgeInsets.symmetric(
                          horizontal: 12,
                          vertical: 10,
                        ),
                      ),
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

  /// Shipping Quote Attachment - Custom shipping offer from seller
  ///
  /// **SHIPPING QUOTE FIX:** Added isActiveQuote parameter to show active/expired status.
  Widget _buildShippingQuoteAttachment(
    BuildContext context,
    ShippingQuoteAttachment shipping, {
    bool isActiveQuote = false,
  }) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final itemName = shipping.linkedItemName.trim().isEmpty
        ? 'Penawaran Ongkir'
        : shipping.linkedItemName;

    // UX TRUTH HARDENING: Use server-authoritative status instead of client calculation
    // Backend manages the real state - frontend just displays it
    final serverStatus = shipping.status.toUpperCase();

    // Status display logic based on server status:
    // - ACTIVE: "Penawaran Aktif" (green badge) - usable for checkout
    // - EXPIRED: "Kadaluarsa" (red badge) - past validity period
    // - USED: "Sudah digunakan" (gray badge) - already used in an order
    // - INVALID: "Item tidak tersedia" (orange badge) - listing unavailable
    String statusLabel;
    Color statusColor;
    Color statusBgColor;
    bool canInteract = false;

    switch (serverStatus) {
      case 'ACTIVE':
        statusLabel = 'Penawaran Aktif';
        statusColor = AppColors.successGreen;
        statusBgColor = AppColors.successGreen.withValues(alpha: 0.15);
        canInteract =
            isActiveQuote; // Only active quotes can be interacted with
        break;
      case 'EXPIRED':
        statusLabel = 'Kadaluarsa';
        statusColor = AppColors.statusError;
        statusBgColor = AppColors.statusError.withValues(alpha: 0.15);
        canInteract = false;
        break;
      case 'USED':
        statusLabel = 'Sudah digunakan';
        statusColor = AppColors.neutralGray500;
        statusBgColor = AppColors.neutralGray500.withValues(alpha: 0.15);
        canInteract = false;
        break;
      case 'INVALID':
        statusLabel = 'Item tidak tersedia';
        statusColor = AppColors.statusError; // Use error red for invalid
        statusBgColor = AppColors.statusError.withValues(alpha: 0.15);
        canInteract = false;
        break;
      default:
        statusLabel = 'Status Tidak Diketahui';
        statusColor = AppColors.neutralGray500;
        statusBgColor = AppColors.neutralGray500.withValues(alpha: 0.15);
        canInteract = false;
    }

    return Container(
      constraints: const BoxConstraints(maxWidth: 280),
      decoration: BoxDecoration(
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: canInteract
              ? AppColors.successGreen.withValues(alpha: 0.5)
              : AppColors.primaryRed.withValues(alpha: 0.3),
          width: 1.5,
        ),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header with status indicator
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
            decoration: BoxDecoration(
              color: statusBgColor,
              borderRadius: const BorderRadius.vertical(
                top: Radius.circular(11),
              ),
            ),
            child: Row(
              children: [
                Icon(
                  Icons.local_shipping_outlined,
                  size: 18,
                  color: statusColor,
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    'Penawaran Ongkir',
                    style: TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w600,
                      color: statusColor,
                    ),
                  ),
                ),
                // Status badge
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 6,
                    vertical: 2,
                  ),
                  decoration: BoxDecoration(
                    color: statusColor.withValues(alpha: 0.2),
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: Text(
                    statusLabel,
                    style: TextStyle(
                      fontSize: 10,
                      fontWeight: FontWeight.w600,
                      color: statusColor,
                    ),
                  ),
                ),
              ],
            ),
          ),

          // Content
          Padding(
            padding: const EdgeInsets.all(12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Item Name
                Text(
                  itemName,
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w500,
                    color: isDark
                        ? AppColors.neutralWhite
                        : AppColors.neutralGray900,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),

                const SizedBox(height: 10),

                // Shipping Option
                Row(
                  children: [
                    Text(
                      shipping.displayName,
                      style: TextStyle(
                        fontSize: 14,
                        fontWeight: FontWeight.w500,
                        color: isDark
                            ? AppColors.neutralWhite
                            : AppColors.neutralGray900,
                      ),
                    ),
                  ],
                ),

                const SizedBox(height: 8),

                // Shipping Rate
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Text(
                      'Biaya',
                      style: TextStyle(
                        fontSize: 12,
                        color: AppColors.neutralGray500,
                      ),
                    ),
                    Text(
                      _formatCurrency(shipping.rate),
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.bold,
                        color: AppColors.primaryRed,
                      ),
                    ),
                  ],
                ),

                // Estimated Days
                if (shipping.estimatedDays != null) ...[
                  const SizedBox(height: 4),
                  Text(
                    'Estimasi: ${shipping.estimatedDays}',
                    style: TextStyle(
                      fontSize: 11,
                      color: AppColors.neutralGray500,
                    ),
                  ),
                ],

                // Notes
                if (shipping.notes != null && shipping.notes!.isNotEmpty) ...[
                  const SizedBox(height: 8),
                  Container(
                    padding: const EdgeInsets.all(8),
                    decoration: BoxDecoration(
                      color: AppColors.neutralGray100,
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: Text(
                      shipping.notes!,
                      style: TextStyle(
                        fontSize: 11,
                        color: AppColors.neutralGray600,
                      ),
                    ),
                  ),
                ],

                // Validity
                const SizedBox(height: 8),
                Row(
                  children: [
                    Icon(
                      Icons.access_time,
                      size: 11,
                      color: AppColors.neutralGray500,
                    ),
                    const SizedBox(width: 4),
                    Text(
                      'Berlaku sampai ${_formatDate(shipping.validUntil)}',
                      style: TextStyle(
                        fontSize: 10,
                        color: AppColors.neutralGray500,
                      ),
                    ),
                  ],
                ),

                // Action Buttons - only enabled for active quotes
                if (onPurchase != null || onNegotiate != null) ...[
                  const SizedBox(height: 12),
                  Row(
                    children: [
                      if (onNegotiate != null) ...[
                        Expanded(
                          child: OutlinedButton(
                            onPressed: canInteract ? onNegotiate : null,
                            style: OutlinedButton.styleFrom(
                              padding: const EdgeInsets.symmetric(
                                horizontal: 8,
                                vertical: 6,
                              ),
                              foregroundColor: canInteract
                                  ? null
                                  : AppColors.neutralGray400,
                              side: BorderSide(
                                color: canInteract
                                    ? AppColors.statusError
                                    : AppColors.neutralGray300,
                              ),
                            ),
                            child: const Text(
                              'Tolak',
                              style: TextStyle(fontSize: 12),
                            ),
                          ),
                        ),
                        const SizedBox(width: 8),
                      ],
                      if (onPurchase != null)
                        Expanded(
                          child: ElevatedButton(
                            onPressed: canInteract ? onPurchase : null,
                            style: ElevatedButton.styleFrom(
                              backgroundColor: canInteract
                                  ? AppColors.primaryRed
                                  : AppColors.neutralGray300,
                              foregroundColor: Colors.white,
                              padding: const EdgeInsets.symmetric(
                                horizontal: 8,
                                vertical: 6,
                              ),
                            ),
                            child: Text(
                              canInteract ? 'Pilih' : 'Tidak Tersedia',
                              style: const TextStyle(fontSize: 12),
                            ),
                          ),
                        ),
                    ],
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }

  // =============================================================================
  // TRUE ATTACHMENTS - Local payload (not object references)
  // =============================================================================

  Widget _buildLocationAttachment(
    BuildContext context,
    LocationAttachment location,
  ) {
    return _buildPlaceholder(context, 'Location Attachment', Icons.location_on);
  }

  Widget _buildPlaceholder(BuildContext context, String label, IconData icon) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return Container(
      constraints: const BoxConstraints(maxWidth: 280),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isFromCurrentUser
            ? colorScheme.onPrimary.withValues(alpha: 0.1)
            : colorScheme.primary.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isFromCurrentUser
              ? colorScheme.onPrimary.withValues(alpha: 0.3)
              : colorScheme.primary.withValues(alpha: 0.3),
        ),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            icon,
            size: 48,
            color: isFromCurrentUser
                ? colorScheme.onPrimary.withValues(alpha: 0.6)
                : colorScheme.primary.withValues(alpha: 0.6),
          ),
          const SizedBox(height: 8),
          Text(
            label,
            style: theme.textTheme.labelMedium?.copyWith(
              color: isFromCurrentUser
                  ? colorScheme.onPrimary
                  : colorScheme.onSurface,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            'Coming soon...',
            style: theme.textTheme.bodySmall?.copyWith(
              color: isFromCurrentUser
                  ? colorScheme.onPrimary.withValues(alpha: 0.7)
                  : colorScheme.onSurfaceVariant,
              fontStyle: FontStyle.italic,
            ),
          ),
        ],
      ),
    );
  }

  // =============================================================================
  // HELPER METHODS
  // =============================================================================

  String _formatCurrency(double amount) {
    return 'Rp ${amount.toStringAsFixed(0).replaceAllMapped(RegExp(r'(\d{1,3})(?=(\d{3})+(?!\d))'), (Match m) => '${m[1]}.')}';
  }

  String _formatDate(DateTime date) {
    return '${date.day}/${date.month}/${date.year}';
  }

  // ============================================================================
  // NEGOTIATION HELPERS
  // ============================================================================

  Color _getNegotiationStatusColor(String status) {
    switch (status.toLowerCase()) {
      case 'pending':
      case 'countered':
        return AppColors.primaryRed;
      case 'accepted':
      case 'completed':
        return AppColors.successGreen;
      case 'rejected':
      case 'cancelled':
      case 'expired':
        return AppColors.statusError;
      default:
        return AppColors.neutralGray500;
    }
  }

  String _getNegotiationStatusLabel(String status) {
    switch (status.toLowerCase()) {
      case 'pending':
        return 'Menunggu';
      case 'countered':
        return 'Ditawar Balik';
      case 'accepted':
      case 'completed':
        return 'Disetujui • Belum dipesan';
      case 'rejected':
        return 'Ditolak';
      case 'cancelled':
        return 'Dibatalkan';
      case 'expired':
        return 'Kadaluarsa';
      default:
        return status;
    }
  }
}
