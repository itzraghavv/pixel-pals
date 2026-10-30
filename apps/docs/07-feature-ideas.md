# 07 — Feature Ideas (backlog)

**Status:** Ideas only — **nothing here is committed scope.** Revisit after M4 (see [06](./06-roadmap.md)) and pick 1–3. Each chosen idea gets its own short PRD before any code is written.

The goal of these is to give Pixel Pals its own identity instead of being "a Gather clone", while exercising **new pieces of infrastructure** so each one adds portfolio value.

## How to choose

Score each candidate 1–5 on:

- **Uniqueness** — does it make Pixel Pals feel different from Gather?
- **Infra learning** — does it introduce something new (a queue, a CRDT, scheduled jobs, an LLM)?
- **Demo-ability** — can someone see it working in a 30-second GIF?
- **Effort** — inverse; lower effort scores higher.

## Candidates

### 1. Pals — pet companions

Each user has a small pixel pet that follows their avatar around. It levels up from time spent in spaces, focus sessions or chatting, and has moods. Other people can see it and interact with it ("pet", "feed").

- **Why it fits:** It's in the name and gives the world its own character.
- **Infra it exercises:** Persistent game state, a background worker for XP/mood ticks, event-driven updates over the realtime channel.
- **Rough size:** M

### 2. Focus rooms / co-working

Spaces can enable a shared Pomodoro timer. Avatars show a focus/break status, and focus time feeds streaks and a weekly leaderboard.

- **Why it fits:** It gives people a reason to come back daily (study groups, remote devs).
- **Infra it exercises:** A server-authoritative timer synced across clients, scheduled jobs (weekly reset), Redis sorted sets for leaderboards.
- **Rough size:** S–M

### 3. Pair-coding desks

Interactable desks that open a shared live code editor for the people sitting at them.

- **Why it fits:** It's a developer-focused twist and a strong technical showcase.
- **Infra it exercises:** CRDTs (Yjs) with a Go relay or `y-websocket` sidecar, document persistence, interactable-object protocol.
- **Rough size:** L

### 4. Space replay

Every realtime event is logged to a stream, so you can scrub a timeline and replay who moved where and what was said.

- **Why it fits:** It's unusual, visual, and great for a demo.
- **Infra it exercises:** Event sourcing, NATS JetStream or Redis Streams, retention policies, deterministic replay in the client.
- **Rough size:** M–L

### 5. AI room host (NPC)

An NPC in each space that greets newcomers, answers "who's here / what did I miss?" by summarising recent chat, and can run small icebreakers.

- **Why it fits:** It makes empty spaces feel alive.
- **Infra it exercises:** LLM API integration, async job queue, rate limiting and cost controls, NPCs as server-side "bot clients" in the room.
- **Rough size:** M

### 6. Mini-games in the world

Small multiplayer games played on tiles in the space (tag, a quiz corner, tic-tac-toe tables).

- **Why it fits:** It's social, fun, and plays to the pixel-art theme.
- **Infra it exercises:** A per-game state machine inside the room goroutine, tick-based updates, fairness and anti-cheat on the server.
- **Rough size:** M

## Notes

- Ideas 1 and 2 pair well (pets level up from focus time).
- Idea 4 depends on chat and movement events being well-defined. Keep the protocol in [02](./02-realtime-prd.md) versioned so it stays possible.
- Add new ideas here with the same template rather than expanding the core PRDs.
