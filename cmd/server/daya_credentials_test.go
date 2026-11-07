package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestCredentialStore(t *testing.T) *DayaCredentialStore {
	t.Helper()
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "daya.key")
	if err := os.WriteFile(keyPath, bytes.Repeat([]byte{0x5a}, 32), 0600); err != nil {
		t.Fatal(err)
	}
	return NewDayaCredentialStore(dir, keyPath)
}

func TestDayaCredentialStoreEncryptsAndRoundTrips(t *testing.T) {
	store := newTestCredentialStore(t)
	if err := store.Save("test-user", "very-secret-password"); err != nil {
		t.Fatal(err)
	}

	username, saved, err := store.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !saved || username != "test-user" {
		t.Fatalf("unexpected status: username=%q saved=%v", username, saved)
	}

	data, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("test-user")) || bytes.Contains(data, []byte("very-secret-password")) {
		t.Fatal("credential file contains plaintext credentials")
	}
	info, err := os.Stat(store.path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("credential file permissions = %04o, want 0600", info.Mode().Perm())
	}
}

func TestDayaCredentialStoreKeepsPasswordWhenBlank(t *testing.T) {
	store := newTestCredentialStore(t)
	if err := store.Save("first-user", "saved-password"); err != nil {
		t.Fatal(err)
	}
	if err := store.Save("second-user", ""); err != nil {
		t.Fatal(err)
	}

	store.mu.Lock()
	username, password, err := store.loadLocked()
	store.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if username != "second-user" || password != "saved-password" {
		t.Fatalf("credentials were not preserved: username=%q password-preserved=%v", username, password == "saved-password")
	}
}

func TestDayaCredentialStoreDelete(t *testing.T) {
	store := newTestCredentialStore(t)
	if err := store.Save("test-user", "saved-password"); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(); err != nil {
		t.Fatal(err)
	}
	username, saved, err := store.Status()
	if err != nil {
		t.Fatal(err)
	}
	if saved || username != "" {
		t.Fatalf("credentials still present after delete: username=%q saved=%v", username, saved)
	}
}

func TestDayaCredentialStoreRejectsTamperedCiphertext(t *testing.T) {
	store := newTestCredentialStore(t)
	if err := store.Save("test-user", "saved-password"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	var record encryptedDayaCredentials
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(record.Ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext[len(ciphertext)-1] ^= 1
	record.Ciphertext = base64.StdEncoding.EncodeToString(ciphertext)
	data, err = json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Status(); err == nil {
		t.Fatal("tampered credential file was accepted")
	}
}

func TestDayaCredentialStoreTreatsExpiredSessionAsMissing(t *testing.T) {
	store := newTestCredentialStore(t)
	if err := store.SaveAccessTokenUntil("expired-session-token", time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	token, _, err := store.LoadAccessToken()
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		t.Fatal("expired Daya session was returned as active")
	}
}


func TestDayaCredentialStoreEncryptsAndRestoresBothSessionCookies(t *testing.T) {
	store := newTestCredentialStore(t)
	expires := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	session := DayaSession{Token: "private-token-value", ExpiresAt: expires, Cookies: []DayaSessionCookie{
		{Name: "token", Value: "private-token-value", Path: "/", Secure: true, HTTPOnly: true},
		{Name: "cookiesession1", Value: "private-captcha-cookie", Path: "/", Secure: true, HTTPOnly: true},
	}}
	if err := store.SaveAccessSession(session); err != nil { t.Fatal(err) }
	loaded, _, err := store.LoadAccessSession()
	if err != nil { t.Fatal(err) }
	if loaded.Token != session.Token || !loaded.ExpiresAt.Equal(expires) || len(loaded.Cookies) != 2 {
		t.Fatalf("session round trip mismatch: token=%v expiry=%v cookies=%d", loaded.Token == session.Token, loaded.ExpiresAt, len(loaded.Cookies))
	}
	data, err := os.ReadFile(store.sessionPath)
	if err != nil { t.Fatal(err) }
	for _, secret := range []string{"private-token-value", "private-captcha-cookie"} {
		if bytes.Contains(data, []byte(secret)) { t.Fatalf("session file contains plaintext value %q", secret) }
	}
	var record encryptedDayaCredentials
	if err := json.Unmarshal(data, &record); err != nil { t.Fatal(err) }
	if record.Version != 2 { t.Fatalf("session version=%d, want 2", record.Version) }
}

func TestDayaCredentialStoreReadsLegacyV1Session(t *testing.T) {
	store := newTestCredentialStore(t)
	key, err := store.readKey()
	if err != nil { t.Fatal(err) }
	block, err := aes.NewCipher(key)
	if err != nil { t.Fatal(err) }
	gcm, err := cipher.NewGCM(block)
	if err != nil { t.Fatal(err) }
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil { t.Fatal(err) }
	token := "legacy-token"
	expires := time.Now().Add(time.Hour)
	ciphertext := gcm.Seal(nil, nonce, []byte(token), []byte("daya-assistant/daya-session/v1"))
	record := encryptedDayaCredentials{Version: 1, Nonce: base64.StdEncoding.EncodeToString(nonce), Ciphertext: base64.StdEncoding.EncodeToString(ciphertext), UpdatedAt: time.Now(), ExpiresAt: expires}
	data, err := json.Marshal(record)
	if err != nil { t.Fatal(err) }
	if err := os.WriteFile(store.sessionPath, data, 0600); err != nil { t.Fatal(err) }
	loaded, _, err := store.LoadAccessSession()
	if err != nil { t.Fatal(err) }
	if loaded.Token != token || len(loaded.Cookies) != 1 || loaded.Cookies[0].Name != "token" { t.Fatalf("legacy session not restored: %#v", loaded) }
}
