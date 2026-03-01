package models

import "time"

type News struct {
	Categories []string
	Link       string
	Data       time.Time
	SourceName string
}

type Source struct {
	Id        int64
	Name      string
	FeedLink  string
	CreatedAt time.Time
}

type Articlestruct struct {
	Id          int64
	SourceId    int64
	Link        string
	PublishedAt time.Time
	PostedAt    time.Time
	CreatedAt   time.Time
}
