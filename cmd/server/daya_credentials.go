package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const credentialAAD = "daya-assistant/daya-credentials/v1"

type encryptedDayaCredentials struct {
	Version    int       `json:"version"`
	Nonce      string    `json:"nonce"`
	Ciphertext string    `json:"ciphertext"`
	UpdatedAt  time.Time `json:"updated_at"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"`
}

type DayaSessionCookie struct {
	Name     string        `json:"name"`
	Value    string        `json:"value"`
	Domain   string        `json:"domain,omitempty"`
	Path     string        `json:"path,omitempty"`
	Expires  time.Time     `json:"expires,omitempty"`
	MaxAge   int           `json:"max_age,omitempty"`
	Secure   bool          `json:"secure,omitempty"`
	HTTPOnly bool          `json:"http_only,omitempty"`
	SameSite http.SameSite `json:"same_site,omitempty"`
}
type DayaSession struct {
	Token     string              `json:"token"`
	Cookies   []DayaSessionCookie `json:"cookies"`
	ExpiresAt time.Time           `json:"expires_at,omitempty"`
}

const dayaSessionAADv1 = "daya-assistant/daya-session/v1"
const dayaSessionAADv2 = "daya-assistant/daya-session/v2"

func dayaSessionCookieFromHTTP(cookie *http.Cookie) (DayaSessionCookie, bool) {
	if cookie == nil || (cookie.Name != "token" && cookie.Name != "cookiesession1") || cookie.Value == "" || len(cookie.Value) > 32<<10 {
		return DayaSessionCookie{}, false
	}
	return DayaSessionCookie{Name: cookie.Name, Value: cookie.Value, Domain: cookie.Domain, Path: cookie.Path, Expires: cookie.Expires, MaxAge: cookie.MaxAge, Secure: cookie.Secure, HTTPOnly: cookie.HttpOnly, SameSite: cookie.SameSite}, true
}

func (c DayaSessionCookie) httpCookie() *http.Cookie {
	return &http.Cookie{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path, Expires: c.Expires, MaxAge: c.MaxAge, Secure: c.Secure, HttpOnly: c.HTTPOnly, SameSite: c.SameSite}
}

type DayaCredentialStore struct {
	mu          sync.Mutex
	path        string
	sessionPath string
	keyPath     string
}

type DayaSettingsPageData struct {
	Username   string
	Saved      bool
	HasSession bool
	LoginAt    string
	Monitor    DayaMonitorSnapshot
	Error      string
	Notice     string
}

var dayaSettingsPage = template.Must(template.New("daya-settings").Parse(`<!doctype html>
<html lang="fa" dir="rtl">
<head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>اتصال به دایا · دستیار دایا</title>
<style>
*{box-sizing:border-box}body{margin:0;background:#f4f6fa;color:#182230;font:14px Tahoma,Arial}
.wrap{max-width:760px;margin:30px auto;padding:0 16px}.head{display:flex;justify-content:space-between;align-items:center;gap:12px;flex-wrap:wrap;margin-bottom:18px}
h1{font-size:22px;margin:0}h2{font-size:16px;margin:0 0 14px}.muted,.foot{color:#667085}
.nav{display:flex;gap:7px;flex-wrap:wrap}.nav a{display:inline-block;padding:9px 12px;border:1px solid #d0d5dd;border-radius:9px;color:#344054;text-decoration:none;background:#fff}
.nav a.active{background:#175cd3;color:#fff;border-color:#175cd3}
.card{background:#fff;border:1px solid #e4e7ec;border-radius:13px;padding:18px}
label{display:block;margin:14px 0 6px;font-weight:bold}input{width:100%;padding:11px;border:1px solid #d0d5dd;border-radius:8px;font:inherit}
button{border:0;border-radius:8px;background:#175cd3;color:#fff;padding:10px 14px;font:inherit;cursor:pointer}
.row{display:flex;gap:8px;align-items:center;flex-wrap:wrap;margin-top:16px}.danger{background:#fff;color:#b42318;border:1px solid #fecdca}
.msg{padding:12px;border-radius:9px;margin:12px 0;line-height:1.8;background:#ecfdf3;color:#05603a}
.err{background:#fef3f2;color:#b42318}.note{padding:12px;border-radius:9px;margin:12px 0;line-height:1.9;background:#fffaeb;border:1px solid #fedf89;color:#93370d}
.tag{display:inline-block;padding:6px 10px;border-radius:16px;background:#ecfdf3;color:#067647;font-size:12px}
.foot{font-size:12px;line-height:1.9;margin-top:14px}
</style></head>
<body><main class="wrap">
<div class="head"><div><h1>اتصال به سامانه دایا</h1><div class="muted" style="margin-top:7px">مدیریت اطلاعات ورود برای آزمایش اتصال</div></div>
<nav class="nav"><a href="/">سفارش‌های فعال</a><a href="/history">تاریخچه</a><a class="active" href="/settings">اتصال به دایا</a></nav></div>
{{if .Error}}<div class="msg err">{{.Error}}</div>{{end}}{{if .Notice}}<div class="msg">{{.Notice}}</div>{{end}}
<section class="card">
<h2>اطلاعات حساب دایا</h2>
{{if .Saved}}<p><span class="tag">اطلاعات ورود ذخیره شده</span></p>{{else}}<p class="muted">هنوز اطلاعات ورود دایا ثبت نشده است.</p>{{end}}
<form method="post" action="/settings/daya-credentials" autocomplete="off">
<label for="daya-username">نام کاربری</label>
<input id="daya-username" name="username" type="text" maxlength="200" value="{{.Username}}" autocomplete="username" required>
<label for="daya-password">رمز عبور</label>
<input id="daya-password" name="password" type="password" maxlength="1024" autocomplete="new-password" placeholder="{{if .Saved}}برای حفظ رمز قبلی خالی بگذارید؛ برای تغییر، رمز جدید را وارد کنید{{else}}رمز عبور حساب دایا{{end}}" {{if not .Saved}}required{{end}}>
<div class="row"><button type="submit">ذخیره اطلاعات ورود</button></div>
</form>
{{if .Saved}}
<form method="post" action="/settings/daya-credentials/delete" onsubmit="return confirm('اطلاعات ورود دایا حذف شود؟')">
<div class="row"><button class="danger" type="submit">حذف اطلاعات ذخیره‌شده</button></div>
</form>
{{end}}
{{if .HasSession}}<div class="msg">آخرین ورود عادی دایا با موفقیت انجام شد: {{.LoginAt}}. کوکی‌های نشست به‌صورت رمزنگاری‌شده روی سرور ذخیره شده‌اند.</div>{{else}}<div class="note">اطلاعات ورود در سرور به‌صورت رمزنگاری‌شده نگهداری می‌شوند. هنوز ورود موفقی برای این حساب ثبت نشده است.</div>{{end}}
<section class="card" id="daya-monitor" style="margin:14px 0">
<h2>پایش نشست دایا</h2>
<p id="daya-monitor-state" class="muted">{{if .Monitor.Running}}پایش فعال است؛ درخواست فقط‌خواندنی GET /orders ارسال می‌شود.{{else if .Monitor.NeedsLogin}}ورود دستی با CAPTCHA لازم است.{{else}}پایش فعال نیست.{{end}}</p>
<div class="foot">تعداد بررسی‌ها: <b id="daya-monitor-checks">{{.Monitor.Checks}}</b> · آخرین HTTP: <b id="daya-monitor-http">{{if .Monitor.LastStatus}}{{.Monitor.LastStatus}}{{else}}—{{end}}</b></div>
<div class="foot">آخرین بررسی: <time id="daya-monitor-last">{{if not .Monitor.LastCheck.IsZero}}{{.Monitor.LastCheck.Format "2006-01-02 15:04:05 MST"}}{{else}}—{{end}}</time></div>
<div class="foot">بررسی بعدی: <time id="daya-monitor-next">{{if not .Monitor.NextCheck.IsZero}}{{.Monitor.NextCheck.Format "2006-01-02 15:04:05 MST"}}{{else}}—{{end}}</time> · پنجره پایش: ۱۵:۳۰ تا ۰۲:۰۰ تهران</div>
<p id="daya-monitor-error" class="foot">{{.Monitor.LastError}}</p>
</section>
<section class="note">
برای ورود به دایا، تصویر CAPTCHA را دریافت کن و عدد داخل تصویر را خودت وارد کن. پاسخ CAPTCHA در سرور ذخیره نمی‌شود.
</section>
<div class="row"><button type="button" id="load-daya-captcha" {{if not .Saved}}disabled title="ابتدا اطلاعات ورود را ذخیره کنید"{{end}}>دریافت تصویر CAPTCHA</button></div>
<form method="post" action="/settings/daya-login" autocomplete="off">
<input type="hidden" id="daya-captcha-id" name="captcha_id">
<label for="daya-captcha-answer">عدد داخل تصویر</label>
<div class="row" style="align-items:flex-start">
<img id="daya-captcha-image" alt="تصویر CAPTCHA دایا" hidden style="max-width:220px;min-height:64px;border:1px solid #d0d5dd;border-radius:8px;background:#fff">
</div>
<input id="daya-captcha-answer" name="captcha_answer" type="text" inputmode="numeric" pattern="[0-9۰-۹٠-٩]{1,12}" maxlength="12" autocomplete="off" placeholder="عدد تصویر را وارد کن" required>
<div class="row"><button type="submit" {{if not .Saved}}disabled title="ابتدا اطلاعات ورود را ذخیره کنید"{{end}}>ورود به دایا</button></div>
</form>
<script>
(() => {
  const state = document.getElementById("daya-monitor-state");
  const checks = document.getElementById("daya-monitor-checks");
  const status = document.getElementById("daya-monitor-http");
  const last = document.getElementById("daya-monitor-last");
  const next = document.getElementById("daya-monitor-next");
  const error = document.getElementById("daya-monitor-error");
  async function refreshMonitor() {
    try {
      const response = await fetch("/api/daya/session-monitor", {cache: "no-store", headers: {"Accept": "application/json"}});
      if (!response.ok) return;
      const data = await response.json();
      state.textContent = data.running ? "پایش فعال است؛ بررسی فقط‌خواندنی GET /orders انجام می‌شود." :
        (data.needs_login ? "نشست معتبر نیست؛ ورود دستی با CAPTCHA لازم است." : "پایش فعال نیست.");
      checks.textContent = String(data.checks || 0);
      status.textContent = data.last_status ? String(data.last_status) : "—";
      last.textContent = data.last_check ? new Date(data.last_check).toLocaleString("fa-IR") : "—";
      next.textContent = data.next_check ? new Date(data.next_check).toLocaleString("fa-IR") : "—";
      error.textContent = data.last_error || "";
    } catch (_) {}
  }
  refreshMonitor();
  window.setInterval(refreshMonitor, 15000);
})();
(() => {
  const button = document.getElementById("load-daya-captcha");
  if (!button) return;
  button.addEventListener("click", async () => {
    button.disabled = true;
    button.textContent = "در حال دریافت تصویر…";
    try {
      const response = await fetch("/api/daya/captcha", {cache: "no-store", headers: {"Accept": "application/json"}});
      const payload = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(payload.error || payload.message || "دریافت CAPTCHA ناموفق بود (HTTP " + response.status + ")");
      document.getElementById("daya-captcha-id").value = payload.captcha_id;
      const image = document.getElementById("daya-captcha-image");
      image.src = payload.image_data_url;
      image.hidden = false;
      document.getElementById("daya-captcha-answer").value = "";
      document.getElementById("daya-captcha-answer").focus();
      button.textContent = "دریافت تصویر جدید";
    } catch (error) {
      button.textContent = "خطا؛ دوباره تلاش کن";
      alert(error.message || "دریافت CAPTCHA ناموفق بود");
    } finally {
      button.disabled = false;
    }
  });
})();
</script>
<div class="foot">این بخش فقط ورود به دایا و دریافت نشست را انجام می‌دهد؛ هیچ سفارش خرید یا فروشی ارسال نمی‌شود.</div>
<div class="foot">رمز عبور در صفحه دوباره نمایش داده نمی‌شود. پس از ثبت، برای حفظ رمز قبلی کادر رمز را خالی بگذارید. برای جایگزینی رمز، رمز جدید را وارد کنید.</div>
</section></main></body></html>`))

func NewDayaCredentialStore(dataDir, keyPath string) *DayaCredentialStore {
	return &DayaCredentialStore{
		path:        filepath.Join(dataDir, "daya-credentials.enc.json"),
		sessionPath: filepath.Join(dataDir, "daya-session.enc.json"),
		keyPath:     keyPath,
	}
}

func (s *DayaCredentialStore) readKey() ([]byte, error) {
	info, err := os.Stat(s.keyPath)
	if err != nil {
		return nil, fmt.Errorf("کلید رمزنگاری پیدا نشد: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 {
		return nil, errors.New("مجوز فایل کلید رمزنگاری ناامن است")
	}
	key, err := os.ReadFile(s.keyPath)
	if err != nil {
		return nil, fmt.Errorf("خواندن کلید رمزنگاری: %w", err)
	}
	if len(key) != 32 {
		return nil, errors.New("طول کلید رمزنگاری باید دقیقاً ۳۲ بایت باشد")
	}
	return key, nil
}

func (s *DayaCredentialStore) loadLocked() (string, string, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("خواندن اطلاعات ورود: %w", err)
	}
	var record encryptedDayaCredentials
	if err := json.Unmarshal(data, &record); err != nil {
		return "", "", errors.New("ساختار فایل اطلاعات ورود معتبر نیست")
	}
	if record.Version != 1 {
		return "", "", errors.New("نسخه فایل اطلاعات ورود پشتیبانی نمی‌شود")
	}
	nonce, err := base64.StdEncoding.DecodeString(record.Nonce)
	if err != nil {
		return "", "", errors.New("مقدار nonce در فایل اطلاعات ورود نامعتبر است")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(record.Ciphertext)
	if err != nil {
		return "", "", errors.New("داده رمزنگاری‌شده اطلاعات ورود نامعتبر است")
	}
	key, err := s.readKey()
	if err != nil {
		return "", "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}
	if len(nonce) != gcm.NonceSize() {
		return "", "", errors.New("اندازه nonce اطلاعات ورود نامعتبر است")
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, []byte(credentialAAD))
	if err != nil {
		return "", "", errors.New("رمزگشایی اطلاعات ورود ناموفق بود؛ کلید یا فایل ممکن است تغییر کرده باشد")
	}
	parts := strings.SplitN(string(plain), "\x00", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("محتوای اطلاعات ورود ناقص است")
	}
	return parts[0], parts[1], nil
}

func (s *DayaCredentialStore) Save(username, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	username = strings.TrimSpace(username)
	if username == "" || len(username) > 200 || strings.ContainsRune(username, '\x00') {
		return errors.New("نام کاربری دایا معتبر نیست")
	}
	if password == "" {
		previousUser, previousPassword, err := s.loadLocked()
		if err != nil {
			return err
		}
		if previousPassword == "" {
			return errors.New("برای ذخیره اولیه، رمز عبور دایا را وارد کنید")
		}
		if username == "" {
			username = previousUser
		}
		password = previousPassword
	}
	if len(password) > 1024 || strings.ContainsRune(password, '\x00') {
		return errors.New("رمز عبور واردشده معتبر نیست")
	}

	key, err := s.readKey()
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("تولید nonce رمزنگاری: %w", err)
	}
	plain := []byte(username + "\x00" + password)
	ciphertext := gcm.Seal(nil, nonce, plain, []byte(credentialAAD))
	record := encryptedDayaCredentials{
		Version:    1,
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
		UpdatedAt:  time.Now().UTC(),
	}
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(encoded)
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil {
		_ = os.Remove(tmp)
		return writeErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	// A credential change invalidates the previous authenticated session.
	if err := os.Remove(s.sessionPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("حذف نشست قبلی پس از تغییر اطلاعات ورود: %w", err)
	}
	if dir, err := os.Open(filepath.Dir(s.path)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}

func (s *DayaCredentialStore) Status() (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	username, password, err := s.loadLocked()
	if err != nil {
		return "", false, err
	}
	return username, password != "", nil
}

func (s *DayaCredentialStore) Load() (string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *DayaCredentialStore) SaveAccessToken(token string) error {
	return s.SaveAccessTokenUntil(token, time.Now().Add(7*time.Hour))
}

func (s *DayaCredentialStore) SaveAccessTokenUntil(token string, expiresAt time.Time) error {
	return s.SaveAccessSession(DayaSession{
		Token:     token,
		ExpiresAt: expiresAt,
		Cookies:   []DayaSessionCookie{{Name: "token", Value: token, Path: "/"}},
	})
}

func (s *DayaCredentialStore) SaveAccessSession(session DayaSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if session.Token == "" || len(session.Token) > 64<<10 || strings.ContainsRune(session.Token, '\x00') {
		return errors.New("توکن دریافتی دایا معتبر نیست")
	}
	cookies := make([]DayaSessionCookie, 0, len(session.Cookies)+1)
	seen := make(map[string]bool)
	for _, cookie := range session.Cookies {
		if cookie.Name != "token" && cookie.Name != "cookiesession1" {
			return errors.New("نوع کوکی نشست دایا پشتیبانی نمی‌شود")
		}
		if cookie.Value == "" || len(cookie.Value) > 32<<10 || strings.ContainsAny(cookie.Value, "\r\n;") {
			return errors.New("مقدار کوکی نشست دایا معتبر نیست")
		}
		if !seen[cookie.Name] {
			seen[cookie.Name] = true
			cookies = append(cookies, cookie)
		}
	}
	if !seen["token"] {
		cookies = append(cookies, DayaSessionCookie{Name: "token", Value: session.Token, Path: "/"})
	} else {
		for i := range cookies {
			if cookies[i].Name == "token" {
				cookies[i].Value = session.Token
			}
		}
	}
	session.Cookies = cookies

	key, err := s.readKey()
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("تولید nonce نشست: %w", err)
	}
	plain, err := json.Marshal(session)
	if err != nil {
		return err
	}
	ciphertext := gcm.Seal(nil, nonce, plain, []byte(dayaSessionAADv2))
	record := encryptedDayaCredentials{
		Version:    2,
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
		UpdatedAt:  time.Now().UTC(),
		ExpiresAt:  session.ExpiresAt,
	}
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.sessionPath), 0700); err != nil {
		return err
	}
	tmp := s.sessionPath + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(encoded)
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil {
		_ = os.Remove(tmp)
		return writeErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if err := os.Rename(tmp, s.sessionPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if dir, err := os.Open(filepath.Dir(s.sessionPath)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}

func (s *DayaCredentialStore) LoadAccessToken() (string, time.Time, error) {
	session, updatedAt, err := s.LoadAccessSession()
	if err != nil {
		return "", time.Time{}, err
	}
	return session.Token, updatedAt, nil
}

func (s *DayaCredentialStore) LoadAccessSession() (DayaSession, time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.sessionPath)
	if errors.Is(err, os.ErrNotExist) {
		return DayaSession{}, time.Time{}, nil
	}
	if err != nil {
		return DayaSession{}, time.Time{}, fmt.Errorf("خواندن نشست دایا: %w", err)
	}
	var record encryptedDayaCredentials
	if err := json.Unmarshal(data, &record); err != nil {
		return DayaSession{}, time.Time{}, errors.New("ساختار فایل نشست دایا معتبر نیست")
	}
	if record.Version != 1 && record.Version != 2 {
		return DayaSession{}, time.Time{}, errors.New("نسخه فایل نشست دایا پشتیبانی نمی‌شود")
	}
	if !record.ExpiresAt.IsZero() && !record.ExpiresAt.After(time.Now()) {
		return DayaSession{}, record.UpdatedAt, nil
	}

	nonce, err := base64.StdEncoding.DecodeString(record.Nonce)
	if err != nil {
		return DayaSession{}, time.Time{}, errors.New("nonce نشست دایا نامعتبر است")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(record.Ciphertext)
	if err != nil {
		return DayaSession{}, time.Time{}, errors.New("داده نشست دایا نامعتبر است")
	}
	key, err := s.readKey()
	if err != nil {
		return DayaSession{}, time.Time{}, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return DayaSession{}, time.Time{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return DayaSession{}, time.Time{}, err
	}
	if len(nonce) != gcm.NonceSize() {
		return DayaSession{}, time.Time{}, errors.New("اندازه nonce نشست دایا نامعتبر است")
	}

	aad := []byte(dayaSessionAADv1)
	if record.Version == 2 {
		aad = []byte(dayaSessionAADv2)
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return DayaSession{}, time.Time{}, errors.New("رمزگشایی نشست دایا ناموفق بود")
	}

	var session DayaSession
	if record.Version == 1 {
		session = DayaSession{
			Token:     string(plain),
			ExpiresAt: record.ExpiresAt,
			Cookies:   []DayaSessionCookie{{Name: "token", Value: string(plain), Path: "/"}},
		}
	} else if err := json.Unmarshal(plain, &session); err != nil {
		return DayaSession{}, time.Time{}, errors.New("محتوای نشست دایا معتبر نیست")
	}
	if session.Token == "" || len(session.Token) > 64<<10 {
		return DayaSession{}, time.Time{}, errors.New("توکن نشست دایا خالی یا نامعتبر است")
	}
	if session.ExpiresAt.IsZero() {
		session.ExpiresAt = record.ExpiresAt
	}
	if !session.ExpiresAt.IsZero() && !session.ExpiresAt.After(time.Now()) {
		return DayaSession{}, record.UpdatedAt, nil
	}

	foundToken := false
	for _, cookie := range session.Cookies {
		if cookie.Name == "token" && cookie.Value == session.Token {
			foundToken = true
		}
		if cookie.Name != "token" && cookie.Name != "cookiesession1" {
			return DayaSession{}, time.Time{}, errors.New("نوع کوکی در نشست ذخیره‌شده معتبر نیست")
		}
	}
	if !foundToken {
		session.Cookies = append(session.Cookies, DayaSessionCookie{Name: "token", Value: session.Token, Path: "/"})
	}
	return session, record.UpdatedAt, nil
}

func (s *DayaCredentialStore) SessionStatus() (time.Time, bool, error) {
	token, updatedAt, err := s.LoadAccessToken()
	return updatedAt, token != "", err
}

func (s *DayaCredentialStore) Delete() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var firstErr error
	for _, path := range []string{s.path, s.sessionPath} {
		err := os.Remove(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *DayaCredentialStore) Render(w http.ResponseWriter, errorMessage, notice string) {
	s.RenderWithMonitor(w, errorMessage, notice, DayaMonitorSnapshot{})
}
func (s *DayaCredentialStore) RenderWithMonitor(w http.ResponseWriter, errorMessage, notice string, monitor DayaMonitorSnapshot) {
	username, saved, err := s.Status()
	if err != nil && errorMessage == "" {
		errorMessage = "خواندن اطلاعات ذخیره‌شده ممکن نشد: " + err.Error()
	}
	loginAt, hasSession, sessionErr := s.SessionStatus()
	if sessionErr != nil && errorMessage == "" {
		errorMessage = "خواندن نشست ذخیره‌شده دایا ممکن نشد."
	}
	loginAtText := ""
	if hasSession {
		loginAtText = formatJalaliDateTime(loginAt.In(tehran()))
	}
	data := DayaSettingsPageData{
		Username:   username,
		Saved:      saved,
		HasSession: hasSession,
		LoginAt:    loginAtText,
		Monitor:    monitor,
		Error:      errorMessage,
		Notice:     notice,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := dayaSettingsPage.Execute(w, data); err != nil {
		// Do not include credential values in logs or error messages.
		fmt.Fprintln(w, "خطا در نمایش تنظیمات دایا")
	}
}
