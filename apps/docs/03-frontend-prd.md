# 03 — Frontend PRD

**Status:** Draft · **App:** `web` · **Stack:** Next.js (App Router) + TypeScript + Phaser 3

## 1. Goals

- A polished, responsive web client that makes the backend demoable.
- Smooth 60 fps world rendering with client-side prediction and server reconciliation.
- A simple editor for placing elements, and an admin console for the catalogue.

## 2. Non-goals

- Mobile-first touch controls (P2: on-screen D-pad).
- Offline mode, PWA install.

## 3. Information architecture

| Route | Auth | Purpose |
|-------|------|---------|
| `/` | public | Landing page: pitch, screenshots, "Get started" |
| `/signin`, `/signup` | public | Auth forms |
| `/onboarding/avatar` | user | Pick avatar (first login, also from settings) |
| `/dashboard` | user | My spaces grid, "New space" dialog (template picker), invites |
| `/space/[id]` | member | The world: canvas + A/V tiles + chat + participant list |
| `/space/[id]/edit` | owner | Editor: element palette, place/remove, save |
| `/admin` | admin | Avatars, elements, maps (with map builder) |
| `/invite/[token]` | user | Accept invite → redirect to space |

## 4. Architecture

- **Next.js** for routing, auth-aware layouts and SSR of non-game pages. The world page is a client component.
- **Phaser 3** mounted inside a React component for the world and editor canvases. React owns UI chrome (sidebar, chat, A/V tiles); Phaser owns the tilemap and sprites.
- **Bridge:** a typed event emitter between React and Phaser (`world.on("player-moved")`, `world.emit("focus-user", id)`).
- **Server state:** TanStack Query with an API client generated from the backend OpenAPI spec (`openapi-typescript` + `openapi-fetch`).
- **Client state:** Zustand stores — `session`, `room` (players, self, proximity group), `media` (mic/cam state, tracks).
- **Realtime client:** `RealtimeClient` class wrapping WebSocket: typed messages, auto-reconnect with exponential backoff + jitter, re-`join` on reconnect, heartbeat.
- **Auth:** access token kept in memory; refresh via HttpOnly cookie on load and on `401`. Route protection via middleware + layout checks.
- **Styling:** Tailwind + shadcn/ui; pixel-art font for in-world labels.

## 5. Requirements

### 5.1 Auth & onboarding

| ID | Pri | Requirement |
|----|-----|-------------|
| FE-AUTH-01 | P0 | Sign up / sign in forms with client validation (zod) and server error display. |
| FE-AUTH-02 | P0 | Authenticated routes redirect to `/signin?next=…`. |
| FE-AUTH-03 | P1 | Silent token refresh; logout clears memory token and cookie. |
| FE-AUTH-04 | P0 | Avatar picker shows all avatars; selection persisted via `POST /user/metadata`. |

### 5.2 Dashboard

| ID | Pri | Requirement |
|----|-----|-------------|
| FE-DASH-01 | P0 | Grid of my spaces with thumbnail, name, dimensions; "Enter", "Edit", "Delete" (confirm dialog). |
| FE-DASH-02 | P0 | "New space" dialog: choose map template or blank (width/height inputs), name. |
| FE-DASH-03 | P1 | Copy invite link; list spaces I was invited to. |
| FE-DASH-04 | P0 | Empty, loading and error states for every list. |

### 5.3 World (space view)

| ID | Pri | Requirement |
|----|-----|-------------|
| FE-WORLD-01 | P0 | Render the grid (32 px tiles), all elements at their positions, own avatar and other players with name labels. |
| FE-WORLD-02 | P0 | Arrow keys / WASD move one tile; hold to repeat at ~6 tiles/s. |
| FE-WORLD-03 | P0 | **Client-side prediction:** move locally immediately; on `movement-rejected` snap to the server position. Predict collisions locally using the same static-element grid. |
| FE-WORLD-04 | P0 | **Interpolation:** remote players tween between tiles (~120 ms) instead of teleporting. |
| FE-WORLD-05 | P0 | Camera follows self; zoom in/out; map bounds clamp. |
| FE-WORLD-06 | P0 | Participants sidebar (name, avatar, "nearby" badge); click to locate. |
| FE-WORLD-07 | P0 | Connection status pill (connected / reconnecting / offline); input disabled while disconnected. |
| FE-WORLD-08 | P1 | Walking animation with 4 facing directions (sprite sheets from avatar catalogue). |
| FE-WORLD-09 | P1 | Proximity ring around self; nearby players highlighted. |
| FE-WORLD-10 | P1 | Text chat panel (space-wide), unread badge, enter to send. |
| FE-WORLD-11 | P2 | Emote reactions above avatars; minimap. |
| FE-WORLD-12 | P2 | Click-to-move with client-side A* pathfinding sending one `move` per step. |

### 5.4 Editor

| ID | Pri | Requirement |
|----|-----|-------------|
| FE-EDIT-01 | P0 | Palette of elements from `GET /elements`; drag onto the grid snaps to tiles; ghost preview shows valid/invalid placement. |
| FE-EDIT-02 | P0 | Select a placed element and delete it. |
| FE-EDIT-03 | P1 | Undo/redo for local edits; batch save. |
| FE-EDIT-04 | P1 | Paint "private area" zones (ties to RT-12). |
| FE-EDIT-05 | P2 | Generate thumbnail from canvas on save. |

### 5.5 Admin console

| ID | Pri | Requirement |
|----|-----|-------------|
| FE-ADM-01 | P0 | CRUD-ish screens for avatars and elements with image upload (presigned URL flow). |
| FE-ADM-02 | P1 | Map builder reusing the editor component, saving as a map template. |

### 5.6 Quality

| ID | Pri | Requirement |
|----|-----|-------------|
| FE-Q-01 | P0 | TypeScript strict; ESLint + Prettier in CI. |
| FE-Q-02 | P1 | Unit tests (Vitest) for realtime client, prediction/reconciliation and stores. |
| FE-Q-03 | P1 | Playwright E2E: sign up → create space → join from two browser contexts → see each other move. |
| FE-Q-04 | P1 | Lighthouse ≥ 90 on landing and dashboard; keyboard navigable UI chrome. |

## 6. Acceptance criteria

- Two users in different browsers see each other move within 100 ms locally, with no visible jitter.
- Disconnecting the network for 10 s then reconnecting resumes the session automatically.
- Editor changes appear for users already in the space (via `space-updated`).

## 7. Open questions

- Phaser vs. PixiJS: Phaser chosen for built-in tilemaps, input and camera; revisit if bundle size becomes an issue (lazy-load the world route).
- Art assets: use an open licence pixel-art pack (e.g. from itch.io / OpenGameArt) and credit it in the README.
