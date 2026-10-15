// สร้าง docs/db/seed.sql จาก TMDB: หนังกำลังฉายในไทย + หนังยอดนิยมที่ไม่ซ้ำ รวม 100 เรื่อง (กติกาเดียวกับ getMovies() ใน src/api/tmdb.js)
// วิธีรัน (Node 18+):  node --env-file=.env docs/db/make-seed.mjs
// ต้องรัน schema.sql ก่อน แล้วค่อยรัน seed.sql ที่ได้
import { writeFileSync } from 'node:fs';

const BASE = 'https://api.themoviedb.org/3';
const KEY = process.env.TMDB_KEY || process.env.REACT_APP_TMDB_KEY;
const REGION = 'TH';
const LIMIT = 100;
const OUT = new URL('./seed.sql', import.meta.url);

if (!KEY) throw new Error('ไม่พบ API key: ตั้ง TMDB_KEY หรือ REACT_APP_TMDB_KEY ก่อน');

async function getJSON(path, params = {}) {
  const url = new URL(BASE + path);
  url.searchParams.set('api_key', KEY);
  for (const [name, value] of Object.entries(params)) url.searchParams.set(name, value);
  const res = await fetch(url);
  if (!res.ok) throw new Error(`TMDB ตอบกลับ ${res.status} (${path})`);
  return res.json();
}

// ไล่ดึงรายการทีละหน้า เก็บเฉพาะเรื่องที่ไม่อยู่ใน seen จนได้ครบ limit
async function collect(path, limit, seen) {
  const picked = [];
  let dates = null;
  for (let page = 1; picked.length < limit; page++) {
    const data = await getJSON(path, { region: REGION, language: 'en-US', page });
    dates ??= data.dates;
    for (const m of data.results) {
      if (seen.has(m.id)) continue;
      seen.add(m.id);
      picked.push(m);
      if (picked.length === limit) break;
    }
    if (page >= data.total_pages) break;
  }
  return { picked, dates };
}

// ชื่อและเรื่องย่อภาษาไทย ถ้าไม่มีคำแปล TMDB จะคืนชื่อเดิมมา เราเก็บเป็น NULL แทน
async function getThai(m) {
  const th = await getJSON(`/movie/${m.id}`, { language: 'th-TH' });
  return {
    titleTh: th.title && th.title !== m.title && th.title !== m.original_title ? th.title : null,
    overviewTh: th.overview || null,
  };
}

// ทำทีละ size เรื่อง เพื่อไม่ยิง TMDB พร้อมกันเยอะเกิน
async function inBatches(items, size, fn) {
  const out = [];
  for (let i = 0; i < items.length; i += size) {
    out.push(...await Promise.all(items.slice(i, i + size).map(fn)));
  }
  return out;
}

function sql(v) {
  if (v === null || v === undefined || v === '') return 'NULL';
  if (typeof v === 'number' || typeof v === 'boolean') return String(v);
  return `'${String(v).replace(/'/g, "''")}'`;
}

const knownGenres = new Set((await getJSON('/genre/movie/list', { language: 'en-US' })).genres.map(g => g.id));
const seen = new Set();
const nowPlaying = await collect('/movie/now_playing', LIMIT, seen);
const popular = await collect('/movie/popular', LIMIT - nowPlaying.picked.length, seen);
const movies = [...nowPlaying.picked, ...popular.picked];
const thai = await inBatches(movies, 10, getThai);

const lines = [
  `-- สร้างโดย docs/db/make-seed.mjs เมื่อ ${new Date().toISOString()}`,
  `-- region ${REGION}: now_playing ${nowPlaying.picked.length} เรื่อง + popular ${popular.picked.length} เรื่อง = ${movies.length} เรื่อง`,
  '-- This product uses the TMDB API but is not endorsed or certified by TMDB.',
  '',
  'BEGIN;',
  '',
  'INSERT INTO movies (id, title, title_th, original_title, original_language, overview, overview_th,',
  '                    poster_path, backdrop_path, release_date, vote_average, vote_count, popularity, adult, video)',
  'VALUES',
  movies.map((m, i) => '    (' + [
    m.id, m.title, thai[i].titleTh, m.original_title, m.original_language, m.overview, thai[i].overviewTh,
    m.poster_path, m.backdrop_path, m.release_date, m.vote_average, m.vote_count, m.popularity, m.adult, m.video,
  ].map(sql).join(', ') + ')').join(',\n'),
  'ON CONFLICT (id) DO UPDATE SET',
  '    title = EXCLUDED.title, title_th = EXCLUDED.title_th, original_title = EXCLUDED.original_title,',
  '    original_language = EXCLUDED.original_language, overview = EXCLUDED.overview, overview_th = EXCLUDED.overview_th,',
  '    poster_path = EXCLUDED.poster_path, backdrop_path = EXCLUDED.backdrop_path, release_date = EXCLUDED.release_date,',
  '    vote_average = EXCLUDED.vote_average, vote_count = EXCLUDED.vote_count, popularity = EXCLUDED.popularity,',
  '    adult = EXCLUDED.adult, video = EXCLUDED.video, updated_at = now();',
  '',
  `DELETE FROM movie_genres WHERE movie_id IN (${movies.map(m => m.id).join(', ')});`,
  'INSERT INTO movie_genres (movie_id, genre_id, position) VALUES',
  movies.flatMap(m => m.genre_ids.filter(id => knownGenres.has(id)).map((g, pos) => `    (${m.id}, ${g}, ${pos})`)).join(',\n') + ';',
  '',
  `DELETE FROM movie_lists WHERE region = ${sql(REGION)};`,
  'INSERT INTO movie_lists (list, region, movie_id, rank, range_start, range_end) VALUES',
  [
    ...nowPlaying.picked.map((m, i) => `    ('now_playing', ${sql(REGION)}, ${m.id}, ${i + 1}, ${sql(nowPlaying.dates?.minimum)}, ${sql(nowPlaying.dates?.maximum)})`),
    ...popular.picked.map((m, i) => `    ('popular', ${sql(REGION)}, ${m.id}, ${i + 1}, NULL, NULL)`),
  ].join(',\n') + ';',
  '',
  'COMMIT;',
  '',
];

writeFileSync(OUT, lines.join('\n'));
console.log(`เขียน ${OUT.pathname}: now_playing ${nowPlaying.picked.length} + popular ${popular.picked.length} = ${movies.length} เรื่อง`);
