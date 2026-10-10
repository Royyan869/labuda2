import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/domain/entities/resource_projection.dart';
import 'package:hishumi/shared/shared.dart';

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
  final VoidCallback? onPurchase;
  final String? currentUserId;
  final String? contextId; // chatId, commentId, or contentId

  const AttachmentWidget({
    super.key,
    this.attachment,
    this.isFromCurrentUser = false,
    this.onTap,
    this.onPurchase,
    this.currentUserId,
    this.contextId,
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
      return _buildShippingQuoteAttachment(context, attachment);
    }

    // Fallback for unknown attachment types
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(AppShape.r8),
      ),
      child: Text(
        'Unsupported attachment: ${attachment.runtimeType}',
        style: context.typeRoles.labelMicro.copyWith(
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
        borderRadius: BorderRadius.circular(AppShape.r12),
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
            padding: const EdgeInsets.symmetric(
              horizontal: AppMetrics.p12,
              vertical: AppMetrics.p8,
            ),
            decoration: BoxDecoration(
              color: statusColor.withValues(alpha: 0.1),
              borderRadius: const BorderRadius.vertical(
                top: Radius.circular(AppShape.r11),
              ),
            ),
            child: Row(
              children: [
                Icon(
                  Icons.handshake_outlined,
                  size: AppIconSize.action,
                  color: statusColor,
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    headerLabel,
                    style: context.typeRoles.titleCompact.copyWith(
                      fontWeight: FontWeight.w600,
                      color: statusColor,
                    ),
                  ),
                ),
              ],
            ),
          ),
          Padding(
            padding: const EdgeInsets.all(AppMetrics.p12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Harga Penawaran',
                  style: context.typeRoles.labelMicro.copyWith(
                    color: scheme.onSurfaceVariant,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  _formatCurrency(proposal.price.toDouble()),
                  style: context.typeRoles.titleCompact.copyWith(
                    fontWeight: FontWeight.w700,
                    color: scheme.onSurface,
                  ),
                ),
                if (proposal.note != null && proposal.note!.isNotEmpty) ...[
                  const SizedBox(height: 8),
                  Text(
                    proposal.note!,
                    style: context.typeRoles.labelMicro.copyWith(
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
  /// Actionability is SERVER-OWNED: the buyer may act only when the Commerce
  /// projection marks this quote `viewerActionable` (current + buyer identity +
  /// not expired/used). The conversation renders the value and never recomputes
  /// quote lifecycle itself.
  Widget _buildShippingQuoteAttachment(
    BuildContext context,
    ShippingQuoteAttachment shipping,
  ) {
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

    switch (serverStatus) {
      case 'ACTIVE':
        statusLabel = 'Penawaran Aktif';
        statusColor = context.statusColors.success;
        statusBgColor = context.statusColors.success.withValues(alpha: 0.15);
        break;
      case 'EXPIRED':
        statusLabel = 'Kadaluarsa';
        statusColor = context.statusColors.error;
        statusBgColor = context.statusColors.error.withValues(alpha: 0.15);
        break;
      case 'USED':
        statusLabel = 'Sudah digunakan';
        statusColor = scheme.onSurfaceVariant;
        statusBgColor = scheme.onSurfaceVariant.withValues(alpha: 0.15);
        break;
      case 'INVALID':
        statusLabel = 'Item tidak tersedia';
        statusColor = context.statusColors.error; // Use error red for invalid
        statusBgColor = context.statusColors.error.withValues(alpha: 0.15);
        break;
      default:
        statusLabel = 'Status Tidak Diketahui';
        statusColor = scheme.onSurfaceVariant;
        statusBgColor = scheme.onSurfaceVariant.withValues(alpha: 0.15);
    }

    // SERVER AUTHORITY: only the Commerce projection decides whether the buyer
    // may act. Never derived from status, viewer id, or a client comparison.
    final canInteract = shipping.viewerActionable;

    return Container(
      constraints: const BoxConstraints(maxWidth: 280),
      decoration: BoxDecoration(
        color: scheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(
          color: canInteract
              ? context.statusColors.success.withValues(alpha: 0.5)
              : scheme.primary.withValues(alpha: 0.3),
          width: 1.5,
        ),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header with status indicator
          Container(
            padding: const EdgeInsets.symmetric(
              horizontal: AppMetrics.p12,
              vertical: AppMetrics.p8,
            ),
            decoration: BoxDecoration(
              color: statusBgColor,
              borderRadius: const BorderRadius.vertical(
                top: Radius.circular(AppShape.r11),
              ),
            ),
            child: Row(
              children: [
                Icon(
                  Icons.local_shipping_outlined,
                  size: AppIconSize.action,
                  color: statusColor,
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    'Penawaran Ongkir',
                    style: context.typeRoles.titleCompact.copyWith(
                      fontWeight: FontWeight.w600,
                      color: statusColor,
                    ),
                  ),
                ),
                // Status badge
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: AppMetrics.p8,
                    vertical: AppMetrics.p4,
                  ),
                  decoration: BoxDecoration(
                    color: statusColor.withValues(alpha: 0.2),
                    borderRadius: BorderRadius.circular(AppShape.r10),
                  ),
                  child: Text(
                    statusLabel,
                    style: context.typeRoles.labelMicro.copyWith(
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
            padding: const EdgeInsets.all(AppMetrics.p12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // Item Name
                Text(
                  itemName,
                  style: context.typeRoles.bodyDense.copyWith(
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
                      style: context.typeRoles.bodyDense.copyWith(
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
                      style: context.typeRoles.labelMicro.copyWith(
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                    Text(
                      _formatCurrency(shipping.rate),
                      style: context.typeRoles.titleCompact.copyWith(
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
                    padding: const EdgeInsets.all(AppMetrics.p8),
                    decoration: BoxDecoration(
                      color: scheme.surfaceContainerHigh,
                      borderRadius: BorderRadius.circular(AppShape.r6),
                    ),
                    child: Text(
                      shipping.notes!,
                      style: context.typeRoles.labelMicro.copyWith(
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
                      size: AppIconSize.inlineGlyph,
                      color: scheme.onSurfaceVariant,
                    ),
                    const SizedBox(width: 4),
                    Text(
                      'Berlaku sampai ${AppFormatters.formatDate(shipping.validUntil)}',
                      style: context.typeRoles.labelMicro.copyWith(
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                  ],
                ),

                // Single CTA: continue toward checkout WITH this quote. The
                // conversation only navigates; Commerce validates the quote at
                // checkout and marks it USED on order creation. There is no
                // buyer "reject" transition — an unused quote simply expires.
                if (onPurchase != null) ...[
                  const SizedBox(height: 12),
                  Row(
                    children: [
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
                              horizontal: AppMetrics.p8,
                              vertical: AppMetrics.p8,
                            ),
                          ),
                          child: Text(
                            canInteract
                                ? 'Gunakan Ongkir'
                                : 'Tidak Tersedia',
                            style: Theme.of(context).textTheme.labelLarge,
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
      padding: const EdgeInsets.all(AppMetrics.p16),
      decoration: BoxDecoration(
        color: isFromCurrentUser
            ? colorScheme.onPrimary.withValues(alpha: 0.1)
            : colorScheme.primary.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(AppShape.r12),
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
            size: AppIconSize.display,
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
}
