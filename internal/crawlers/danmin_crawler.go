package crawlers

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"hello-go/internal/models"
)

// DanminCrawler는 개발자 단민 블로그를 크롤링합니다.
type DanminCrawler struct {
	client *http.Client
}

func NewDanminCrawler() *DanminCrawler {
	return &DanminCrawler{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *DanminCrawler) GetSource() models.BlogSource {
	return models.BlogSource{
		Name: "단민",
		URL:  "https://www.jeong-min.com",
	}
}

// Crawl은 개발자 단민 블로그를 크롤링합니다.
func (c *DanminCrawler) Crawl() ([]models.BlogPost, error) {
	var allPosts []models.BlogPost

	log.Printf("개발자 단민 블로그 크롤링 시작")

	// 개발자 단민 블로그 메인 페이지 크롤링
	posts, err := c.crawlMainPage("https://www.jeong-min.com/posts")
	if err != nil {
		log.Printf("메인 페이지 크롤링 실패: %v", err)
		return nil, err
	}

	allPosts = append(allPosts, posts...)

	// 중복 제거 (URL 기준)
	seen := make(map[string]bool)
	var uniquePosts []models.BlogPost
	for _, post := range allPosts {
		if !seen[post.URL] {
			seen[post.URL] = true
			uniquePosts = append(uniquePosts, post)
		}
	}

	// 최신순으로 정렬
	sort.Slice(uniquePosts, func(i, j int) bool {
		return uniquePosts[i].PublishedAt.After(uniquePosts[j].PublishedAt)
	})

	log.Printf("개발자 단민 블로그 크롤링 완료: 총 %d개 포스트 발견", len(uniquePosts))
	return uniquePosts, nil
}

// crawlMainPage는 메인 페이지에서 포스트를 크롤링합니다.
func (c *DanminCrawler) crawlMainPage(url string) ([]models.BlogPost, error) {
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("페이지 요청 실패: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("페이지 응답 오류: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("HTML 파싱 실패: %w", err)
	}

	var posts []models.BlogPost

	// 포스트 링크 찾기 (실제 HTML 구조에 맞춤)
	doc.Find("a[href*='/']").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		// 숫자로 시작하는 포스트 링크만 처리 (예: /83-parcel-rsc/)
		if !regexp.MustCompile(`^/\d+`).MatchString(href) {
			return
		}

		// 절대 URL로 변환
		if strings.HasPrefix(href, "/") {
			href = "https://www.jeong-min.com" + href
		}

		// 이미 수집된 포스트인지 확인
		for _, existingPost := range posts {
			if existingPost.URL == href {
				return
			}
		}

		// 제목 찾기 (실제 HTML 구조에 맞춤)
		titleElement := s.Find("div.title")
		if titleElement.Length() == 0 {
			return
		}

		title := strings.TrimSpace(titleElement.Text())
		if title == "" {
			return
		}

		// 카테고리 이름이나 메뉴 항목 제외
		categoryNames := []string{"Dev", "Experience", "회고", "인턴회고", "All"}
		for _, category := range categoryNames {
			if strings.EqualFold(title, category) {
				return
			}
		}

		// 포스트 상세 정보 가져오기
		post, err := c.crawlPostDetail(href, title)
		if err != nil {
			log.Printf("포스트 상세 정보 가져오기 실패 (%s): %v", href, err)
			// 기본 정보로 포스트 생성
			post = models.BlogPost{
				Title:       title,
				URL:         href,
				Author:      "단민",
				PublishedAt: time.Now(),
				Summary:     "개발자 단민의 기술 블로그 포스트",
				Source:      "단민",
				Category:    "개발",
				Image:       "",
			}
		}

		posts = append(posts, post)
		log.Printf("단민 포스트 발견: %s (카테고리: %s)", post.Title, post.Category)
	})

	return posts, nil
}

// crawlPostDetail은 포스트 상세 페이지에서 정보를 가져옵니다.
func (c *DanminCrawler) crawlPostDetail(url, title string) (models.BlogPost, error) {
	resp, err := c.client.Get(url)
	if err != nil {
		return models.BlogPost{}, fmt.Errorf("상세 페이지 요청 실패: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models.BlogPost{}, fmt.Errorf("상세 페이지 응답 오류: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return models.BlogPost{}, fmt.Errorf("상세 페이지 HTML 파싱 실패: %w", err)
	}

	post := models.BlogPost{
		Title:  title,
		URL:    url,
		Author: "단민",
		Source: "단민",
	}

	// 발행일 찾기 (실제 HTML 구조에 맞춤)
	doc.Find("div.css-dror6n").Each(func(i int, s *goquery.Selection) {
		dateText := strings.TrimSpace(s.Text())
		log.Printf("발견된 날짜 텍스트: %s", dateText)
		// YYYY.MM.DD 형식인지 확인
		if matched, _ := regexp.MatchString(`^\d{4}\.\d{2}\.\d{2}$`, dateText); matched {
			// YYYY.MM.DD 형식을 YYYY-MM-DD로 변환
			dateStr := strings.ReplaceAll(dateText, ".", "-")
			if t, err := time.Parse("2006-01-02", dateStr); err == nil {
				post.PublishedAt = t
				log.Printf("날짜 파싱 성공: %s -> %v", dateText, t)
			} else {
				log.Printf("날짜 파싱 실패: %s, 에러: %v", dateText, err)
			}
		}
	})

	// 발행일을 찾지 못한 경우 현재 시간으로 설정
	if post.PublishedAt.IsZero() {
		post.PublishedAt = time.Now()
	}

	// 요약 찾기
	doc.Find("p").Each(func(i int, s *goquery.Selection) {
		if post.Summary == "" {
			text := strings.TrimSpace(s.Text())
			if len(text) > 50 && len(text) < 200 {
				post.Summary = text
			}
		}
	})

	// 요약을 찾지 못한 경우 기본값 설정
	if post.Summary == "" {
		post.Summary = "개발자 단민의 기술 블로그 포스트"
	}

	// 썸네일 이미지 찾기
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		if post.Image == "" {
			if src, exists := s.Attr("src"); exists {
				if strings.HasPrefix(src, "http") {
					post.Image = src
				} else if strings.HasPrefix(src, "/") {
					post.Image = "https://www.jeong-min.com" + src
				}
			}
		}
	})

	// 카테고리 자동 분류
	post.Category = c.determineCategory(post.Title, post.Summary, post.URL)

	return post, nil
}

// determineCategory는 제목과 요약을 기반으로 카테고리를 결정합니다.
func (c *DanminCrawler) determineCategory(title, summary, url string) string {
	text := strings.ToLower(title + " " + summary)

	if strings.Contains(text, "react") || strings.Contains(text, "next.js") || strings.Contains(text, "frontend") || strings.Contains(text, "ui") || strings.Contains(text, "component") {
		return "프론트엔드"
	}
	if strings.Contains(text, "인턴") || strings.Contains(text, "회고") || strings.Contains(text, "경험") {
		return "경험"
	}
	if strings.Contains(text, "개발") || strings.Contains(text, "coding") || strings.Contains(text, "프로그래밍") {
		return "개발"
	}
	if strings.Contains(text, "ci") || strings.Contains(text, "cd") || strings.Contains(text, "deploy") || strings.Contains(text, "배포") {
		return "DevOps"
	}

	return "기타"
}