package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

//go:embed index.html favicon.svg
var staticFiles embed.FS

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type tailer struct {
	path  string
	mu    sync.Mutex
	buf   []byte
	inode uint64
}

type broker struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
}

func newBroker() *broker {
	return &broker{clients: make(map[*websocket.Conn]struct{})}
}

func (b *broker) add(conn *websocket.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clients[conn] = struct{}{}
}

func (b *broker) remove(conn *websocket.Conn) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.clients, conn)
	conn.Close()
}

func (b *broker) broadcast(msg []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for conn := range b.clients {
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			delete(b.clients, conn)
			conn.Close()
		}
	}
}

type wsMessage struct {
	Type  string   `json:"type"`
	Path  string   `json:"path,omitempty"`
	Paths []string `json:"paths,omitempty"`
	Data  string   `json:"data,omitempty"`
}

func (t *tailer) snapshot() []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]byte, len(t.buf))
	copy(out, t.buf)
	return out
}

func startTailer(t *tailer, b *broker) {
	go func() {
		var f *os.File
		var currentInode uint64
		var offset int64

		for {
			if _, err := os.Stat(t.path); err == nil {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}

		openFile := func(sendReset bool) error {
			var err error
			f, err = os.Open(t.path)
			if err != nil {
				return err
			}

			stat, err := f.Stat()
			if err != nil {
				f.Close()
				return err
			}

			// Get inode from stat (requires syscall)
			if sys, ok := stat.Sys().(*syscall.Stat_t); ok {
				currentInode = sys.Ino
			}
			offset = 0

			// Store inode in tailer struct for registry to check
			t.mu.Lock()
			t.inode = currentInode
			if sendReset {
				// Reset buffer and notify clients
				t.buf = nil
			}
			t.mu.Unlock()

			if sendReset {
				msg, _ := json.Marshal(wsMessage{Type: "reset", Path: t.path})
				b.broadcast(msg)
				log.Printf("reopened %s (inode %d) after file change", t.path, currentInode)
			} else {
				log.Printf("opened %s (inode %d)", t.path, currentInode)
			}

			return nil
		}

		if err := openFile(false); err != nil {
			log.Printf("failed to open %s: %v", t.path, err)
			return
		}
		defer func() {
			if f != nil {
				f.Close()
			}
		}()

		tmp := make([]byte, 32*1024)
		for {
			// Check if file was truncated or replaced
			stat, err := os.Stat(t.path)
			if err == nil {
				fileSize := stat.Size()
				var fileInode uint64
				if sys, ok := stat.Sys().(*syscall.Stat_t); ok {
					fileInode = sys.Ino
				}

				// Detect truncation (file size < current offset) or replacement (different inode)
				if fileSize < offset || (fileInode != 0 && fileInode != currentInode) {
					log.Printf("detected file change on %s (truncation: %v, replacement: %v)",
						t.path, fileSize < offset, fileInode != currentInode)

					if f != nil {
						f.Close()
					}

					if err := openFile(true); err != nil {
						log.Printf("failed to reopen %s: %v", t.path, err)
						return
					}
				}
			}

			n, err := f.Read(tmp)
			if n > 0 {
				chunk := make([]byte, n)
				copy(chunk, tmp[:n])
				offset += int64(n)

				t.mu.Lock()
				t.buf = append(t.buf, chunk...)
				t.mu.Unlock()

				msg, _ := json.Marshal(wsMessage{Type: "append", Path: t.path, Data: string(chunk)})
				b.broadcast(msg)
			}
			if err == io.EOF {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			if err != nil {
				log.Printf("read error on %s: %v", t.path, err)
				return
			}
		}
	}()
}

type registry struct {
	mu       sync.Mutex
	patterns []string
	tailers  map[string]*tailer
	paths    []string
	broker   *broker
}

func newRegistry(patterns []string, b *broker) *registry {
	return &registry{
		patterns: patterns,
		tailers:  make(map[string]*tailer),
		broker:   b,
	}
}

func (r *registry) scan() {
	var matches []string
	for _, pattern := range r.patterns {
		m, err := filepath.Glob(pattern)
		if err != nil {
			log.Printf("glob error for %s: %v", pattern, err)
			continue
		}
		matches = append(matches, m...)
	}
	sort.Strings(matches)

	r.mu.Lock()
	defer r.mu.Unlock()

	matchSet := make(map[string]struct{}, len(matches))
	for _, p := range matches {
		matchSet[p] = struct{}{}
	}

	changed := false

	// Remove tailers for files that no longer exist
	for p := range r.tailers {
		if _, exists := matchSet[p]; !exists {
			delete(r.tailers, p)
			log.Printf("removed: %s", p)
			changed = true
		}
	}

	// Check for inode changes on existing files
	for p, t := range r.tailers {
		stat, err := os.Stat(p)
		if err != nil {
			// File disappeared between glob and stat, will be caught in next scan
			continue
		}

		var currentInode uint64
		if sys, ok := stat.Sys().(*syscall.Stat_t); ok {
			currentInode = sys.Ino
		}

		t.mu.Lock()
		storedInode := t.inode
		t.mu.Unlock()

		// If inode changed, restart the tailer
		if currentInode != 0 && storedInode != 0 && currentInode != storedInode {
			log.Printf("inode changed: %s", p)
			delete(r.tailers, p)
			newTailer := &tailer{path: p}
			r.tailers[p] = newTailer
			startTailer(newTailer, r.broker)
			changed = true
		}
	}

	// Add tailers for new files
	for _, p := range matches {
		if _, exists := r.tailers[p]; !exists {
			t := &tailer{path: p}
			r.tailers[p] = t
			startTailer(t, r.broker)
			log.Printf("discovered: %s", p)
			changed = true
		}
	}

	if changed {
		r.paths = make([]string, 0, len(r.tailers))
		for p := range r.tailers {
			r.paths = append(r.paths, p)
		}
		sort.Strings(r.paths)

		msg, _ := json.Marshal(wsMessage{Type: "paths", Paths: r.paths})
		r.broker.broadcast(msg)
	}
}

func (r *registry) getPaths() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.paths))
	copy(out, r.paths)
	return out
}

func (r *registry) getTailer(path string) *tailer {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tailers[path]
}

func (r *registry) watch() {
	for {
		time.Sleep(2 * time.Second)
		r.scan()
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <glob-pattern> [glob-pattern...]\n", os.Args[0])
		os.Exit(1)
	}
	patterns := os.Args[1:]

	host := os.Getenv("RALPH_LOGS_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("RALPH_LOGS_PORT")
	if port == "" {
		log.Fatal("RALPH_LOGS_PORT environment variable is required")
	}

	b := newBroker()
	reg := newRegistry(patterns, b)
	reg.scan()
	go reg.watch()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("websocket upgrade error: %v", err)
			return
		}

		paths := reg.getPaths()
		initPath := ""
		initData := ""
		if len(paths) > 0 {
			initPath = paths[0]
			if t := reg.getTailer(initPath); t != nil {
				initData = string(t.snapshot())
			}
		}

		initMsg, _ := json.Marshal(wsMessage{Type: "init", Paths: paths, Path: initPath, Data: initData})
		if err := conn.WriteMessage(websocket.TextMessage, initMsg); err != nil {
			conn.Close()
			return
		}

		b.add(conn)

		go func() {
			defer b.remove(conn)
			for {
				_, raw, err := conn.ReadMessage()
				if err != nil {
					return
				}
				var msg wsMessage
				if json.Unmarshal(raw, &msg) != nil {
					continue
				}
				if msg.Type == "select" {
					t := reg.getTailer(msg.Path)
					if t == nil {
						continue
					}
					data := t.snapshot()
					resp, _ := json.Marshal(wsMessage{Type: "init", Path: msg.Path, Data: string(data)})
					if conn.WriteMessage(websocket.TextMessage, resp) != nil {
						return
					}
				}
			}
		}()
	})

	http.HandleFunc("/favicon.svg", func(w http.ResponseWriter, r *http.Request) {
		data, _ := staticFiles.ReadFile("favicon.svg")
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Write(data)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data, _ := staticFiles.ReadFile("index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})

	addr := host + ":" + port
	log.Printf("serving on http://%s — watching %d patterns", addr, len(patterns))
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
