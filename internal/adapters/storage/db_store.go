package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"tracktrades/internal/domain/portfolio"
	"tracktrades/internal/ports"
)

// SQLitePortfolioStore persists all portfolios in a single SQLite database file using database/sql.
type SQLitePortfolioStore struct {
	db *sql.DB
	mu sync.RWMutex
}

var _ ports.PortfolioStore = (*SQLitePortfolioStore)(nil)

func NewDBPortfolioStore(path string) (*SQLitePortfolioStore, error) {
	if err := ensureSQLiteDir(path); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite-simple", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	store := &SQLitePortfolioStore{db: db}
	if err := store.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLitePortfolioStore) initSchema() error {
	ctx := context.Background()
	_, err := s.db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS portfolios (name TEXT PRIMARY KEY, data TEXT NOT NULL);")
	return err
}

func (s *SQLitePortfolioStore) Create(ctx context.Context, name string, cash float64) (*portfolio.Portfolio, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if name == "" {
		return nil, errors.New("portfolio name is required")
	}

	exists, err := s.exists(ctx, name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fs.ErrExist
	}

	p := portfolio.New(name, cash)
	if err := s.saveUnlocked(ctx, name, p); err != nil {
		return nil, err
	}
	return clonePortfolio(p), nil
}

func (s *SQLitePortfolioStore) List(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, "SELECT name FROM portfolios ORDER BY name;")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return names, nil
}

func (s *SQLitePortfolioStore) Load(ctx context.Context, name string) (*portfolio.Portfolio, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if name == "" {
		return nil, errors.New("portfolio name is required")
	}

	var data string
	err := s.db.QueryRowContext(ctx, "SELECT data FROM portfolios WHERE name=?", name).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fs.ErrNotExist
	}
	if err != nil {
		return nil, err
	}

	var p portfolio.Portfolio
	if err := json.Unmarshal([]byte(data), &p); err != nil {
		return nil, err
	}
	if p.Positions == nil {
		p.Positions = make(map[string]*portfolio.Position)
	}
	return clonePortfolio(&p), nil
}

func (s *SQLitePortfolioStore) Save(ctx context.Context, name string, p *portfolio.Portfolio) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if name == "" {
		return errors.New("portfolio name is required")
	}
	return s.saveUnlocked(ctx, name, p)
}

func (s *SQLitePortfolioStore) Remove(ctx context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if name == "" {
		return errors.New("portfolio name is required")
	}
	res, err := s.db.ExecContext(ctx, "DELETE FROM portfolios WHERE name=?", name)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fs.ErrNotExist
	}
	return nil
}

func (s *SQLitePortfolioStore) saveUnlocked(ctx context.Context, name string, p *portfolio.Portfolio) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "INSERT INTO portfolios(name, data) VALUES(?, ?) ON CONFLICT(name) DO UPDATE SET data=excluded.data;", name, string(data))
	return err
}

func (s *SQLitePortfolioStore) exists(ctx context.Context, name string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM portfolios WHERE name=?)", name).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func clonePortfolio(p *portfolio.Portfolio) *portfolio.Portfolio {
	if p == nil {
		return nil
	}
	cp := *p
	if p.Positions != nil {
		cp.Positions = make(map[string]*portfolio.Position, len(p.Positions))
		for k, v := range p.Positions {
			pos := *v
			cp.Positions[k] = &pos
		}
	}
	return &cp
}

func ensureSQLiteDir(path string) error {
	if path == "" || path == ":memory:" {
		return nil
	}

	if strings.HasPrefix(path, "file:") {
		path = strings.TrimPrefix(path, "file:")
	}
	if i := strings.Index(path, "?"); i >= 0 {
		path = path[:i]
	}
	if path == "" || path == ":memory:" {
		return nil
	}

	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
