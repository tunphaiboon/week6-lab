package store

import (
    "errors"
    "sync"
    "context"                             // 👈 เพิ่มอันนี้
	"github.com/jackc/pgx/v5"             // 👈 เพิ่มอันนี้
	"github.com/jackc/pgx/v5/pgxpool"
)

type Menu struct {
    ID    int     `json:"id"`
    Name  string  `json:"name"`
    Price float64 `json:"price"`
    Type  string  `json:"type"`
    Stock int    `json:"stock"`
}

var ErrNotFound = errors.New("menu not found")

type MenuStore interface {
    List(menuType string) []Menu
    Get(id int) (Menu, error)
    Add(m Menu) Menu
    Delete(id int) error
}

type MemoryStore struct {
    mu     sync.RWMutex
    menus  []Menu
    nextID int
}

func NewMemoryStore() *MemoryStore {
    return &MemoryStore{
        menus: []Menu{
            {ID: 1, Name: "ต้มยำกุ้ง", Price: 120, Type: "soup"},
            {ID: 2, Name: "พิซซ่าฮาวายเอี้ยน", Price: 199, Type: "pizza"},
            {ID: 3, Name: "พิซซ่าเห็ด", Price: 179, Type: "pizza"},
        },
        nextID: 4,
    }
}

func (s *MemoryStore) List(menuType string) []Menu {
    s.mu.RLock()
    defer s.mu.RUnlock()
    if menuType == "" {
        out := make([]Menu, len(s.menus))
        copy(out, s.menus)          // ส่งสำเนาออกไป ไม่ส่งของจริง
        return out
    }
    filtered := []Menu{}
    for _, m := range s.menus {
        if m.Type == menuType {
            filtered = append(filtered, m)
        }
    }
    return filtered
}

func (s *MemoryStore) Get(id int) (Menu, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    for _, m := range s.menus {
        if m.ID == id {
            return m, nil
        }
    }
    return Menu{}, ErrNotFound
}

func (s *MemoryStore) Add(m Menu) Menu {
    s.mu.Lock()
    defer s.mu.Unlock()
    m.ID = s.nextID
    s.nextID++
    s.menus = append(s.menus, m)
    return m
}

func (s *MemoryStore) Delete(id int) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    for i, m := range s.menus {
        if m.ID == id {
            s.menus = append(s.menus[:i], s.menus[i+1:]...)
            return nil
        }
    }
    return ErrNotFound
}
type PostgresStore struct {
    Pool *pgxpool.Pool
}

func (s *PostgresStore) List(ctx context.Context, menuType string) ([]Menu, error) {
    rows, err := s.Pool.Query(ctx,
        `SELECT id, name, price, type, stock FROM menus
         WHERE ($1 = '' OR type = $1)
         ORDER BY id`, menuType)
    if err != nil { return nil, err }
    defer rows.Close()

    menus := []Menu{}
    for rows.Next() {
        var m Menu
        if err := rows.Scan(&m.ID, &m.Name, &m.Price, &m.Type, &m.Stock); err != nil {
            return nil, err
        }
        menus = append(menus, m)
    }
    return menus, rows.Err()
}

func (s *PostgresStore) Get(ctx context.Context, id int) (Menu, error) {
    var m Menu
    err := s.Pool.QueryRow(ctx,
        `SELECT id, name, price, type, stock FROM menus WHERE id = $1`, id).
        Scan(&m.ID, &m.Name, &m.Price, &m.Type, &m.Stock)
    if errors.Is(err, pgx.ErrNoRows) {
        return Menu{}, ErrNotFound
    }
    return m, err
}
func (s *PostgresStore) Add(ctx context.Context, m Menu) (Menu, error) {
    return Menu{}, errors.New("ยังไม่ได้เขียน")
}

func (s *PostgresStore) Delete(ctx context.Context, id int) error {
    return errors.New("ยังไม่ได้เขียน")
}