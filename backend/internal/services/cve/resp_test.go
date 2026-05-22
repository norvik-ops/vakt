package cve

// fakeRESPServer is a minimal Redis RESP protocol server for use in tests.
// It supports only SET (with EX/PX) and GET — enough to exercise the Redis
// caching path in CVEEnrichmentService without an external Redis instance or
// miniredis dependency.

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type fakeRESPServer struct {
	mu       sync.Mutex
	store    map[string]storedEntry
	addr     string
	listener net.Listener
	done     chan struct{}
}

type storedEntry struct {
	value  []byte
	expiry time.Time // zero means no expiry
}

func newFakeRESPServer(initial map[string][]byte) *fakeRESPServer {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic("fakeRESPServer: listen: " + err.Error())
	}

	s := &fakeRESPServer{
		store:    make(map[string]storedEntry),
		addr:     ln.Addr().String(),
		listener: ln,
		done:     make(chan struct{}),
	}

	// Pre-populate with any values passed from the caller.
	for k, v := range initial {
		s.store[k] = storedEntry{value: v}
	}

	go s.serve()
	return s
}

func (s *fakeRESPServer) Close() {
	close(s.done)
	_ = s.listener.Close()
}

func (s *fakeRESPServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.done:
				return
			default:
				return
			}
		}
		go s.handleConn(conn)
	}
}

func (s *fakeRESPServer) handleConn(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)

	for {
		cmd, err := readCommand(r)
		if err != nil {
			return
		}
		if len(cmd) == 0 {
			continue
		}

		switch strings.ToUpper(cmd[0]) {
		case "COMMAND":
			// redis/go-redis sends COMMAND DOCS on connect; reply with empty array.
			_, _ = fmt.Fprintf(w, "*0\r\n")
		case "SET":
			s.handleSET(w, cmd)
		case "GET":
			s.handleGET(w, cmd)
		case "PING":
			_, _ = fmt.Fprintf(w, "+PONG\r\n")
		default:
			_, _ = fmt.Fprintf(w, "-ERR unknown command '%s'\r\n", cmd[0])
		}
		_ = w.Flush()
	}
}

func (s *fakeRESPServer) handleSET(w *bufio.Writer, cmd []string) {
	// SET key value [EX seconds | PX milliseconds | EXAT unix-time-seconds | PXAT unix-time-ms | KEEPTTL]
	if len(cmd) < 3 {
		_, _ = fmt.Fprintf(w, "-ERR wrong number of arguments for SET\r\n")
		return
	}
	key := cmd[1]
	val := []byte(cmd[2])

	entry := storedEntry{value: val}

	// Parse optional expiry flags.
	for i := 3; i < len(cmd)-1; i++ {
		switch strings.ToUpper(cmd[i]) {
		case "EX":
			secs, err := strconv.ParseInt(cmd[i+1], 10, 64)
			if err == nil {
				entry.expiry = time.Now().Add(time.Duration(secs) * time.Second)
			}
			i++
		case "PX":
			ms, err := strconv.ParseInt(cmd[i+1], 10, 64)
			if err == nil {
				entry.expiry = time.Now().Add(time.Duration(ms) * time.Millisecond)
			}
			i++
		}
	}

	s.mu.Lock()
	s.store[key] = entry
	s.mu.Unlock()

	_, _ = fmt.Fprintf(w, "+OK\r\n")
}

func (s *fakeRESPServer) handleGET(w *bufio.Writer, cmd []string) {
	if len(cmd) < 2 {
		_, _ = fmt.Fprintf(w, "-ERR wrong number of arguments for GET\r\n")
		return
	}
	key := cmd[1]

	s.mu.Lock()
	entry, ok := s.store[key]
	s.mu.Unlock()

	if !ok || (!entry.expiry.IsZero() && time.Now().After(entry.expiry)) {
		_, _ = fmt.Fprintf(w, "$-1\r\n") // null bulk string
		return
	}

	_, _ = fmt.Fprintf(w, "$%d\r\n%s\r\n", len(entry.value), entry.value)
}

// readCommand reads a single RESP array command from the reader.
func readCommand(r *bufio.Reader) ([]string, error) {
	line, err := readLine(r)
	if err != nil {
		return nil, err
	}
	if len(line) == 0 {
		return nil, nil
	}

	if line[0] == '*' {
		// Array
		count, err := strconv.Atoi(line[1:])
		if err != nil || count < 0 {
			return nil, fmt.Errorf("resp: invalid array length: %s", line)
		}
		args := make([]string, 0, count)
		for i := 0; i < count; i++ {
			arg, err := readBulkString(r)
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}
		return args, nil
	}

	// Inline command fallback.
	parts := strings.Fields(line)
	return parts, nil
}

func readBulkString(r *bufio.Reader) (string, error) {
	line, err := readLine(r)
	if err != nil {
		return "", err
	}
	if len(line) == 0 || line[0] != '$' {
		return "", fmt.Errorf("resp: expected bulk string, got: %q", line)
	}
	n, err := strconv.Atoi(line[1:])
	if err != nil || n < 0 {
		return "", fmt.Errorf("resp: invalid bulk length: %s", line)
	}
	buf := make([]byte, n+2) // +2 for \r\n
	if _, err := readFull(r, buf); err != nil {
		return "", err
	}
	return string(buf[:n]), nil
}

func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func readFull(r *bufio.Reader, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
