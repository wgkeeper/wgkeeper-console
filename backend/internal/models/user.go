package models

import (
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	ID                 string    `gorm:"primaryKey;type:uuid" json:"id"`
	Username           string    `gorm:"type:varchar(32);uniqueIndex;not null" json:"username"`
	PasswordHash       string    `gorm:"type:varchar(72);not null" json:"-"`
	IsAdmin            bool      `gorm:"not null;default:false" json:"isAdmin"`
	MustChangePassword bool      `gorm:"not null;default:false" json:"mustChangePassword"`
	CreatedAt          time.Time `gorm:"autoCreateTime;not null" json:"createdAt"`
	UpdatedAt          time.Time `gorm:"autoUpdateTime;not null" json:"updatedAt"`
}

func (User) TableName() string { return "users" }

func (u *User) BeforeCreate(_ *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}

func EnsureDefaultAdmin(db *gorm.DB, username string, password string) error {
	if username == "" {
		username = "admin"
	}

	var count int64
	if err := db.Model(&User{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		if password != "" {
			slog.Info("BOOTSTRAP_ADMIN_PASSWORD is set but ignored — users already exist")
		}
		return nil
	}

	if password == "" {
		return errors.New("BOOTSTRAP_ADMIN_PASSWORD is required when no users exist")
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	admin := User{
		Username:           username,
		PasswordHash:       string(passwordHash),
		IsAdmin:            true,
		MustChangePassword: true,
	}

	return db.Create(&admin).Error
}
