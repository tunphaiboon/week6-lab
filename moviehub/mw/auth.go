package mw

import (
    "context"
    "encoding/json"
    "net/http"
    "strings"

    "moviehub/auth"
)

type Who struct {
    ID   int64
    Role string
}

type ctxKey struct{}

func WhoFrom(ctx context.Context) Who {
    w, _ := ctx.Value(ctxKey{}).(Who)
    return w
}

func deny(w http.ResponseWriter, status int, code, msg string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]any{
        "error": map[string]string{"code": code, "message": msg},
    })
}

// RequireLogin คือประตูตรวจตั๋ว ไม่มีตั๋วหรือตั๋วใช้ไม่ได้ ตอบ 401
func RequireLogin(iss *auth.Issuer, next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        header := r.Header.Get("Authorization")
        token, found := strings.CutPrefix(header, "Bearer ")
        if !found || token == "" {
            deny(w, http.StatusUnauthorized, "LOGIN_REQUIRED", "กรุณาเข้าสู่ระบบก่อน")
            return
        }
        id, role, err := iss.Parse(token)
        if err != nil {
            deny(w, http.StatusUnauthorized, "TOKEN_INVALID", "ตั๋วใช้ไม่ได้หรือหมดอายุ กรุณาเข้าสู่ระบบใหม่")
            return
        }
        ctx := context.WithValue(r.Context(), ctxKey{}, Who{ID: id, Role: role})
        next(w, r.WithContext(ctx))
    }
}