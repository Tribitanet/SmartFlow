package source

import "sm_bot/models"

type RssSource struct {
	Link       string
	SourceId   int64
	SourceName string
}

func NewRssSource(x models.Source) RssSource {
	return RssSource{
		Link:       x.FeedLink,
		SourceId:   x.Id,
		SourceName: x.Name,
	}
}
