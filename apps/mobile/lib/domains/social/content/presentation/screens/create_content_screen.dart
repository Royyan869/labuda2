import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/social/content/presentation/widgets/create_content/content_app_bar.dart';
import 'package:labuda/domains/social/content/presentation/widgets/content_media_handler.dart';
import 'package:labuda/domains/social/content/presentation/widgets/create_content/content_event_handlers.dart';
import 'package:labuda/domains/social/content/presentation/widgets/create_content/content_modals.dart';
import 'package:labuda/domains/social/content/presentation/widgets/create_content/content_scrollable_content.dart';
import 'package:labuda/domains/social/content/presentation/widgets/create_content/content_submission_handler.dart';
import 'package:labuda/domains/social/content/presentation/widgets/create_content/content_toolbar_section.dart';
import 'package:labuda/domains/social/content/presentation/widgets/create_content/content_type_visibility_header.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/widgets/blocked_action_gate.dart';
import 'package:labuda/shared/entities/post_location.dart' as loc;
import 'package:labuda/shared/widgets/user_search_bottom_sheet.dart';

class CreateContentScreen extends ConsumerStatefulWidget {
  const CreateContentScreen({super.key});

  @override
  ConsumerState<CreateContentScreen> createState() =>
      _CreateContentScreenState();
}

class _CreateContentScreenState extends ConsumerState<CreateContentScreen> {
  final _contentController = TextEditingController();
  final _formKey = GlobalKey<FormState>();
  final List<File> _selectedImages = [];
  final List<File> _selectedVideos = [];
  final List<String> _hashtags = [];
  final List<String> _mentionedUserIds = [];
  List<String> _captionMentionedUserIds = [];
  loc.PostLocation? _selectedLocation;
  String _postVisibility = 'Public';
  bool _isSubmitting = false;
  bool _hasUnsavedChanges = false;
  late final ContentMediaHandler _mediaHandler;

  @override
  void initState() {
    super.initState();
    _mediaHandler = ContentMediaHandler();
    _contentController.addListener(_onContentChanged);
  }

  void _onContentChanged() {
    final hasContent =
        _contentController.text.trim().isNotEmpty ||
        _selectedImages.isNotEmpty ||
        _selectedVideos.isNotEmpty ||
        _selectedLocation != null;
    if (_hasUnsavedChanges != hasContent) {
      setState(() => _hasUnsavedChanges = hasContent);
    }
  }

  @override
  void dispose() {
    _contentController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final authenticatedUser = ref.watch(authenticatedUserProvider);

    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) {
        if (didPop) return;
        _handleClose();
      },
      child: Scaffold(
        resizeToAvoidBottomInset: true,
        appBar: ContentAppBar(
          isSubmitting: _isSubmitting,
          canSubmit: _canSubmit(),
          onClose: _handleClose,
          onSubmit: _handleSubmit,
        ),
        body: Form(
          key: _formKey,
          child: Column(
            children: [
              ContentVisibilityHeader(
                authenticatedUser: authenticatedUser,
                postVisibility: _postVisibility,
                onVisibilityChanged: (value) =>
                    setState(() => _postVisibility = value),
              ),
              Expanded(
                child: ContentScrollableContent(
                  contentController: _contentController,
                  isDark: isDark,
                  selectedImages: _selectedImages,
                  selectedVideos: _selectedVideos,
                  selectedLocation: _selectedLocation,
                  hashtags: _hashtags,
                  onContentChanged: _handleContentChanged,
                  onMentionsChanged: (ids) {
                    setState(() => _captionMentionedUserIds = ids);
                  },
                  onImageReorder: _handleImageReorder,
                  onImageRemove: (index) =>
                      setState(() => _selectedImages.removeAt(index)),
                  onVideoRemove: () => setState(() => _selectedVideos.clear()),
                  onLocationEdit: _handleAddLocation,
                  onHashtagEdit: _handleAddHashtag,
                  onLocationRemove: () =>
                      setState(() => _selectedLocation = null),
                  onHashtagRemove: (h) =>
                      setState(() => _hashtags.remove(h.replaceFirst('#', ''))),
                ),
              ),
              ContentToolbarSection(
                mediaHandler: _mediaHandler,
                selectedImages: _selectedImages,
                selectedVideos: _selectedVideos,
                taggedPeopleCount: _mentionedUserIds.length,
                hasLocation: _selectedLocation != null,
                onMediaAdded: (images, videos) {
                  setState(() {
                    _selectedImages.addAll(images);
                    _selectedVideos.addAll(videos);
                  });
                  _onContentChanged();
                },
                onTagPeople: _handleTagPeople,
                onAddLocation: _handleAddLocation,
                isDark: isDark,
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _handleContentChanged(String value) {
    setState(() => _hasUnsavedChanges = true);
    final RegExp hashtagRegex = RegExp(r'#[a-zA-Z0-9_]+');
    final matches = hashtagRegex.allMatches(value);
    final hashtags = matches.map((match) => match.group(0)!).toList();
    setState(() {
      _hashtags.clear();
      _hashtags.addAll(hashtags.map((tag) => tag.substring(1)));
    });
  }

  void _handleImageReorder(int oldIndex, int newIndex) {
    setState(() {
      if (newIndex > oldIndex) newIndex -= 1;
      final item = _selectedImages.removeAt(oldIndex);
      _selectedImages.insert(newIndex, item);
    });
  }

  void _handleClose() =>
      _hasUnsavedChanges ? _showExitModal() : _navigateBack();

  void _navigateBack() {
    if (Navigator.of(context).canPop()) {
      Navigator.of(context).pop();
    }
  }

  void _handleAddLocation() async {
    final loc = await ContentEventHandlers.handleAddLocation(
      context: context,
      currentLocation: _selectedLocation,
    );
    if (loc != null) {
      setState(() => _selectedLocation = loc);
      _onContentChanged();
    }
  }

  void _handleTagPeople() async {
    final ids = await UserSearchBottomSheet.show(
      context: context,
      alreadyTaggedUserIds: _mentionedUserIds,
      maxSelections: 50,
    );
    if (ids != null && ids.isNotEmpty) {
      setState(() {
        _mentionedUserIds.clear();
        _mentionedUserIds.addAll(ids);
      });
      _onContentChanged();
    }
  }

  void _handleAddHashtag() => ContentModals.showInputModal(
    context: context,
    title: 'Add Hashtag',
    hintText: 'Enter hashtag (without #)...',
    onAdd: (h) {
      if (h.isNotEmpty && !_hashtags.contains(h)) {
        setState(() => _hashtags.add(h));
      }
    },
  );

  bool _canSubmit() =>
      _contentController.text.trim().isNotEmpty ||
      _selectedImages.isNotEmpty ||
      _selectedVideos.isNotEmpty;

  void _handleSubmit() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _isSubmitting = true);

    try {
      final authState = ref.read(authControllerProvider);
      if (authState is! AuthStateAuthenticated) {
        throw Exception('User not authenticated');
      }

      // Pre-flight gate: backend will reject this with EMAIL_VERIFICATION_REQUIRED
      // anyway. Pre-flighting here avoids burning a background upload only to
      // surface the failure after the user has already navigated to home.
      if (!authState.emailVerified) {
        await showBlockedActionGate(
          context,
          actionDescription: 'membuat konten',
        );
        return;
      }

      final uploadProgressNotifier = ref.read(uploadProgressProvider.notifier);
      final taskId = ContentSubmissionHandler.startUploadTask(
        notifier: uploadProgressNotifier,
        imageCount: _selectedImages.length,
        videoCount: _selectedVideos.length,
      );

      AppSnackBar.showSuccess(
        context,
        'Starting upload... Check progress on home',
      );
      ref.read(navigationHandlerProvider).navigateToHome();

      unawaited(
        ContentSubmissionHandler.performBackgroundUpload(
          user: authState.user,
          taskId: taskId,
          notifier: uploadProgressNotifier,
          selectedImages: _selectedImages,
          selectedVideos: _selectedVideos,
          content: _contentController.text,
          hashtags: _hashtags,
          mentionedUserIds: {
            ..._mentionedUserIds,
            ..._captionMentionedUserIds,
          }.toList(),
          postVisibility: _postVisibility,
          selectedLocation: _selectedLocation,
          container: ref.container,
        ),
      );
    } catch (e) {
      if (mounted) {
        AppSnackBar.showError(context, 'Gagal mengunggah. Coba lagi.');
      }
    } finally {
      if (mounted) setState(() => _isSubmitting = false);
    }
  }

  void _showExitModal() {
    ContentModals.showExitDialog(
      context: context,
      onDiscard: () {
        _hasUnsavedChanges = false;
        _navigateBack();
      },
    );
  }
}
