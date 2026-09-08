-- 000001_init_schema.up.sql

CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    vk_id BIGINT NOT NULL UNIQUE,
    first_name VARCHAR(128) NOT NULL DEFAULT '',
    last_name VARCHAR(128) NOT NULL DEFAULT '',
    avatar_url TEXT NOT NULL DEFAULT '',
    ref_code VARCHAR(32) NOT NULL UNIQUE,
    referred_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS balances (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    amount_kopecks BIGINT NOT NULL DEFAULT 0 CHECK (amount_kopecks >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS balance_transactions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount_kopecks BIGINT NOT NULL,
    balance_after_kopecks BIGINT NOT NULL,
    type VARCHAR(32) NOT NULL CHECK (type IN ('welcome_bonus', 'referral_reward', 'token_charge', 'deposit', 'adjustment')),
    reference_id VARCHAR(64),
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS referrals (
    id BIGSERIAL PRIMARY KEY,
    referrer_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    referee_id BIGINT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    reward_kopecks BIGINT NOT NULL DEFAULT 200,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_no_self_referral CHECK (referrer_id <> referee_id)
);

CREATE INDEX IF NOT EXISTS idx_users_vk_id ON users(vk_id);
CREATE INDEX IF NOT EXISTS idx_users_ref_code ON users(ref_code);
CREATE INDEX IF NOT EXISTS idx_transactions_user_id ON balance_transactions(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_referrals_referrer_id ON referrals(referrer_id);
