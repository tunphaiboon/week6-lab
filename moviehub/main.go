package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "time"
    "string"
    "github.com/jackc/pgx/v5/pgxpool"

    "moviehub/auth"
    "moviehub/handler"
    "moviehub/mw"
    "moviehub/store"
)

func main() {
    secret := os.Getenv("JWT_SECRET")
    if len(secret) < 32 {
        log.Fatal("JWT_SECRET ต้องยาวอย่างน้อย 32 ตัวอักษร")
    }
    pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
    if err != nil {
        log.Fatal(err)
    }
    defer pool.Close()
    if err := pool.Ping(context.Background()); err != nil {
        log.Fatal("ต่อฐานข้อมูลไม่ติด ", err)
    }

    tickets := &auth.Issuer{Secret: []byte(secret), TTL: time.Hour}
    h := &handler.Handler{Store: &store.Store{Pool: pool}, Tickets: tickets}

    mux := http.NewServeMux()
    mux.HandleFunc("POST /api/auth/register", h.Register)
    mux.HandleFunc("POST /api/auth/login", h.Login)
    mux.HandleFunc("GET /api/movies", h.SearchMovies)
    mux.HandleFunc("GET /api/movies/{id}/reviews", h.ListReviews)

    // log.Println("MovieHub API ฟังอยู่ที่ :8080")
    // log.Fatal(http.ListenAndServe(":8080", mw.Logging(mux)))
     origins := strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")
    log.Println("MovieHub API ฟังอยู่ที่ :8080 ต้อนรับ", origins)
    log.Fatal(http.ListenAndServe(":8080", mw.Logging(mw.CORS(origins, mux))))
}