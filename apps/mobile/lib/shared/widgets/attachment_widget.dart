import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/shared/shared.dart';

/// Universal Attachment Widget — workflow payload renderer
///
/// Routes a workflow attachment (negotiation proposal, shipping quote,
/// location) to its domain-specific card.
///
/// NOT a resource renderer: a shared resource reference is displayed by the
/// server-resolved projection card (authority) or, when the row carries no
/// projection, by the transport-snapshot shell in the communication surface.
/// There is deliberately no reference branch here — a second rendering path
/// for the same resource is how the client grew a parallel truth.
class AttachmentWidget extends ConsumerWidget {
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
    final attachment = this.attachment;
    if (attachment == null) return const SizedBox.shrink();
    return _buildAttachment(context, attachment);
  }

  /// Build attachment widgets
  Widget _buildAttachment(BuildContext context, Attachment attachment) {
    // Workflow Payloads → Keep custom rendering (domain-specific logic)
    if (attachment is LocationAttachment) {
      return _buildLocationAttachment(context, attachment);
    } else if (attachment is NegotiationProposalAttachment) {
      return _buildNegotiationProposalAttachment(context, attachment);
    } else if (attachment is ShippingQuoteAttachment) {
      return _buildShippingQuoteAttachment(
        context,
        attachment,
        isActiveQuote: attachment.offerId == activeQuoteOfferId,
      );
    }

    // Fallback for unknown attachment types
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Text(
        'Unsupported attachment: ${attachment.runtimeType}',
        style: TextStyle(
          fontSize: 12,
          fontStyle: FontStyle.italic,
          color: scheme.onSurfaceVariant,
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

  /// Negotiation Proposal Attachment - Live backend proposal (initial / counter)
  ///
  /// Renders a minimal truthful card mirroring backend payload only.
  /// No accept/reject buttons (action wiring deferred until UX is finalized).
  Widget _buildNegotiationProposalAttachment(
    BuildContext context,
    NegotiationProposalAttachment proposal,
  ) {
    final scheme = Theme.of(context).colorScheme;
    final statusColor = AppColors.coinPrimary;
    final headerLabel = proposal.isInitialProposal
        ? 'Penawaran Awal'
        : 'Penawaran Balasan • Ronde ${proposal.proposalSequence}';

    return Container(
      constraints: const BoxConstraints(maxWidth: 280),
      decoration: BoxDecoration(
        color: scheme.surface,
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
                    color: scheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  _formatCurrency(proposal.price.toDouble()),
                  style: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.w700,
                    color: scheme.onSurface,
                  ),
                ),
                if (proposal.note != null && proposal.note!.isNotEmpty) ...[
                  const SizedBox(height: 8),
                  Text(
                    proposal.note!,
                    style: TextStyle(
                      fontSize: 12,
                      color: scheme.onSurface,
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

  /// Shipping Quote Attachment - Custom shipping offer from seller
  ///
  /// **SHIPPING QUOTE FIX:** Added isActiveQuote parameter to show active/expired status.
  Widget _buildShippingQuoteAttachment(
    BuildContext context,
    ShippingQuoteAttachment shipping, {
    bool isActiveQuote = false,
  }) {
    final scheme = Theme.of(context).colorScheme;
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
    // - INVALID: "Item tidak tersedia" (orange badge) - forSale unavailable
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
        statusColor = scheme.onSurfaceVariant;
        statusBgColor = scheme.onSurfaceVariant.withValues(alpha: 0.15);
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
        statusColor = scheme.onSurfaceVariant;
        statusBgColor = scheme.onSurfaceVariant.withValues(alpha: 0.15);
        canInteract = false;
    }

    return Container(
      constraints: const BoxConstraints(maxWidth: 280),
      decoration: BoxDecoration(
        color: scheme.surface,
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
                    color: scheme.onSurface,
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
                        color: scheme.onSurface,
                      ),
                    ),
                  ],
                ),

                const SizedBox(height: 8),

                // Shipping Rate — ALL-IN (ongkir + packing) per business truth
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Text(
                      'Ongkir + Packing',
                      style: TextStyle(
                        fontSize: 12,
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                    Text(
                      _formatCurrency(shipping.rate),
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.bold,
                        color: scheme.primary,
                      ),
                    ),
                  ],
                ),

                // Notes
                if (shipping.notes != null && shipping.notes!.isNotEmpty) ...[
                  const SizedBox(height: 8),
                  Container(
                    padding: const EdgeInsets.all(8),
                    decoration: BoxDecoration(
                      color: scheme.surfaceContainerHigh,
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: Text(
                      shipping.notes!,
                      style: TextStyle(
                        fontSize: 11,
                        color: scheme.onSurfaceVariant,
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
                      color: scheme.onSurfaceVariant,
                    ),
                    const SizedBox(width: 4),
                    Text(
                      'Berlaku sampai ${_formatDate(shipping.validUntil)}',
                      style: TextStyle(
                        fontSize: 10,
                        color: scheme.onSurfaceVariant,
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
                                  : scheme.onSurfaceVariant,
                              side: BorderSide(
                                color: canInteract
                                    ? scheme.error
                                    : scheme.outlineVariant,
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
                                  ? scheme.primary
                                  : scheme.surfaceContainerHighest,
                              foregroundColor: canInteract
                                  ? scheme.onPrimary
                                  : scheme.onSurfaceVariant,
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

  /// Money is grouped by the single formatting authority — a workflow payload
  /// never grows its own thousand-separator implementation.
  String _formatCurrency(double amount) {
    return 'Rp ${formatGroupedAmount(amount.round())}';
  }

  String _formatDate(DateTime date) {
    return '${date.day}/${date.month}/${date.year}';
  }

}
