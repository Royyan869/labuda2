String formatChatHandle(String username) {
  final trimmed = username.trim();
  if (trimmed.isEmpty) return '';
  if (trimmed.startsWith('@')) return trimmed;
  return '@$trimmed';
}
