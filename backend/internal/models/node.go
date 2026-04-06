package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"wg-keeper-backend/internal/config"
	"wg-keeper-backend/internal/crypto"
)

type NodeStatus string

const (
	NodeStatusOnline  NodeStatus = "online"
	NodeStatusOffline NodeStatus = "offline"
)

type Node struct {
	ID        string     `gorm:"primaryKey;type:uuid" json:"id"`
	Name      string     `gorm:"type:varchar(255);not null" json:"name"`
	Address   string     `gorm:"type:varchar(512);not null;uniqueIndex" json:"address"`
	APIKey    string     `gorm:"type:text;not null" json:"-"`
	Status    NodeStatus `gorm:"type:varchar(16);not null" json:"status"`
	Version   *string    `gorm:"type:varchar(32)" json:"version,omitempty"`
	CreatedAt time.Time  `gorm:"autoCreateTime;not null" json:"createdAt"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime;not null" json:"updatedAt"`
}

func (Node) TableName() string { return "nodes" }

func (n *Node) BeforeCreate(_ *gorm.DB) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	key := config.Current().APIKeyEncryptionKey
	if len(key) == 0 {
		return nil
	}
	encrypted, err := crypto.Encrypt(n.APIKey, key)
	if err != nil {
		return err
	}
	n.APIKey = encrypted
	return nil
}

func (n *Node) AfterFind(_ *gorm.DB) error {
	key := config.Current().APIKeyEncryptionKey
	if len(key) == 0 {
		return nil
	}
	decrypted, err := crypto.Decrypt(n.APIKey, key)
	if err != nil {
		return err
	}
	n.APIKey = decrypted
	return nil
}
