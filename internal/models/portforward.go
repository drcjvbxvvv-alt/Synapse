package models

import (
	"time"

	"gorm.io/gorm"
)

// PortForwardSession Port-Forward 會話記錄
type PortForwardSession struct {
	gorm.Model
	ClusterID     uint      `gorm:"not null;index"`
	ClusterName   string
	Namespace     string    `gorm:"not null"`
	PodName       string    `gorm:"not null"`
	PodPort       int       `gorm:"not null"` // Pod 側的埠
	LocalPort     int       `gorm:"not null"` // 後端伺服器監聽埠
	UserID        uint      `gorm:"not null"`
	Username      string
	Status        string    `gorm:"default:'active'"` // active / stopped
	StoppedAt     *time.Time
}
