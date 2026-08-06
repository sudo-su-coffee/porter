package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, no cgo — registers as "sqlite"
)

// Store persists VMs, projects, and domains in a local SQLite database
// (one file, no server, no admin/user tables — a single prototype
// admin account lives in Config instead, see config.go).
//
// Each table is a simple id -> JSON-blob mapping rather than a fully
// normalized schema: VM/Project/Domain already have canonical Go
// structs with JSON tags used throughout api.go and the dashboard, so
// storing them as JSON keeps this file small and keeps "add a field to
// VM" a one-place change instead of a migration. SQLite still buys us
// atomic single-file writes, safe concurrent reads, and simple SQL
// querying over the whole fleet if that's ever needed later.
type Store struct {
	db *sql.DB

	trafficMu sync.RWMutex
	traffic   map[string][]*TrafficEntry // vm_id -> ring buffer (in-memory only, same as before)
}

const schema = `
CREATE TABLE IF NOT EXISTS vms (
	id   TEXT PRIMARY KEY,
	data TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS projects (
	id   TEXT PRIMARY KEY,
	data TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS domains (
	vm_id  TEXT NOT NULL,
	domain TEXT NOT NULL,
	data   TEXT NOT NULL,
	PRIMARY KEY (vm_id, domain)
);
`

// NewStore opens (creating if needed) the SQLite database at path and
// ensures its schema exists. Fatal on failure — same as the original
// JSON store's behavior when its state file couldn't be opened, just
// made explicit instead of silently starting empty.
func NewStore(path string) *Store {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		log.Fatalf("open sqlite store %q: %v", path, err)
	}
	// modernc.org/sqlite serializes access per-connection; keeping a
	// single connection avoids SQLITE_BUSY errors from this process
	// racing itself, which matters more than any write-throughput loss
	// at prototype scale.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		log.Fatalf("init sqlite schema in %q: %v", path, err)
	}
	return &Store{db: db, traffic: map[string][]*TrafficEntry{}}
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// --- VM operations ---

func (s *Store) PutVM(vm *VM) {
	data, err := json.Marshal(vm)
	if err != nil {
		log.Printf("store: marshal vm %s: %v", vm.ID, err)
		return
	}
	if _, err := s.db.Exec(`INSERT INTO vms (id, data) VALUES (?, ?)
		ON CONFLICT(id) DO UPDATE SET data = excluded.data`, vm.ID, data); err != nil {
		log.Printf("store: put vm %s: %v", vm.ID, err)
	}
}

func (s *Store) GetVM(id string) (*VM, bool) {
	var data []byte
	err := s.db.QueryRow(`SELECT data FROM vms WHERE id = ?`, id).Scan(&data)
	if err != nil {
		return nil, false
	}
	var vm VM
	if err := json.Unmarshal(data, &vm); err != nil {
		log.Printf("store: unmarshal vm %s: %v", id, err)
		return nil, false
	}
	return &vm, true
}

func (s *Store) ListVMs() []*VM {
	rows, err := s.db.Query(`SELECT data FROM vms`)
	if err != nil {
		log.Printf("store: list vms: %v", err)
		return nil
	}
	defer rows.Close()
	out := make([]*VM, 0)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			continue
		}
		var vm VM
		if err := json.Unmarshal(data, &vm); err != nil {
			continue
		}
		out = append(out, &vm)
	}
	return out
}

func (s *Store) DeleteVM(id string) {
	if _, err := s.db.Exec(`DELETE FROM vms WHERE id = ?`, id); err != nil {
		log.Printf("store: delete vm %s: %v", id, err)
	}
	if _, err := s.db.Exec(`DELETE FROM domains WHERE vm_id = ?`, id); err != nil {
		log.Printf("store: delete domains for vm %s: %v", id, err)
	}
	s.trafficMu.Lock()
	delete(s.traffic, id)
	s.trafficMu.Unlock()
}

// --- Project operations ---

func (s *Store) PutProject(p *Project) {
	data, err := json.Marshal(p)
	if err != nil {
		log.Printf("store: marshal project %s: %v", p.ID, err)
		return
	}
	if _, err := s.db.Exec(`INSERT INTO projects (id, data) VALUES (?, ?)
		ON CONFLICT(id) DO UPDATE SET data = excluded.data`, p.ID, data); err != nil {
		log.Printf("store: put project %s: %v", p.ID, err)
	}
}

func (s *Store) GetProject(id string) (*Project, bool) {
	var data []byte
	err := s.db.QueryRow(`SELECT data FROM projects WHERE id = ?`, id).Scan(&data)
	if err != nil {
		return nil, false
	}
	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		log.Printf("store: unmarshal project %s: %v", id, err)
		return nil, false
	}
	return &p, true
}

func (s *Store) ListProjects() []*Project {
	rows, err := s.db.Query(`SELECT data FROM projects`)
	if err != nil {
		log.Printf("store: list projects: %v", err)
		return nil
	}
	defer rows.Close()
	out := make([]*Project, 0)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			continue
		}
		var p Project
		if err := json.Unmarshal(data, &p); err != nil {
			continue
		}
		out = append(out, &p)
	}
	return out
}

func (s *Store) DeleteProject(id string) {
	if _, err := s.db.Exec(`DELETE FROM projects WHERE id = ?`, id); err != nil {
		log.Printf("store: delete project %s: %v", id, err)
	}
}

// --- Domains ---

func (s *Store) AddDomain(vmID string, d *Domain) {
	d.VMID = vmID
	data, err := json.Marshal(d)
	if err != nil {
		log.Printf("store: marshal domain %s: %v", d.Domain, err)
		return
	}
	if _, err := s.db.Exec(`INSERT INTO domains (vm_id, domain, data) VALUES (?, ?, ?)
		ON CONFLICT(vm_id, domain) DO UPDATE SET data = excluded.data`, vmID, d.Domain, data); err != nil {
		log.Printf("store: add domain %s for vm %s: %v", d.Domain, vmID, err)
	}
}

func (s *Store) ListDomains(vmID string) []*Domain {
	rows, err := s.db.Query(`SELECT data FROM domains WHERE vm_id = ? ORDER BY rowid`, vmID)
	if err != nil {
		log.Printf("store: list domains for vm %s: %v", vmID, err)
		return nil
	}
	defer rows.Close()
	out := make([]*Domain, 0)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			continue
		}
		var d Domain
		if err := json.Unmarshal(data, &d); err != nil {
			continue
		}
		out = append(out, &d)
	}
	return out
}

func (s *Store) RemoveDomain(vmID, domain string) bool {
	res, err := s.db.Exec(`DELETE FROM domains WHERE vm_id = ? AND domain = ?`, vmID, domain)
	if err != nil {
		log.Printf("store: remove domain %s for vm %s: %v", domain, vmID, err)
		return false
	}
	n, _ := res.RowsAffected()
	return n > 0
}

// --- Traffic ring buffer (bounded, in-memory only — same as before) ---

const trafficRingSize = 200

func (s *Store) AddTraffic(vmID string, e *TrafficEntry) {
	s.trafficMu.Lock()
	defer s.trafficMu.Unlock()
	buf := append(s.traffic[vmID], e)
	if len(buf) > trafficRingSize {
		buf = buf[len(buf)-trafficRingSize:]
	}
	s.traffic[vmID] = buf
}

func (s *Store) ListTraffic(vmID string, limit int) []*TrafficEntry {
	s.trafficMu.RLock()
	defer s.trafficMu.RUnlock()
	buf := s.traffic[vmID]
	if limit <= 0 || limit > len(buf) {
		limit = len(buf)
	}
	start := len(buf) - limit
	out := make([]*TrafficEntry, limit)
	copy(out, buf[start:])
	return out
}
