package crawlers

import (
	"log"
	"sync"
	"time"

	"hello-go/internal/filters"
	"hello-go/internal/models"
)

// CrawlerManager는 여러 블로그 크롤러를 병렬로 실행하는 매니저입니다.
type CrawlerManager struct {
	crawlers []models.BlogCrawler
	filter   models.BlogPostFilter
}

// NewCrawlerManager는 새로운 CrawlerManager 인스턴스를 생성합니다.
func NewCrawlerManager() *CrawlerManager {
	return &CrawlerManager{
		crawlers: []models.BlogCrawler{
			NewTossCrawler(),
			// NewDaangnCrawler(),
			// NewNaverCrawler(),
			// NewKakaoCrawler(),
		},
		filter: filters.NewTechFilter(),
	}
}

// CrawlAll은 모든 블로그를 병렬로 크롤링합니다.
func (m *CrawlerManager) CrawlAll() ([]models.BlogPost, error) {
	var allPosts []models.BlogPost
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 각 크롤러를 고루틴으로 실행
	for _, crawler := range m.crawlers {
		wg.Add(1)
		go func(c models.BlogCrawler) {
			defer wg.Done()

			log.Printf("크롤링 시작: %s", c.GetSource().Name)
			start := time.Now()

			posts, err := c.Crawl()
			if err != nil {
				log.Printf("크롤링 실패 (%s): %v", c.GetSource().Name, err)
				return
			}

			// 필터링 적용
			filteredPosts := m.filter.Filter(posts)

			// 스레드 안전하게 결과 추가
			mu.Lock()
			allPosts = append(allPosts, filteredPosts...)
			mu.Unlock()

			duration := time.Since(start)
			log.Printf("크롤링 완료 (%s): %d개 포스트, %v", c.GetSource().Name, len(filteredPosts), duration)
		}(crawler)
	}

	// 모든 고루틴 완료 대기
	wg.Wait()

	log.Printf("전체 크롤링 완료: 총 %d개 포스트", len(allPosts))
	return allPosts, nil
}

// GetCrawlerStatus는 각 크롤러의 상태를 확인합니다.
func (m *CrawlerManager) GetCrawlerStatus() []map[string]interface{} {
	var status []map[string]interface{}

	for _, crawler := range m.crawlers {
		source := crawler.GetSource()
		status = append(status, map[string]interface{}{
			"name":   source.Name,
			"url":    source.URL,
			"status": "준비됨",
		})
	}

	return status
}
