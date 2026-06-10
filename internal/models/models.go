package models

import "time"

type User struct {
	ID         uint         `gorm:"primarykey; autoIncrement:true"`
	Username   string       `gorm:"unique;not null"`
	Password   string       `gorm:"not null" json:"-"`
	Channels   []*Channel   `gorm:"many2many:user_channels;"`
	Topics     []*Topic     `gorm:"many2many:user_topics;"`
	StopThemes []*StopTheme `gorm:"many2many:user_stop_themes;"`
}

type Channel struct {
	ID    uint    `gorm:"primarykey;autoIncrement:true"`
	Link  string  `gorm:"not null"`
	Name  string  `gorm:"not null"`
	Users []*User `gorm:"many2many:user_channels;"`
	News  []News  `gorm:"foreignKey:ChannelID"`
}

type News struct {
	ID                uint         `gorm:"primarykey;autoIncrement:true" json:"id"`
	CreatedAt         time.Time    `gorm:"autoCreateTime" json:"created_at"`
	Body              string       `gorm:"not null" json:"body"`
	MessageLink       string       `gorm:"not null" json:"message_link"`
	ChannelID         uint         `gorm:"not null" json:"channel_id"`
	Topics            []*Topic     `gorm:"many2many:news_topics;" json:"topics"`
	StopThemes        []*StopTheme `gorm:"many2many:news_stop_themes;" json:"stop_themes"`
	Channel           Channel      `json:"channel"`
	Duplicates        []News       `gorm:"many2many:news_duplicates;joinForeignKey:news_id;joinReferences:DuplicateID" json:"duplicates"`
	WithoutTopics     bool         `json:"-"`
	TopicsCheckedAt   *time.Time   `json:"-"`
	StopThemesCheckedAt *time.Time `json:"-"`
	DeduplicationCheckedAt *time.Time `json:"-"`
}

type Topic struct {
	ID   uint    `gorm:"primarykey;autoIncrement:true"`
	Name string  `gorm:"not null"`
	News []*News `gorm:"many2many:news_topics;"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

type StopTheme struct {
	ID   uint    `gorm:"primarykey;autoIncrement:true"`
	Name string  `gorm:"not null"`
	News []*News `gorm:"many2many:news_stop_themes;"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

type UserReadNews struct {
	ID     uint      `gorm:"primarykey;autoIncrement:true"`
	UserID uint      `gorm:"not null;index"`
	NewsID uint      `gorm:"not null;index"`
	ReadAt time.Time `gorm:"autoCreateTime"`
}

type UserSavedNews struct {
	ID      uint      `gorm:"primarykey;autoIncrement:true"`
	UserID  uint      `gorm:"not null;index"`
	NewsID  uint      `gorm:"not null;index"`
	SavedAt time.Time `gorm:"autoCreateTime"`
}
