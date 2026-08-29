import 'package:flutter/material.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/domains/commerce/catalog/shared/domain/entities/commerce_viewer_capabilities.dart';
import 'package:labuda/shared/governance/seller_tier_badge.dart';
import 'package:labuda/shared/models/seller_identity_data.dart';
import 'package:labuda/shared/widgets/popup_more_options_button.dart';
import 'package:labuda/shared/widgets/media_viewer_widget.dart';
import 'package:labuda/shared/widgets/seller_identity_view.dart';

enum CommerceDetailShellState { loading, error, notFound, content }

enum CommerceDetailValueLayout { auto, horizontal, vertical }

class CommerceDetailScaffold extends StatelessWidget {
  final PreferredSizeWidget? appBar;
  final Widget body;
  final Widget? bottomNavigationBar;
  final bool extendBody;
  final Color? backgroundColor;

  const CommerceDetailScaffold({
    super.key,
    this.appBar,
    required this.body,
    this.bottomNavigationBar,
    this.extendBody = false,
    this.backgroundColor,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      backgroundColor: backgroundColor ?? theme.colorScheme.surface,
      appBar: appBar,
      extendBody: extendBody,
      body: body,
      bottomNavigationBar: bottomNavigationBar,
    );
  }
}

class CommerceDetailShell extends StatelessWidget {
  final PreferredSizeWidget? appBar;
  final CommerceDetailShellState state;
  final String? title;
  final String? primaryValue;
  final Widget? media;
  final Widget? sellerIdentity;
  final Widget? statusBadgeArea;
  final List<Widget> domainSections;
  final List<Widget> supportingSections;
  final Widget? recommendationsSection;
  final Widget? bottomNavigationBar;
  final Future<void> Function()? onRefresh;
  final Widget? loadingBuilder;
  final Widget? errorBuilder;
  final WidgetBuilder? notFoundBuilder;
  final Color? backgroundColor;
  final bool extendBody;
  final EdgeInsetsGeometry horizontalPadding;
  final double sectionGap;
  final double bottomScrollPadding;

  const CommerceDetailShell({
    super.key,
    this.appBar,
    required this.state,
    this.title,
    this.primaryValue,
    this.media,
    this.sellerIdentity,
    this.statusBadgeArea,
    this.domainSections = const <Widget>[],
    this.supportingSections = const <Widget>[],
    this.recommendationsSection,
    this.bottomNavigationBar,
    this.onRefresh,
    this.loadingBuilder,
    this.errorBuilder,
    this.notFoundBuilder,
    this.backgroundColor,
    this.extendBody = false,
    this.horizontalPadding = const EdgeInsets.symmetric(horizontal: 16),
    this.sectionGap = 16,
    this.bottomScrollPadding = 112,
  });

  @override
  Widget build(BuildContext context) {
    return CommerceDetailScaffold(
      appBar: appBar,
      backgroundColor: backgroundColor,
      extendBody: extendBody,
      bottomNavigationBar: bottomNavigationBar,
      body: _buildBody(context),
    );
  }

  Widget _buildBody(BuildContext context) {
    switch (state) {
      case CommerceDetailShellState.loading:
        return loadingBuilder ??
            const Center(child: CircularProgressIndicator());
      case CommerceDetailShellState.error:
        return errorBuilder ??
            const Center(child: Text('Data belum bisa dimuat.'));
      case CommerceDetailShellState.notFound:
        return notFoundBuilder?.call(context) ??
            const Center(child: Text('Data tidak ditemukan'));
      case CommerceDetailShellState.content:
        final resolvedHorizontalPadding = horizontalPadding.resolve(
          Directionality.of(context),
        );
        final scrollable = SafeArea(
          top: false,
          bottom: false,
          child: RefreshIndicator(
            onRefresh: onRefresh ?? () async {},
            child: SingleChildScrollView(
              physics: onRefresh != null
                  ? const AlwaysScrollableScrollPhysics()
                  : const ClampingScrollPhysics(),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  if (media != null) ...[media!, SizedBox(height: sectionGap)],
                  Padding(
                    padding: EdgeInsets.fromLTRB(
                      resolvedHorizontalPadding.left,
                      0,
                      resolvedHorizontalPadding.right,
                      bottomNavigationBar == null
                          ? 24
                          : bottomScrollPadding +
                                MediaQuery.of(context).padding.bottom,
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: _buildContentChildren(context),
                    ),
                  ),
                ],
              ),
            ),
          ),
        );

        if (onRefresh == null) {
          return scrollable;
        }

        return scrollable;
    }
  }

  List<Widget> _buildContentChildren(BuildContext context) {
    final children = <Widget>[];
    final theme = Theme.of(context);
    if (statusBadgeArea != null) {
      children
        ..add(statusBadgeArea!)
        ..add(SizedBox(height: sectionGap));
    }

    if (title != null) {
      children.add(
        Text(
          title!,
          style: theme.textTheme.headlineSmall?.copyWith(
            fontWeight: FontWeight.w700,
            color: theme.colorScheme.onSurface,
          ),
          softWrap: true,
        ),
      );
    }

    if (primaryValue != null) {
      if (children.isNotEmpty) {
        children.add(const SizedBox(height: 8));
      }
      children.add(
        Text(
          primaryValue!,
          style: theme.textTheme.headlineMedium?.copyWith(
            color: theme.colorScheme.primary,
            fontWeight: FontWeight.w800,
          ),
          softWrap: true,
        ),
      );
    }

    if (sellerIdentity != null) {
      if (children.isNotEmpty) {
        children.add(SizedBox(height: sectionGap));
      }
      children.add(sellerIdentity!);
    }

    if (domainSections.isNotEmpty) {
      if (children.isNotEmpty) {
        children.add(SizedBox(height: sectionGap));
      }
      children.addAll(_withSectionGaps(domainSections));
    }

    if (supportingSections.isNotEmpty) {
      if (children.isNotEmpty) {
        children.add(SizedBox(height: sectionGap));
      }
      children.addAll(_withSectionGaps(supportingSections));
    }

    if (recommendationsSection != null) {
      if (children.isNotEmpty) {
        children.add(SizedBox(height: sectionGap));
      }
      children.add(recommendationsSection!);
    }

    if (children.isEmpty) {
      children.add(const SizedBox.shrink());
    }

    return children;
  }

  List<Widget> _withSectionGaps(List<Widget> widgets) {
    if (widgets.isEmpty) {
      return const <Widget>[];
    }

    final children = <Widget>[];
    for (var i = 0; i < widgets.length; i++) {
      children.add(widgets[i]);
      if (i != widgets.length - 1) {
        children.add(SizedBox(height: sectionGap));
      }
    }
    return children;
  }
}

class CommerceDetailSectionCard extends StatelessWidget {
  final Widget child;
  final EdgeInsetsGeometry padding;
  final EdgeInsetsGeometry margin;
  final BorderRadiusGeometry borderRadius;
  final Color? backgroundColor;
  final BorderSide? borderSide;

  const CommerceDetailSectionCard({
    super.key,
    required this.child,
    this.padding = const EdgeInsets.all(16),
    this.margin = EdgeInsets.zero,
    this.borderRadius = const BorderRadius.all(Radius.circular(16)),
    this.backgroundColor,
    this.borderSide,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Container(
      margin: margin,
      padding: padding,
      decoration: BoxDecoration(
        color: backgroundColor ?? theme.colorScheme.surfaceContainerHighest,
        borderRadius: borderRadius,
        border: Border.all(
          color: borderSide?.color ?? theme.colorScheme.outlineVariant,
          width: borderSide?.width ?? 1,
        ),
      ),
      child: child,
    );
  }
}

class CommerceDetailAppBarActionButton extends StatelessWidget {
  final IconData icon;
  final IconData? activeIcon;
  final bool isActive;
  final bool isLoading;
  final String tooltip;
  final String semanticsLabel;
  final String? loadingTooltip;
  final String? loadingSemanticsLabel;
  final VoidCallback? onPressed;
  final Color? activeColor;
  final Color? inactiveColor;
  final Color? disabledColor;

  const CommerceDetailAppBarActionButton({
    super.key,
    required this.icon,
    required this.tooltip,
    required this.semanticsLabel,
    required this.onPressed,
    this.activeIcon,
    this.isActive = false,
    this.isLoading = false,
    this.loadingTooltip,
    this.loadingSemanticsLabel,
    this.activeColor,
    this.inactiveColor,
    this.disabledColor,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final enabled = onPressed != null && !isLoading;
    final resolvedColor = !enabled
        ? (disabledColor ??
              theme.colorScheme.onSurfaceVariant.withValues(alpha: 0.38))
        : isActive
        ? (activeColor ?? theme.colorScheme.primary)
        : (inactiveColor ?? theme.colorScheme.onSurfaceVariant);
    final effectiveTooltip = isLoading ? (loadingTooltip ?? tooltip) : tooltip;
    final effectiveSemanticsLabel = isLoading
        ? (loadingSemanticsLabel ?? semanticsLabel)
        : semanticsLabel;
    final resolvedIcon = isActive && activeIcon != null ? activeIcon! : icon;

    final iconWidget = isLoading
        ? SizedBox.square(
            dimension: 20,
            child: CircularProgressIndicator(
              strokeWidth: 2,
              valueColor: AlwaysStoppedAnimation<Color>(resolvedColor),
            ),
          )
        : Icon(resolvedIcon, size: 20, color: resolvedColor);

    return Semantics(
      button: true,
      selected: isActive,
      enabled: enabled,
      label: effectiveSemanticsLabel,
      child: Tooltip(
        message: effectiveTooltip,
        child: Material(
          type: MaterialType.transparency,
          child: InkResponse(
            onTap: enabled ? onPressed : null,
            containedInkWell: true,
            customBorder: const CircleBorder(),
            highlightShape: BoxShape.circle,
            radius: 24,
            child: SizedBox(
              width: 48,
              height: 48,
              child: Center(child: iconWidget),
            ),
          ),
        ),
      ),
    );
  }
}

class CommerceDetailAppBarActions extends StatelessWidget {
  final List<Widget> actions;

  const CommerceDetailAppBarActions({
    super.key,
    this.actions = const <Widget>[],
  });

  @override
  Widget build(BuildContext context) {
    if (actions.isEmpty) {
      return const SizedBox.shrink();
    }

    return Row(mainAxisSize: MainAxisSize.min, children: actions);
  }
}

class CommerceDetailMoreOptionsButton extends StatelessWidget {
  final bool isAuthor;
  final PopupMoreOptionsContentType contentType;
  final VoidCallback? onReport;
  final VoidCallback? onEdit;
  final VoidCallback? onCancel;
  final VoidCallback? onDelete;
  final VoidCallback? onShare;
  final VoidCallback? onBlock;
  final bool isDeleting;
  final Color? iconColor;

  const CommerceDetailMoreOptionsButton({
    super.key,
    required this.isAuthor,
    required this.contentType,
    this.onReport,
    this.onEdit,
    this.onCancel,
    this.onDelete,
    this.onShare,
    this.onBlock,
    this.isDeleting = false,
    this.iconColor,
  });

  @override
  Widget build(BuildContext context) {
    return Builder(
      builder: (buttonContext) {
        return CommerceDetailAppBarActionButton(
          icon: Icons.more_vert,
          tooltip: 'Lainnya',
          semanticsLabel: 'Lainnya',
          onPressed: isDeleting
              ? null
              : () {
                  _showMenu(buttonContext);
                },
          inactiveColor:
              iconColor ?? Theme.of(buttonContext).colorScheme.onSurfaceVariant,
          disabledColor:
              iconColor?.withValues(alpha: 0.38) ??
              Theme.of(
                buttonContext,
              ).colorScheme.onSurfaceVariant.withValues(alpha: 0.38),
        );
      },
    );
  }

  Future<void> _showMenu(BuildContext context) async {
    final overlay = Overlay.of(context).context.findRenderObject() as RenderBox;
    final box = context.findRenderObject() as RenderBox;
    final position = RelativeRect.fromRect(
      Rect.fromPoints(
        box.localToGlobal(Offset.zero, ancestor: overlay),
        box.localToGlobal(box.size.bottomRight(Offset.zero), ancestor: overlay),
      ),
      Offset.zero & overlay.size,
    );

    final selected = await showMenu<String>(
      context: context,
      position: position,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
      elevation: 8,
      items: _buildMenuItems(context),
    );

    if (selected != null) {
      _handleMenuSelection(selected);
    }
  }

  List<PopupMenuEntry<String>> _buildMenuItems(BuildContext context) {
    final items = <PopupMenuEntry<String>>[];
    final theme = Theme.of(context);

    if (!isAuthor) {
      final reportLabel = switch (contentType) {
        PopupMoreOptionsContentType.listing ||
        PopupMoreOptionsContentType.auction => 'Laporkan produk',
        PopupMoreOptionsContentType.profile => 'Report User',
        _ => 'Report',
      };

      items.add(
        PopupMenuItem<String>(
          value: 'report',
          child: Row(
            children: [
              const Icon(Icons.report_outlined, size: 20),
              const SizedBox(width: 12),
              Text(reportLabel),
            ],
          ),
        ),
      );
    }

    if (isAuthor && onEdit != null) {
      items.add(
        PopupMenuItem<String>(
          value: 'edit',
          child: Row(
            children: [
              const Icon(Icons.edit_outlined, size: 20),
              const SizedBox(width: 12),
              const Text('Edit'),
            ],
          ),
        ),
      );
    }

    if (isAuthor && onDelete != null) {
      items.add(
        PopupMenuItem<String>(
          value: 'delete',
          enabled: !isDeleting,
          child: Row(
            children: [
              isDeleting
                  ? const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : Icon(
                      Icons.delete_outline,
                      size: 20,
                      color: theme.colorScheme.error,
                    ),
              const SizedBox(width: 12),
              Text(
                isDeleting ? 'Deleting...' : 'Delete',
                style: TextStyle(color: theme.colorScheme.error),
              ),
            ],
          ),
        ),
      );
    }

    if (isAuthor &&
        contentType == PopupMoreOptionsContentType.auction &&
        onCancel != null) {
      items.add(
        PopupMenuItem<String>(
          value: 'cancel',
          child: Row(
            children: [
              Icon(
                Icons.cancel_outlined,
                size: 20,
                color: theme.colorScheme.tertiary,
              ),
              const SizedBox(width: 12),
              Text(
                'Cancel Auction',
                style: TextStyle(color: theme.colorScheme.tertiary),
              ),
            ],
          ),
        ),
      );
    }

    return items;
  }

  void _handleMenuSelection(String value) {
    switch (value) {
      case 'report':
        onReport?.call();
        break;
      case 'edit':
        onEdit?.call();
        break;
      case 'delete':
        onDelete?.call();
        break;
      case 'cancel':
        onCancel?.call();
        break;
    }
  }
}

class CommerceDetailActionBarLayout extends StatelessWidget {
  final List<Widget> secondaryActions;
  final String primaryLabel;
  final VoidCallback? onPrimaryPressed;
  final bool showPrimary;
  final bool isPrimaryLoading;
  final Color? primaryBackgroundColor;
  final Color? primaryForegroundColor;
  final Color? primaryDisabledBackgroundColor;
  final EdgeInsetsGeometry padding;
  final double gap;
  final double primaryHeight;

  const CommerceDetailActionBarLayout({
    super.key,
    this.secondaryActions = const [],
    required this.primaryLabel,
    required this.onPrimaryPressed,
    this.showPrimary = true,
    this.isPrimaryLoading = false,
    this.primaryBackgroundColor,
    this.primaryForegroundColor,
    this.primaryDisabledBackgroundColor,
    this.padding = const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
    this.gap = 12,
    this.primaryHeight = 48,
  });

  @override
  Widget build(BuildContext context) {
    if (!showPrimary && secondaryActions.isEmpty) {
      return const SizedBox.shrink();
    }

    final theme = Theme.of(context);
    final actions = <Widget>[];

    if (secondaryActions.isNotEmpty) {
      for (var i = 0; i < secondaryActions.length; i++) {
        actions.add(secondaryActions[i]);
        if (i != secondaryActions.length - 1) {
          actions.add(SizedBox(width: gap));
        }
      }
    }

    if (showPrimary) {
      if (actions.isNotEmpty) {
        actions.add(SizedBox(width: gap));
      }
      actions.add(
        Expanded(
          child: SizedBox(
            height: primaryHeight,
            child: ElevatedButton(
              onPressed: isPrimaryLoading ? null : onPrimaryPressed,
              style: ElevatedButton.styleFrom(
                backgroundColor:
                    primaryBackgroundColor ?? theme.colorScheme.primary,
                foregroundColor:
                    primaryForegroundColor ?? theme.colorScheme.onPrimary,
                disabledBackgroundColor:
                    primaryDisabledBackgroundColor ??
                    theme.colorScheme.surfaceContainerHigh,
                disabledForegroundColor: theme.colorScheme.onSurfaceVariant,
                padding: const EdgeInsets.symmetric(horizontal: 16),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(8),
                ),
              ),
              child: isPrimaryLoading
                  ? SizedBox(
                      height: 18,
                      width: 18,
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        valueColor: AlwaysStoppedAnimation<Color>(
                          theme.colorScheme.onPrimary,
                        ),
                      ),
                    )
                  : Text(
                      primaryLabel,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(fontWeight: FontWeight.bold),
                    ),
            ),
          ),
        ),
      );
    }

    return CommerceDetailStickyActionBar(
      padding: padding,
      child: Row(children: actions),
    );
  }
}

class CommerceDetailLabelValue extends StatelessWidget {
  final String label;
  final String value;
  final CommerceDetailValueLayout layout;
  final EdgeInsetsGeometry padding;
  final TextStyle? labelStyle;
  final TextStyle? valueStyle;

  const CommerceDetailLabelValue({
    super.key,
    required this.label,
    required this.value,
    this.layout = CommerceDetailValueLayout.auto,
    this.padding = const EdgeInsets.only(bottom: 8),
    this.labelStyle,
    this.valueStyle,
  });

  bool get _shouldStack {
    if (layout == CommerceDetailValueLayout.vertical) {
      return true;
    }
    if (layout == CommerceDetailValueLayout.horizontal) {
      return false;
    }

    final normalizedLabel = label.trim().toLowerCase();
    const stackedLabels = <String>{
      'origin',
      'opsi pengiriman',
      'breeder',
      'bloodline',
      'sertifikat',
      'certificate',
      'certificates',
    };

    if (stackedLabels.contains(normalizedLabel)) {
      return true;
    }

    if (value.contains('\n')) {
      return true;
    }

    if (value.length >= 32) {
      return true;
    }

    if (value.contains(',') && value.length >= 24) {
      return true;
    }

    return false;
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final resolvedLabelStyle =
        labelStyle ??
        theme.textTheme.bodySmall?.copyWith(
          color: theme.colorScheme.onSurfaceVariant,
        );
    final resolvedValueStyle =
        valueStyle ??
        theme.textTheme.bodyMedium?.copyWith(
          fontWeight: FontWeight.w600,
          color: theme.colorScheme.onSurface,
        );

    return Padding(
      padding: padding,
      child: _shouldStack
          ? Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(label, style: resolvedLabelStyle, softWrap: true),
                const SizedBox(height: 4),
                Text(value, style: resolvedValueStyle, softWrap: true),
              ],
            )
          : Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(
                  flex: 4,
                  child: Text(label, style: resolvedLabelStyle, softWrap: true),
                ),
                const SizedBox(width: 12),
                Expanded(
                  flex: 6,
                  child: Text(
                    value,
                    style: resolvedValueStyle,
                    textAlign: TextAlign.end,
                    softWrap: true,
                  ),
                ),
              ],
            ),
    );
  }
}

class CommerceDetailStickyActionBar extends StatelessWidget {
  final Widget child;
  final EdgeInsetsGeometry padding;
  final Color? backgroundColor;
  final BorderSide? borderSide;
  final List<BoxShadow>? boxShadow;

  const CommerceDetailStickyActionBar({
    super.key,
    required this.child,
    this.padding = const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
    this.backgroundColor,
    this.borderSide,
    this.boxShadow,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Container(
      decoration: BoxDecoration(
        color: backgroundColor ?? theme.colorScheme.surface,
        border: Border(
          top:
              borderSide ?? BorderSide(color: theme.colorScheme.outlineVariant),
        ),
        boxShadow:
            boxShadow ??
            [
              BoxShadow(
                color: theme.colorScheme.shadow.withValues(alpha: 0.08),
                blurRadius: 10,
                offset: const Offset(0, -2),
              ),
            ],
      ),
      padding: padding,
      child: SafeArea(top: false, child: child),
    );
  }
}

class CommerceDetailActionToggle extends StatelessWidget {
  final IconData icon;
  final IconData activeIcon;
  final String label;
  final String activeLabel;
  final bool isActive;
  final bool isLoading;
  final bool showLabel;
  final VoidCallback? onTap;
  final Color? activeColor;
  final Color? inactiveColor;

  const CommerceDetailActionToggle({
    super.key,
    required this.icon,
    required this.activeIcon,
    required this.label,
    required this.activeLabel,
    required this.isActive,
    this.isLoading = false,
    this.showLabel = true,
    this.onTap,
    this.activeColor,
    this.inactiveColor,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final resolvedColor = isActive
        ? (activeColor ?? theme.colorScheme.primary)
        : (inactiveColor ?? theme.colorScheme.onSurfaceVariant);
    final effectiveLabel = isActive ? activeLabel : label;

    return Semantics(
      button: true,
      selected: isActive,
      label: effectiveLabel,
      child: Tooltip(
        message: effectiveLabel,
        child: InkWell(
          onTap: isLoading ? null : onTap,
          borderRadius: BorderRadius.circular(12),
          child: ConstrainedBox(
            constraints: const BoxConstraints(minWidth: 44, minHeight: 44),
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 4),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  SizedBox(
                    height: 20,
                    width: 20,
                    child: isLoading
                        ? CircularProgressIndicator(
                            strokeWidth: 2,
                            valueColor: AlwaysStoppedAnimation<Color>(
                              theme.colorScheme.primary,
                            ),
                          )
                        : Icon(
                            isActive ? activeIcon : icon,
                            color: resolvedColor,
                            size: 20,
                          ),
                  ),
                  if (showLabel) ...[
                    const SizedBox(height: 4),
                    Text(
                      effectiveLabel,
                      style: theme.textTheme.labelSmall?.copyWith(
                        color: resolvedColor,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ],
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class CommerceDetailRoleAwareActionBar extends StatelessWidget {
  final CommerceViewerCapabilities capabilities;
  final Widget? ownerChild;
  final Widget? buyerChild;
  final Widget? guestChild;
  final Widget? fallbackChild;
  final EdgeInsetsGeometry padding;
  final Color? backgroundColor;
  final BorderSide? borderSide;
  final List<BoxShadow>? boxShadow;

  const CommerceDetailRoleAwareActionBar({
    super.key,
    required this.capabilities,
    this.ownerChild,
    this.buyerChild,
    this.guestChild,
    this.fallbackChild,
    this.padding = const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
    this.backgroundColor,
    this.borderSide,
    this.boxShadow,
  });

  @override
  Widget build(BuildContext context) {
    final child = switch (capabilities.role) {
      'owner' => ownerChild ?? fallbackChild,
      'buyer' => buyerChild ?? fallbackChild,
      _ => guestChild ?? fallbackChild,
    };

    if (child == null) {
      return const SizedBox.shrink();
    }

    return CommerceDetailStickyActionBar(
      padding: padding,
      backgroundColor: backgroundColor,
      borderSide: borderSide,
      boxShadow: boxShadow,
      child: child,
    );
  }
}

class CommerceDetailMediaGallery extends StatelessWidget {
  final String cacheKey;
  final List<MediaEntity> media;
  final String Function(MediaEntity media, int index) logicalCacheKeyBuilder;
  final double height;
  final BoxFit fit;
  final Alignment alignment;
  final Widget fallback;
  final BorderRadius? borderRadius;
  final Color? fallbackColor;

  const CommerceDetailMediaGallery({
    super.key,
    required this.cacheKey,
    required this.media,
    required this.logicalCacheKeyBuilder,
    this.height = 300,
    this.fit = BoxFit.cover,
    this.alignment = Alignment.center,
    this.fallback = const Center(child: Icon(Icons.image_not_supported)),
    this.borderRadius,
    this.fallbackColor,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final logicalCacheFingerprint = media
        .asMap()
        .entries
        .map((entry) => logicalCacheKeyBuilder(entry.value, entry.key))
        .join('|');

    final gallery = media.isNotEmpty
        ? SizedBox(
            height: height,
            child: Container(
              color: fallbackColor ?? theme.colorScheme.surfaceContainerHighest,
              child: MediaViewerWidget(
                key: ValueKey(
                  'commerce-detail-media:$cacheKey:$logicalCacheFingerprint',
                ),
                media: media,
              ),
            ),
          )
        : SizedBox(
            height: height,
            child: Container(
              color: fallbackColor ?? theme.colorScheme.surfaceContainerHighest,
              child: Center(
                child: Icon(
                  Icons.image_outlined,
                  size: 64,
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
            ),
          );

    if (borderRadius == null) {
      return gallery;
    }

    return ClipRRect(borderRadius: borderRadius!, child: gallery);
  }
}

class CommerceDetailSellerIdentityCard extends StatelessWidget {
  final SellerIdentityData? identity;
  final bool isDegraded;
  final String redactionLabel;
  final String? sellerTier;
  final VoidCallback? onTap;
  final double avatarSize;
  final EdgeInsetsGeometry padding;
  final BorderRadiusGeometry borderRadius;
  final bool showTierBadge;
  final EdgeInsetsGeometry tierBadgePadding;

  const CommerceDetailSellerIdentityCard({
    super.key,
    required this.identity,
    required this.isDegraded,
    required this.redactionLabel,
    this.sellerTier,
    this.onTap,
    this.avatarSize = 48,
    this.padding = const EdgeInsets.all(12),
    this.borderRadius = const BorderRadius.all(Radius.circular(12)),
    this.showTierBadge = true,
    this.tierBadgePadding = const EdgeInsets.symmetric(horizontal: 12),
  });

  @override
  Widget build(BuildContext context) {
    if (isDegraded) {
      return _buildDegraded(context);
    }

    final resolvedIdentity = identity;
    if (resolvedIdentity == null || !resolvedIdentity.hasIdentity) {
      return const SizedBox.shrink();
    }

    final theme = Theme.of(context);
    final surfaceColor = theme.colorScheme.surfaceContainerHighest;
    final content = Container(
      padding: padding,
      decoration: BoxDecoration(
        color: surfaceColor,
        borderRadius: borderRadius,
      ),
      child: SellerIdentityView(
        identity: resolvedIdentity,
        variant: SellerIdentityViewVariant.detail,
        size: avatarSize,
      ),
    );

    final row = onTap == null
        ? content
        : GestureDetector(
            behavior: HitTestBehavior.opaque,
            onTap: onTap,
            child: content,
          );

    if (!showTierBadge || sellerTier == null) {
      return row;
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        row,
        const SizedBox(height: 8),
        Padding(
          padding: tierBadgePadding,
          child: SellerTierBadge(tier: sellerTier),
        ),
      ],
    );
  }

  Widget _buildDegraded(BuildContext context) {
    final theme = Theme.of(context);

    return Container(
      padding: padding,
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerHighest,
        borderRadius: borderRadius,
      ),
      child: Row(
        children: [
          CircleAvatar(
            radius: avatarSize / 2,
            backgroundColor: theme.colorScheme.surfaceContainerLow,
            child: Icon(
              Icons.person,
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              redactionLabel,
              style: theme.textTheme.bodyMedium?.copyWith(
                fontWeight: FontWeight.w700,
                fontStyle: FontStyle.italic,
                color: theme.colorScheme.onSurfaceVariant,
              ),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
          ),
        ],
      ),
    );
  }
}
