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
	ID                uint         `gorm:"primarykey;autoIncrement:true"`
	CreatedAt         time.Time    `gorm:"autoCreateTime"`
	Body              string       `gorm:"not null"`
	MessageLink       string       `gorm:"not null"`
	ChannelID         uint         `gorm:"not null"`
	Topics            []*Topic     `gorm:"many2many:news_topics;"`
	StopThemes        []*StopTheme `gorm:"many2many:news_stop_themes;"`
	Channel           Channel
	Duplicates        []News `gorm:"many2many:news_duplicates;joinForeignKey:news_id;joinReferences:DuplicateID"`
	WithoutTopics        bool
	TopicsCheckedAt      *time.Time
	StopThemesCheckedAt  *time.Time
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
