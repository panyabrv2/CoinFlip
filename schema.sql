--
-- PostgreSQL database dump
--

\restrict V8dZuc4fgrvmIyShPAwsfPJJiquSoSkrVzda6eaD7RWscyjwevZ0KuphFfZJgaJ

-- Dumped from database version 16.10
-- Dumped by pg_dump version 16.10

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: twist_admin; Type: SCHEMA; Schema: -; Owner: postgres
--

CREATE SCHEMA twist_admin;


ALTER SCHEMA twist_admin OWNER TO postgres;

--
-- Name: twist_bots; Type: SCHEMA; Schema: -; Owner: postgres
--

CREATE SCHEMA twist_bots;


ALTER SCHEMA twist_bots OWNER TO postgres;

--
-- Name: twist_business; Type: SCHEMA; Schema: -; Owner: postgres
--

CREATE SCHEMA twist_business;


ALTER SCHEMA twist_business OWNER TO postgres;

--
-- Name: twist_rocket; Type: SCHEMA; Schema: -; Owner: postgres
--

CREATE SCHEMA twist_rocket;


ALTER SCHEMA twist_rocket OWNER TO postgres;

--
-- Name: twist_roulette; Type: SCHEMA; Schema: -; Owner: postgres
--

CREATE SCHEMA twist_roulette;


ALTER SCHEMA twist_roulette OWNER TO postgres;

--
-- Name: pgcrypto; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;


--
-- Name: EXTENSION pgcrypto; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION pgcrypto IS 'cryptographic functions';


--
-- Name: postgres_fdw; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS postgres_fdw WITH SCHEMA public;


--
-- Name: EXTENSION postgres_fdw; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION postgres_fdw IS 'foreign-data wrapper for remote PostgreSQL servers';


--
-- Name: item_kind; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.item_kind AS ENUM (
    'gift',
    'card'
);


ALTER TYPE public.item_kind OWNER TO postgres;

--
-- Name: withdrawal_status; Type: TYPE; Schema: public; Owner: postgres
--

CREATE TYPE public.withdrawal_status AS ENUM (
    'waiting',
    'taken',
    'declined',
    'succeed'
);


ALTER TYPE public.withdrawal_status OWNER TO postgres;

--
-- Name: generate_item_id(public.item_kind); Type: FUNCTION; Schema: public; Owner: postgres
--

CREATE FUNCTION public.generate_item_id(kind public.item_kind) RETURNS integer
    LANGUAGE plpgsql
    AS $$
DECLARE
    new_id integer;
BEGIN
    IF kind = 'card' THEN
        SELECT nextval('public.card_item_seq')::int INTO new_id;
    ELSIF kind = 'gift' THEN
        SELECT nextval('public.gift_item_seq')::int INTO new_id;
    ELSE
        RAISE EXCEPTION 'Unknown item_kind: %', kind;
    END IF;
    RETURN new_id;
END;
$$;


ALTER FUNCTION public.generate_item_id(kind public.item_kind) OWNER TO postgres;

--
-- Name: recalc_task_completed(integer); Type: FUNCTION; Schema: twist_business; Owner: postgres
--

CREATE FUNCTION twist_business.recalc_task_completed(p_task_id integer) RETURNS void
    LANGUAGE plpgsql
    AS $$
                              begin
                                  update twist_business.tasks t
                                  set completed   = (t.task_run_limit > 0 and t.task_runs_total >= t.task_run_limit),
                                      task_active = case
                                                        when (t.task_run_limit > 0 and t.task_runs_total >= t.task_run_limit)
                                                            then false
                                                        else t.task_active
                                          end
                                  where t.task_id = p_task_id;
                              end;
                              $$;


ALTER FUNCTION twist_business.recalc_task_completed(p_task_id integer) OWNER TO postgres;

--
-- Name: tasks_before_update_limit(); Type: FUNCTION; Schema: twist_business; Owner: postgres
--

CREATE FUNCTION twist_business.tasks_before_update_limit() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
                                       begin
                                           if tg_op = 'UPDATE' and
                                              new.task_run_limit is distinct from old.task_run_limit then
                                               new.completed :=
                                                       (new.task_run_limit > 0 and new.task_runs_total >= new.task_run_limit);
                                               if new.completed then
                                                   new.task_active := false;
                                               end if;
                                           end if;
                                           return new;
                                       end;
                                       $$;


ALTER FUNCTION twist_business.tasks_before_update_limit() OWNER TO postgres;

--
-- Name: utp_after_delete(); Type: FUNCTION; Schema: twist_business; Owner: postgres
--

CREATE FUNCTION twist_business.utp_after_delete() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
                              BEGIN
                                  UPDATE twist_business.tasks
                                  SET task_runs_total = GREATEST(task_runs_total - 1, 0)
                                  WHERE task_id = OLD.task_id;

                                  PERFORM twist_business.recalc_task_completed(OLD.task_id);
                                  RETURN NULL;
                              END;
                              $$;


ALTER FUNCTION twist_business.utp_after_delete() OWNER TO postgres;

--
-- Name: utp_after_insert(); Type: FUNCTION; Schema: twist_business; Owner: postgres
--

CREATE FUNCTION twist_business.utp_after_insert() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
                              BEGIN
                                  UPDATE twist_business.tasks
                                  SET task_runs_total = task_runs_total + 1
                                  WHERE task_id = NEW.task_id;

                                  PERFORM twist_business.recalc_task_completed(NEW.task_id);
                                  RETURN NULL;
                              END;
                              $$;


ALTER FUNCTION twist_business.utp_after_insert() OWNER TO postgres;

--
-- Name: card_item_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.card_item_seq
    START WITH 3
    INCREMENT BY 1
    MINVALUE 3
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.card_item_seq OWNER TO postgres;

--
-- Name: gift_item_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.gift_item_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.gift_item_seq OWNER TO postgres;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: actions_log; Type: TABLE; Schema: twist_admin; Owner: postgres
--

CREATE TABLE twist_admin.actions_log (
    id bigint NOT NULL,
    admin_id bigint,
    action text,
    target_table text,
    record_id text,
    old_data jsonb,
    new_data jsonb,
    created_at timestamp with time zone
);


ALTER TABLE twist_admin.actions_log OWNER TO postgres;

--
-- Name: actions_log_id_seq; Type: SEQUENCE; Schema: twist_admin; Owner: postgres
--

CREATE SEQUENCE twist_admin.actions_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE twist_admin.actions_log_id_seq OWNER TO postgres;

--
-- Name: actions_log_id_seq; Type: SEQUENCE OWNED BY; Schema: twist_admin; Owner: postgres
--

ALTER SEQUENCE twist_admin.actions_log_id_seq OWNED BY twist_admin.actions_log.id;


--
-- Name: users; Type: TABLE; Schema: twist_admin; Owner: postgres
--

CREATE TABLE twist_admin.users (
    id bigint NOT NULL,
    username text,
    password text,
    role text DEFAULT 'viewer'::text,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone
);


ALTER TABLE twist_admin.users OWNER TO postgres;

--
-- Name: users_id_seq; Type: SEQUENCE; Schema: twist_admin; Owner: postgres
--

CREATE SEQUENCE twist_admin.users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE twist_admin.users_id_seq OWNER TO postgres;

--
-- Name: users_id_seq; Type: SEQUENCE OWNED BY; Schema: twist_admin; Owner: postgres
--

ALTER SEQUENCE twist_admin.users_id_seq OWNED BY twist_admin.users.id;


--
-- Name: roulette_bots; Type: TABLE; Schema: twist_bots; Owner: postgres
--

CREATE TABLE twist_bots.roulette_bots (
    user_id bigint NOT NULL,
    auth_token text NOT NULL
);


ALTER TABLE twist_bots.roulette_bots OWNER TO postgres;

--
-- Name: gift_deposits; Type: TABLE; Schema: twist_business; Owner: postgres
--

CREATE TABLE twist_business.gift_deposits (
    deposit_id integer NOT NULL,
    item_id integer NOT NULL,
    user_id bigint,
    created_at timestamp with time zone DEFAULT (now() AT TIME ZONE 'UTC'::text)
);


ALTER TABLE twist_business.gift_deposits OWNER TO postgres;

--
-- Name: gift_deposits_deposit_id_seq; Type: SEQUENCE; Schema: twist_business; Owner: postgres
--

CREATE SEQUENCE twist_business.gift_deposits_deposit_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE twist_business.gift_deposits_deposit_id_seq OWNER TO postgres;

--
-- Name: gift_deposits_deposit_id_seq; Type: SEQUENCE OWNED BY; Schema: twist_business; Owner: postgres
--

ALTER SEQUENCE twist_business.gift_deposits_deposit_id_seq OWNED BY twist_business.gift_deposits.deposit_id;


--
-- Name: items; Type: TABLE; Schema: twist_business; Owner: postgres
--

CREATE TABLE twist_business.items (
    item_id integer NOT NULL,
    name text,
    photo_url text,
    background text,
    background_rarity numeric(3,2),
    model text,
    model_rarity numeric(3,2),
    symbol text,
    symbol_rarity numeric(3,2),
    cost_ton numeric(12,4),
    number bigint,
    user_id bigint,
    locked boolean DEFAULT false NOT NULL,
    type public.item_kind DEFAULT 'gift'::public.item_kind NOT NULL,
    owned_gift_id text,
    msg_id bigint,
    can_withdraw boolean DEFAULT true NOT NULL
);


ALTER TABLE twist_business.items OWNER TO postgres;

--
-- Name: payments; Type: TABLE; Schema: twist_business; Owner: postgres
--

CREATE TABLE twist_business.payments (
    payment_id text NOT NULL,
    external_payment_id bigint NOT NULL,
    crypto_pay_amount numeric(12,4),
    actually_paid_amount numeric(12,4),
    amount_usd numeric(12,4),
    pay_address text,
    pay_currency text,
    order_description text,
    payment_status text,
    price_currency text,
    user_id bigint,
    created_at timestamp with time zone DEFAULT (now() AT TIME ZONE 'UTC'::text)
);


ALTER TABLE twist_business.payments OWNER TO postgres;

--
-- Name: payments_view; Type: VIEW; Schema: twist_business; Owner: postgres
--

CREATE VIEW twist_business.payments_view AS
 SELECT payment_id,
    created_at,
    user_id,
    external_payment_id,
    crypto_pay_amount,
    actually_paid_amount,
    amount_usd,
    pay_currency,
    price_currency,
    payment_status,
    pay_address,
    order_description
   FROM twist_business.payments p;


ALTER VIEW twist_business.payments_view OWNER TO postgres;

--
-- Name: tasks; Type: TABLE; Schema: twist_business; Owner: postgres
--

CREATE TABLE twist_business.tasks (
    task_id integer NOT NULL,
    task_name text NOT NULL,
    task_price numeric(12,4) DEFAULT 0 NOT NULL,
    task_link text,
    task_active boolean DEFAULT true NOT NULL,
    task_type text DEFAULT 'tgchat'::text NOT NULL,
    task_photo text,
    task_start_at_msk timestamp without time zone DEFAULT (now() AT TIME ZONE 'Europe/Moscow'::text) NOT NULL,
    task_run_limit integer DEFAULT 0 NOT NULL,
    task_runs_total integer DEFAULT 0 NOT NULL,
    completed boolean DEFAULT false NOT NULL,
    CONSTRAINT tasks_run_limit_nonneg CHECK ((task_run_limit >= 0)),
    CONSTRAINT tasks_runs_total_nonneg CHECK ((task_runs_total >= 0))
);


ALTER TABLE twist_business.tasks OWNER TO postgres;

--
-- Name: tasks_task_id_seq; Type: SEQUENCE; Schema: twist_business; Owner: postgres
--

ALTER TABLE twist_business.tasks ALTER COLUMN task_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME twist_business.tasks_task_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);


--
-- Name: user_task_progress; Type: TABLE; Schema: twist_business; Owner: postgres
--

CREATE TABLE twist_business.user_task_progress (
    user_id bigint NOT NULL,
    task_id integer NOT NULL,
    done_at timestamp with time zone
);


ALTER TABLE twist_business.user_task_progress OWNER TO postgres;

--
-- Name: users; Type: TABLE; Schema: twist_business; Owner: postgres
--

CREATE TABLE twist_business.users (
    user_id bigint NOT NULL,
    username character varying(32),
    first_name text,
    last_name text,
    photo_url text,
    ref_code bigint,
    registration_date timestamp with time zone DEFAULT (now() AT TIME ZONE 'UTC'::text) NOT NULL,
    ton_balance numeric(12,4) DEFAULT 0 NOT NULL,
    wallet_address text,
    tasks_done integer DEFAULT 0 NOT NULL,
    last_active_at timestamp with time zone DEFAULT (now() AT TIME ZONE 'UTC'::text) NOT NULL,
    games_count integer DEFAULT 0 NOT NULL,
    win_count integer DEFAULT 0 NOT NULL,
    bets_sum numeric(12,4) DEFAULT 0 NOT NULL,
    win_sum numeric(12,4) DEFAULT 0 NOT NULL,
    session_seconds integer DEFAULT 0 NOT NULL,
    total_earn_referral numeric(12,4) DEFAULT 0 NOT NULL,
    country_code text
);


ALTER TABLE twist_business.users OWNER TO postgres;

--
-- Name: withdrawals; Type: TABLE; Schema: twist_business; Owner: postgres
--

CREATE TABLE twist_business.withdrawals (
    id integer NOT NULL,
    user_id bigint NOT NULL,
    item_id integer NOT NULL,
    status public.withdrawal_status DEFAULT 'waiting'::public.withdrawal_status NOT NULL,
    created_at timestamp with time zone DEFAULT (now() AT TIME ZONE 'UTC'::text) NOT NULL,
    taken_at timestamp with time zone,
    completed_at timestamp with time zone,
    parent_id integer,
    message_id bigint
);


ALTER TABLE twist_business.withdrawals OWNER TO postgres;

--
-- Name: withdrawals_id_seq; Type: SEQUENCE; Schema: twist_business; Owner: postgres
--

CREATE SEQUENCE twist_business.withdrawals_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE twist_business.withdrawals_id_seq OWNER TO postgres;

--
-- Name: withdrawals_id_seq; Type: SEQUENCE OWNED BY; Schema: twist_business; Owner: postgres
--

ALTER SEQUENCE twist_business.withdrawals_id_seq OWNED BY twist_business.withdrawals.id;


--
-- Name: bets; Type: TABLE; Schema: twist_rocket; Owner: postgres
--

CREATE TABLE twist_rocket.bets (
    bet_id integer NOT NULL,
    game_id integer NOT NULL,
    user_id bigint,
    bet_type text,
    bet_item jsonb,
    cost_ton numeric(12,4),
    created_at timestamp with time zone DEFAULT (now() AT TIME ZONE 'utc'::text) NOT NULL,
    outcome smallint,
    cashout_x numeric(12,4),
    payout_ton numeric(18,4),
    cashed_out_at timestamp with time zone
);


ALTER TABLE twist_rocket.bets OWNER TO postgres;

--
-- Name: bets_bet_id_seq; Type: SEQUENCE; Schema: twist_rocket; Owner: postgres
--

CREATE SEQUENCE twist_rocket.bets_bet_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE twist_rocket.bets_bet_id_seq OWNER TO postgres;

--
-- Name: bets_bet_id_seq; Type: SEQUENCE OWNED BY; Schema: twist_rocket; Owner: postgres
--

ALTER SEQUENCE twist_rocket.bets_bet_id_seq OWNED BY twist_rocket.bets.bet_id;


--
-- Name: games; Type: TABLE; Schema: twist_rocket; Owner: postgres
--

CREATE TABLE twist_rocket.games (
    game_id integer NOT NULL,
    table_id integer NOT NULL,
    game_hash text NOT NULL,
    rng_seed bytea NOT NULL,
    game_phase text,
    created_at timestamp with time zone DEFAULT (now() AT TIME ZONE 'utc'::text) NOT NULL,
    betting_started_at timestamp with time zone,
    launching_started_at timestamp with time zone,
    flying_started_at timestamp with time zone,
    crashed_at timestamp with time zone,
    finished_at timestamp with time zone,
    ts0 bigint,
    curve_param numeric(10,6),
    tick_ms integer,
    x_max numeric(12,4),
    crash_x numeric(12,4),
    total_bet_ton numeric(18,4),
    total_payout_ton numeric(18,4),
    rtp_actual numeric(6,4)
);


ALTER TABLE twist_rocket.games OWNER TO postgres;

--
-- Name: games_game_id_seq; Type: SEQUENCE; Schema: twist_rocket; Owner: postgres
--

CREATE SEQUENCE twist_rocket.games_game_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE twist_rocket.games_game_id_seq OWNER TO postgres;

--
-- Name: games_game_id_seq; Type: SEQUENCE OWNED BY; Schema: twist_rocket; Owner: postgres
--

ALTER SEQUENCE twist_rocket.games_game_id_seq OWNED BY twist_rocket.games.game_id;


--
-- Name: messages; Type: TABLE; Schema: twist_rocket; Owner: postgres
--

CREATE TABLE twist_rocket.messages (
    id bigint NOT NULL,
    user_id bigint NOT NULL,
    text text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE twist_rocket.messages OWNER TO postgres;

--
-- Name: messages_id_seq; Type: SEQUENCE; Schema: twist_rocket; Owner: postgres
--

CREATE SEQUENCE twist_rocket.messages_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE twist_rocket.messages_id_seq OWNER TO postgres;

--
-- Name: messages_id_seq; Type: SEQUENCE OWNED BY; Schema: twist_rocket; Owner: postgres
--

ALTER SEQUENCE twist_rocket.messages_id_seq OWNED BY twist_rocket.messages.id;


--
-- Name: tables; Type: TABLE; Schema: twist_rocket; Owner: postgres
--

CREATE TABLE twist_rocket.tables (
    table_id integer NOT NULL,
    table_name text,
    max_bet numeric(10,2) DEFAULT 0 NOT NULL,
    max_commission numeric(4,3) DEFAULT 0.150 NOT NULL
);


ALTER TABLE twist_rocket.tables OWNER TO postgres;

--
-- Name: tables_table_id_seq; Type: SEQUENCE; Schema: twist_rocket; Owner: postgres
--

CREATE SEQUENCE twist_rocket.tables_table_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE twist_rocket.tables_table_id_seq OWNER TO postgres;

--
-- Name: tables_table_id_seq; Type: SEQUENCE OWNED BY; Schema: twist_rocket; Owner: postgres
--

ALTER SEQUENCE twist_rocket.tables_table_id_seq OWNED BY twist_rocket.tables.table_id;


--
-- Name: bets; Type: TABLE; Schema: twist_roulette; Owner: postgres
--

CREATE TABLE twist_roulette.bets (
    bet_id integer NOT NULL,
    game_id integer NOT NULL,
    bet_type text,
    bet_item jsonb,
    created_at timestamp with time zone DEFAULT (now() AT TIME ZONE 'utc'::text) NOT NULL,
    user_id bigint,
    cost_ton numeric(12,4),
    outcome smallint
);


ALTER TABLE twist_roulette.bets OWNER TO postgres;

--
-- Name: bets_bet_id_seq; Type: SEQUENCE; Schema: twist_roulette; Owner: postgres
--

CREATE SEQUENCE twist_roulette.bets_bet_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE twist_roulette.bets_bet_id_seq OWNER TO postgres;

--
-- Name: bets_bet_id_seq; Type: SEQUENCE OWNED BY; Schema: twist_roulette; Owner: postgres
--

ALTER SEQUENCE twist_roulette.bets_bet_id_seq OWNED BY twist_roulette.bets.bet_id;


--
-- Name: games; Type: TABLE; Schema: twist_roulette; Owner: postgres
--

CREATE TABLE twist_roulette.games (
    game_id integer NOT NULL,
    table_id integer NOT NULL,
    game_hash text NOT NULL,
    game_phase text,
    winner_id bigint,
    created_at timestamp with time zone DEFAULT (now() AT TIME ZONE 'utc'::text) NOT NULL,
    countdown_started_at timestamp with time zone,
    result_started_at timestamp with time zone,
    rng_seed bytea,
    winner_commission_rate numeric(4,3),
    winner_commission_amount numeric(12,4),
    finished_at timestamp with time zone
);


ALTER TABLE twist_roulette.games OWNER TO postgres;

--
-- Name: games_game_id_seq; Type: SEQUENCE; Schema: twist_roulette; Owner: postgres
--

CREATE SEQUENCE twist_roulette.games_game_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE twist_roulette.games_game_id_seq OWNER TO postgres;

--
-- Name: games_game_id_seq; Type: SEQUENCE OWNED BY; Schema: twist_roulette; Owner: postgres
--

ALTER SEQUENCE twist_roulette.games_game_id_seq OWNED BY twist_roulette.games.game_id;


--
-- Name: tables; Type: TABLE; Schema: twist_roulette; Owner: postgres
--

CREATE TABLE twist_roulette.tables (
    table_id integer NOT NULL,
    table_name text,
    max_bet numeric(10,2) DEFAULT 0 NOT NULL,
    max_commission numeric(4,3) DEFAULT 0.150 NOT NULL
);


ALTER TABLE twist_roulette.tables OWNER TO postgres;

--
-- Name: tables_table_id_seq; Type: SEQUENCE; Schema: twist_roulette; Owner: postgres
--

CREATE SEQUENCE twist_roulette.tables_table_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE twist_roulette.tables_table_id_seq OWNER TO postgres;

--
-- Name: tables_table_id_seq; Type: SEQUENCE OWNED BY; Schema: twist_roulette; Owner: postgres
--

ALTER SEQUENCE twist_roulette.tables_table_id_seq OWNED BY twist_roulette.tables.table_id;


--
-- Name: actions_log id; Type: DEFAULT; Schema: twist_admin; Owner: postgres
--

ALTER TABLE ONLY twist_admin.actions_log ALTER COLUMN id SET DEFAULT nextval('twist_admin.actions_log_id_seq'::regclass);


--
-- Name: users id; Type: DEFAULT; Schema: twist_admin; Owner: postgres
--

ALTER TABLE ONLY twist_admin.users ALTER COLUMN id SET DEFAULT nextval('twist_admin.users_id_seq'::regclass);


--
-- Name: gift_deposits deposit_id; Type: DEFAULT; Schema: twist_business; Owner: postgres
--

ALTER TABLE ONLY twist_business.gift_deposits ALTER COLUMN deposit_id SET DEFAULT nextval('twist_business.gift_deposits_deposit_id_seq'::regclass);


--
-- Name: withdrawals id; Type: DEFAULT; Schema: twist_business; Owner: postgres
--

ALTER TABLE ONLY twist_business.withdrawals ALTER COLUMN id SET DEFAULT nextval('twist_business.withdrawals_id_seq'::regclass);


--
-- Name: bets bet_id; Type: DEFAULT; Schema: twist_rocket; Owner: postgres
--

ALTER TABLE ONLY twist_rocket.bets ALTER COLUMN bet_id SET DEFAULT nextval('twist_rocket.bets_bet_id_seq'::regclass);


--
-- Name: games game_id; Type: DEFAULT; Schema: twist_rocket; Owner: postgres
--

ALTER TABLE ONLY twist_rocket.games ALTER COLUMN game_id SET DEFAULT nextval('twist_rocket.games_game_id_seq'::regclass);


--
-- Name: messages id; Type: DEFAULT; Schema: twist_rocket; Owner: postgres
--

ALTER TABLE ONLY twist_rocket.messages ALTER COLUMN id SET DEFAULT nextval('twist_rocket.messages_id_seq'::regclass);


--
-- Name: tables table_id; Type: DEFAULT; Schema: twist_rocket; Owner: postgres
--

ALTER TABLE ONLY twist_rocket.tables ALTER COLUMN table_id SET DEFAULT nextval('twist_rocket.tables_table_id_seq'::regclass);


--
-- Name: bets bet_id; Type: DEFAULT; Schema: twist_roulette; Owner: postgres
--

ALTER TABLE ONLY twist_roulette.bets ALTER COLUMN bet_id SET DEFAULT nextval('twist_roulette.bets_bet_id_seq'::regclass);


--
-- Name: games game_id; Type: DEFAULT; Schema: twist_roulette; Owner: postgres
--

ALTER TABLE ONLY twist_roulette.games ALTER COLUMN game_id SET DEFAULT nextval('twist_roulette.games_game_id_seq'::regclass);


--
-- Name: tables table_id; Type: DEFAULT; Schema: twist_roulette; Owner: postgres
--

ALTER TABLE ONLY twist_roulette.tables ALTER COLUMN table_id SET DEFAULT nextval('twist_roulette.tables_table_id_seq'::regclass);


--
-- Name: actions_log actions_log_pkey; Type: CONSTRAINT; Schema: twist_admin; Owner: postgres
--

ALTER TABLE ONLY twist_admin.actions_log
    ADD CONSTRAINT actions_log_pkey PRIMARY KEY (id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: twist_admin; Owner: postgres
--

ALTER TABLE ONLY twist_admin.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: roulette_bots roulette_bots_auth_token_key; Type: CONSTRAINT; Schema: twist_bots; Owner: postgres
--

ALTER TABLE ONLY twist_bots.roulette_bots
    ADD CONSTRAINT roulette_bots_auth_token_key UNIQUE (auth_token);


--
-- Name: roulette_bots roulette_bots_pkey; Type: CONSTRAINT; Schema: twist_bots; Owner: postgres
--

ALTER TABLE ONLY twist_bots.roulette_bots
    ADD CONSTRAINT roulette_bots_pkey PRIMARY KEY (user_id);


--
-- Name: gift_deposits gift_deposits_pk; Type: CONSTRAINT; Schema: twist_business; Owner: postgres
--

ALTER TABLE ONLY twist_business.gift_deposits
    ADD CONSTRAINT gift_deposits_pk PRIMARY KEY (deposit_id);


--
-- Name: items items_pkey; Type: CONSTRAINT; Schema: twist_business; Owner: postgres
--

ALTER TABLE ONLY twist_business.items
    ADD CONSTRAINT items_pkey PRIMARY KEY (item_id);


--
-- Name: payments payments_pk; Type: CONSTRAINT; Schema: twist_business; Owner: postgres
--

ALTER TABLE ONLY twist_business.payments
    ADD CONSTRAINT payments_pk PRIMARY KEY (payment_id);


--
-- Name: tasks tasks_pkey; Type: CONSTRAINT; Schema: twist_business; Owner: postgres
--

ALTER TABLE ONLY twist_business.tasks
    ADD CONSTRAINT tasks_pkey PRIMARY KEY (task_id);


--
-- Name: user_task_progress user_task_progress_pkey; Type: CONSTRAINT; Schema: twist_business; Owner: postgres
--

ALTER TABLE ONLY twist_business.user_task_progress
    ADD CONSTRAINT user_task_progress_pkey PRIMARY KEY (user_id, task_id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: twist_business; Owner: postgres
--

ALTER TABLE ONLY twist_business.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (user_id);


--
-- Name: withdrawals withdrawals_pkey; Type: CONSTRAINT; Schema: twist_business; Owner: postgres
--

ALTER TABLE ONLY twist_business.withdrawals
    ADD CONSTRAINT withdrawals_pkey PRIMARY KEY (id);


--
-- Name: bets bets_pkey; Type: CONSTRAINT; Schema: twist_rocket; Owner: postgres
--

ALTER TABLE ONLY twist_rocket.bets
    ADD CONSTRAINT bets_pkey PRIMARY KEY (bet_id);


--
-- Name: games games_pkey; Type: CONSTRAINT; Schema: twist_rocket; Owner: postgres
--

ALTER TABLE ONLY twist_rocket.games
    ADD CONSTRAINT games_pkey PRIMARY KEY (game_id);


--
-- Name: messages messages_pkey; Type: CONSTRAINT; Schema: twist_rocket; Owner: postgres
--

ALTER TABLE ONLY twist_rocket.messages
    ADD CONSTRAINT messages_pkey PRIMARY KEY (id);


--
-- Name: tables tables_pkey; Type: CONSTRAINT; Schema: twist_rocket; Owner: postgres
--

ALTER TABLE ONLY twist_rocket.tables
    ADD CONSTRAINT tables_pkey PRIMARY KEY (table_id);


--
-- Name: tables tables_table_name_key; Type: CONSTRAINT; Schema: twist_rocket; Owner: postgres
--

ALTER TABLE ONLY twist_rocket.tables
    ADD CONSTRAINT tables_table_name_key UNIQUE (table_name);


--
-- Name: bets bets_pkey; Type: CONSTRAINT; Schema: twist_roulette; Owner: postgres
--

ALTER TABLE ONLY twist_roulette.bets
    ADD CONSTRAINT bets_pkey PRIMARY KEY (bet_id);


--
-- Name: games games_pkey; Type: CONSTRAINT; Schema: twist_roulette; Owner: postgres
--

ALTER TABLE ONLY twist_roulette.games
    ADD CONSTRAINT games_pkey PRIMARY KEY (game_id);


--
-- Name: tables tables_pkey; Type: CONSTRAINT; Schema: twist_roulette; Owner: postgres
--

ALTER TABLE ONLY twist_roulette.tables
    ADD CONSTRAINT tables_pkey PRIMARY KEY (table_id);


--
-- Name: tables tables_table_name_uniq; Type: CONSTRAINT; Schema: twist_roulette; Owner: postgres
--

ALTER TABLE ONLY twist_roulette.tables
    ADD CONSTRAINT tables_table_name_uniq UNIQUE (table_name);


--
-- Name: idx_twist_admin_users_deleted_at; Type: INDEX; Schema: twist_admin; Owner: postgres
--

CREATE INDEX idx_twist_admin_users_deleted_at ON twist_admin.users USING btree (deleted_at);


--
-- Name: idx_twist_admin_users_username; Type: INDEX; Schema: twist_admin; Owner: postgres
--

CREATE UNIQUE INDEX idx_twist_admin_users_username ON twist_admin.users USING btree (username);


--
-- Name: gift_deposits_created_at_idx; Type: INDEX; Schema: twist_business; Owner: postgres
--

CREATE INDEX gift_deposits_created_at_idx ON twist_business.gift_deposits USING btree (created_at DESC);


--
-- Name: gift_deposits_item_id_idx; Type: INDEX; Schema: twist_business; Owner: postgres
--

CREATE INDEX gift_deposits_item_id_idx ON twist_business.gift_deposits USING btree (item_id);


--
-- Name: gift_deposits_user_id_idx; Type: INDEX; Schema: twist_business; Owner: postgres
--

CREATE INDEX gift_deposits_user_id_idx ON twist_business.gift_deposits USING btree (user_id);


--
-- Name: idx_items_cost_ton; Type: INDEX; Schema: twist_business; Owner: postgres
--

CREATE INDEX idx_items_cost_ton ON twist_business.items USING btree (cost_ton);


--
-- Name: idx_items_type; Type: INDEX; Schema: twist_business; Owner: postgres
--

CREATE INDEX idx_items_type ON twist_business.items USING btree (type);


--
-- Name: idx_items_unlocked; Type: INDEX; Schema: twist_business; Owner: postgres
--

CREATE INDEX idx_items_unlocked ON twist_business.items USING btree (user_id) WHERE (locked = false);


--
-- Name: idx_items_user_locked; Type: INDEX; Schema: twist_business; Owner: postgres
--

CREATE INDEX idx_items_user_locked ON twist_business.items USING btree (user_id, locked);


--
-- Name: idx_tasks_active; Type: INDEX; Schema: twist_business; Owner: postgres
--

CREATE INDEX idx_tasks_active ON twist_business.tasks USING btree (task_active);


--
-- Name: idx_tasks_completed; Type: INDEX; Schema: twist_business; Owner: postgres
--

CREATE INDEX idx_tasks_completed ON twist_business.tasks USING btree (completed);


--
-- Name: idx_tasks_name; Type: INDEX; Schema: twist_business; Owner: postgres
--

CREATE INDEX idx_tasks_name ON twist_business.tasks USING btree (task_name);


--
-- Name: idx_tasks_start_msk; Type: INDEX; Schema: twist_business; Owner: postgres
--

CREATE INDEX idx_tasks_start_msk ON twist_business.tasks USING btree (task_start_at_msk);


--
-- Name: idx_users_last_active_at; Type: INDEX; Schema: twist_business; Owner: postgres
--

CREATE INDEX idx_users_last_active_at ON twist_business.users USING btree (last_active_at DESC);


--
-- Name: idx_users_ref_code; Type: INDEX; Schema: twist_business; Owner: postgres
--

CREATE INDEX idx_users_ref_code ON twist_business.users USING btree (ref_code);


--
-- Name: idx_utp_task; Type: INDEX; Schema: twist_business; Owner: postgres
--

CREATE INDEX idx_utp_task ON twist_business.user_task_progress USING btree (task_id);


--
-- Name: idx_utp_user; Type: INDEX; Schema: twist_business; Owner: postgres
--

CREATE INDEX idx_utp_user ON twist_business.user_task_progress USING btree (user_id);


--
-- Name: idx_withdrawals_completed; Type: INDEX; Schema: twist_business; Owner: postgres
--

CREATE INDEX idx_withdrawals_completed ON twist_business.withdrawals USING btree (completed_at DESC);


--
-- Name: idx_withdrawals_created; Type: INDEX; Schema: twist_business; Owner: postgres
--

CREATE INDEX idx_withdrawals_created ON twist_business.withdrawals USING btree (created_at DESC);


--
-- Name: idx_withdrawals_item_id; Type: INDEX; Schema: twist_business; Owner: postgres
--

CREATE INDEX idx_withdrawals_item_id ON twist_business.withdrawals USING btree (item_id);


--
-- Name: idx_withdrawals_parent; Type: INDEX; Schema: twist_business; Owner: postgres
--

CREATE INDEX idx_withdrawals_parent ON twist_business.withdrawals USING btree (parent_id);


--
-- Name: idx_withdrawals_status; Type: INDEX; Schema: twist_business; Owner: postgres
--

CREATE INDEX idx_withdrawals_status ON twist_business.withdrawals USING btree (status);


--
-- Name: idx_withdrawals_user_id; Type: INDEX; Schema: twist_business; Owner: postgres
--

CREATE INDEX idx_withdrawals_user_id ON twist_business.withdrawals USING btree (user_id);


--
-- Name: items_owned_gift_id_uq; Type: INDEX; Schema: twist_business; Owner: postgres
--

CREATE UNIQUE INDEX items_owned_gift_id_uq ON twist_business.items USING btree (owned_gift_id) WHERE (owned_gift_id IS NOT NULL);


--
-- Name: payments_created_at_idx; Type: INDEX; Schema: twist_business; Owner: postgres
--

CREATE INDEX payments_created_at_idx ON twist_business.payments USING btree (created_at);


--
-- Name: ux_users_username_notnull; Type: INDEX; Schema: twist_business; Owner: postgres
--

CREATE UNIQUE INDEX ux_users_username_notnull ON twist_business.users USING btree (username) WHERE (username IS NOT NULL);


--
-- Name: idx_rocket_bets_created_at; Type: INDEX; Schema: twist_rocket; Owner: postgres
--

CREATE INDEX idx_rocket_bets_created_at ON twist_rocket.bets USING btree (created_at DESC);


--
-- Name: idx_rocket_bets_game_id; Type: INDEX; Schema: twist_rocket; Owner: postgres
--

CREATE INDEX idx_rocket_bets_game_id ON twist_rocket.bets USING btree (game_id);


--
-- Name: idx_rocket_bets_game_outcome; Type: INDEX; Schema: twist_rocket; Owner: postgres
--

CREATE INDEX idx_rocket_bets_game_outcome ON twist_rocket.bets USING btree (game_id, outcome);


--
-- Name: idx_rocket_bets_user_game; Type: INDEX; Schema: twist_rocket; Owner: postgres
--

CREATE INDEX idx_rocket_bets_user_game ON twist_rocket.bets USING btree (user_id, game_id);


--
-- Name: idx_rocket_bets_user_id; Type: INDEX; Schema: twist_rocket; Owner: postgres
--

CREATE INDEX idx_rocket_bets_user_id ON twist_rocket.bets USING btree (user_id);


--
-- Name: idx_rocket_games_finished_at; Type: INDEX; Schema: twist_rocket; Owner: postgres
--

CREATE INDEX idx_rocket_games_finished_at ON twist_rocket.games USING btree (finished_at DESC);


--
-- Name: idx_rocket_games_table_phase_created; Type: INDEX; Schema: twist_rocket; Owner: postgres
--

CREATE INDEX idx_rocket_games_table_phase_created ON twist_rocket.games USING btree (table_id, game_phase, created_at DESC);


--
-- Name: idx_rocket_tables_name; Type: INDEX; Schema: twist_rocket; Owner: postgres
--

CREATE INDEX idx_rocket_tables_name ON twist_rocket.tables USING btree (table_name);


--
-- Name: idx_twist_rocket_messages_created_at; Type: INDEX; Schema: twist_rocket; Owner: postgres
--

CREATE INDEX idx_twist_rocket_messages_created_at ON twist_rocket.messages USING btree (created_at DESC);


--
-- Name: idx_bets_created_at; Type: INDEX; Schema: twist_roulette; Owner: postgres
--

CREATE INDEX idx_bets_created_at ON twist_roulette.bets USING btree (created_at DESC);


--
-- Name: idx_bets_game_id; Type: INDEX; Schema: twist_roulette; Owner: postgres
--

CREATE INDEX idx_bets_game_id ON twist_roulette.bets USING btree (game_id);


--
-- Name: idx_bets_game_outcome; Type: INDEX; Schema: twist_roulette; Owner: postgres
--

CREATE INDEX idx_bets_game_outcome ON twist_roulette.bets USING btree (game_id, outcome);


--
-- Name: idx_bets_user_game; Type: INDEX; Schema: twist_roulette; Owner: postgres
--

CREATE INDEX idx_bets_user_game ON twist_roulette.bets USING btree (user_id, game_id);


--
-- Name: idx_bets_user_id; Type: INDEX; Schema: twist_roulette; Owner: postgres
--

CREATE INDEX idx_bets_user_id ON twist_roulette.bets USING btree (user_id);


--
-- Name: idx_games_finished_at; Type: INDEX; Schema: twist_roulette; Owner: postgres
--

CREATE INDEX idx_games_finished_at ON twist_roulette.games USING btree (finished_at DESC);


--
-- Name: idx_games_table_phase_created; Type: INDEX; Schema: twist_roulette; Owner: postgres
--

CREATE INDEX idx_games_table_phase_created ON twist_roulette.games USING btree (table_id, game_phase, created_at DESC);


--
-- Name: idx_tables_name; Type: INDEX; Schema: twist_roulette; Owner: postgres
--

CREATE INDEX idx_tables_name ON twist_roulette.tables USING btree (table_name);


--
-- Name: tasks trg_tasks_before_update_limit; Type: TRIGGER; Schema: twist_business; Owner: postgres
--

CREATE TRIGGER trg_tasks_before_update_limit BEFORE UPDATE OF task_run_limit ON twist_business.tasks FOR EACH ROW EXECUTE FUNCTION twist_business.tasks_before_update_limit();


--
-- Name: user_task_progress trg_utp_after_delete; Type: TRIGGER; Schema: twist_business; Owner: postgres
--

CREATE TRIGGER trg_utp_after_delete AFTER DELETE ON twist_business.user_task_progress FOR EACH ROW EXECUTE FUNCTION twist_business.utp_after_delete();


--
-- Name: user_task_progress trg_utp_after_insert; Type: TRIGGER; Schema: twist_business; Owner: postgres
--

CREATE TRIGGER trg_utp_after_insert AFTER INSERT ON twist_business.user_task_progress FOR EACH ROW EXECUTE FUNCTION twist_business.utp_after_insert();


--
-- Name: gift_deposits gift_deposits_items_item_id_fk; Type: FK CONSTRAINT; Schema: twist_business; Owner: postgres
--

ALTER TABLE ONLY twist_business.gift_deposits
    ADD CONSTRAINT gift_deposits_items_item_id_fk FOREIGN KEY (item_id) REFERENCES twist_business.items(item_id) ON DELETE CASCADE;


--
-- Name: gift_deposits gift_deposits_users_user_id_fk; Type: FK CONSTRAINT; Schema: twist_business; Owner: postgres
--

ALTER TABLE ONLY twist_business.gift_deposits
    ADD CONSTRAINT gift_deposits_users_user_id_fk FOREIGN KEY (user_id) REFERENCES twist_business.users(user_id) ON DELETE SET NULL;


--
-- Name: payments payments_users_user_id_fk; Type: FK CONSTRAINT; Schema: twist_business; Owner: postgres
--

ALTER TABLE ONLY twist_business.payments
    ADD CONSTRAINT payments_users_user_id_fk FOREIGN KEY (user_id) REFERENCES twist_business.users(user_id);


--
-- Name: user_task_progress user_task_progress_task_id_fkey; Type: FK CONSTRAINT; Schema: twist_business; Owner: postgres
--

ALTER TABLE ONLY twist_business.user_task_progress
    ADD CONSTRAINT user_task_progress_task_id_fkey FOREIGN KEY (task_id) REFERENCES twist_business.tasks(task_id) ON DELETE CASCADE;


--
-- Name: user_task_progress user_task_progress_user_id_fkey; Type: FK CONSTRAINT; Schema: twist_business; Owner: postgres
--

ALTER TABLE ONLY twist_business.user_task_progress
    ADD CONSTRAINT user_task_progress_user_id_fkey FOREIGN KEY (user_id) REFERENCES twist_business.users(user_id) ON DELETE CASCADE;


--
-- Name: withdrawals withdrawals_item_id_fkey; Type: FK CONSTRAINT; Schema: twist_business; Owner: postgres
--

ALTER TABLE ONLY twist_business.withdrawals
    ADD CONSTRAINT withdrawals_item_id_fkey FOREIGN KEY (item_id) REFERENCES twist_business.items(item_id) ON DELETE RESTRICT;


--
-- Name: withdrawals withdrawals_parent_id_fkey; Type: FK CONSTRAINT; Schema: twist_business; Owner: postgres
--

ALTER TABLE ONLY twist_business.withdrawals
    ADD CONSTRAINT withdrawals_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES twist_business.withdrawals(id) ON DELETE CASCADE;


--
-- Name: withdrawals withdrawals_user_id_fkey; Type: FK CONSTRAINT; Schema: twist_business; Owner: postgres
--

ALTER TABLE ONLY twist_business.withdrawals
    ADD CONSTRAINT withdrawals_user_id_fkey FOREIGN KEY (user_id) REFERENCES twist_business.users(user_id) ON DELETE CASCADE;


--
-- Name: bets bets_game_id_fkey; Type: FK CONSTRAINT; Schema: twist_rocket; Owner: postgres
--

ALTER TABLE ONLY twist_rocket.bets
    ADD CONSTRAINT bets_game_id_fkey FOREIGN KEY (game_id) REFERENCES twist_rocket.games(game_id) ON DELETE CASCADE;


--
-- Name: games games_table_id_fkey; Type: FK CONSTRAINT; Schema: twist_rocket; Owner: postgres
--

ALTER TABLE ONLY twist_rocket.games
    ADD CONSTRAINT games_table_id_fkey FOREIGN KEY (table_id) REFERENCES twist_rocket.tables(table_id) ON DELETE RESTRICT;


--
-- Name: bets bets_game_fk; Type: FK CONSTRAINT; Schema: twist_roulette; Owner: postgres
--

ALTER TABLE ONLY twist_roulette.bets
    ADD CONSTRAINT bets_game_fk FOREIGN KEY (game_id) REFERENCES twist_roulette.games(game_id) ON DELETE CASCADE;


--
-- Name: games games_table_fk; Type: FK CONSTRAINT; Schema: twist_roulette; Owner: postgres
--

ALTER TABLE ONLY twist_roulette.games
    ADD CONSTRAINT games_table_fk FOREIGN KEY (table_id) REFERENCES twist_roulette.tables(table_id) ON DELETE RESTRICT;


--
-- PostgreSQL database dump complete
--

\unrestrict V8dZuc4fgrvmIyShPAwsfPJJiquSoSkrVzda6eaD7RWscyjwevZ0KuphFfZJgaJ

