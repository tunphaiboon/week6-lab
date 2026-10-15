package handler

import (
    "encoding/json"
    "errors"
    "net/http"
    "strings"
    "unicode/utf8"

    "moviehub/mw"
    "moviehub/store"
)

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
    who := mw.WhoFrom(r.Context())
    m, err := h.Store.MemberByID(r.Context(), who.ID)
    if errors.Is(err, store.ErrNotFound) {
        writeError(w, http.StatusUnauthorized, "LOGIN_REQUIRED", "กรุณาเข้าสู่ระบบก่อน")
        return
    }
    if err != nil {
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusOK, m)
}

type reviewReq struct {
    Text string `json:"text"`
}

func (h *Handler) AddReview(w http.ResponseWriter, r *http.Request) {
    movieID, ok := pathID(r, "id")
    if !ok {
        writeError(w, http.StatusBadRequest, "BAD_ID", "เลขหนังต้องเป็นตัวเลข")
        return
    }
    var req reviewReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "BAD_JSON", "อ่านกล่อง JSON ไม่ออก")
        return
    }
    text := strings.TrimSpace(req.Text)
    if utf8.RuneCountInString(text) < 10 {
        writeError(w, http.StatusBadRequest, "REVIEW_TOO_SHORT", "รีวิวสั้นเกินไป")
        return
    }
    who := mw.WhoFrom(r.Context()) // เลขสมาชิกมาจากตั๋วเท่านั้น
    rv, err := h.Store.AddReview(r.Context(), who.ID, movieID, text)
    switch {
    case errors.Is(err, store.ErrDuplicate):
        writeError(w, http.StatusConflict, "ALREADY_REVIEWED", "คุณรีวิวเรื่องนี้ไปแล้ว")
        return
    case errors.Is(err, store.ErrNoMovie):
        writeError(w, http.StatusNotFound, "MOVIE_NOT_FOUND", "ไม่พบหนังเรื่องนี้")
        return
    case err != nil:
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusCreated, map[string]any{"id": rv.ID, "text": rv.Text, "createdAt": rv.CreatedAt})
}