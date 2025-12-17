# Design: Mobile Share Link Feature

## Overview

Enable sharing URLs directly from a phone (iOS/Android share sheet) to the bm database as "Read Later" entries.

---

## The Problem

Currently, adding bookmarks requires:
1. Being at your computer
2. Running the TUI and pressing `L` (Read Later) or `i` (Quick Add)

**Goal:** Share a link from your phone and have it appear in bm instantly.

---

## Architecture Options

### Option 1: Local Server + Tailscale (Recommended Starting Point)

```
Phone → Tailscale VPN → Your Machine (bm serve) → SQLite
```

**How it works:**
- New `bm serve` command runs HTTP server on your machine
- Tailscale creates private network between phone and machine
- iOS Shortcut sends POST request with URL
- Server adds bookmark to local SQLite DB

**Pros:**
- Simple to implement
- Data stays local
- Free (Tailscale free tier)
- Foundation for other approaches

**Cons:**
- Machine must be running
- Requires Tailscale setup on both devices

**Implementation:**
```go
// New subcommand
bm serve --port 8080 --token YOUR_SECRET_TOKEN

// Endpoint
POST /api/readlater
Authorization: Bearer YOUR_SECRET_TOKEN
Body: {"url": "https://example.com"}

// Response
{"status": "ok", "title": "Example Site", "folder": "Read Later"}
```

---

### Option 2: Always-On VPS

```
Phone → Internet → VPS (bm serve) → SQLite on VPS
```

**How it works:**
- Run `bm serve` on a cheap VPS (Hetzner, DigitalOcean: ~$4-5/mo)
- Sync database back to local machine periodically
- Or just use bm on the VPS via SSH/mosh

**Pros:**
- Always available
- No home machine needed
- Can access from anywhere

**Cons:**
- Monthly cost (~$50/year)
- Data on remote server
- Need to sync or access remotely

**Providers:**
- Hetzner Cloud: €3.79/mo (cheapest)
- DigitalOcean: $4/mo
- Vultr: $5/mo
- Fly.io: Free tier available

---

### Option 3: Cloud Database (Turso/libSQL)

```
Phone → Turso Edge → libSQL Database
Your Machine → Turso Edge → Same Database
```

**How it works:**
- Replace SQLite with Turso (libSQL - SQLite compatible)
- Both phone shortcut and local bm connect to same cloud DB
- No server needed - direct database access

**Pros:**
- Always available
- No server to maintain
- Generous free tier (9GB, 500M reads/mo)
- SQLite-compatible (minimal code changes)

**Cons:**
- Vendor dependency
- Data in cloud
- Requires refactoring storage layer
- Need thin API layer for phone (Turso doesn't expose raw SQL to shortcuts)

**Turso Free Tier:**
- 9 GB storage
- 500 million row reads/month
- 25 million row writes/month
- 3 databases

**Implementation Notes:**
- Would need `internal/storage/turso.go` alongside `sqlite.go`
- Or a small edge function (Cloudflare Workers, Vercel) as API

---

### Option 4: Serverless Function + Cloud Storage

```
Phone → Cloudflare Worker → D1/KV/R2
Your Machine → Sync → Local SQLite
```

**How it works:**
- Cloudflare Worker receives URLs, stores in D1 (SQLite at edge)
- Local bm syncs from D1 periodically
- Or: Worker appends to a JSON file in R2/S3

**Pros:**
- Extremely cheap (likely free forever)
- Always available
- Fast globally

**Cons:**
- Two-way sync complexity
- More moving parts
- Potential conflicts

---

### Option 5: Simple Queue (No Sync)

```
Phone → Apple Notes/Reminders/Telegram
Manual → Copy to bm when at computer
```

**How it works:**
- Share links to a dedicated note or chat
- Process them in batch when at computer
- No infrastructure needed

**Pros:**
- Zero setup
- Works today
- No sync issues

**Cons:**
- Manual step required
- Links pile up

---

## Comparison Matrix

| Approach | Always On | Cost | Complexity | Data Location |
|----------|-----------|------|------------|---------------|
| Local + Tailscale | No | Free | Low | Local |
| VPS | Yes | ~$50/yr | Low | Remote |
| Turso | Yes | Free* | Medium | Cloud |
| Serverless | Yes | Free* | High | Cloud |
| Manual Queue | N/A | Free | None | N/A |

*Within free tier limits

---

## Recommended Path

### Phase 1: Local Server (Do First)
1. Implement `bm serve` command
2. Add `/api/readlater` endpoint
3. Add token auth from config
4. Create iOS Shortcut
5. Test with Tailscale

This gives you a working solution and the server code is reusable.

### Phase 2: Evaluate Usage
- Is the "machine must be on" limitation actually a problem?
- How often do you share links when away from home?

### Phase 3: If Needed, Go Cloud
- **Easiest:** Move to VPS, run same code
- **Cleanest:** Turso migration (keep SQLite compatibility)

---

## Implementation Details

### Config Additions

```go
type Config struct {
    QuickAddFolder     string   `json:"quickAddFolder"`
    CullExcludeDomains []string `json:"cullExcludeDomains"`
    // New fields
    ServerPort         int      `json:"serverPort"`         // default: 8080
    ServerToken        string   `json:"serverToken"`        // required for serve
    ServerUseAI        bool     `json:"serverUseAI"`        // use AI for title/tags
}
```

### New Package: `internal/server`

```go
package server

type Server struct {
    store    *model.Store
    storage  *storage.SQLiteStorage
    config   *storage.Config
    aiClient *ai.Client  // optional
}

func (s *Server) Start(port int) error
func (s *Server) handleReadLater(w http.ResponseWriter, r *http.Request)
```

### API Design

```
POST /api/readlater
Authorization: Bearer <token>
Content-Type: application/json

Request:
{
  "url": "https://example.com/article",
  "title": "Optional title",      // optional, AI fills if empty
  "tags": ["optional", "tags"],   // optional, AI suggests if empty
  "folder": "Read Later"          // optional, uses config default
}

Response (200):
{
  "status": "ok",
  "bookmark": {
    "id": "uuid",
    "title": "Article Title",
    "url": "https://example.com/article",
    "folder": "Read Later",
    "tags": ["suggested", "tags"]
  }
}

Response (401):
{"error": "unauthorized"}

Response (400):
{"error": "invalid url"}
```

### iOS Shortcut Setup

1. Create new Shortcut
2. "Receive **URLs** from **Share Sheet**"
3. "Get contents of URL"
   - URL: `http://100.x.x.x:8080/api/readlater`
   - Method: POST
   - Headers: `Authorization: Bearer YOUR_TOKEN`
   - Body: JSON `{"url": "[Shortcut Input]"}`
4. "Show Result" (optional, for confirmation)

### Security Considerations

- Token should be long random string (32+ chars)
- HTTPS recommended for non-Tailscale setups
- Rate limiting (prevent abuse if exposed)
- URL validation (only http/https)

---

## Questions to Explore

1. **Turso experience:** Has anyone used Turso with Go? Migration difficulty?
2. **Sync strategies:** If using cloud, one-way or two-way sync?
3. **Offline handling:** Queue on phone if server unreachable?
4. **Multiple devices:** Want to access bm from multiple machines?

---

## Resources

- [Tailscale](https://tailscale.com/) - Free mesh VPN
- [Turso](https://turso.tech/) - SQLite at the edge
- [libSQL Go driver](https://github.com/tursodatabase/libsql-client-go)
- [Cloudflare D1](https://developers.cloudflare.com/d1/) - Serverless SQLite
- [iOS Shortcuts Guide](https://support.apple.com/guide/shortcuts/welcome/ios)
