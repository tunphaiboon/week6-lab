package handler

import (
    "encoding/json"
    "errors"
    "net/http"
    "strings"
    "unicode/utf8"

    "moviehub/auth"
    "moviehub/store"
)

type registerReq struct {
    Email       string `json:"email"`
    Password    string `json:"password"`
    DisplayName string `json:"displayName"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
    var req registerReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "BAD_JSON", "อ่านกล่อง JSON ไม่ออก")
        return
    }
    email := strings.ToLower(strings.TrimSpace(req.Email))
    name := strings.TrimSpace(req.DisplayName)
    switch {
    case !strings.Contains(email, "@") || len(email) > 255:
        writeError(w, http.StatusBadRequest, "BAD_EMAIL", "รูปแบบอีเมลไม่ถูกต้อง")
        return
    case utf8.RuneCountInString(req.Password) < 8 || len(req.Password) > 72:
        writeError(w, http.StatusBadRequest, "BAD_PASSWORD", "รหัสผ่านต้องยาว 8 ตัวอักษรขึ้นไป และไม่เกิน 72 ไบต์")
        return
    case name == "" || utf8.RuneCountInString(name) > 50:
        writeError(w, http.StatusBadRequest, "BAD_NAME", "ต้องมีชื่อที่แสดง และยาวไม่เกิน 50 ตัวอักษร")
        return
    }
    hash, err := auth.HashPassword(req.Password)
    if err != nil {
        internal(w, err)
        return
    }
    m, err := h.Store.CreateMember(r.Context(), email, hash, name)
    if errors.Is(err, store.ErrEmailTaken) {
        writeError(w, http.StatusConflict, "EMAIL_TAKEN", "อีเมลนี้มีบัญชีอยู่แล้ว")
        return
    }
    if err != nil {
        internal(w, err)
        return
    }
    writeJSON(w, http.StatusCreated, m)
}