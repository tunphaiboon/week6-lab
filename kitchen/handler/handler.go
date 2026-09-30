package handler
import (
    "encoding/json"
    "errors"
    "log"
    "net/http"
    "strconv"
    "time"

    "kitchen/store"
)

type Handler struct {
    Store store.MenuStore
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

func (h *Handler) ListMenu(w http.ResponseWriter, r *http.Request) {
    menus, err := h.Store.List(r.Context(), r.URL.Query().Get("type"))
    if err != nil {
        writeError(w, http.StatusInternalServerError, "STORE_UNAVAILABLE", "อ่านคลังไม่สำเร็จ")
        return
    }
    writeJSON(w, http.StatusOK, menus)
}

func (h *Handler) GetMenu(w http.ResponseWriter, r *http.Request) {
    id, err := strconv.Atoi(r.PathValue("id"))
    if err != nil {
        writeError(w, http.StatusBadRequest, "BAD_ID", "เลขจานต้องเป็นตัวเลข")
        return
    }
    menu, err := h.Store.Get(r.Context(), id)
    if errors.Is(err, store.ErrNotFound) {
        writeError(w, http.StatusNotFound, "MENU_NOT_FOUND", "ไม่พบเมนูหมายเลขนี้")
        return
    }
    if err != nil {
        writeError(w, http.StatusInternalServerError, "STORE_UNAVAILABLE", "อ่านคลังไม่สำเร็จ")
        return
    }
    writeJSON(w, http.StatusOK, menu)
}

func (h *Handler) CreateMenu(w http.ResponseWriter, r *http.Request) {
    var m store.Menu
    if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
        writeError(w, http.StatusBadRequest, "BAD_JSON", "อ่านกล่อง JSON ไม่ออก")
        return
    }
    if m.Name == "" || m.Price <= 0 {
        writeError(w, http.StatusBadRequest, "MISSING_FIELD",
            "ต้องมีชื่อเมนู และราคาต้องมากกว่าศูนย์")
        return
    }
    created, err := h.Store.Add(r.Context(), m)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "STORE_UNAVAILABLE", "เพิ่มเมนูไม่สำเร็จ")
        return
    }
    writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) DeleteMenu(w http.ResponseWriter, r *http.Request) {
    id, err := strconv.Atoi(r.PathValue("id"))
    if err != nil {
        writeError(w, http.StatusBadRequest, "BAD_ID", "เลขจานต้องเป็นตัวเลข")
        return
    }
    err = h.Store.Delete(r.Context(), id)
    if errors.Is(err, store.ErrNotFound) {
        writeError(w, http.StatusNotFound, "MENU_NOT_FOUND", "ไม่พบเมนูหมายเลขนี้")
        return
    }
    if err != nil {
        writeError(w, http.StatusInternalServerError, "STORE_UNAVAILABLE", "ลบไม่สำเร็จ")
        return
    }
    w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Boom(w http.ResponseWriter, r *http.Request) {
    panic("หม้อระเบิด")
}

func (h *Handler) Slow(w http.ResponseWriter, r *http.Request) {
    time.Sleep(3 * time.Second)
    w.Write([]byte("จานที่ใช้เวลานานเสร็จแล้ว\n"))
}

func (h *Handler) SlowCtx(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()                     // สายจูงของใบสั่งใบนี้
    log.Println("เริ่มตุ๋นขาหมู ใช้เวลา 5 วินาที")
    select {
    case <-time.After(5 * time.Second):
        log.Println("ตุ๋นเสร็จ เสิร์ฟได้")
        w.Write([]byte("ขาหมูตุ๋นเสร็จแล้ว\n"))
    case <-ctx.Done():
        log.Println("ลูกค้าเดินออกจากร้านแล้ว เลิกตุ๋น เหตุผล", ctx.Err())
        return
    }
}

func (h *Handler) Routes() *http.ServeMux {
    mux := http.NewServeMux()
    mux.HandleFunc("GET /menus", h.ListMenu)
    mux.HandleFunc("GET /menus/{id}", h.GetMenu)
    mux.HandleFunc("POST /menus", h.CreateMenu)
    mux.HandleFunc("DELETE /menus/{id}", h.DeleteMenu)
    mux.HandleFunc("GET /boom", h.Boom)
    mux.HandleFunc("GET /slow", h.Slow)
    mux.HandleFunc("GET /slow-ctx", h.SlowCtx)
    return mux
}