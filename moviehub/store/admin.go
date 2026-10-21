package store

import (
    "context"
    "errors"
    "time"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgconn"
)

var (
    ErrDuplicateID = errors.New("duplicate id")
    ErrBadGenre    = errors.New("bad genre")
)

// posterSQL ประกอบ URL โปสเตอร์: ถ้า admin ใส่ URL เต็มมา (http...) ใช้ตามนั้น ถ้าเป็น path ของ TMDB (/xxx.jpg) เติมโดเมนให้
const posterSQL = `CASE WHEN m.poster_path LIKE 'http%' THEN m.poster_path
                        ELSE 'https://image.tmdb.org/t/p/w500' || m.poster_path END`

// ---------- Dashboard ----------

type TopMovie struct {
    ID          int64   `json:"id"`
    Title       string  `json:"title"`
    TitleTh     *string `json:"titleTh"`
    Poster      *string `json:"poster"`
    VoteCount   int64   `json:"voteCount"`
    VoteAverage float64 `json:"voteAverage"`
}

type Stats struct {
    Movies        int64      `json:"movies"`
    Members       int64      `json:"members"`
    Admins        int64      `json:"admins"`
    Votes         int64      `json:"votes"`
    WishlistItems int64      `json:"wishlistItems"`
    NewestMembers []Member   `json:"newestMembers"`
    TopVoted      []TopMovie `json:"topVoted"`
}

// Stats รวมตัวเลขของทั้งเว็บในคำขอเดียว ให้หน้า Dashboard ยิงครั้งเดียวจบ
func (s *Store) Stats(ctx context.Context) (Stats, error) {
    var st Stats
    err := s.Pool.QueryRow(ctx,
        `SELECT (SELECT count(*) FROM movies),
                (SELECT count(*) FROM members),
                (SELECT count(*) FROM members WHERE role = 'admin'),
                (SELECT count(*) FROM votes),
                (SELECT count(*) FROM wishlist_items)`).
        Scan(&st.Movies, &st.Members, &st.Admins, &st.Votes, &st.WishlistItems)
    if err != nil {
        return st, err
    }

    rows, err := s.Pool.Query(ctx,
        `SELECT id, email, display_name, role, created_at
         FROM members ORDER BY created_at DESC, id DESC LIMIT 5`)
    if err != nil {
        return st, err
    }
    st.NewestMembers = []Member{}
    for rows.Next() {
        var m Member
        if err := rows.Scan(&m.ID, &m.Email, &m.DisplayName, &m.Role, &m.CreatedAt); err != nil {
            rows.Close()
            return st, err
        }
        st.NewestMembers = append(st.NewestMembers, m)
    }
    rows.Close()
    if err := rows.Err(); err != nil {
        return st, err
    }

    // ใช้ view movie_vote_stats ที่ออกแบบไว้ใน schema: คะแนนเฉลี่ยจากสมาชิกเว็บเรา ไม่ใช่ของ TMDB
    rows, err = s.Pool.Query(ctx,
        `SELECT m.id, m.title, m.title_th, `+posterSQL+`,
                vs.vote_count, vs.vote_average::float8
         FROM movie_vote_stats vs
         JOIN movies m ON m.id = vs.movie_id
         ORDER BY vs.vote_average DESC, vs.vote_count DESC, m.title
         LIMIT 5`)
    if err != nil {
        return st, err
    }
    defer rows.Close()
    st.TopVoted = []TopMovie{}
    for rows.Next() {
        var t TopMovie
        if err := rows.Scan(&t.ID, &t.Title, &t.TitleTh, &t.Poster, &t.VoteCount, &t.VoteAverage); err != nil {
            return st, err
        }
        st.TopVoted = append(st.TopVoted, t)
    }
    return st, rows.Err()
}

// ---------- แนวหนัง (ใช้ทำ dropdown ในฟอร์ม) ----------

type Genre struct {
    ID     int     `json:"id"`
    NameEn string  `json:"nameEn"`
    NameTh *string `json:"nameTh"`
}

func (s *Store) ListGenres(ctx context.Context) ([]Genre, error) {
    rows, err := s.Pool.Query(ctx, `SELECT id, name_en, name_th FROM genres ORDER BY name_en`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    out := []Genre{}
    for rows.Next() {
        var g Genre
        if err := rows.Scan(&g.ID, &g.NameEn, &g.NameTh); err != nil {
            return nil, err
        }
        out = append(out, g)
    }
    return out, rows.Err()
}

// ---------- หนัง: มุมมองของ admin ----------

// MovieInput คือช่องที่ admin แก้ได้ (ฟิลด์ที่ TMDB sync ทับ เช่น popularity vote_count ไม่ให้แก้)
type MovieInput struct {
    Title            string  `json:"title"`
    TitleTh          *string `json:"titleTh"`
    OriginalTitle    string  `json:"originalTitle"`
    OriginalLanguage string  `json:"originalLanguage"`
    Overview         *string `json:"overview"`
    OverviewTh       *string `json:"overviewTh"`
    PosterPath       *string `json:"posterPath"`
    ReleaseDate      *string `json:"releaseDate"` // "YYYY-MM-DD" หรือ null
    GenreID          *int    `json:"genreId"`     // แนวหลัก (position 0) หรือ null
}

// Impact บอกว่าถ้าลบหนังเรื่องนี้ ข้อมูลของสมาชิกจะหายตามไปเท่าไร (ON DELETE CASCADE)
type Impact struct {
    Reviews  int64 `json:"reviews"`
    Votes    int64 `json:"votes"`
    Wishlist int64 `json:"wishlist"`
}

type AdminMovie struct {
    ID int64 `json:"id"`
    MovieInput
    Poster      *string   `json:"poster"` // URL เต็มสำหรับพรีวิวในฟอร์ม
    VoteAverage float64   `json:"voteAverage"`
    UpdatedAt   time.Time `json:"updatedAt"`
    Impact      Impact    `json:"impact"`
}

// AdminGetMovie อ่านทุกช่องที่ฟอร์มต้องใช้ พร้อมจำนวนข้อมูลที่ผูกกับหนังเรื่องนี้
func (s *Store) AdminGetMovie(ctx context.Context, id int64) (AdminMovie, error) {
    var m AdminMovie
    var release *time.Time
    err := s.Pool.QueryRow(ctx,
        `SELECT m.id, m.title, m.title_th, m.original_title, m.original_language,
                m.overview, m.overview_th, m.poster_path, m.release_date,
                (SELECT genre_id FROM movie_genres WHERE movie_id = m.id AND position = 0),
                `+posterSQL+`, round(m.vote_average, 1)::float8, m.updated_at,
                (SELECT count(*) FROM reviews        WHERE movie_id = m.id),
                (SELECT count(*) FROM votes          WHERE movie_id = m.id),
                (SELECT count(*) FROM wishlist_items WHERE movie_id = m.id)
         FROM movies m WHERE m.id = $1`, id).
        Scan(&m.ID, &m.Title, &m.TitleTh, &m.OriginalTitle, &m.OriginalLanguage,
            &m.Overview, &m.OverviewTh, &m.PosterPath, &release, &m.GenreID,
            &m.Poster, &m.VoteAverage, &m.UpdatedAt,
            &m.Impact.Reviews, &m.Impact.Votes, &m.Impact.Wishlist)
    if errors.Is(err, pgx.ErrNoRows) {
        return m, ErrNotFound
    }
    if release != nil {
        d := release.Format("2006-01-02")
        m.ReleaseDate = &d
    }
    return m, err
}

// setMainGenre ตั้งแนวหลัก (position 0) ของหนัง: ลบของเดิมที่ตำแหน่ง 0 หรือที่เป็นแนวเดียวกันก่อน แล้วใส่ใหม่
func setMainGenre(ctx context.Context, tx pgx.Tx, movieID int64, genreID *int) error {
    if genreID == nil {
        _, err := tx.Exec(ctx, `DELETE FROM movie_genres WHERE movie_id = $1 AND position = 0`, movieID)
        return err
    }
    if _, err := tx.Exec(ctx,
        `DELETE FROM movie_genres WHERE movie_id = $1 AND (position = 0 OR genre_id = $2)`,
        movieID, *genreID); err != nil {
        return err
    }
    _, err := tx.Exec(ctx,
        `INSERT INTO movie_genres (movie_id, genre_id, position) VALUES ($1, $2, 0)`,
        movieID, *genreID)
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign key: ไม่มีแนวนี้
        return ErrBadGenre
    }
    return err
}

// CreateMovie เพิ่มหนังที่ไม่ได้มาจาก TMDB (id = 0 ให้ระบบออกเลขเอง เริ่มที่ 9000000 กันชนกับเลขของ TMDB)
func (s *Store) CreateMovie(ctx context.Context, id int64, in MovieInput) (AdminMovie, error) {
    tx, err := s.Pool.Begin(ctx)
    if err != nil {
        return AdminMovie{}, err
    }
    defer tx.Rollback(ctx) // ถ้า Commit ไปแล้ว Rollback จะไม่มีผล

    if id == 0 {
        if err := tx.QueryRow(ctx,
            `SELECT greatest(coalesce(max(id), 0) + 1, 9000000) FROM movies`).Scan(&id); err != nil {
            return AdminMovie{}, err
        }
    }
    _, err = tx.Exec(ctx,
        `INSERT INTO movies (id, title, title_th, original_title, original_language,
                             overview, overview_th, poster_path, release_date)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::date)`,
        id, in.Title, in.TitleTh, in.OriginalTitle, in.OriginalLanguage,
        in.Overview, in.OverviewTh, in.PosterPath, in.ReleaseDate)
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) && pgErr.Code == "23505" {
        return AdminMovie{}, ErrDuplicateID
    }
    if err != nil {
        return AdminMovie{}, err
    }
    if err := setMainGenre(ctx, tx, id, in.GenreID); err != nil {
        return AdminMovie{}, err
    }
    if err := tx.Commit(ctx); err != nil {
        return AdminMovie{}, err
    }
    return s.AdminGetMovie(ctx, id)
}

// UpdateMovie แก้ช่องที่ admin ดูแล ไม่แตะคะแนน/ความนิยมที่มาจาก TMDB
func (s *Store) UpdateMovie(ctx context.Context, id int64, in MovieInput) (AdminMovie, error) {
    tx, err := s.Pool.Begin(ctx)
    if err != nil {
        return AdminMovie{}, err
    }
    defer tx.Rollback(ctx)

    tag, err := tx.Exec(ctx,
        `UPDATE movies
         SET title = $2, title_th = $3, original_title = $4, original_language = $5,
             overview = $6, overview_th = $7, poster_path = $8, release_date = $9::date,
             updated_at = now()
         WHERE id = $1`,
        id, in.Title, in.TitleTh, in.OriginalTitle, in.OriginalLanguage,
        in.Overview, in.OverviewTh, in.PosterPath, in.ReleaseDate)
    if err != nil {
        return AdminMovie{}, err
    }
    if tag.RowsAffected() == 0 {
        return AdminMovie{}, ErrNotFound
    }
    if err := setMainGenre(ctx, tx, id, in.GenreID); err != nil {
        return AdminMovie{}, err
    }
    if err := tx.Commit(ctx); err != nil {
        return AdminMovie{}, err
    }
    return s.AdminGetMovie(ctx, id)
}

// DeleteMovie ลบหนัง รีวิว โหวต และ wishlist ของเรื่องนั้นหายตามด้วย ON DELETE CASCADE ใน schema
func (s *Store) DeleteMovie(ctx context.Context, id int64) error {
    tag, err := s.Pool.Exec(ctx, `DELETE FROM movies WHERE id = $1`, id)
    if err != nil {
        return err
    }
    if tag.RowsAffected() == 0 {
        return ErrNotFound
    }
    return nil
}