package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type NodeStatus string

const (
	NodeStatusOnline  NodeStatus = "online"
	NodeStatusOffline NodeStatus = "offline"
)

type Node struct {
	ID        string       `gorm:"primaryKey;type:uuid" json:"id"`
	Name      string       `gorm:"type:varchar(255);not null" json:"name"`
	Address   string       `gorm:"type:varchar(512);not null;uniqueIndex" json:"address"`
	APIKey    SecretString `gorm:"type:text;not null" json:"-"`
	Status    NodeStatus   `gorm:"type:varchar(16);not null" json:"status"`
	Version   *string      `gorm:"type:varchar(32)" json:"version,omitempty"`
	CreatedAt time.Time    `gorm:"autoCreateTime;not null" json:"createdAt"`
	UpdatedAt time.Time    `gorm:"autoUpdateTime;not null" json:"updatedAt"`
}

func (Node) TableName() string { return "nodes" }

func (n *Node) BeforeCreate(_ *gorm.DB) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	return nil
}
