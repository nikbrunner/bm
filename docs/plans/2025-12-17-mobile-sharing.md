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

### Option 2: Dedicated Home Server (Raspberry Pi)

```
Phone → Tailscale → Raspberry Pi (bm serve) → SQLite on Pi
                          ↓
              Optional: sync/backup to main machines
```

**How it works:**
- Raspberry Pi runs 24/7 at home (tiny power draw)
- `bm serve` runs as a systemd service
- Tailscale connects your phone to the Pi
- All data stays in your home

**Pros:**
- Always available (at home)
- Very low power (~3-5W)
- One-time cost, no subscription
- Data stays local/private
- Can run other services too (Pi-hole, etc.)
- Full TUI access via SSH

**Cons:**
- Initial setup effort
- Not accessible if home internet is down
- Another device to maintain

#### Hardware Options

| Device | Power | Cost | CPU | RAM | Best For |
|--------|-------|------|-----|-----|----------|
| **Raspberry Pi Zero 2 W** | ~1W | $15 | 4-core ARM | 512MB | Minimal, just bm |
| **Raspberry Pi 4** | ~3-5W | $55-75 | 4-core ARM | 2-8GB | Multiple services |
| **Raspberry Pi 5** | ~5-8W | $80-100 | 4-core ARM | 4-8GB | Best performance |
| **Intel N100 Mini PC** | ~10W | $150 | 4-core x86 | 8-16GB | Want x86, more power |
| **Old Laptop** | ~15-30W | Free? | Varies | Varies | Built-in UPS (battery) |

**Recommendation:** Raspberry Pi 4 (4GB) or Pi 5 - best balance of cost, power, and capability.

#### What You'll Need

- Raspberry Pi (any model with WiFi or Ethernet)
- MicroSD card (32GB+ recommended) or USB SSD for reliability
- Power supply (official Pi PSU recommended)
- Case (optional but keeps dust out)
- Ethernet cable (more reliable than WiFi)

Total cost: ~$70-120 one-time

#### Pi Setup Overview

1. **Flash OS:**
   ```bash
   # Use Raspberry Pi Imager to flash Raspberry Pi OS Lite (64-bit)
   # Enable SSH and set hostname during imaging
   ```

2. **Install Go & bm:**
   ```bash
   # SSH into Pi
   ssh pi@raspberrypi.local

   # Install Go
   wget https://go.dev/dl/go1.21.5.linux-arm64.tar.gz
   sudo tar -C /usr/local -xzf go1.21.5.linux-arm64.tar.gz
   echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.bashrc
   source ~/.bashrc

   # Clone and build bm
   git clone https://github.com/nikbrunner/bm.git
   cd bm
   go install ./cmd/bm
   ```

3. **Install Tailscale:**
   ```bash
   curl -fsSL https://tailscale.com/install.sh | sh
   sudo tailscale up
   # Follow the auth link
   ```

4. **Create systemd service:**
   ```bash
   sudo tee /etc/systemd/system/bm-server.service << 'EOF'
   [Unit]
   Description=bm bookmark server
   After=network-online.target
   Wants=network-online.target

   [Service]
   Type=simple
   User=pi
   WorkingDirectory=/home/pi
   ExecStart=/home/pi/go/bin/bm serve
   Restart=always
   RestartSec=10
   Environment=ANTHROPIC_API_KEY=your-key-here

   [Install]
   WantedBy=multi-user.target
   EOF

   sudo systemctl enable bm-server
   sudo systemctl start bm-server
   ```

5. **Verify:**
   ```bash
   # Check status
   sudo systemctl status bm-server

   # Test from phone (after Tailscale setup)
   curl -X POST http://100.x.x.x:8080/api/readlater \
     -H "Authorization: Bearer YOUR_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"url": "https://example.com"}'
   ```

#### Reliability Considerations

- **SD card wear:** SQLite writes can wear out SD cards. Options:
  - Use a USB SSD instead (~$30 for 128GB)
  - Or mount `/home/pi/.config/bm` to a USB drive
  - Or use `sync` mode sparingly and batch writes

- **Power outages:** Consider a small UPS (~$30) or just accept occasional reboots

- **Backups:** Cron job to copy `bookmarks.db` somewhere:
  ```bash
  # Add to crontab -e
  0 3 * * * cp ~/.config/bm/bookmarks.db ~/backups/bookmarks-$(date +\%Y\%m\%d).db
  ```

#### Accessing bm from Main Machines

**Option A: SSH + TUI**
```bash
# From MacBook or Linux desktop
ssh pi@100.x.x.x  # Tailscale IP
bm  # Run TUI on Pi, displayed on your terminal
```

**Option B: Sync database to local machines**
```bash
# Pull DB from Pi to local
scp pi@100.x.x.x:~/.config/bm/bookmarks.db ~/.config/bm/

# Or use rsync for incremental sync
rsync -av pi@100.x.x.x:~/.config/bm/ ~/.config/bm/
```

**Option C: Mount Pi storage via SSHFS**
```bash
# Mount Pi's config dir locally
sshfs pi@100.x.x.x:~/.config/bm ~/.config/bm
# Now local bm reads/writes directly to Pi
```

---

### Option 3: Always-On VPS

```
Phone → Internet → VPS (bm serve) → SQLite on VPS
```

**How it works:**
- Run `bm serve` on a cheap VPS (Hetzner, DigitalOcean: ~$4-5/mo)
- Sync database back to local machine periodically
- Or just use bm on the VPS via SSH/mosh

**Pros:**
- Always available, even away from home
- No hardware to maintain at home
- Can access from anywhere
- Same setup as Pi (just different machine)

**Cons:**
- Monthly cost (~$50/year)
- Data on remote server
- Latency (minor)

**Providers:**
- Hetzner Cloud: €3.79/mo (cheapest, EU)
- DigitalOcean: $4/mo
- Vultr: $5/mo
- Fly.io: Free tier available

---

### Option 4: Cloud Database (Turso/libSQL)

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

### Option 5: Serverless Function + Cloud Storage

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

### Option 6: Simple Queue (No Sync)

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

| Approach | Always On | Cost | Complexity | Data Location | Best For |
|----------|-----------|------|------------|---------------|----------|
| Local + Tailscale | No | Free | Low | Local | Testing first |
| **Home Server (Pi)** | **Yes*** | **~$80 once** | **Medium** | **Local** | **Privacy + always-on** |
| VPS | Yes | ~$50/yr | Low | Remote | Simplicity, no hardware |
| Turso | Yes | Free* | Medium | Cloud | Multi-device sync |
| Serverless | Yes | Free* | High | Cloud | Max scalability |
| Manual Queue | N/A | Free | None | N/A | Zero effort |

*Within free tier limits / **When home internet is up*

**Recommendation:** Home Server (Pi) offers the best balance - always available, data stays local, one-time cost.

---

## Recommended Path

### Phase 1: Implement `bm serve`
1. Implement `bm serve` command
2. Add `/api/readlater` endpoint
3. Add token auth from config
4. Test locally on your MacBook/Linux machine

### Phase 2: Test with Tailscale
1. Install Tailscale on your machine + phone
2. Create iOS Shortcut
3. Test the full flow
4. Use for a week to validate the workflow

### Phase 3: Deploy to Home Server
1. Get a Raspberry Pi (or use old hardware)
2. Set up Pi with Go, bm, Tailscale
3. Create systemd service for `bm serve`
4. Move your bookmarks.db to Pi
5. Update iOS Shortcut to point to Pi's Tailscale IP

### Phase 4 (Optional): Multi-device Access
Choose one:
- **SSH:** Just SSH into Pi and run `bm` TUI
- **SSHFS:** Mount Pi's storage on your machines
- **Sync:** Periodic rsync of database
- **Cloud:** If you need true multi-device, consider Turso

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

### Networking
- [Tailscale](https://tailscale.com/) - Free mesh VPN (recommended)
- [Tailscale iOS App](https://apps.apple.com/app/tailscale/id1470499037)

### Home Server Hardware
- [Raspberry Pi](https://www.raspberrypi.com/) - Official site
- [Pi Locator](https://rpilocator.com/) - Find Pi in stock
- [Argon ONE case](https://argon40.com/) - Nice Pi case with passive cooling

### Cloud Options
- [Turso](https://turso.tech/) - SQLite at the edge
- [libSQL Go driver](https://github.com/tursodatabase/libsql-client-go)
- [Cloudflare D1](https://developers.cloudflare.com/d1/) - Serverless SQLite
- [Hetzner Cloud](https://www.hetzner.com/cloud) - Cheap VPS

### iOS
- [iOS Shortcuts Guide](https://support.apple.com/guide/shortcuts/welcome/ios)
- [Shortcuts subreddit](https://www.reddit.com/r/shortcuts/) - Community examples
