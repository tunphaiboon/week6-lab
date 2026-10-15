package handler

import (
    "encoding/json"
    "log"
    "net/http"
    "strconv"
    "moviehub/auth"
    "moviehub/store"
    "errors"
)

type Handler struct {
    Store   *store.Store
    Tickets *auth.Issuer
}


func writeJSON(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
    writeJSON(w, status, map[string]any{
        "error": map[string]string{"code": code, "message": message},
    })
}

// internal จดรายละเอียดไว้ใน log ฝั่งเรา แต่บอกผู้ใช้แค่ว่าระบบขัดข้อง
func internal(w http.ResponseWriter, err error) {
    log.Printf("internal error: %v", err)
    writeError(w, http.StatusInternalServerError, "INTERNAL", "ระบบขัดข้อง ลองใหม่อีกครั้ง")
}

func pathID(r *http.Request, name string) (int64, bool) {
    id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
    return id, err == nil && id > 0
}

func (h *Handler) SearchMovies(w http.ResponseWriter, r *http.Request) {
    items, err := h.Store.SearchMovies(r.Context(), r.URL.Query().Get("q"))
    if err != nil {
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) ListReviews(w http.ResponseWriter, r *http.Request) {
    movieID, ok := pathID(r, "id")
    if !ok {
        writeError(w, http.StatusBadRequest, "BAD_ID", "เลขหนังต้องเป็นตัวเลข")
        return
    }
    items, err := h.Store.ListReviews(r.Context(), movieID)
    if err != nil {
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (h *Handler) GetMovie(w http.ResponseWriter, r *http.Request) {
    id, ok := pathID(r, "id")
    if !ok {
        writeError(w, http.StatusBadRequest, "BAD_ID", "เลขหนังต้องเป็นตัวเลข")
        return
    }
    m, err := h.Store.GetMovie(r.Context(), id)
    if errors.Is(err, store.ErrNotFound) {
        writeError(w, http.StatusNotFound, "MOVIE_NOT_FOUND", "ไม่พบหนังเรื่องนี้")
        return
    }
    if err != nil {
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusOK, m)
}