/// Typed delivery status for the initial verification email.
///
/// Replaces the coarse `initialSendFailed` boolean with three explicit
/// states that drive distinct UI in the Verify Email portal.
library;

/// Sealed class describing the outcome of the initial verification-email
/// delivery attempt (the send that fires during `signUpWithEmail` before
/// the portal opens).
sealed class VerificationDeliveryStatus {
  const VerificationDeliveryStatus();
}

/// The verification email was sent successfully at [sentAt].
///
/// Portal behaviour:
/// - Shows "email sent" copy.
/// - Computes remaining cooldown from [sentAt] synchronously — the first
///   frame is guaranteed to show the countdown, not a loading placeholder.
/// - The cooldown persistence (recordSent) was already written before
///   portal publication, so the persisted timestamp matches [sentAt].
class VerificationDeliverySent extends VerificationDeliveryStatus {
  final DateTime sentAt;
  const VerificationDeliverySent(this.sentAt);
}

/// The verification email send explicitly failed with [message].
///
/// Portal behaviour:
/// - Shows explicit failed-delivery copy (not "we sent you an email").
/// - Resend button is enabled immediately — no cooldown was recorded.
/// - Never shows a countdown for this state.
class VerificationDeliveryFailed extends VerificationDeliveryStatus {
  final String message;
  const VerificationDeliveryFailed([this.message = 'Gagal mengirim email verifikasi']);
}

/// No delivery was attempted in this session.
///
/// Reached via: login, cold-start restore, Firebase listener re-sync,
/// protected-domain reconciliation (`requireEmailVerification`).
///
/// Portal behaviour:
/// - Shows neutral copy ("Akun belum terverifikasi") — does NOT claim
///   a new email was just sent.
/// - Resend availability is determined from the persisted cooldown
///   (async read from storage on first frame — brief disabled state
///   acceptable, then transitions to persisted value).
class VerificationDeliveryUnknown extends VerificationDeliveryStatus {
  const VerificationDeliveryUnknown();
}
