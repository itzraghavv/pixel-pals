# 04 — Audio/Video & Chat PRD

**Status:** Draft · **Components:** `realtime`, `api`, LiveKit, `web`

## 1. Goals

- The signature feature: **walk up to someone and you can hear and see them; walk away and you can't.**
- Space-wide text chat.
- Learn WebRTC fundamentals (ICE, STUN/TURN, SFU vs mesh, simulcast) without hand-writing an SFU.

## 2. Non-goals

- Recording, transcription, large stage/broadcast mode.
- Building our own SFU (possible stretch learning exercise, not v1).

## 3. Approach

**Why an SFU and not peer-to-peer mesh:** mesh needs *n−1* uploads per user and falls over beyond ~5 people. An SFU (Selective Forwarding Unit) receives one upload per user and forwards to whoever should receive it.

**Why LiveKit:** open-source, self-hostable, written in Go (pion/webrtc under the hood), has a Go server SDK for token minting and a JS client SDK. It fits the stack and is free to run locally in Docker.

### Flow

```
1. Client joins space via realtime WS.
2. Realtime computes proximity groups (RT-11) → sends `proximity {groupId, peers}`.
3. Client calls  POST /api/v1/space/{id}/av-token  → API mints a LiveKit JWT
   for room "space:{spaceId}" with identity = userId.
4. Client connects to LiveKit once per space, publishes mic/cam.
5. Client subscribes ONLY to tracks of userIds in its current proximity group,
   unsubscribes when they leave the group.
```

One LiveKit room per space; proximity is enforced by **selective subscription**. A stricter variant (P1) has the server, not the client, enforce it via LiveKit's server API (`UpdateSubscriptions`) or per-group rooms so a modified client can't eavesdrop.

## 4. Requirements

### 4.1 Audio/video

| ID | Pri | Requirement |
|----|-----|-------------|
| AV-01 | P0 | `POST /api/v1/space/{id}/av-token` (member only) returns `{url, token}`; token TTL 1 h; grants publish/subscribe to `space:{id}` only. |
| AV-02 | P0 | Pre-join device check: pick mic/camera, preview, mute toggles. Joining works with no devices (listen-only). |
| AV-03 | P0 | Subscribe to tracks only for peers in current proximity group; video tiles bar shows them; tiles fade in/out on group change. |
| AV-04 | P0 | Mute/unmute mic and camera; state broadcast so others see a muted icon. |
| AV-05 | P1 | Spatial audio volume: gain falls off with tile distance inside the group. |
| AV-06 | P1 | Server-enforced subscriptions (see §3) so proximity can't be bypassed client-side. |
| AV-07 | P1 | Private areas (RT-12) produce isolated groups — people outside can't hear even if adjacent. |
| AV-08 | P1 | Simulcast + adaptive stream enabled; video capped at 360p in groups > 4. |
| AV-09 | P2 | Screen share to the current group. |
| AV-10 | P0 | TURN available (LiveKit's embedded TURN or coturn) so users behind symmetric NAT connect. |

### 4.2 Text chat

| ID | Pri | Requirement |
|----|-----|-------------|
| CHAT-01 | P1 | Space-wide chat over the realtime WS (`chat` message); broadcast to the room. |
| CHAT-02 | P1 | Persist last 200 messages per space; `GET /api/v1/space/{id}/messages?before=` for history. |
| CHAT-03 | P1 | Server sanitises/escapes; client renders as plain text with URL autolinking. |
| CHAT-04 | P2 | "Nearby" chat channel scoped to proximity group; direct messages. |

## 5. Acceptance criteria

- Two users ≤ 3 tiles apart hear each other within 1.5 s of becoming adjacent; at 5 tiles they stop receiving each other's tracks within 1 s.
- With AV-06, a client that ignores `proximity` and tries to subscribe to all tracks receives nothing outside its group.
- Works across two different networks (one on mobile hotspot) through TURN.

## 6. Open questions

- One LiveKit room per space vs. one per proximity group: per-space is simpler and avoids reconnects when groups change; per-group gives hard isolation. Start per-space + server-enforced subscriptions.
- Hosting: self-hosted LiveKit needs UDP ports open; LiveKit Cloud free tier is an alternative for the demo deployment.
