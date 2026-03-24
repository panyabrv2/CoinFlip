CREATE SCHEMA IF NOT EXISTS twist_business;

CREATE TABLE IF NOT EXISTS twist_business.game_rounds (
    game_id            BIGINT PRIMARY KEY,
    phase              TEXT NOT NULL CHECK (phase IN ('waiting', 'betting', 'gettingResult', 'finished')),
    hash               TEXT NOT NULL,
    seed               TEXT NOT NULL,
    result_side        TEXT NULL CHECK (result_side IN ('heads', 'tails')),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    betting_started_at TIMESTAMPTZ NULL,
    result_started_at  TIMESTAMPTZ NULL,
    finished_at        TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS twist_business.series_sessions (
    id                    BIGSERIAL PRIMARY KEY,
    user_id               BIGINT NOT NULL,
    initial_game_id       BIGINT NOT NULL REFERENCES twist_business.game_rounds(game_id),
    active                BOOLEAN NOT NULL DEFAULT TRUE,
    stage                 TEXT NOT NULL CHECK (stage IN ('in_round', 'awaiting_choice', 'cashed_out', 'lost')),
    round_game_id         BIGINT NULL REFERENCES twist_business.game_rounds(game_id),
    current_side          TEXT NULL CHECK (current_side IN ('heads', 'tails')),
    stake_ton             NUMERIC(20,8) NOT NULL CHECK (stake_ton > 0),
    wins                  INT NOT NULL DEFAULT 0 CHECK (wins >= 0),
    multiplier            NUMERIC(20,8) NOT NULL DEFAULT 1.0 CHECK (multiplier >= 1),
    claimable_ton         NUMERIC(20,8) NOT NULL DEFAULT 0,
    cashed_out_payout_ton NUMERIC(20,8) NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at             TIMESTAMPTZ NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_series_sessions_user_active
    ON twist_business.series_sessions(user_id)
    WHERE active = TRUE;

CREATE TABLE IF NOT EXISTS twist_business.game_bets (
    id                BIGSERIAL PRIMARY KEY,
    game_id           BIGINT NOT NULL REFERENCES twist_business.game_rounds(game_id) ON DELETE CASCADE,
    user_id           BIGINT NOT NULL,
    side              TEXT NOT NULL CHECK (side IN ('heads', 'tails')),
    mode              TEXT NOT NULL CHECK (mode IN ('single', 'series')),
    series_session_id BIGINT NULL REFERENCES twist_business.series_sessions(id),
    item_id           BIGINT NOT NULL,
    item_type         TEXT NOT NULL,
    item_name         TEXT NOT NULL,
    item_photo_url    TEXT NULL,
    stake_ton         NUMERIC(20,8) NOT NULL CHECK (stake_ton > 0),
    status            TEXT NOT NULL DEFAULT 'accepted'
                      CHECK (status IN (
                          'accepted',
                          'single_win',
                          'single_lose',
                          'series_awaiting_choice',
                          'series_lost',
                          'series_cashed_out',
                          'cancelled'
                      )),
    payout_ton        NUMERIC(20,8) NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    settled_at        TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS ix_game_bets_game_id
    ON twist_business.game_bets(game_id);

CREATE INDEX IF NOT EXISTS ix_game_bets_user_id
    ON twist_business.game_bets(user_id);

CREATE INDEX IF NOT EXISTS ix_game_bets_series_session_id
    ON twist_business.game_bets(series_session_id);

CREATE TABLE IF NOT EXISTS twist_business.series_steps (
    id               BIGSERIAL PRIMARY KEY,
    session_id       BIGINT NOT NULL REFERENCES twist_business.series_sessions(id) ON DELETE CASCADE,
    game_id          BIGINT NULL REFERENCES twist_business.game_rounds(game_id),
    event            TEXT NOT NULL CHECK (event IN ('win', 'lose', 'cashout')),
    chosen_side      TEXT NULL CHECK (chosen_side IN ('heads', 'tails')),
    wins_after       INT NOT NULL,
    multiplier_after NUMERIC(20,8) NOT NULL,
    claimable_after  NUMERIC(20,8) NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_series_steps_session_id
    ON twist_business.series_steps(session_id);