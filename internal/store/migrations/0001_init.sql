CREATE TABLE users (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  email         TEXT NOT NULL UNIQUE,
  name          TEXT NOT NULL DEFAULT '',
  password_hash TEXT NOT NULL DEFAULT '',
  role          TEXT NOT NULL DEFAULT 'member',
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at    TEXT
);

CREATE TABLE sessions (
  id         TEXT PRIMARY KEY,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE api_tokens (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name         TEXT NOT NULL DEFAULT '',
  token_hash   TEXT NOT NULL UNIQUE,
  prefix       TEXT NOT NULL DEFAULT '',
  last_used_at TEXT,
  created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

-- Box-wide settings (title, setup flag); everything else is personal.
CREATE TABLE settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL DEFAULT ''
);

CREATE TABLE user_settings (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  key     TEXT NOT NULL,
  value   TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (user_id, key)
);

CREATE TABLE cache (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL DEFAULT '',
  fetched_at TEXT NOT NULL
);



-- ---------- Pocket Capsule ----------
-- A capsule belongs to the person who made it (capsules.user_id). Others
-- reach it only as members (people on this box who add to it or open it
-- with you), as recipients (anyone with an email, through their own link)
-- or through its "add something" link (people without an account). Box
-- admins have no way in to a capsule they are not in.
--
-- Sealed means sealed: while a capsule is sealed nothing in it is shown to
-- anyone — its maker included — until the day it opens (or its maker breaks
-- the seal early, which stays on record). Times are UTC ISO strings.

CREATE TABLE capsules (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,  -- the maker
  title         TEXT NOT NULL,
  about         TEXT NOT NULL DEFAULT '',        -- written on the outside, seen while sealed
  template      TEXT NOT NULL DEFAULT 'self',
  emoji         TEXT NOT NULL DEFAULT '⏳',
  color         TEXT NOT NULL DEFAULT '',
  state         TEXT NOT NULL DEFAULT 'filling', -- filling | sealed | opened
  opens_at      TEXT NOT NULL,                   -- when it may be opened
  open_rule     TEXT NOT NULL DEFAULT 'date',    -- date | birthday | anniversary
  for_name      TEXT NOT NULL DEFAULT '',        -- whose birthday / anniversary
  for_date      TEXT NOT NULL DEFAULT '',        -- their birth or wedding date (YYYY-MM-DD)
  for_age       INTEGER NOT NULL DEFAULT 0,      -- open on their Nth birthday / anniversary
  surprise      INTEGER NOT NULL DEFAULT 1,      -- what others add stays hidden until the opening
  slot          INTEGER NOT NULL DEFAULT 1,      -- things can still be dropped in after sealing (unseen)
  deliver       TEXT NOT NULL DEFAULT 'on_open', -- recipients get it when it is opened | on the day itself
  remind_days   INTEGER NOT NULL DEFAULT 7,      -- a reminder this many days before (0 = none)
  sections      TEXT NOT NULL DEFAULT '[]',      -- what of "the world today" is kept on sealing
  snapshot      TEXT NOT NULL DEFAULT '{}',      -- the world on the day it was sealed
  sealed_at     TEXT,
  opened_at     TEXT,
  opened_by     INTEGER REFERENCES users(id) ON DELETE SET NULL,
  early         INTEGER NOT NULL DEFAULT 0,      -- the seal was broken before the day
  early_reason  TEXT NOT NULL DEFAULT '',
  ready_sent    INTEGER NOT NULL DEFAULT 0,
  remind_sent   INTEGER NOT NULL DEFAULT 0,
  delivered_at  TEXT,
  reflection    TEXT NOT NULL DEFAULT '',        -- the letter back, written after opening
  then_now      TEXT NOT NULL DEFAULT '',        -- "then and now", written at the opening
  reply_to      INTEGER REFERENCES capsules(id) ON DELETE SET NULL,
  give_key      TEXT NOT NULL UNIQUE,            -- the "add something" link for people without an account
  give_open     INTEGER NOT NULL DEFAULT 0,
  calendar_url  TEXT NOT NULL DEFAULT '',
  source        TEXT NOT NULL DEFAULT '',        -- the app that made it (link token), '' here
  ext_id        TEXT NOT NULL DEFAULT '',        -- for idempotent imports
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX capsules_user ON capsules(user_id, state, opens_at);
CREATE INDEX capsules_due ON capsules(state, opens_at);
CREATE UNIQUE INDEX capsules_ext ON capsules(user_id, ext_id) WHERE ext_id != '';

-- People on this box in a capsule: contributors add to it, viewers only
-- open it with you.
CREATE TABLE members (
  capsule_id INTEGER NOT NULL REFERENCES capsules(id) ON DELETE CASCADE,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role       TEXT NOT NULL DEFAULT 'contributor',  -- contributor | viewer
  added_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  PRIMARY KEY (capsule_id, user_id)
);
CREATE INDEX members_user ON members(user_id);

-- Who it is for, by email: each gets their own link (/o/<key>) that shows a
-- countdown until it opens and the whole capsule after.
CREATE TABLE recipients (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,   -- the capsule's maker
  capsule_id  INTEGER NOT NULL REFERENCES capsules(id) ON DELETE CASCADE,
  name        TEXT NOT NULL DEFAULT '',
  email       TEXT NOT NULL DEFAULT '',
  key         TEXT NOT NULL UNIQUE,
  sent_at     TEXT,
  viewed_at   TEXT,
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX recipients_capsule ON recipients(capsule_id);
CREATE INDEX recipients_user ON recipients(user_id);

-- Pictures, recordings and files, kept in the database: a capsule has to
-- outlive whatever app they came from.
CREATE TABLE uploads (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,   -- who put it up (a guest's: the capsule's maker)
  capsule_id  INTEGER REFERENCES capsules(id) ON DELETE CASCADE,
  name        TEXT NOT NULL DEFAULT '',
  mime        TEXT NOT NULL DEFAULT '',
  bytes       INTEGER NOT NULL DEFAULT 0,
  width       INTEGER NOT NULL DEFAULT 0,
  height      INTEGER NOT NULL DEFAULT 0,
  data        BLOB NOT NULL,
  thumb       BLOB,
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX uploads_user ON uploads(user_id);
CREATE INDEX uploads_capsule ON uploads(capsule_id);

-- What is inside. An item keeps its author's name, so a guest's letter or
-- one from someone whose account was removed still reads right.
CREATE TABLE items (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  capsule_id   INTEGER NOT NULL REFERENCES capsules(id) ON DELETE CASCADE,
  user_id      INTEGER REFERENCES users(id) ON DELETE SET NULL,          -- the author (NULL: a guest)
  author_name  TEXT NOT NULL DEFAULT '',
  kind         TEXT NOT NULL,          -- letter | photo | audio | video | file | prediction | question | goal | list | link
  title        TEXT NOT NULL DEFAULT '',
  body         TEXT NOT NULL DEFAULT '',
  upload_id    INTEGER REFERENCES uploads(id) ON DELETE SET NULL,
  confidence   INTEGER NOT NULL DEFAULT 0,       -- a prediction: how sure, 0–100
  outcome      TEXT NOT NULL DEFAULT '',         -- after opening: yes | partly | no (prediction, goal)
  answer       TEXT NOT NULL DEFAULT '',         -- after opening: the answer to a question, a note on anything
  entries      TEXT NOT NULL DEFAULT '[]',       -- a list: [{label, value}]
  url          TEXT NOT NULL DEFAULT '',
  from_app     TEXT NOT NULL DEFAULT '',         -- notes | photos | voice | … when it came from another app
  from_url     TEXT NOT NULL DEFAULT '',
  after_seal   INTEGER NOT NULL DEFAULT 0,       -- dropped in through the slot
  guest        INTEGER NOT NULL DEFAULT 0,
  position     INTEGER NOT NULL DEFAULT 0,
  source       TEXT NOT NULL DEFAULT '',
  created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX items_capsule ON items(capsule_id, position, id);
CREATE INDEX items_user ON items(user_id);

-- What happened to a capsule: made, sealed, something dropped in, opened…
CREATE TABLE history (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  capsule_id  INTEGER NOT NULL REFERENCES capsules(id) ON DELETE CASCADE,
  user_id     INTEGER REFERENCES users(id) ON DELETE SET NULL,
  who         TEXT NOT NULL DEFAULT '',
  what        TEXT NOT NULL,          -- made | sealed | added | opened | early | reminded | ready | delivered | reflected | member | recipient
  detail      TEXT NOT NULL DEFAULT '',
  at          TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX history_capsule ON history(capsule_id, at);
CREATE INDEX history_at ON history(at);

-- Yearly capsules: on a day of the year, a nudge to write the next one.
CREATE TABLE rituals (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title       TEXT NOT NULL,
  template    TEXT NOT NULL DEFAULT 'self',
  month       INTEGER NOT NULL,
  day         INTEGER NOT NULL,
  years       INTEGER NOT NULL DEFAULT 1,   -- the capsule opens this many years later
  active      INTEGER NOT NULL DEFAULT 1,
  last_year   INTEGER NOT NULL DEFAULT 0,   -- the year the nudge last went
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX rituals_user ON rituals(user_id);
