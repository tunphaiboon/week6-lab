-- MovieHub database schema (PostgreSQL 14+)
-- ที่มาของข้อมูล: หนัง Now Playing จาก TMDB  ดูคำอธิบายทั้งหมดใน docs/db-schema.md
-- This product uses the TMDB API but is not endorsed or certified by TMDB.

BEGIN;

-- ---------------------------------------------------------------------------
-- genres: แนวหนังของ TMDB (มี 19 แนว เปลี่ยนน้อยมาก) เก็บชื่อ 2 ภาษา
-- ---------------------------------------------------------------------------
CREATE TABLE genres (
    id          INTEGER      PRIMARY KEY,           -- รหัสแนวของ TMDB เช่น 28 = Action
    name_en     VARCHAR(50)  NOT NULL,
    name_th     VARCHAR(50)
);

-- ---------------------------------------------------------------------------
-- movies: ข้อมูลหนัง 1 แถวต่อ 1 เรื่อง  id ใช้เลขเดียวกับ TMDB
-- ---------------------------------------------------------------------------
CREATE TABLE movies (
    id                 BIGINT        PRIMARY KEY,   -- TMDB movie id (ไม่ใช้ SERIAL เพราะลิงก์ /movies/{id} ต้องตรงกับ TMDB)
    title              VARCHAR(255)  NOT NULL,      -- ชื่อตามภาษาที่ขอ (JSON: title หรือ name)
    title_th           VARCHAR(255),                -- ชื่อภาษาไทย (ได้จาก request language=th-TH)
    original_title     VARCHAR(255)  NOT NULL,      -- ชื่อภาษาต้นฉบับ (JSON: original_title หรือ original_name)
    original_language  CHAR(2)       NOT NULL,      -- ISO 639-1 เช่น en, th, ja
    overview           TEXT,                        -- เรื่องย่อ (ยาวได้หลายร้อยตัวอักษร)
    overview_th        TEXT,
    poster_path        VARCHAR(100),                -- เก็บแค่ path เช่น /abc.jpg  ประกอบ URL ตอนตอบ API
    backdrop_path      VARCHAR(100),
    release_date       DATE,                        -- บางเรื่อง TMDB ส่ง "" มา ให้เก็บเป็น NULL
    vote_average       NUMERIC(5,3)  NOT NULL DEFAULT 0 CHECK (vote_average BETWEEN 0 AND 10),
    vote_count         INTEGER       NOT NULL DEFAULT 0 CHECK (vote_count >= 0),
    popularity         NUMERIC(12,4) NOT NULL DEFAULT 0,
    adult              BOOLEAN       NOT NULL DEFAULT FALSE,
    video              BOOLEAN       NOT NULL DEFAULT FALSE,
    created_at         TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ   NOT NULL DEFAULT now()   -- อัปเดตทุกครั้งที่ sync จาก TMDB
);

CREATE INDEX idx_movies_release_date ON movies (release_date DESC);
CREATE INDEX idx_movies_popularity   ON movies (popularity DESC);
-- ค้นหาชื่อ (GET /api/movies?q=)  ถ้าข้อมูลเยอะค่อยเปลี่ยนเป็น pg_trgm
CREATE INDEX idx_movies_title_lower  ON movies (lower(title));

-- ---------------------------------------------------------------------------
-- movie_genres: หนัง 1 เรื่องมีได้หลายแนว (JSON: genre_ids)
-- position เก็บลำดับเดิม เพราะหน้าเว็บแสดงแค่แนวแรก (position = 0)
-- ---------------------------------------------------------------------------
CREATE TABLE movie_genres (
    movie_id   BIGINT    NOT NULL REFERENCES movies (id) ON DELETE CASCADE,
    genre_id   INTEGER   NOT NULL REFERENCES genres (id),
    position   SMALLINT  NOT NULL,
    PRIMARY KEY (movie_id, genre_id),
    UNIQUE (movie_id, position)
);

CREATE INDEX idx_movie_genres_genre ON movie_genres (genre_id);

-- ---------------------------------------------------------------------------
-- movie_lists: หนังเรื่องไหนอยู่ในรายการไหน (now_playing / popular) ของประเทศไหน
-- แยกจาก movies เพราะรายการเปลี่ยนทุกสัปดาห์ แต่ข้อมูลหนังยังอยู่ (รีวิวไม่หาย)
-- popular เก็บเฉพาะเรื่องที่ "ไม่อยู่ใน now_playing" ของ region เดียวกัน เพื่อให้ 2 รายการรวมกันได้ครบ 100 เรื่องไม่ซ้ำ
-- sync ทีละ region: ลบของ region นั้นทิ้งแล้ว insert ชุดใหม่ใน transaction เดียว
-- ---------------------------------------------------------------------------
CREATE TABLE movie_lists (
    list          VARCHAR(20)  NOT NULL CHECK (list IN ('now_playing', 'popular')),
    region        CHAR(2)      NOT NULL,            -- ISO 3166-1 เช่น TH, US
    movie_id      BIGINT       NOT NULL REFERENCES movies (id) ON DELETE CASCADE,
    rank          SMALLINT     NOT NULL CHECK (rank >= 1),  -- ลำดับในรายการตามที่ TMDB เรียงมา เริ่มที่ 1
    range_start   DATE,                             -- now_playing เท่านั้น: ช่วงที่ TMDB ถือว่า "กำลังฉาย" ตอน sync
    range_end     DATE,
    synced_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (list, region, movie_id),
    UNIQUE (list, region, rank),
    CHECK (range_start <= range_end)
);

CREATE INDEX idx_movie_lists_movie ON movie_lists (movie_id);

-- ===========================================================================
-- ข้อมูลของเว็บเราเอง (ไม่ได้มาจาก TMDB)
-- ===========================================================================

-- ---------------------------------------------------------------------------
-- members: สมาชิกของเว็บ
-- ---------------------------------------------------------------------------
CREATE TABLE members (
    id             BIGSERIAL     PRIMARY KEY,
    email          VARCHAR(255)  NOT NULL,
    password_hash  VARCHAR(255)  NOT NULL,          -- bcrypt จาก golang.org/x/crypto/bcrypt ห้ามเก็บรหัสผ่านตรง ๆ
    display_name   VARCHAR(50)   NOT NULL CHECK (char_length(btrim(display_name)) >= 1),
    avatar_url     VARCHAR(500),
    role           VARCHAR(10)   NOT NULL DEFAULT 'member' CHECK (role IN ('member', 'admin')),
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT now()
);

-- อีเมลห้ามซ้ำโดยไม่สนตัวพิมพ์เล็กใหญ่ (Go ควร lower() ก่อนบันทึกและก่อนค้นตอน login ด้วย)
CREATE UNIQUE INDEX uq_members_email ON members (lower(email));

-- ---------------------------------------------------------------------------
-- reviews: รีวิวจาก ReviewForm (POST /api/movies/{id}/reviews)
-- สมาชิก 1 คนรีวิวหนัง 1 เรื่องได้ 1 ครั้ง (แก้ไขได้)
-- ---------------------------------------------------------------------------
CREATE TABLE reviews (
    id          BIGSERIAL    PRIMARY KEY,
    member_id   BIGINT       NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    movie_id    BIGINT       NOT NULL REFERENCES movies (id) ON DELETE CASCADE,
    text        TEXT         NOT NULL CHECK (char_length(btrim(text)) >= 10),  -- กติกาเดียวกับหน้าเว็บ
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (member_id, movie_id)
);

CREATE INDEX idx_reviews_movie_created ON reviews (movie_id, created_at DESC);

-- ---------------------------------------------------------------------------
-- votes: คะแนนที่สมาชิกให้หนัง 1–10 (สเกลเดียวกับ vote_average ของ TMDB)
-- 1 คน 1 เรื่อง 1 คะแนน โหวตใหม่ = แก้คะแนนเดิม (upsert)
-- ---------------------------------------------------------------------------
CREATE TABLE votes (
    member_id   BIGINT       NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    movie_id    BIGINT       NOT NULL REFERENCES movies (id) ON DELETE CASCADE,
    score       SMALLINT     NOT NULL CHECK (score BETWEEN 1 AND 10),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (member_id, movie_id)
);

CREATE INDEX idx_votes_movie ON votes (movie_id);

-- คะแนนเฉลี่ยของเว็บเราต่อเรื่อง (คำนวณสด ไม่ต้องเก็บซ้ำ ถ้าวันหลังช้าค่อยเปลี่ยนเป็น materialized view)
CREATE VIEW movie_vote_stats AS
SELECT movie_id,
       count(*)                  AS vote_count,
       round(avg(score), 1)      AS vote_average
FROM votes
GROUP BY movie_id;

-- ---------------------------------------------------------------------------
-- wishlist_items: หนังที่สมาชิกอยากดู สมาชิก 1 คนมี wishlist ของตัวเอง 1 รายการ
-- ---------------------------------------------------------------------------
CREATE TABLE wishlist_items (
    member_id   BIGINT        NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    movie_id    BIGINT        NOT NULL REFERENCES movies (id) ON DELETE CASCADE,
    note        VARCHAR(200),                       -- โน้ตสั้น ๆ เช่น "ดูกับแฟน"
    added_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    PRIMARY KEY (member_id, movie_id)               -- เรื่องเดิมใส่ซ้ำไม่ได้
);

CREATE INDEX idx_wishlist_member_added ON wishlist_items (member_id, added_at DESC);

-- ---------------------------------------------------------------------------
-- seed: แนวหนังทั้งหมดของ TMDB (/genre/movie/list, en-US + th-TH)
-- ---------------------------------------------------------------------------
INSERT INTO genres (id, name_en, name_th) VALUES
    (28,    'Action',          'บู๊'),
    (12,    'Adventure',       'ผจญ'),
    (16,    'Animation',       'แอนนิเมชั่น'),
    (35,    'Comedy',          'ตลก'),
    (80,    'Crime',           'อาชญากรรม'),
    (99,    'Documentary',     'สารคดี'),
    (18,    'Drama',           'หนังชีวิต'),
    (10751, 'Family',          'ครอบครัว'),
    (14,    'Fantasy',         'จินตนาการ'),
    (36,    'History',         'ประวัติศาสตร์'),
    (27,    'Horror',          'สยองขวัญ'),
    (10402, 'Music',           'ดนตรี'),
    (9648,  'Mystery',         'ลึกลับ'),
    (10749, 'Romance',         'หนังรักโรแมนติก'),
    (878,   'Science Fiction', 'นิยายวิทยาศาสตร์'),
    (10770, 'TV Movie',        'ภาพยนตร์โทรทัศน์'),
    (53,    'Thriller',        'ระทึกขวัญ'),
    (10752, 'War',             'สงคราม'),
    (37,    'Western',         'หนังคาวบอยตะวันตก');

COMMIT;
