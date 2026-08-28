// Copyright (c) 2026 NorvikOps. All rights reserved.
// SPDX-License-Identifier: Elastic-2.0

package auth

// R1-SA13-02 — der Widerrufsspeicher darf nicht unauthentifiziert befuellbar sein.
//
// Logout rief RevokeToken als ERSTES, und RevokeToken prueft nichts: es schreibt
// den Schluessel nach Redis und in die PG-Ausweichtabelle. Ein beliebiger
// Bearer-String landete damit im Speicher — der Handler antwortete danach zwar
// 401, aber der Eintrag war geschrieben. Ein Fremder konnte den Speicher fuellen,
// ohne je ein gueltiges Token zu besitzen.
//
// Der Test beobachtet das, statt es zu behaupten: ein mitschreibender
// Redis-Ersatz zaehlt die tatsaechlich angekommenen Befehle. Ein Test, der nur
// den Statuscode prueft, waere ueber genau diesem Defekt gruen geblieben — 401
// kam auch vorher.

import (
	"bufio"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"aidanwoods.dev/go-paseto"
	"github.com/redis/go-redis/v9"
)

// recordingRedis spricht gerade so viel RESP, dass go-redis zufrieden ist, und
// merkt sich, welche Befehle ankamen.
type recordingRedis struct {
	ln  net.Listener
	mu  sync.Mutex
	cmd []string
}

func startRecordingRedis(t *testing.T) *recordingRedis {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	r := &recordingRedis{ln: ln}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go r.serve(conn)
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return r
}

func (r *recordingRedis) serve(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	br := bufio.NewReader(conn)
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "*") { // nur Arrays sind Befehle
			continue
		}
		n, err := strconv.Atoi(line[1:])
		if err != nil || n <= 0 {
			return
		}
		var parts []string
		for i := 0; i < n; i++ {
			if _, err := br.ReadString('\n'); err != nil { // $<len>
				return
			}
			arg, err := br.ReadString('\n')
			if err != nil {
				return
			}
			parts = append(parts, strings.TrimSpace(arg))
		}
		if len(parts) == 0 {
			continue
		}
		cmd := strings.ToUpper(parts[0])
		r.mu.Lock()
		r.cmd = append(r.cmd, cmd)
		r.mu.Unlock()
		// go-redis v9 eroeffnet mit HELLO 3. Ein "+OK" darauf ist keine gueltige
		// Antwort und laesst den Client die Verbindung aufgeben — dann kaeme nie
		// ein SET an und der Haupttest waere vakuoes gruen. Ein Fehler auf HELLO
		// ist dagegen der dokumentierte Weg: der Client faellt auf RESP2 zurueck.
		if cmd == "HELLO" {
			_, _ = conn.Write([]byte("-ERR unknown command 'HELLO'\r\n"))
			continue
		}
		_, _ = conn.Write([]byte("+OK\r\n"))
	}
}

func (r *recordingRedis) writes() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, c := range r.cmd {
		if c == "SET" {
			n++
		}
	}
	return n
}

func (r *recordingRedis) client() *redis.Client {
	_, port, _ := net.SplitHostPort(r.ln.Addr().String())
	return redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:" + port,
		DialTimeout: time.Second,
		ReadTimeout: time.Second,
		MaxRetries:  -1,
	})
}

func TestLogout_ForgedTokenNeverReachesTheDenyStore(t *testing.T) {
	rec := startRecordingRedis(t)
	key := paseto.NewV4SymmetricKey()
	h := &Handler{service: &Service{key: key, redis: rec.client()}}

	res := postLogout(t, h, "nicht-mal-ein-token")
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("ein gefaelschtes Token muss 401 bekommen, war %d", res.Code)
	}
	if got := rec.writes(); got != 0 {
		t.Fatalf("der Widerrufsspeicher wurde von einem unauthentifizierten Aufrufer "+
			"befuellt: %d SET(s). Die Echtheitspruefung muss VOR dem Widerruf stehen", got)
	}
}

// Gegenprobe — ohne sie waere der Test oben auch dann gruen, wenn Logout gar
// nichts mehr widerruft. Ein gueltiges Token MUSS den Speicher erreichen.
func TestLogout_ValidTokenDoesReachTheDenyStore(t *testing.T) {
	rec := startRecordingRedis(t)
	key := paseto.NewV4SymmetricKey()
	h := &Handler{service: &Service{key: key, redis: rec.client()}}

	tok, err := IssueAccessToken(key, Claims{
		UserID: "11111111-1111-1111-1111-111111111111",
		OrgID:  "22222222-2222-2222-2222-222222222222",
		Roles:  []string{"Admin"},
	})
	if err != nil {
		t.Fatalf("token bauen: %v", err)
	}
	_ = postLogout(t, h, tok)
	if rec.writes() == 0 {
		t.Fatal("ein gueltiges Token muss widerrufen werden — kein SET angekommen; " +
			"der Test oben waere damit vakuoes")
	}
}
