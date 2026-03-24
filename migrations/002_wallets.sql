CREATE TABLE IF NOT EXISTS twist_business.user_wallets (
    user_id     BIGINT PRIMARY KEY REFERENCES twist_business.users(user_id) ON DELETE CASCADE,
    balance_ton NUMERIC(20,8) NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS twist_business.wallet_transactions (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT NOT NULL REFERENCES twist_business.users(user_id) ON DELETE CASCADE,
    game_id           BIGINT NULL REFERENCES twist_business.game_rounds(game_id) ON DELETE SET NULL,
    series_session_id BIGINT NULL REFERENCES twist_business.series_sessions(id) ON DELETE SET NULL,
    kind              TEXT NOT NULL CHECK (kind IN ('single_win', 'series_cashout')),
    amount_ton        NUMERIC(20,8) NOT NULL CHECK (amount_ton > 0),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_wallet_tx_single_win
    ON twist_business.wallet_transactions(user_id, game_id, kind)
    WHERE kind = 'single_win';

CREATE UNIQUE INDEX IF NOT EXISTS ux_wallet_tx_series_cashout
    ON twist_business.wallet_transactions(series_session_id, kind)
    WHERE kind = 'series_cashout';