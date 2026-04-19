// vvs-stb — IPTV Set-Top Box device API binary.
//
// Runs on a public-facing VPS. Zero DB access.
// All data fetched from vvs-core via NATS RPC over WireGuard.
//
// Authentication: every URL contains an opaque subscription token (64-char hex).
// Revoke a token by marking the SubscriptionKey revoked in vvs-core admin.
//
// Routes:
//
//	GET /apis/siptv/playlist/{token}   — M3U8 for SIPTV app
//	GET /apis/tvzone/playlist/{token}  — M3U8 for generic players (VLC, Tivimate)
//	GET /apis/tvip/playlist/{token}    — M3U8 for TVIP STBs
//	GET /epg/{token}.xml               — XMLTV EPG (3 days default)
//	GET /stream/{token}/{channelID}    — 302 redirect to actual stream URL
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"
	"github.com/urfave/cli/v3"
	iptvnats "github.com/atvirokodosprendimai/vvs/internal/modules/iptv/adapters/nats"
)

func main() {
	if err := godotenv.Load(); err == nil {
		log.Println("Loaded config from .env")
	}

	cmd := &cli.Command{
		Name:  "vvs-stb",
		Usage: "VVS IPTV STB Device API (public-facing, no DB)",
		Commands: []*cli.Command{
			serveCommand(),
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

func serveCommand() *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "Start the STB device API HTTP server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "addr",
				Usage:   "HTTP listen address",
				Value:   ":8082",
				Sources: cli.EnvVars("STB_ADDR"),
			},
			&cli.StringFlag{
				Name:     "nats-url",
				Usage:    "NATS URL for vvs-core bridge (e.g. nats://10.0.0.1:4222)",
				Sources:  cli.EnvVars("NATS_URL"),
				Required: true,
			},
			&cli.StringFlag{
				Name:    "nats-portal-password",
				Usage:   "Password for the 'portal' NATS user (required when vvs-core uses per-user auth)",
				Sources: cli.EnvVars("NATS_PORTAL_PASSWORD"),
			},
			&cli.StringFlag{
				Name:    "nats-auth-token",
				Usage:   "Deprecated: use --nats-portal-password instead",
				Sources: cli.EnvVars("NATS_AUTH_TOKEN"),
				Hidden:  true,
			},
			&cli.StringFlag{
				Name:    "base-url",
				Usage:   "Public base URL for generating stream redirect URLs (e.g. https://stb.example.com)",
				Sources: cli.EnvVars("VVS_BASE_URL"),
				Value:   "http://localhost:8082",
			},
			&cli.BoolFlag{
				Name:    "proxy-enabled",
				Usage:   "Proxy stream requests instead of redirecting (hides upstream URLs)",
				Sources: cli.EnvVars("STB_PROXY_ENABLED"),
				Value:   false,
			},
		},
		Action: runSTB,
	}
}

func runSTB(ctx context.Context, cmd *cli.Command) error {
	addr := cmd.String("addr")
	natsURL := cmd.String("nats-url")
	natsToken := cmd.String("nats-auth-token")
	baseURL := strings.TrimRight(cmd.String("base-url"), "/")
	proxyEnabled := cmd.Bool("proxy-enabled")

	opts := []nats.Option{nats.Name("vvs-stb")}
	if portalPwd := cmd.String("nats-portal-password"); portalPwd != "" {
		opts = append(opts, nats.UserInfo("portal", portalPwd))
	} else if natsToken != "" {
		opts = append(opts, nats.Token(natsToken))
	}
	nc, err := nats.Connect(natsURL, opts...)
	if err != nil {
		return fmt.Errorf("connect nats: %w", err)
	}
	defer nc.Close()
	slog.Info("connected to NATS", "url", natsURL)

	client := iptvnats.NewSTBNATSClient(nc, 5*time.Second)

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestLogger(&redactingFormatter{})) // tokens redacted from path logs
	r.Use(middleware.Recoverer)

	// ── M3U8 playlist endpoints ─────────────────────────────────────────────
	r.Get("/apis/siptv/playlist/{token}", playlistHandler(client, baseURL, "siptv"))
	r.Get("/apis/tvzone/playlist/{token}", playlistHandler(client, baseURL, "tvzone"))
	r.Get("/apis/tvip/playlist/{token}", playlistHandler(client, baseURL, "tvip"))

	// ── EPG ─────────────────────────────────────────────────────────────────
	r.Get("/epg/{token}.xml", epgHandler(client))
	r.Get("/epg/{token}/now.json", epgShortHandler(client))

	// ── Device config ────────────────────────────────────────────────────────
	r.Get("/getconfig", getConfigHandler(client, baseURL))

	// ── Stream (redirect or transparent proxy) ──────────────────────────────
	r.Get("/stream/{token}/{channelID}", streamHandler(client, proxyEnabled))

	// ── DVR ──────────────────────────────────────────────────────────────────
	r.Get("/dvr/{token}/{channelID}/{startUnix}", dvrHandler(client))

	srv := &http.Server{Addr: addr, Handler: r}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		slog.Info("vvs-stb listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-sigCh:
		slog.Info("shutting down")
	case err := <-errCh:
		return err
	}

	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutCtx)
}

// ── M3U8 generation ───────────────────────────────────────────────────────────

func playlistHandler(client *iptvnats.STBNATSClient, baseURL, appFmt string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := chi.URLParam(r, "token")
		playlist, err := client.GetPlaylist(r.Context(), token)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/x-mpegURL")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "#EXTM3U")
		for _, ch := range playlist.Channels {
			streamURL := fmt.Sprintf("%s/stream/%s/%s", baseURL, token, ch.ID)
			switch appFmt {
			case "siptv":
				fmt.Fprintf(w, "#EXTINF:-1 tvg-id=%q tvg-logo=%q group-title=%q,%s\n%s\n",
					ch.EPGSource, ch.LogoURL, ch.Category, ch.Name, streamURL)
			case "tvip":
				fmt.Fprintf(w, "#EXTINF:-1 tvg-name=%q tvg-logo=%q group-title=%q,%s\n%s\n",
					ch.Name, ch.LogoURL, ch.Category, ch.Name, streamURL)
			default: // tvzone / generic
				fmt.Fprintf(w, "#EXTINF:-1 tvg-id=%q tvg-logo=%q group-title=%q,%s\n%s\n",
					ch.EPGSource, ch.LogoURL, ch.Category, ch.Name, streamURL)
			}
		}
	}
}

// ── EPG ───────────────────────────────────────────────────────────────────────

func epgHandler(client *iptvnats.STBNATSClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Chi captures token from /epg/{token}.xml path.
		// The .xml suffix is part of the wildcard so we strip it.
		token := strings.TrimSuffix(chi.URLParam(r, "token"), ".xml")
		days := 3
		xmltv, err := client.GetEPG(r.Context(), token, days)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, xmltv)
	}
}

// ── Device config ─────────────────────────────────────────────────────────────

func getConfigHandler(client *iptvnats.STBNATSClient, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		mac := r.URL.Query().Get("mac")
		if mac == "" {
			mac = r.Header.Get("X-STB-MAC") // MAG boxes send this header
		}

		cfg, err := client.GetConfig(r.Context(), token, mac)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"token":      cfg.Token,
			"server_url": baseURL,
			"epg_url":    baseURL + "/epg/" + cfg.Token + ".xml",
			"timezone":   "Europe/Vilnius",
			"active":     cfg.Active,
		})
	}
}

func epgShortHandler(client *iptvnats.STBNATSClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := chi.URLParam(r, "token")
		entries, err := client.GetEPGShort(r.Context(), token)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entries)
	}
}

// ── Stream redirect or transparent proxy ─────────────────────────────────────

var streamProxyClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return nil // follow redirects
	},
}

func streamHandler(client *iptvnats.STBNATSClient, proxy bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := chi.URLParam(r, "token")
		channelID := chi.URLParam(r, "channelID")
		streamURL, err := client.GetChannel(r.Context(), token, channelID)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if !proxy {
			http.Redirect(w, r, streamURL, http.StatusFound)
			return
		}
		// Transparent proxy: forward request to upstream and stream response back.
		// Hides the upstream URL from clients.
		upReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, streamURL, nil)
		if err != nil {
			http.Error(w, "upstream error", http.StatusBadGateway)
			return
		}
		// Forward useful headers.
		for _, h := range []string{"Range", "Accept", "User-Agent"} {
			if v := r.Header.Get(h); v != "" {
				upReq.Header.Set(h, v)
			}
		}
		resp, err := streamProxyClient.Do(upReq)
		if err != nil {
			http.Error(w, "upstream error", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		// Copy upstream headers that matter for streaming.
		for _, h := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"} {
			if v := resp.Header.Get(h); v != "" {
				w.Header().Set(h, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		buf := make([]byte, 32*1024)
		for {
			n, rerr := resp.Body.Read(buf)
			if n > 0 {
				if _, werr := w.Write(buf[:n]); werr != nil {
					return
				}
			}
			if rerr != nil {
				return
			}
		}
	}
}

// ── DVR ───────────────────────────────────────────────────────────────────────

func dvrHandler(client *iptvnats.STBNATSClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := chi.URLParam(r, "token")
		channelID := chi.URLParam(r, "channelID")
		startUnixStr := chi.URLParam(r, "startUnix")

		startUnix, err := strconv.ParseInt(startUnixStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid startUnix", http.StatusBadRequest)
			return
		}
		startAt := time.Unix(startUnix, 0).UTC()

		streamURL, err := client.GetDVR(r.Context(), token, channelID, startAt)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotImplemented)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		http.Redirect(w, r, streamURL, http.StatusFound)
	}
}


// ── Token-redacting request logger ───────────────────────────────────────────
// Tokens are 64-char hex strings embedded in URL paths.
// Logging them verbatim would leak bearer credentials to log aggregators.

var hexTokenRe = regexp.MustCompile(`/[0-9a-f]{64}`)

type (
	redactingFormatter struct{}
	redactingLogEntry  struct{ r *http.Request }
)

func (*redactingFormatter) NewLogEntry(r *http.Request) middleware.LogEntry {
	return &redactingLogEntry{r: r}
}

func (e *redactingLogEntry) Write(status, bytes int, _ http.Header, elapsed time.Duration, _ interface{}) {
	path := hexTokenRe.ReplaceAllString(e.r.URL.Path, "/[TOKEN]")
	slog.Info("request",
		"method", e.r.Method,
		"path", path,
		"status", status,
		"bytes", bytes,
		"elapsed", elapsed.Round(time.Millisecond),
	)
}

func (e *redactingLogEntry) Panic(v interface{}, stack []byte) {
	slog.Error("panic", "err", v, "stack", string(stack))
}
