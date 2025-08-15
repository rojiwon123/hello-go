package models

import (
	"time"
)

// BlogPost는 블로그 포스트의 기본 정보를 담는 구조체입니다.
type BlogPost struct {
	Title       string    `json:"title"`
	URL         string    `json:"url"`
	Author      string    `json:"author"`
	PublishedAt time.Time `json:"published_at"`
	Summary     string    `json:"summary"`
	Source      string    `json:"source"`
	Category    string    `json:"category"`
	Image       string    `json:"image"`
}

// BlogSource는 블로그 소스 정보를 담는 구조체입니다.
type BlogSource struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// BlogCrawler는 블로그 크롤링을 위한 인터페이스입니다.
type BlogCrawler interface {
	Crawl() ([]BlogPost, error)
	GetSource() BlogSource
}

// BlogPostFilter는 블로그 포스트 필터링을 위한 인터페이스입니다.
type BlogPostFilter interface {
	Filter(posts []BlogPost) []BlogPost
}
