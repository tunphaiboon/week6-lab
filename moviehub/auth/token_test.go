package auth

import (
    "encoding/base64"
    "strings"
    "testing"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

var secret = []byte("กุญแจลับสำหรับทดสอบเท่านั้น-ยาวพอแน่นอน")

// sign เซ็นเนื้อความที่กำหนดเอง ด้วยวิธีเซ็นและกุญแจที่กำหนดเอง
func sign(t *testing.T, method jwt.SigningMethod, key any, claims jwt.MapClaims) string {
    s, err := jwt.NewWithClaims(method, claims).SignedString(key)
    if err != nil {
        t.Fatal(err)
    }
    return s
}

func TestForgedTickets(t *testing.T) {
    iss := &Issuer{Secret: secret, TTL: time.Hour}
    genuine, _ := iss.Issue(1, "member")
    now := time.Now()
    good := jwt.MapClaims{"sub": "1", "role": "admin", "exp": now.Add(time.Hour).Unix()}

    // 1) แก้ member เป็น admin ในเนื้อความ แต่คงลายเซ็นเดิม
    p := strings.Split(genuine, ".")
    body, _ := base64.RawURLEncoding.DecodeString(p[1])
    body = []byte(strings.Replace(string(body), "member", "admin", 1))
    edited := p[0] + "." + base64.RawURLEncoding.EncodeToString(body) + "." + p[2]

    cases := []struct {
        name  string
        token string
    }{
        {"แก้เนื้อความ", edited},
        {"เซ็นด้วยกุญแจอื่น", sign(t, jwt.SigningMethodHS256, []byte("กุญแจของคนอื่น-ยาวพอเหมือนกันนะ"), good)},
        {"อ้างว่าไม่ต้องเซ็น", sign(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, good)},
        {"กุญแจถูกแต่วิธีเซ็นอื่น", sign(t, jwt.SigningMethodHS512, secret, good)},
        {"ไม่มีวันหมดอายุ", sign(t, jwt.SigningMethodHS256, secret, jwt.MapClaims{"sub": "1", "role": "admin"})},
        {"หมดอายุแล้ว", sign(t, jwt.SigningMethodHS256, secret, jwt.MapClaims{"sub": "1", "role": "member", "exp": now.Add(-time.Minute).Unix()})},
    }

    if _, role, err := iss.Parse(genuine); err != nil || role != "member" {
        t.Fatalf("ตั๋วจริงต้องผ่าน แต่ได้ %v", err)
    }
    t.Log("ตั๋วจริง ผ่าน")
    for _, c := range cases {
        _, _, err := iss.Parse(c.token)
        if err == nil {
            t.Errorf("%s ผ่านเข้ามาได้ ทั้งที่ต้องถูกปฏิเสธ", c.name)
            continue
        }
        t.Logf("%s ถูกปฏิเสธ %v", c.name, err)
    }
}