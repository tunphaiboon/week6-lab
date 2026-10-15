package main

import (
    "fmt"
    "strings"
    "time"

    "golang.org/x/crypto/bcrypt"

    "moviehub/auth"
)

func main() {
    // 1) รหัสเดียวกัน บดสองครั้ง
    a, _ := auth.HashPassword("popcorn2569")
    b, _ := auth.HashPassword("popcorn2569")
    fmt.Println(a)
    fmt.Println(b)

    // 2) ตรวจรหัสถูกกับรหัสผิด
    fmt.Println("popcorn2569 ตรงกับ a:", auth.CheckPassword(a, "popcorn2569"))
    fmt.Println("popcorn2569 ตรงกับ b:", auth.CheckPassword(b, "popcorn2569"))
    fmt.Println("popcorn2568 ตรงกับ a:", auth.CheckPassword(a, "popcorn2568"))

    // 3) หมุนปุ่มความช้า
    for _, cost := range []int{8, 10, 12} {
        start := time.Now()
        bcrypt.GenerateFromPassword([]byte("popcorn2569"), cost)
        fmt.Printf("cost %d ใช้เวลา %v\n", cost, time.Since(start).Round(time.Millisecond))
    }

    // 4) เพดาน 72 ไบต์
    _, err := auth.HashPassword(strings.Repeat("a", 73))
    fmt.Println("73 ไบต์:", err)
}