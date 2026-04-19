package http

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"log"
	"net/http"

	"github.com/pquerna/otp/totp"
	"github.com/starfederation/datastar-go/datastar"
)

// ── Login TOTP step ───────────────────────────────────────────────────────────

func (h *Handlers) loginTOTPPage(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(totpPendingCookie)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if _, ok := lookupTOTPPending(cookie.Value); !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	TOTPLoginPage("").Render(r.Context(), w)
}

func (h *Handlers) loginTOTPSSE(w http.ResponseWriter, r *http.Request) {
	ip := r.RemoteAddr
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ip = xff
	}
	if !globalLoginLimiter.allow(ip) {
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(totpLoginError("Too many attempts. Try again in 15 minutes."))
		return
	}

	pendingCookie, err := r.Cookie(totpPendingCookie)
	if err != nil || pendingCookie.Value == "" {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if h.totpUsers == nil || h.createSessionCmd == nil {
		log.Println("loginTOTPSSE: TOTP not configured")
		http.Error(w, "not configured", http.StatusInternalServerError)
		return
	}

	var signals struct {
		TOTPCode string `json:"totpCode"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Consume nonce — single use, resolves to userID server-side.
	userID, ok := consumeTOTPPending(pendingCookie.Value)
	if !ok {
		// Nonce expired or already used.
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	u, err := h.totpUsers.FindByID(r.Context(), userID)
	if err != nil || u == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if !u.VerifyTOTP(signals.TOTPCode) {
		globalLoginLimiter.record(ip)
		// Re-store nonce so user can retry without going back to login.
		nonce := newTOTPPending(u.ID)
		http.SetCookie(w, &http.Cookie{
			Name:     totpPendingCookie,
			Value:    nonce,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   totpPendingMaxAge,
		})
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(totpLoginError("Invalid code. Try again."))
		return
	}

	// Code valid — create a fresh session.
	token, err := h.createSessionCmd.Handle(r.Context(), u.ID)
	if err != nil {
		log.Printf("loginTOTPSSE: createSession: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Clear pending cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     totpPendingCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	maxAge := h.maxAge
	if maxAge <= 0 {
		maxAge = 86400
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
		Secure:   h.secureCookie,
	})
	sse := datastar.NewSSE(w, r)
	sse.Redirect("/")
}

// ── Profile 2FA setup ─────────────────────────────────────────────────────────

func (h *Handlers) profileTOTPSetupPage(w http.ResponseWriter, r *http.Request) {
	u := userFromContext(r)
	if u == nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if h.totpUsers == nil {
		http.Error(w, "not configured", http.StatusInternalServerError)
		return
	}

	// Generate a fresh TOTP key (not saved yet — user must confirm with a valid code).
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "VVS ISP",
		AccountName: u.Username,
	})
	if err != nil {
		log.Printf("profileTOTPSetupPage: generate: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Render QR code as base64 PNG data URI.
	img, err := key.Image(200, 200)
	if err != nil {
		log.Printf("profileTOTPSetupPage: image: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	qrDataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	TOTPSetupPage(u, key.Secret(), qrDataURI).Render(r.Context(), w)
}

func (h *Handlers) enableTOTPSSE(w http.ResponseWriter, r *http.Request) {
	u := userFromContext(r)
	if u == nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if h.totpUsers == nil {
		http.Error(w, "not configured", http.StatusInternalServerError)
		return
	}

	var signals struct {
		NewTOTPSecret string `json:"newTotpSecret"`
		TOTPCode      string `json:"totpCode"`
	}
	if err := datastar.ReadSignals(r, &signals); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if !totp.Validate(signals.TOTPCode, signals.NewTOTPSecret) {
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(totpSetupError("Invalid code — make sure the QR code is scanned correctly."))
		return
	}

	full, err := h.totpUsers.FindByID(r.Context(), u.ID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	full.EnableTOTP(signals.NewTOTPSecret)
	if err := h.totpUsers.Save(r.Context(), full); err != nil {
		log.Printf("enableTOTPSSE: save: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.Redirect("/profile")
}

func (h *Handlers) disableTOTPSSE(w http.ResponseWriter, r *http.Request) {
	u := userFromContext(r)
	if u == nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if h.totpUsers == nil {
		http.Error(w, "not configured", http.StatusInternalServerError)
		return
	}

	full, err := h.totpUsers.FindByID(r.Context(), u.ID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	full.DisableTOTP()
	if err := h.totpUsers.Save(r.Context(), full); err != nil {
		log.Printf("disableTOTPSSE: save: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.Redirect("/profile")
}
