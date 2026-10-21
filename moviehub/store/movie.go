package store
import (
    "context"
    "errors"

    "github.com/jackc/pgx/v5"
)

// MovieDetail คือหนังหนึ่งเรื่องพร้อมเรื่องย่อ ใช้กับหน้ารายละเอียด
type MovieDetail struct {
    Movie
    Detail *string `json:"detail"`
}

// GetMovie หาหนังจากเลข ไม่พบตอบ ErrNotFound
func (s *Store) GetMovie(ctx context.Context, id int64) (MovieDetail, error) {
    var m MovieDetail
    err := s.Pool.QueryRow(ctx,
        `SELECT m.id, m.title, m.title_th,
                (SELECT coalesce(g.name_th, g.name_en) FROM movie_genres mg
                   JOIN genres g ON g.id = mg.genre_id
                  WHERE mg.movie_id = m.id AND mg.position = 0),
                extract(year FROM m.release_date)::int,
                round(m.vote_average, 1)::float8,
                'https://image.tmdb.org/t/p/w500' || m.poster_path,
                coalesce(m.overview_th, m.overview)
         FROM movies m
         WHERE m.id = $1`, id).
        Scan(&m.ID, &m.Title, &m.TitleTh, &m.Genre, &m.Year, &m.Rating, &m.Poster, &m.Detail)
    if errors.Is(err, pgx.ErrNoRows) {
        return m, ErrNotFound
    }
    return m, err
}