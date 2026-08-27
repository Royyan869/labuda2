// Compatibility barrel — forwards all exports from the domain modules.
// All existing imports of '@/lib/api' continue to resolve here without changes.
// Domain source of truth: src/lib/api/ (client, orders, disputes, finance,
// users, moderation, alerts, sellers, platform).
export * from './api/index'
