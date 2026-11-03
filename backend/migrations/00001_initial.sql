-- +goose Up
CREATE EXTENSION IF NOT EXISTS citext;
CREATE TABLE avatars (id text PRIMARY KEY, name text NOT NULL, image_url text NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE users (id text PRIMARY KEY, username citext NOT NULL UNIQUE, password_hash text NOT NULL, role text NOT NULL CHECK (role IN ('user','admin')), avatar_id text REFERENCES avatars(id), created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE refresh_tokens (id text PRIMARY KEY, user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE, token_hash text NOT NULL UNIQUE, expires_at timestamptz NOT NULL, revoked_at timestamptz);
CREATE INDEX ON refresh_tokens(user_id);
CREATE TABLE elements (id text PRIMARY KEY, width integer NOT NULL CHECK(width > 0), height integer NOT NULL CHECK(height > 0), static boolean NOT NULL, image_url text NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE maps (id text PRIMARY KEY, name text NOT NULL, width integer NOT NULL CHECK(width > 0), height integer NOT NULL CHECK(height > 0), thumbnail_url text NOT NULL DEFAULT '');
CREATE TABLE map_elements (id text PRIMARY KEY, map_id text NOT NULL REFERENCES maps(id) ON DELETE CASCADE, element_id text NOT NULL REFERENCES elements(id), x integer NOT NULL CHECK(x >= 0), y integer NOT NULL CHECK(y >= 0));
CREATE INDEX ON map_elements(map_id);
CREATE TABLE spaces (id text PRIMARY KEY, name text NOT NULL, width integer NOT NULL CHECK(width > 0), height integer NOT NULL CHECK(height > 0), thumbnail_url text NOT NULL DEFAULT '', creator_id text NOT NULL REFERENCES users(id), created_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX ON spaces(creator_id);
CREATE TABLE space_elements (id text PRIMARY KEY, space_id text NOT NULL REFERENCES spaces(id) ON DELETE CASCADE, element_id text NOT NULL REFERENCES elements(id), x integer NOT NULL CHECK(x >= 0), y integer NOT NULL CHECK(y >= 0));
CREATE INDEX ON space_elements(space_id);
CREATE TABLE space_members (space_id text NOT NULL REFERENCES spaces(id) ON DELETE CASCADE, user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE, role text NOT NULL CHECK(role IN ('owner','member')), PRIMARY KEY(space_id,user_id));
CREATE TABLE space_invites (token_hash text PRIMARY KEY, space_id text NOT NULL REFERENCES spaces(id) ON DELETE CASCADE, expires_at timestamptz NOT NULL);
CREATE TABLE chat_messages (id text PRIMARY KEY, space_id text NOT NULL REFERENCES spaces(id) ON DELETE CASCADE, user_id text NOT NULL REFERENCES users(id), body text NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX ON chat_messages(space_id,created_at);
-- Transactional outbox: mutations remain deliverable if Redis is unavailable.
CREATE TABLE space_events (id bigserial PRIMARY KEY, space_id text NOT NULL, created_at timestamptz NOT NULL DEFAULT now());

-- +goose Down
DROP TABLE space_events, chat_messages, space_invites, space_members, space_elements, spaces, map_elements, maps, elements, refresh_tokens, users, avatars;
DROP EXTENSION IF EXISTS citext;
