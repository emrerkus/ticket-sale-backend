package auth

import (
	"strings"
	"testing"
	"time"
)

func TestPassword_HashVerify(t *testing.T) {
	h, err := HashPassword("supersecret1")
	if err != nil {
		t.Fatal(err)
	}
	if h == "supersecret1" {
		t.Fatal("hash duz sifreyle ayni olmamali")
	}
	if !CheckPassword(h, "supersecret1") {
		t.Fatal("dogru sifre eslesmedi")
	}
	if CheckPassword(h, "yanlis") {
		t.Fatal("yanlis sifre eslesti")
	}

	// Ayni sifre iki farkli hash uretmeli (salt).
	h2, _ := HashPassword("supersecret1")
	if h == h2 {
		t.Fatal("iki hash ayni cikti -> salt yok")
	}
}

func TestJWT_RoundTrip(t *testing.T) {
	tm := NewTokenManager("test-secret", time.Hour)
	tok, err := tm.Generate("user-123")
	if err != nil {
		t.Fatal(err)
	}
	got, err := tm.Parse(tok)
	if err != nil {
		t.Fatal(err)
	}
	if got != "user-123" {
		t.Fatalf("beklenen user-123, gelen %q", got)
	}
}

func TestJWT_RejectsWrongSecret(t *testing.T) {
	a := NewTokenManager("secret-a", time.Hour)
	b := NewTokenManager("secret-b", time.Hour)
	tok, _ := a.Generate("u1")
	if _, err := b.Parse(tok); err == nil {
		t.Fatal("baska secret ile imzalanan token kabul edildi")
	}
}

func TestJWT_RejectsExpired(t *testing.T) {
	tm := NewTokenManager("s", -time.Minute) // gecmiste dolan
	tok, _ := tm.Generate("u1")
	if _, err := tm.Parse(tok); err == nil {
		t.Fatal("suresi dolmus token kabul edildi")
	}
}

func TestJWT_RejectsNoneAlg(t *testing.T) {
	tm := NewTokenManager("s", time.Hour)
	// alg=none, imzasiz bir token (klasik saldiri).
	// header {"alg":"none","typ":"JWT"} . payload {"sub":"admin"} . (imza yok)
	none := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiJhZG1pbiJ9."
	_, err := tm.Parse(none)
	if err == nil {
		t.Fatal("alg=none token kabul edildi")
	}
	if !strings.Contains(err.Error(), "gecersiz") {
		t.Fatalf("beklenen 'gecersiz token', gelen: %v", err)
	}
}
