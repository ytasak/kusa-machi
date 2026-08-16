# Anonymous Matching MVP — Claude Code Handoff Spec

## 0. Goal

Build a lightweight anonymous matching simulation game that can run inside the `kusa` SNS iframe, while also being usable directly from its own URL.

Core concept:

- Every day, each participant receives one randomly generated fictional persona.
- The persona's "status" attributes are fixed for that day.
- The participant may edit only lightweight self-expression fields.
- Participants browse other personas and spend a limited daily Like budget.
- Mutual Likes create a Match.
- All game data is ephemeral and resets daily at 00:00 JST.
- No historical progression, ranking, chat, or persistent profile.

Primary experience:
1. `Matching market simulation`
2. Daily persona gacha as the entry experience
3. Roleplay is optional and left to the user

This is a private hobby MVP. Prefer simple, explicit implementation over extensibility or overengineering.

---

# 1. Tech Stack

## Backend
- Go
- HTTP router: `chi`
- PostgreSQL
- SQL access: `sqlc`
- Migration: `golang-migrate`

## Frontend
- Svelte + Vite
- CSS Modules

## Deployment
- Single-container deployment is acceptable
- Frontend and API must be served from the same Origin
- App must work:
  - inside kusa iframe
  - when opened directly by URL

---

# 2. Product Principles

## 2.1 Daily ephemerality

All game data is scoped to one calendar day.

At 00:00 JST:
- today's Persona expires
- Likes expire
- Passes expire
- Matches expire
- participant-entered name/hobby/bio expire
- previous-day game data becomes immediately inaccessible
- old data is physically deleted asynchronously

No:
- history
- cumulative stats
- streaks
- rankings
- previous personas
- previous matches
- historical Like counts

## 2.2 No account system

kusa does not provide a user identity to the iframe app.

Identify the browser with an anonymous Cookie.

Cookie deletion may allow a fresh participant identity. This is acceptable for MVP; no anti-reroll protection is required.

---

# 3. User Identity Model

Three concepts must remain separate.

## `cookie_token`
Browser identifier.

- opaque random ID
- UUID v4 or equivalent
- stored only in Cookie
- approximately 30-day expiry
- no game state stored in Cookie

Cookie attributes:
- `HttpOnly`
- `Secure`
- `SameSite=None`
- `Path=/`
- no explicit `Domain`

## `Participant`
Technical participant for one game day.

Relationship:

```text
cookie_token -> Participant -> Persona
```

One Participant per `cookie_token + game_date`.

Participant is not itself visible in the game.

## `Persona`
The fictional person used in the matching market.

All game interactions use `persona_id`, not Participant or Cookie identity.

Like / Pass / Match actors are Personas.

---

# 4. Daily Lifecycle

Timezone: **Asia/Tokyo**

## First access of the day

1. Read or issue anonymous Cookie.
2. Ensure today's Participant exists.
3. If today's Persona does not exist:
   - show "新しい人生を始める"
4. When pressed:
   - server generates Persona once
   - save immediately
   - Persona becomes publicly discoverable immediately
5. Show a short 1–2 second generation animation.
6. Reveal all generated attributes at once.

Persona generation API must be idempotent:
- no Persona today → generate
- already generated → return existing Persona
- never reroll during the same day

## End of day

Home shows a permanent countdown such as:

```text
今日の人生 残り 03:42:18
```

At 23:55, if the app is open, show an in-app modal:

```text
今日の人生はあと5分です
```

At 00:00:
- current day is immediately invalid
- open clients show:
  - `今日の人生が終了しました`
- user must press `新しい人生を始める` to create the next day's Persona

Do not depend on a 00:00 deletion job for correctness.
All queries must scope data by `game_date = today JST`.

---

# 5. Persona Model

## 5.1 System-generated immutable attributes (A)

Generated once per day and immutable:

- age
- gender
- height
- education
- occupation
- annual income

## 5.2 User-editable attributes (B)

Editable any number of times during the day:

- name
- hobby
- bio / one-line message
- profile picture （2026-08-16 追加。§23 の Non-Goal から外した）

Limits:
- name: max 20 chars
- hobby: max 30 chars
- bio: max 60 chars

Rules:
- single-line only
- no newline
- trim leading/trailing whitespace
- whitespace-only becomes unset/null
- plain text only
- no Markdown
- no HTML rendering
- escape output
- explicit URLs (`http://`, `https://`) are prohibited
- do not attempt to detect phone numbers, email addresses, SNS IDs, or all possible external-contact variants
- no NG-word filter in MVP

If name/hobby/bio is unset, omit the field entirely from the rendered Persona card.

## 5.3 Profile picture

Optional. When unset, the card shows a plain default silhouette.

Client:
- center-crop to a square and scale to at most 1024px before uploading
- this only keeps uploads small; it is never a security control

Server (the only thing that is trusted):
- accept JPEG and PNG, reject everything else
- reject bodies over 8MB and images over 40 megapixels, the pixel check running
  on the header before the image is decoded
- always decode and re-encode as JPEG. This is what strips EXIF — an anonymous
  app must never publish someone's GPS coordinates — and discards anything
  smuggled alongside the pixels
- store the long edge at 1024px

Storage and lifetime:
- files under `PHOTO_DIR`, one directory per `game_date`
- the daily cleanup removes directories older than today, so pictures expire
  with the rest of the day's data
- pictures are served with `Content-Type: image/jpeg` and
  `X-Content-Type-Options: nosniff`, and only for today's personas
- a participant may delete their own picture at any time

Known limitation, accepted for this MVP: there is no reporting flow and no
moderation, so an uploaded picture is only removed by its owner or by the
daily reset.

---

# 6. Persona Card Display Order

0. profile picture (default silhouette when unset)
1. name (only when set)
2. age + gender
3. height
4. occupation
5. annual income
6. education
7. hobby (only when set)
8. bio (only when set)

Public Persona cards must not show:
- Like count
- Match count
- popularity rank
- rarity
- overall score
- exposure count
- Participant ID
- Cookie identifier

Use one shared Persona card UI component across:
- Discover
- Received Likes
- Sent Likes
- Matches

Only actions/badges differ by screen.

---

# 7. Persona Generation

Generation order:

```text
age
 -> gender
 -> height
 -> education
 -> occupation
 -> annual_income
```

The goal is **minimum plausibility constraints**, not realistic demographic simulation.

## 7.1 Age

Range: 20–50

Weighted age bands:

| Range | Weight |
|---|---:|
| 20–24 | 20% |
| 25–29 | 25% |
| 30–34 | 20% |
| 35–39 | 15% |
| 40–44 | 10% |
| 45–50 | 10% |

Process:
1. choose band by weight
2. choose age uniformly inside the band

## 7.2 Gender

- male: 50%
- female: 50%

No gender filtering in MVP.

## 7.3 Height

Uniform random:

```text
140–200 cm
```

1 cm increments.

No gender adjustment.

## 7.4 Education

| Education | Weight |
|---|---:|
| 中卒 | 5% |
| 高卒 | 20% |
| 専門卒 | 15% |
| 短大卒 | 10% |
| 大卒 | 35% |
| 大学院卒 | 10% |
| ホイ卒 | 5% |

Age restrictions:
- 大卒: age >= 22
- 大学院卒: age >= 24
- all others: age >= 20

Exclude invalid candidates and renormalize weights.

### Special rule: ホイ卒
`ホイ卒` is an intentional kusa in-joke / rare meme education value.

If education is `ホイ卒`, occupation restrictions are ignored.

## 7.5 Occupation

| Occupation | Weight |
|---|---:|
| 公務員 | 7% |
| 医師 | 2% |
| 看護師 | 6% |
| 教員 | 6% |
| ITエンジニア | 10% |
| 営業 | 10% |
| 事務 | 10% |
| 販売・接客 | 10% |
| 飲食 | 8% |
| 建設 | 8% |
| クリエイター | 6% |
| 自営業 | 6% |
| 経営者 | 2% |
| フリーター | 5% |
| 無職 | 4% |

Restrictions:
- 医師:
  - age >= 24
  - education in {大卒, 大学院卒}
- 教員:
  - age >= 22
  - education in {大卒, 大学院卒}
- 経営者:
  - age >= 25
- 看護師:
  - no additional restriction
- others:
  - no additional restriction
- if education == `ホイ卒`:
  - ignore all occupation restrictions

Exclude invalid candidates and renormalize.

## 7.6 Annual Income

Store unit: **万円**

All values must be in 10万円 increments.

| Occupation | Range (万円) |
|---|---:|
| 公務員 | 300–750 |
| 医師 | 700–1800 |
| 看護師 | 300–750 |
| 教員 | 300–750 |
| ITエンジニア | 300–900 |
| 営業 | 300–900 |
| 事務 | 250–550 |
| 販売・接客 | 250–550 |
| 飲食 | 250–550 |
| 建設 | 300–900 |
| クリエイター | 200–1200 |
| 自営業 | 200–1200 |
| 経営者 | 300–3000 |
| フリーター | 100–300 |
| 無職 | 0–100 |

Age adjustment is intentionally weak:
- 20s: slightly bias lower-to-middle
- 30s: roughly flat
- 40–50: slightly bias middle-to-upper

Extreme combinations must still remain possible.

Do not overengineer this. A simple skewed RNG is sufficient.

No rarity or "good/bad life" score is ever calculated.

---

# 8. Matching Rules

## Daily Like budget

Each Persona has exactly:

```text
10 Likes / day
```

Rules:
- Pass is unlimited
- Like is consumed immediately
- Like cannot be revoked
- unused Likes do not carry over
- Like budget resets with the new day
- Like limit must be enforced server-side in a transaction
- duplicate request must not double-consume Like
- same target may only be Liked once per day
- self-Like is forbidden

Received-Like reply consumes the same shared 10-Like budget.

There is no separate reply quota.

## Match

Mutual Like creates one Match.

Match:
- is unordered
- one Match per Persona pair
- must be idempotent
- stored explicitly rather than calculated every time

On a successful mutual Like:
- Like API returns `matched = true`
- frontend shows Match animation
- Match appears in Match list

No chat or DM in MVP.

---

# 9. Pass Rules

Pass is not permanent on first use.

A previously Passed Persona may be shown again.

Rules:
- maintain `pass_count` per directed Persona pair
- 1st Pass → may return later
- 2nd Pass → may return later
- 3rd Pass → excluded for the rest of the day
- self-Pass forbidden
- once target is Liked/Matched, no more Pass actions

Cooldown:
- a just-Passed Persona should not appear again within the next 5 displayed/evaluated cards
- for MVP, this 5-card cooldown can be maintained in frontend session state
- page reload may lose this cooldown; acceptable
- `pass_count = 3` is server-side and persistent for the day

---

# 10. Discover / Market Exposure

Discover UI:
- one Persona card at a time
- Like button
- Pass button
- no swipe gestures in MVP
- no confirmation dialog
- Pass transitions quickly
- Like gets a short `LIKE` feedback animation
- Match transitions directly to Match animation

Always show:
- remaining Likes `N / 10`
- small badge for number of received Likes

Do not show Match count on Discover screen.

## Candidate selection

`GET /api/discover` returns at most 5 Personas at a time.

Selection constraints:
- not self
- current `game_date`
- Persona exists and is active
- not already Liked by requester
- not already Matched
- `pass_count < 3`
- honor frontend-provided cooldown exclusion IDs when present
- no duplicate Persona within one response
- previous batch overlap is allowed

Priority:
1. lower `exposure_count`
2. random order among similarly exposed candidates

## Exposure counting

Do **not** increment exposure when Discover API returns the batch.

Increment:

```text
exposure_count += 1
```

only after user confirms:
- Like
- or Pass

Thus, exposure represents an actually evaluated profile.

The frontend should prefetch the next batch automatically when current batch is nearly exhausted / exhausted.

Batch boundary should not be visible to the user.

---

# 11. Received Likes

Screen:
- list all Personas who Liked the current Persona
- newest first
- show full public Persona card
- no Like timestamp
- no sender Like-budget information

User may Like back.

Like-back:
- consumes one of the same daily 10 Likes
- mutual Like creates Match

Home:
- show received Like count
- if unseen Likes exist, prominently show:
  - `新しいLikeがあります`
- badge clears when Received Likes screen is opened

Real-time transport is not required.
State refreshes on navigation / request.

---

# 12. Sent Likes

Screen:
- list all Personas current Persona Liked
- newest first
- Like cannot be revoked
- matched targets remain in the list
- matched targets receive a `MATCH` badge

This screen acts as the day's Like-allocation history.

All data disappears at 00:00.

---

# 13. Matches

Screen:
- list matched counterpart Personas
- newest first
- show only counterpart Persona card
- do not repeat user's own Persona
- read-only

Home:
- show today's Match count
- unseen Match causes prominent:
  - `新しいMatchがあります！`
- badge clears when Match screen is opened

Match creation animation:
- show own Persona and matched Persona together
- short `MATCH!`
- copy may include:
  - `今日の人生でマッチしました`

No chat / DM.

---

# 14. Navigation and マイページ

> **Revised.** This section originally read "Home is the navigation hub. No
> persistent bottom tab navigation." That was reversed on 2026-08-16: a
> persistent bottom tab bar is easier to reach one-handed on a phone, which is
> the only form factor that matters here. The Home screen is replaced by
> マイページ, and the countdown moves to a persistent header so it stays visible
> on every screen.

## Persistent chrome

A header is shown on every screen once today's Persona exists:

1. daily countdown
2. remaining Likes `N / 10`

A bottom tab bar is shown on every screen once today's Persona exists.
Tabs, in navigation priority order:

1. 探す
2. Likeされた
3. Match
4. マイページ

Unseen Likes and unseen Matches put a dot on their tab.

Before today's Persona exists, the "新しい人生を始める" screen takes over the
whole app: no header, no tab bar.

## マイページ

Primary information:
1. today's own Persona card
2. remaining Likes `N / 10`
3. received Like count
4. today's Match count

Pushed screens, reached from マイページ and providing a route back to it:

- 送信済みLike
- プロフィール編集

---

# 15. Profile Edit Screen

Top:
- show immutable A attributes read-only

Editable below:
- name
- hobby
- bio

All three are optional.
Changes immediately affect public Persona profile.

No "publish" state.
Persona is publicly visible immediately after generation, even if all B fields are empty.

---

# 16. Main Screens

MVP has exactly these main screens:

1. Discover （tab 探す）
2. Received Likes （tab Likeされた）
3. Matches （tab Match）
4. マイページ （tab; replaces the original Home screen — see §14）
5. Sent Likes （pushed from マイページ）
6. Profile Edit （pushed from マイページ）

Supporting UI / modal states:
- Start New Life
- Persona generation animation
- Match animation
- 23:55 five-minute warning
- 00:00 Life Ended

---

# 17. Frontend State Behavior

Discover fetches batches of 5.

Within the current page session:
- retain current Discover card
- retain fetched batch
- retain batch position
- retain local 5-card Pass cooldown exclusion list

If user moves Discover → Received Likes → Discover:
- resume from the previous card/batch

On full reload/browser restart:
- Discover in-memory state may be lost
- refetch
- unacted Persona may reappear
- this is acceptable

Server state remains authoritative for:
- Likes
- Pass counts
- Matches
- Persona
- Like budget

---

# 18. Database Schema

Use UUID primary keys.

## participants

```sql
CREATE TABLE participants (
    id UUID PRIMARY KEY,
    cookie_token UUID NOT NULL,
    game_date DATE NOT NULL,
    csrf_token TEXT NOT NULL,
    likes_last_seen_at TIMESTAMPTZ NULL,
    matches_last_seen_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (cookie_token, game_date)
);
```

## personas

```sql
CREATE TABLE personas (
    id UUID PRIMARY KEY,
    participant_id UUID NOT NULL UNIQUE
        REFERENCES participants(id)
        ON DELETE CASCADE,

    age SMALLINT NOT NULL,
    gender TEXT NOT NULL,
    height_cm SMALLINT NOT NULL,
    education TEXT NOT NULL,
    occupation TEXT NOT NULL,
    annual_income INTEGER NOT NULL,

    name VARCHAR(20) NULL,
    hobby VARCHAR(30) NULL,
    bio VARCHAR(60) NULL,

    exposure_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Suggested CHECK constraints:
- age between 20 and 50
- height_cm between 140 and 200
- annual_income >= 0
- exposure_count >= 0

## likes

```sql
CREATE TABLE likes (
    id UUID PRIMARY KEY,
    from_persona_id UUID NOT NULL
        REFERENCES personas(id)
        ON DELETE CASCADE,
    to_persona_id UUID NOT NULL
        REFERENCES personas(id)
        ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (from_persona_id, to_persona_id),

    CHECK (from_persona_id <> to_persona_id)
);
```

## passes

```sql
CREATE TABLE passes (
    id UUID PRIMARY KEY,
    from_persona_id UUID NOT NULL
        REFERENCES personas(id)
        ON DELETE CASCADE,
    to_persona_id UUID NOT NULL
        REFERENCES personas(id)
        ON DELETE CASCADE,

    pass_count SMALLINT NOT NULL DEFAULT 1,
    last_passed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (from_persona_id, to_persona_id),

    CHECK (from_persona_id <> to_persona_id),
    CHECK (pass_count BETWEEN 1 AND 3)
);
```

## matches

```sql
CREATE TABLE matches (
    id UUID PRIMARY KEY,

    persona_low_id UUID NOT NULL
        REFERENCES personas(id)
        ON DELETE CASCADE,

    persona_high_id UUID NOT NULL
        REFERENCES personas(id)
        ON DELETE CASCADE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (persona_low_id, persona_high_id),

    CHECK (persona_low_id <> persona_high_id)
);
```

Application must normalize the Persona pair before insert.

---

# 19. Daily Physical Deletion

Correctness uses `game_date`, not deletion timing.

Deletion job:
- periodically delete `participants WHERE game_date < today_jst`
- `ON DELETE CASCADE` removes:
  - personas
  - likes
  - passes
  - matches

Job must be idempotent and retryable.

No persistent historical archive.

---

# 20. CSRF / Web Security

Because identity is Cookie-based and the app can be embedded:

## CSRF

Participant stores one daily CSRF token.

Generation:
- cryptographically secure random bytes
- ~32 bytes
- Base64URL / equivalent opaque token

Lifecycle:
- generated for today's Participant
- valid only for the game day
- returned during initialization/home
- frontend stores in memory
- mutating requests send header such as:

```text
X-CSRF-Token: ...
```

Require CSRF token for:
- Persona generation
- profile update
- Like
- Pass
- any future mutation endpoint

GET endpoints must not mutate game state except the explicitly accepted "last seen" behavior discussed below.

## XSS

All B fields:
- plain text
- escaped when rendered
- no raw HTML insertion
- no Markdown parsing

## Server-side validation

Never trust frontend for:
- Like count
- Like budget
- Persona attributes
- Match creation
- Pass count
- character limits
- newline restriction
- explicit URL restriction

---

# 21. API

Base path:

```text
/api
```

Success payloads do not need a universal wrapper.

Errors use:

```json
{
  "error": {
    "code": "LikeLimitExceeded",
    "message": "like limit exceeded"
  }
}
```

Frontend behavior should branch on `error.code`.

## Domain error codes

- `PersonaNotGenerated`
- `LikeLimitExceeded`
- `AlreadyLiked`
- `TargetPersonaUnavailable`
- `PassLimitReached`
- `SelfActionNotAllowed`
- `DayExpired`
- `InvalidProfileInput`

Suggested HTTP mapping:

| Error | HTTP |
|---|---:|
| InvalidProfileInput | 400 |
| PersonaNotGenerated | 404 |
| TargetPersonaUnavailable | 404 |
| AlreadyLiked | 409 |
| PassLimitReached | 409 |
| DayExpired | 409 |
| LikeLimitExceeded | 422 |
| SelfActionNotAllowed | 422 |

---

## GET `/api/home`

Responsibilities:
- ensure today's Participant exists
- return current app state

Example:

```json
{
  "server_time": "2026-08-16T21:00:00+09:00",
  "game_date": "2026-08-16",
  "persona_generated": true,
  "persona": {},
  "remaining_likes": 7,
  "received_like_count": 4,
  "match_count": 2,
  "has_unseen_likes": true,
  "has_unseen_matches": false,
  "csrf_token": "..."
}
```

If Persona not generated:
- `persona_generated=false`
- Persona may be null

---

## POST `/api/persona`

CSRF required.

Idempotent:
- no Persona today → generate and persist
- exists → return existing Persona

Never reroll.

---

## GET `/api/persona/me`

Return own Persona including A + B attributes.

---

## PATCH `/api/persona/profile`

CSRF required.

Request:

```json
{
  "name": "...",
  "hobby": "...",
  "bio": "..."
}
```

Only B attributes accepted.

Do not silently accept A fields.

---

## GET `/api/discover`

Return max 5 public Persona cards.

Optional query:

```text
?exclude=id1,id2,id3
```

The frontend may send current cooldown exclusions.

API response must not increment exposure.

---

## POST `/api/likes`

CSRF required.

Request:

```json
{
  "persona_id": "..."
}
```

Transactionally:
1. validate current day / current Persona
2. validate target
3. reject self
4. reject duplicate
5. enforce sent Like count < 10
6. insert Like
7. increment target exposure_count
8. check reverse Like
9. if reverse exists, create normalized Match idempotently

Example response:

```json
{
  "remaining_likes": 6,
  "matched": true,
  "match_id": "...",
  "target_persona": {}
}
```

When not matched:
- `match_id` / `target_persona` may be omitted/null

---

## POST `/api/passes`

CSRF required.

Request:

```json
{
  "persona_id": "..."
}
```

Transactionally:
1. validate current day / target
2. reject self
3. reject invalid target state
4. insert or increment Pass
5. cap at 3
6. increment target exposure_count

Response:

```json
{
  "pass_count": 2,
  "excluded_for_today": false
}
```

---

## GET `/api/likes/received`

Return Personas that Liked current Persona.
Newest first.

Opening this screen marks Likes as seen by updating:

```text
participants.likes_last_seen_at
```

No pagination in MVP.

---

## GET `/api/likes/sent`

Return Personas current Persona has Liked.
Newest first.

Each result additionally includes:

```json
{
  "matched": true
}
```

No pagination.

---

## GET `/api/matches`

Return matched counterpart Personas.
Newest first.

Opening this screen updates:

```text
participants.matches_last_seen_at
```

No pagination.

---

# 22. Important Transaction / Concurrency Requirements

## Persona generation
Must be race-safe.

DB unique constraints:
- `(cookie_token, game_date)`
- `personas(participant_id)`

If simultaneous requests occur:
- return the same resulting Persona

## Like budget
Must be server-enforced inside transaction.

Two tabs must never permit >10 Likes.

Duplicate retry:
- must not consume Like twice

## Match
Must be idempotent.

Normalize IDs:

```text
low_id = min(personaA, personaB)
high_id = max(personaA, personaB)
```

DB unique constraint prevents duplicate Match.

---

# 23. Non-Goals for MVP

Do **not** implement unless required to make the core flow work:

- chat
- DM
- blocking
- reporting
- moderation dashboard
- push notifications
- browser notifications
- WebSocket
- SSE
- swipe gestures
- historical records
- cumulative statistics
- rankings
- Like rate
- exposure statistics shown to users
- rarity / SSR / score
- search
- gender filtering
- matching preferences
- pagination
- social login
- kusa integration API
- anti-reroll / anti-Cookie-clear system
- sophisticated abuse prevention
- NG-word dictionary
- phone/email/SNS handle detection
- long-lived profiles
- facial attractiveness score
- "parent wealth" or similar additional stats

---

# 24. Definition of Done

The MVP is complete when this full path works:

```text
Open app
  ↓
Anonymous Cookie issued/read
  ↓
Today's Participant created
  ↓
"新しい人生を始める"
  ↓
Server creates one random Persona
  ↓
Persona immediately joins the market
  ↓
User optionally edits name/hobby/bio
  ↓
User opens Discover
  ↓
5 candidates fetched
  ↓
One Persona displayed at a time
  ↓
Like / Pass
  ↓
10-Like budget is enforced
  ↓
Received Likes can be viewed
  ↓
User can Like back
  ↓
Mutual Like creates Match
  ↓
Match animation appears
  ↓
Sent Likes and Matches can be reviewed
  ↓
Home shows counts and daily countdown
  ↓
23:55 warning
  ↓
00:00 old game state becomes invalid
  ↓
"今日の人生が終了しました"
  ↓
Next day's "新しい人生を始める"
```

If this works reliably, stop adding features and test with real users.

---

# 25. Suggested Project Layout

Claude Code may adjust names, but keep responsibilities explicit.

```text
.
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── participant/
│   ├── persona/
│   ├── matching/
│   ├── discover/
│   ├── http/
│   │   ├── handler/
│   │   ├── middleware/
│   │   └── response/
│   ├── db/
│   │   ├── query/
│   │   └── sqlc/
│   └── clock/
├── migrations/
├── web/
│   ├── src/
│   │   ├── lib/
│   │   ├── components/
│   │   ├── routes/
│   │   └── stores/
│   └── vite.config.*
├── sqlc.yaml
├── go.mod
└── README.md
```

Do not create elaborate repository/service/usecase layers merely for architectural purity.
Use enough separation to keep:
- HTTP
- domain rules
- SQL
- frontend
clear and testable.

---

# 26. Suggested Implementation Order for Claude Code

1. Scaffold Go + chi + PostgreSQL + sqlc + migrations
2. Implement JST game-day clock abstraction
3. Implement Cookie participant identity
4. Implement Participant ensure logic
5. Create migrations/schema
6. Implement Persona generator + unit tests
7. Implement Persona generation endpoint
8. Implement Home endpoint
9. Implement profile edit validation
10. Implement Discover query
11. Implement Pass transaction
12. Implement Like transaction + 10-limit
13. Implement Match creation
14. Implement list APIs
15. Implement CSRF
16. Build Svelte shell / Home
17. Build Persona card
18. Build New Life flow
19. Build Discover flow + 5-card prefetch
20. Build Received / Sent / Match screens
21. Add Match animation
22. Add daily countdown + 23:55 + 00:00 transitions
23. Add physical cleanup job
24. Test iframe behavior
25. Test direct URL behavior
26. Test concurrency around Persona generation and 10-Like budget

---

# 27. Tests That Matter

Prioritize these tests.

## Persona generation
- age always 20–50
- height always 140–200
- university graduate never <22
- graduate school never <24
- doctor restrictions obeyed except ホイ卒
- teacher restrictions obeyed except ホイ卒
- executive age restriction obeyed except ホイ卒
- income is always within occupation range
- income always multiple of 10

## Like
- cannot self-Like
- cannot duplicate Like
- 10 Likes succeed
- 11th Like fails
- concurrent Likes never exceed 10
- retry does not double-consume
- mutual Like creates one Match

## Pass
- pass_count increments 1 → 2 → 3
- 3 means excluded
- cannot exceed 3
- exposure increments once per successful action

## Day boundary
- yesterday's Persona cannot act today
- old Likes/Matches never appear today
- new Participant can be created for same Cookie next day
- new Persona can be generated next day
- previous Persona is not reused

## Security
- invalid CSRF rejected on mutation
- A fields cannot be modified by profile PATCH
- newline rejected
- explicit URL rejected
- HTML is treated as text

---

# 28. Final Instruction to Claude Code

Implement the MVP exactly around the core daily loop.

When a choice is not specified:
1. choose the simplest implementation
2. preserve daily ephemerality
3. preserve the 10-Like market constraint
4. preserve Persona/User separation
5. avoid adding features
6. avoid architecture that is not justified by the current MVP

If implementation details reveal a genuine contradiction in this spec, stop and surface the contradiction rather than silently inventing a new product rule.
