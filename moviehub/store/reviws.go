package store

import (
    "context"
    "time"
)

type ReviewAuthor struct {
    ID          int64  `json:"id"`
    DisplayName string `json:"displayName"`
}

type Review struct {
    ID        int64        `json:"id"`
    MemberID  int64        `json:"memberId"`
    MovieID   int64        `json:"movieId"`
    Text      string       `json:"text"`
    CreatedAt time.Time    `json:"createdAt"`
    UpdatedAt time.Time    `json:"updatedAt"`
    Member    ReviewAuthor `json:"member"`
    Score     *int16       `json:"score"`
}

func (s *Store) ListReviews(ctx context.Context, movieID int64) ([]Review, error) {
    rows, err := s.Pool.Query(ctx,
        `SELECT r.id, r.member_id, r.movie_id, r.text, r.created_at, r.updated_at,
                m.display_name, v.score
         FROM reviews r
         JOIN members m ON m.id = r.member_id
         LEFT JOIN votes v ON v.member_id = r.member_id AND v.movie_id = r.movie_id
         WHERE r.movie_id = $1
         ORDER BY r.created_at DESC`, movieID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    out := []Review{}
    for rows.Next() {
        var r Review
        if err := rows.Scan(&r.ID, &r.MemberID, &r.MovieID, &r.Text,
            &r.CreatedAt, &r.UpdatedAt, &r.Member.DisplayName, &r.Score); err != nil {
            return nil, err
        }
        r.Member.ID = r.MemberID
        out = append(out, r)
    }
    return out, rows.Err()
}
