package models

type User struct {
	ID         uint         `gorm:"primarykey; autoIncrement:true"`
	Username   string       `gorm:"unique;not null"`
	Password   string       `gorm:"not null"`
	Email      string       `gorm:"unique"`
	Channels   []*Channel   `gorm:"many2many:user_channels;"`
	Topics     []*Topic     `gorm:"many2many:user_topics;"`
	StopThemes []*StopTheme `gorm:"many2many:user_stop_themes;"`
}

type Channel struct {
	ID    uint    `gorm:"primarykey"`
	Link  string  `gorm:"not null"`
	Name  string  `gorm:"not null"`
	Users []*User `gorm:"many2many:user_channels;"`
	News  []News  `gorm:"foreignKey:ChannelID"`
}

type News struct {
	ID          uint         `gorm:"primarykey"`
	Body        string       `gorm:"not null"`
	MessageLink string       `gorm:"not null"`
	ChannelID   uint         `gorm:"not null"`
	Topics      []*Topic     `gorm:"many2many:news_topics;"`
	StopThemes  []*StopTheme `gorm:"many2many:news_stop_themes;"`
	Channel     Channel
}

type Topic struct {
	ID   uint    `gorm:"primarykey"`
	Name string  `gorm:"not null"`
	News []*News `gorm:"many2many:news_topics;"`
}

type StopTheme struct {
	ID   uint    `gorm:"primarykey"`
	Name string  `gorm:"not null"`
	News []*News `gorm:"many2many:news_stop_themes;"`
}
