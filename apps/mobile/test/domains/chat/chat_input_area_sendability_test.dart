import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/src/auth/app_role.dart';
import 'package:hishumi/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:hishumi/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:hishumi/domains/chat/chat/presentation/providers/chat_state.dart';
import 'package:hishumi/domains/chat/chat/presentation/widgets/chat_input_area.dart';
import 'package:hishumi/domains/commerce/negotiation/negotiation/domain/entities/negotiation.dart';
import 'package:hishumi/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_notifier.dart';
import 'package:hishumi/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_providers.dart';
import 'package:hishumi/domains/commerce/negotiation/negotiation/presentation/providers/negotiation_state.dart';
import 'package:hishumi/domains/user/identity/authentication/authentication.dart';
import 'package:hishumi/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:hishumi/shared/governance/content_lifecycle.dart';
import 'package:hishumi/shared/providers/auth_status_providers.dart';

const _chatId = '00000000-0000-0000-0000-00000000aaaa';
const _currentUserId = '00000000-0000-0000-0000-00000000bbbb';
const _otherUserId = '00000000-0000-0000-0000-00000000cccc';

class _FakeAuthController extends AuthController {
  @override
  AuthState build() {
    final now = DateTime.utc(2026, 8, 1, 8);
    final user = AuthUser(
      id: _currentUserId,
      createdAt: now,
      updatedAt: now,
      email: 'me@example.com',
      username: 'me',
      isEmailVerified: true,
      accountStatus: AccountStatus.active,
      hasSellerProfile: false,
      hasMarketAuthority: false,
      sellerSubscriptionStatus: 'none',
      roles: const [UserRole.user],
      provider: AuthProvider.email,
      lifecycle: ContentLifecycle.active,
    );
    return AuthState.authenticated(user, emailVerified: true);
  }
}

class _FakeNegotiationNotifier extends NegotiationNotifier {
  @override
  NegotiationState build() => const NegotiationState();
}

/// Carries a specific per-room session into the composer scope so the test
/// can prove the composer renders NO negotiation UI regardless of state
/// (authority lives on the commerce-owned NegotiationProposalCard).
class _SessionNegotiationNotifier extends NegotiationNotifier {
  _SessionNegotiationNotifier(this.session);

  final Negotiation session;

  @override
  NegotiationState build() => NegotiationState(currentNegotiation: session);
}

class _FakeChatDetailNotifier extends ChatDetail {
  @override
  ChatDetailState build(String chatId) {
    return ChatDetailState(
      chat: Chat(
        id: chatId,
        participantIds: const [_currentUserId, _otherUserId],
        participantNames: const {_currentUserId: 'me', _otherUserId: 'other'},
        participantAvatars: const {},
        participantLifecycles: const {_otherUserId: ContentLifecycle.active},
        createdAt: DateTime.utc(2026, 8, 1),
        status: ChatStatus.active,
      ),
    );
  }
}

ProviderScope _wrap(Widget child, {Negotiation? negotiation}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(_FakeAuthController.new),
      currentUserIdProvider.overrideWith((ref) => _currentUserId),
      chatDetailProvider(_chatId).overrideWith(_FakeChatDetailNotifier.new),
      negotiationNotifierProvider.overrideWith(
        negotiation == null
            ? _FakeNegotiationNotifier.new
            : () => _SessionNegotiationNotifier(negotiation),
      ),
    ],
    child: MaterialApp(
      home: Scaffold(
        body: Align(alignment: Alignment.bottomCenter, child: child),
      ),
    ),
  );
}

void main() {
  testWidgets('whitespace-only draft keeps send disabled', (tester) async {
    final controller = TextEditingController();
    var sendCalls = 0;

    await tester.pumpWidget(
      _wrap(
        ChatInputArea(
          chatId: _chatId,
          messageController: controller,
          onSendMessage: (_, {MessageType type = MessageType.text}) async {
            sendCalls += 1;
          },
          onAttachmentTap: () {},
        ),
      ),
    );
    await tester.pump();

    await tester.enterText(find.byType(TextField), '   ');
    await tester.pump();

    // Canonical action row: send is always visible; a whitespace-only draft
    // keeps it disabled, and _handleSendMessage also trims and guards empty
    // content, so no send fires.
    expect(find.byIcon(Icons.send), findsOneWidget);

    await tester.tap(find.byIcon(Icons.send));
    await tester.pump();

    expect(sendCalls, 0);
  });

  testWidgets('empty draft does not send — media-only send path removed', (tester) async {
    final controller = TextEditingController();
    String? capturedContent;
    MessageType? capturedType;

    await tester.pumpWidget(
      _wrap(
        ChatInputArea(
          chatId: _chatId,
          messageController: controller,
          // Codebase factual: sendability is decided inside the widget/notifier
          // chain — the widget carries no canSendMedia flag.
          onSendMessage:
              (content, {MessageType type = MessageType.text}) async {
                capturedContent = content;
                capturedType = type;
              },
          onAttachmentTap: () {},
        ),
      ),
    );
    await tester.pump();

    final sendButton = find.byType(IconButton).last;
    expect(sendButton, findsOneWidget);
    await tester.ensureVisible(sendButton);

    await tester.tap(sendButton);
    await tester.pump();

    // Canonical action row: send always renders but stays disabled while the
    // draft is empty; the old media-only empty-body send path no longer
    // exists in the widget.
    expect(capturedContent, isNull);
    expect(capturedType, isNull);
  });

  Negotiation _session({
    required String sellerId,
    required String buyerId,
    required NegotiationStatus status,
    bool viewerCanAct = false,
    double? agreedPrice,
    DateTime? expiresAt,
  }) {
    final now = DateTime.utc(2026, 8, 1);
    return Negotiation(
      id: 'nego-session-1',
      chatId: _chatId,
      fixedPriceSaleId: 'for-sale-1',
      forSaleName: 'Showa Koi 30cm',
      originalPrice: 1000000,
      buyerId: buyerId,
      buyerName: 'buyer',
      sellerId: sellerId,
      status: status,
      currentOfferPrice: 25000,
      viewerCanAct: viewerCanAct,
      agreedPrice: agreedPrice,
      expiresAt: expiresAt,
      createdAt: now,
      updatedAt: now,
    );
  }

  Widget _input({Negotiation? negotiation}) {
    return _wrap(
      ChatInputArea(
        chatId: _chatId,
        messageController: TextEditingController(),
        onSendMessage: (_, {MessageType type = MessageType.text}) async {},
        onAttachmentTap: () {},
      ),
      negotiation: negotiation,
    );
  }

  testWidgets(
    'composer never renders negotiation actions even on the viewer turn',
    (tester) async {
      // Seller (current user) is the responder, but the composer carries no
      // negotiation UI anymore: Terima/Counter/Tolak live on the
      // commerce-owned NegotiationProposalCard mounted in the stream
      // (owner rule: chat never handles commerce).
      await tester.pumpWidget(
        _input(
          negotiation: _session(
            sellerId: _currentUserId,
            buyerId: _otherUserId,
            status: NegotiationStatus.active,
          ),
        ),
      );
      await tester.pump();

      expect(find.text('Giliran Anda Merespons'), findsNothing);
      expect(find.text('Menunggu Penjual Menjawab'), findsNothing);
      expect(find.text('Terima'), findsNothing);
      expect(find.text('Counter'), findsNothing);
      expect(find.text('Tolak'), findsNothing);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('composer never renders the accepted-deal block or Beli CTA', (
    tester,
  ) async {
    // The sticky deal banner is gone; the Harga Disetujui + Beli + 24h
    // validity block renders only inside the commerce proposal card.
    await tester.pumpWidget(
      _input(
        negotiation: _session(
          sellerId: _otherUserId,
          buyerId: _currentUserId,
          status: NegotiationStatus.accepted,
          agreedPrice: 25000,
          expiresAt: DateTime.now().add(const Duration(hours: 25)),
        ),
      ),
    );
    await tester.pump();

    expect(find.text('Harga Disetujui!'), findsNothing);
    expect(find.textContaining('Rp 25.000'), findsNothing);
    expect(find.textContaining('Harga deal berlaku sisa'), findsNothing);
    expect(find.text('Beli dengan Harga Deal'), findsNothing);
    expect(tester.takeException(), isNull);
  });
}
