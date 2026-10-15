package store

import (
    "context"
    "errors"
   "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgconn"
)

var ErrEmailTaken = errors.New("email taken")

type Member struct {
    ID           int64  `json:"id"`
    Email        string `json:"email"`
    DisplayName  string `json:"displayName"`
    Role         string `json:"role"`
    PasswordHash string `json:"-"` // ไม่มีวันออกไปกับ JSON
}

func (s *Store) CreateMember(ctx context.Context, email, hash, name string) (Member, error) {
    m := Member{Email: email, DisplayName: name}
    err := s.Pool.QueryRow(ctx,
        `INSERT INTO members (email, password_hash, display_name)
         VALUES ($1, $2, $3)
         RETURNING id, role`, email, hash, name).Scan(&m.ID, &m.Role)
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) && pgErr.Code == "23505" {
        return Member{}, ErrEmailTaken
    }
    return m, err
}
func (s *Store) MemberByEmail(ctx context.Context, email string) (Member, error) {
    var m Member
    err := s.Pool.QueryRow(ctx,
        `SELECT id, email, display_name, role, password_hash
         FROM members WHERE lower(email) = $1`, email).
        Scan(&m.ID, &m.Email, &m.DisplayName, &m.Role, &m.PasswordHash)
    if errors.Is(err, pgx.ErrNoRows) {
        return Member{}, ErrNotFound
    }
    return m, err
}