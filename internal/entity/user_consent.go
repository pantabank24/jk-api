package entity

import "time"

// UserConsent เป็นหลักฐานการให้ความยินยอมหนึ่งครั้ง — ไม่ใช่สถานะปัจจุบัน. สถานะ
// "ยอมรับแล้วหรือยัง" คำนวณจากการมีแถวของเวอร์ชันล่าสุดอยู่หรือไม่ (ดู
// auth usecase) เพื่อให้การบัมพ์เวอร์ชันข้อความบังคับให้ยอมรับใหม่ได้เอง
// โดยไม่ต้องไปไล่ล้างธงของใคร
type UserConsent struct {
	ID          uint   `json:"id"           gorm:"primaryKey"`
	UserID      uint   `json:"user_id"      gorm:"index;not null"`
	ConsentType string `json:"consent_type" gorm:"type:varchar(50);not null;default:'pdpa'"`
	Version     int    `json:"version"      gorm:"not null"`
	// Granted แยก "ให้" ออกจาก "ถอน" — ความยินยอมทางการตลาดถอนได้ และการถอนต้องเป็น
	// แถวใหม่ ไม่ใช่การลบแถวเดิม ไม่งั้นหลักฐานว่าเคยให้เมื่อไหร่จะหายไปพร้อมกัน
	Granted bool `json:"granted" gorm:"not null;default:true"`
	// ข้อความเต็ม ณ ตอนที่กดยอมรับ — เก็บไว้เพราะต้นฉบับใน system_configs แก้ได้
	TextSnapshot string    `json:"text_snapshot" gorm:"type:text;default:''"`
	IP           string    `json:"ip"            gorm:"type:varchar(100);default:''"`
	UserAgent    string    `json:"user_agent"    gorm:"type:text;default:''"`
	AcceptedAt   time.Time `json:"accepted_at"`
	CreatedAt    time.Time `json:"created_at"`
}

func (UserConsent) TableName() string { return "user_consents" }
