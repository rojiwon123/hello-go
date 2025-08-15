package crawlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"hello-go/internal/models"

	"io"
	"regexp"

	"github.com/PuerkitoBio/goquery"
)

// min 함수는 두 정수 중 작은 값을 반환합니다.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// NaverCrawler는 네이버 D2 기술 블로그를 크롤링합니다.
type NaverCrawler struct {
	client *http.Client
}

func NewNaverCrawler() *NaverCrawler {
	return &NaverCrawler{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *NaverCrawler) GetSource() models.BlogSource {
	return models.BlogSource{
		Name: "네이버 D2",
		URL:  "https://d2.naver.com/home",
	}
}

// Crawl은 네이버 D2 기술 블로그를 크롤링합니다.
func (c *NaverCrawler) Crawl() ([]models.BlogPost, error) {
	var allPosts []models.BlogPost

	log.Printf("네이버 D2 기술 블로그 크롤링 시작")

	// RSS 피드 크롤링 (여러 소스 시도)
	rssURLs := []string{
		"https://d2.naver.com/d2.atom",
		"https://d2.naver.com/d2.atom?limit=50", // 더 많은 포스트 요청
		"https://d2.naver.com/d2.atom?count=50", // 다른 파라미터 시도
		"https://d2.naver.com/d2.atom?max=50",   // 다른 파라미터 시도
		"https://d2.naver.com/d2.atom?size=50",  // 다른 파라미터 시도
	}

	for _, rssURL := range rssURLs {
		log.Printf("RSS 피드 크롤링 중: %s", rssURL)

		rssPosts, err := c.crawlRSS(rssURL)
		if err != nil {
			log.Printf("RSS 크롤링 실패 (%s): %v", rssURL, err)
			continue
		}

		allPosts = append(allPosts, rssPosts...)
		log.Printf("RSS에서 %d개 포스트 발견", len(rssPosts))

		// 첫 번째 RSS 피드에서 포스트를 찾았으면 중단 (중복 방지)
		if len(rssPosts) > 0 {
			break
		}
	}

	// 추가로 각 포스트의 상세 페이지에서 관련 포스트나 더 많은 정보를 가져오기 시도
	log.Printf("포스트 상세 페이지에서 추가 정보 수집 시도...")
	var additionalPosts []models.BlogPost

	// 처음 몇 개 포스트의 상세 페이지를 확인
	for i, post := range allPosts {
		if i >= 5 { // 처음 5개 포스트만 확인 (성능 고려)
			break
		}

		log.Printf("포스트 상세 페이지 확인 중: %s", post.Title)
		detailPosts, err := c.crawlPostDetail(post.URL)
		if err != nil {
			log.Printf("상세 페이지 크롤링 실패: %v", err)
			continue
		}

		additionalPosts = append(additionalPosts, detailPosts...)
	}

	allPosts = append(allPosts, additionalPosts...)
	log.Printf("상세 페이지에서 %d개 추가 포스트 발견", len(additionalPosts))

	// 중복 제거
	uniquePosts := c.removeDuplicates(allPosts)

	// 최신순으로 정렬
	c.sortByDate(uniquePosts)

	log.Printf("네이버 D2 기술 블로그 크롤링 완료: 총 %d개 포스트 발견", len(uniquePosts))
	return uniquePosts, nil
}

// crawlSinglePage는 특정 URL을 크롤링합니다.
func (c *NaverCrawler) crawlSinglePage(url string) ([]models.BlogPost, error) {
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

	// 네이버 D2 특화 선택자들 추가
	selectors := []string{
		"article",
		".post-item",
		".article-item",
		".blog-post",
		".content-item",
		".post",
		".article",
		"li",
		".item",
		"[class*='post']",
		"[class*='article']",
		"[class*='blog']",
		".news-item",
		".content",
		".contents a",             // 네이버 D2 특화
		".con_area a",             // 네이버 D2 특화
		"#container a",            // 네이버 D2 특화
		"a[href*='/helloworld/']", // 네이버 D2 포스트 링크
		"a[href*='/news/']",       // 네이버 D2 뉴스 링크
		".contents",               // 네이버 D2 콘텐츠 영역
		".con_area",               // 네이버 D2 콘텐츠 영역
	}

	log.Printf("페이지 %s에서 포스트 검색 중...", url)

	for _, selector := range selectors {
		count := doc.Find(selector).Length()
		log.Printf("선택자 '%s'에서 %d개 요소 발견", selector, count)

		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			// 링크가 있는 요소만 처리
			if s.Find("a").Length() > 0 || strings.Contains(selector, "a[") {
				post, err := c.extractPost(s)
				if err != nil {
					return
				}

				if post.Title != "" && post.URL != "" {
					// 중복 체크
					isDuplicate := false
					for _, existingPost := range posts {
						if existingPost.URL == post.URL {
							isDuplicate = true
							break
						}
					}

					if !isDuplicate {
						posts = append(posts, post)
						log.Printf("네이버 D2 포스트 발견: %s (카테고리: %s)", post.Title, post.Category)
					}
				}
			}
		})
	}

	// 페이지 내용 로깅 (디버깅용)
	bodyText := doc.Text()
	log.Printf("페이지 텍스트 길이: %d", len(bodyText))
	if len(bodyText) < 1000 {
		log.Printf("페이지 내용: %s", bodyText)
	}

	// 포스트가 없으면 더 이상 페이지가 없는 것으로 간주
	if len(posts) == 0 {
		return nil, fmt.Errorf("페이지에 포스트가 없습니다.")
	}

	return posts, nil
}

// crawlRSS는 RSS 피드를 크롤링합니다.
func (c *NaverCrawler) crawlRSS(rssURL string) ([]models.BlogPost, error) {
	resp, err := c.client.Get(rssURL)
	if err != nil {
		return nil, fmt.Errorf("RSS 요청 실패: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("RSS 응답 읽기 실패: %v", err)
	}

	// RSS 피드 내용을 멀티라인으로 매칭
	entryPattern := regexp.MustCompile(`(?s)<entry>(.*?)</entry>`)
	entries := entryPattern.FindAllStringSubmatch(string(body), -1)

	var posts []models.BlogPost
	log.Printf("RSS 피드에서 %d개 엔트리 발견", len(entries))

	for i, entry := range entries {
		entryContent := entry[1]

		// 제목 추출
		titleMatch := regexp.MustCompile(`(?s)<title[^>]*>(.*?)</title>`)
		titleMatches := titleMatch.FindStringSubmatch(entryContent)
		title := ""
		if len(titleMatches) > 1 {
			title = strings.TrimSpace(titleMatches[1])
		}

		// 링크 추출
		linkMatch := regexp.MustCompile(`(?s)<link[^>]*href="([^"]*)"[^>]*/>`)
		linkMatches := linkMatch.FindStringSubmatch(entryContent)
		url := ""
		if len(linkMatches) > 1 {
			url = strings.TrimSpace(linkMatches[1])
		}

		// 카테고리 추출
		categoryMatch := regexp.MustCompile(`(?s)<category[^>]*term="([^"]*)"[^>]*/>`)
		categoryMatches := categoryMatch.FindStringSubmatch(entryContent)
		category := ""
		if len(categoryMatches) > 1 {
			category = strings.TrimSpace(categoryMatches[1])
		}

		// ID 추출
		idMatch := regexp.MustCompile(`(?s)<id[^>]*>(.*?)</id>`)
		idMatches := idMatch.FindStringSubmatch(entryContent)
		id := ""
		if len(idMatches) > 1 {
			id = strings.TrimSpace(idMatches[1])
		}

		// 날짜 추출 (멀티라인 매칭)
		// published 시간을 우선적으로 사용
		publishedMatch := regexp.MustCompile(`(?s)<published[^>]*>(.*?)</published>`)
		publishedMatches := publishedMatch.FindStringSubmatch(entryContent)
		publishedAt := time.Now()

		if len(publishedMatches) > 1 {
			dateStr := strings.TrimSpace(publishedMatches[1])
			log.Printf("RSS published 날짜 발견: %s", dateStr)
			if t, err := c.parseAtomTime(dateStr); err == nil {
				publishedAt = t
				log.Printf("RSS published 날짜 파싱 성공: %v", publishedAt)
			} else {
				log.Printf("RSS published 날짜 파싱 실패: %v", err)
			}
		} else {
			// published가 없으면 updated 시간 사용
			dateMatch := regexp.MustCompile(`(?s)<updated[^>]*>(.*?)</updated>`)
			dateMatches := dateMatch.FindStringSubmatch(entryContent)
			if len(dateMatches) > 1 {
				dateStr := strings.TrimSpace(dateMatches[1])
				log.Printf("RSS updated 날짜 발견: %s", dateStr)
				if t, err := c.parseAtomTime(dateStr); err == nil {
					publishedAt = t
					log.Printf("RSS updated 날짜 파싱 성공: %v", publishedAt)
				} else {
					log.Printf("RSS updated 날짜 파싱 실패: %v", err)
				}
			}
		}

		// 내용 추출
		contentMatch := regexp.MustCompile(`(?s)<content[^>]*type="html"[^>]*>(.*?)</content>`)
		contentMatches := contentMatch.FindStringSubmatch(entryContent)
		content := ""
		if len(contentMatches) > 1 {
			content = strings.TrimSpace(contentMatches[1])
		}

		// 요약 추출 (HTML 태그 제거)
		summary := c.stripHTMLTags(content)
		if len(summary) > 200 {
			summary = summary[:200] + "..."
		}

		// 썸네일 이미지 추출
		imageURL := c.extractThumbnail(content)

		// 카테고리 결정
		determinedCategory := c.determineCategory(title, summary, url)
		if determinedCategory != "" {
			category = determinedCategory
		}

		// 첫 번째 포스트에 대해서만 상세한 JSON 출력
		if i == 0 {
			rssData := map[string]interface{}{
				"title":               title,
				"url":                 url,
				"category":            category,
				"id":                  id,
				"publishedAt":         publishedAt,
				"content_length":      len(content),
				"summary_length":      len(summary),
				"imageURL":            imageURL,
				"determinedCategory":  determinedCategory,
				"raw_content_preview": content[:min(len(content), 200)] + "...",
				"raw_entry_content":   entryContent,
			}

			jsonData, _ := json.MarshalIndent(rssData, "", "  ")
			log.Printf("첫 번째 RSS 엔트리 상세 정보:\n%s", string(jsonData))
		}

		post := models.BlogPost{
			Title:       title,
			URL:         url,
			Author:      "네이버 D2",
			PublishedAt: publishedAt,
			Summary:     summary,
			Source:      "네이버 D2",
			Category:    category,
			Image:       imageURL,
		}

		posts = append(posts, post)
	}

	return posts, nil
}

// parseAtomTime은 Atom 날짜 형식을 파싱합니다.
func (c *NaverCrawler) parseAtomTime(dateStr string) (time.Time, error) {
	// Atom 표준 날짜 형식 (ISO 8601)
	formats := []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05.000Z",
		time.RFC3339,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Now(), fmt.Errorf("Atom 날짜 형식을 파싱할 수 없습니다: %s", dateStr)
}

// stripHTMLTags는 HTML 태그와 엔티티를 제거합니다.
func (c *NaverCrawler) stripHTMLTags(html string) string {
	// HTML 엔티티 디코딩
	html = strings.ReplaceAll(html, "&lt;", "<")
	html = strings.ReplaceAll(html, "&gt;", ">")
	html = strings.ReplaceAll(html, "&amp;", "&")
	html = strings.ReplaceAll(html, "&nbsp;", " ")
	html = strings.ReplaceAll(html, "&quot;", "\"")
	html = strings.ReplaceAll(html, "&#39;", "'")

	// HTML 태그 제거
	tagPattern := regexp.MustCompile(`<[^>]*>`)
	html = tagPattern.ReplaceAllString(html, "")

	// 연속된 공백 정리
	spacePattern := regexp.MustCompile(`\s+`)
	html = spacePattern.ReplaceAllString(html, " ")

	return strings.TrimSpace(html)
}

// extractThumbnail은 HTML 콘텐츠에서 썸네일 이미지 URL을 추출합니다.
func (c *NaverCrawler) extractThumbnail(content string) string {
	// 이미지 URL 추출 (HTML 엔티티 처리)
	imgMatch := regexp.MustCompile(`&lt;img[^&]+src=([^&>\s]+)`)
	imgMatches := imgMatch.FindStringSubmatch(content)
	if len(imgMatches) > 1 {
		imageURL := strings.TrimSpace(imgMatches[1])
		// 따옴표 제거
		imageURL = strings.Trim(imageURL, `"'`)

		// 상대 경로인 경우 절대 경로로 변환
		if strings.HasPrefix(imageURL, "/") {
			imageURL = "https://d2.naver.com" + imageURL
		}

		return imageURL
	}
	return ""
}

// extractPost는 개별 포스트 정보를 추출합니다.
func (c *NaverCrawler) extractPost(s *goquery.Selection) (models.BlogPost, error) {
	// 제목 추출 - 더 많은 선택자 시도
	title := ""
	titleSelectors := []string{
		"h1", "h2", "h3", "h4",
		".title", ".post-title", ".article-title", ".blog-title",
		"[class*='title']",
		"a",
	}

	for _, selector := range titleSelectors {
		if titleElem := s.Find(selector).First(); titleElem.Length() > 0 {
			title = strings.TrimSpace(titleElem.Text())
			if title != "" {
				break
			}
		}
	}

	if title == "" {
		return models.BlogPost{}, fmt.Errorf("제목을 찾을 수 없습니다")
	}

	// 메뉴 항목 필터링
	menuItems := []string{
		"hello world", "d2 news", "about d2", "naver developers",
		"deview", "opensource", "d2 startup factory", "검색", "메뉴",
		"home", "main", "index", "menu", "navigation", "nav",
	}

	titleLower := strings.ToLower(title)
	for _, menuItem := range menuItems {
		if strings.Contains(titleLower, menuItem) {
			return models.BlogPost{}, fmt.Errorf("메뉴 항목 제외: %s", title)
		}
	}

	// 링크 추출
	url := ""
	if linkElem := s.Find("a").First(); linkElem.Length() > 0 {
		if href, exists := linkElem.Attr("href"); exists {
			url = strings.TrimSpace(href)
			if !strings.HasPrefix(url, "http") {
				url = "https://d2.naver.com" + url
			}
		}
	}

	if url == "" {
		return models.BlogPost{}, fmt.Errorf("링크를 찾을 수 없습니다")
	}

	// URL이 메인 페이지나 메뉴 페이지인지 확인
	if strings.Contains(url, "/home") || strings.Contains(url, "/main") ||
		strings.Contains(url, "/index") || strings.Contains(url, "/menu") {
		return models.BlogPost{}, fmt.Errorf("메뉴 링크 제외: %s", url)
	}

	// 요약 추출
	summary := ""
	summarySelectors := []string{
		".summary", ".excerpt", ".description", ".content",
		"[class*='summary']", "[class*='excerpt']", "[class*='description']",
		"p",
	}

	for _, selector := range summarySelectors {
		if summaryElem := s.Find(selector).First(); summaryElem.Length() > 0 {
			summary = strings.TrimSpace(summaryElem.Text())
			if summary != "" && summary != title {
				break
			}
		}
	}

	if summary == "" {
		summary = "네이버 D2 기술 블로그 포스트"
	}

	// 날짜 추출
	publishedAt := time.Now()
	dateSelectors := []string{
		".date", ".published", ".post-date", ".article-date",
		"time", "[datetime]",
		"[class*='date']", "[class*='time']",
	}

	for _, selector := range dateSelectors {
		if dateElem := s.Find(selector).First(); dateElem.Length() > 0 {
			if datetime, exists := dateElem.Attr("datetime"); exists {
				if t, err := time.Parse(time.RFC3339, datetime); err == nil {
					publishedAt = t
					break
				}
			} else {
				dateText := strings.TrimSpace(dateElem.Text())
				if t, err := c.parseDate(dateText); err == nil {
					publishedAt = t
					break
				}
			}
		}
	}

	// 작성자 추출
	author := ""
	authorSelectors := []string{
		".author", ".writer", ".byline", ".post-author",
		"[class*='author']", "[class*='writer']",
	}

	for _, selector := range authorSelectors {
		if authorElem := s.Find(selector).First(); authorElem.Length() > 0 {
			author = strings.TrimSpace(authorElem.Text())
			if author != "" {
				break
			}
		}
	}

	if author == "" {
		author = "네이버 D2팀"
	}

	// 이미지 추출
	imageURL := ""
	if imgElem := s.Find("img").First(); imgElem.Length() > 0 {
		if src, exists := imgElem.Attr("src"); exists {
			imageURL = strings.TrimSpace(src)
			if !strings.HasPrefix(imageURL, "http") {
				imageURL = "https://d2.naver.com" + imageURL
			}
		}
	}

	// 카테고리 결정
	category := c.determineCategory(title, summary, url)

	post := models.BlogPost{
		Title:       title,
		URL:         url,
		Author:      author,
		PublishedAt: publishedAt,
		Summary:     summary,
		Source:      "네이버 D2",
		Category:    category,
		Image:       imageURL,
	}

	return post, nil
}

// determineCategory는 제목, 요약, URL을 분석해서 정확한 카테고리를 결정합니다.
func (c *NaverCrawler) determineCategory(title, summary, url string) string {
	titleLower := strings.ToLower(title)
	summaryLower := strings.ToLower(summary)

	// AI/머신러닝 관련
	if strings.Contains(titleLower, "ai") || strings.Contains(summaryLower, "ai") ||
		strings.Contains(titleLower, "머신러닝") || strings.Contains(summaryLower, "머신러닝") ||
		strings.Contains(titleLower, "딥러닝") || strings.Contains(summaryLower, "딥러닝") ||
		strings.Contains(titleLower, "llm") || strings.Contains(summaryLower, "llm") ||
		strings.Contains(titleLower, "챗봇") || strings.Contains(summaryLower, "챗봇") ||
		strings.Contains(titleLower, "시맨틱") || strings.Contains(summaryLower, "시맨틱") ||
		strings.Contains(titleLower, "rag") || strings.Contains(summaryLower, "rag") ||
		strings.Contains(titleLower, "nlp") || strings.Contains(summaryLower, "nlp") ||
		strings.Contains(titleLower, "컴퓨터 비전") || strings.Contains(summaryLower, "컴퓨터 비전") {
		return "AI"
	}

	// 데이터 관련
	if strings.Contains(titleLower, "데이터") || strings.Contains(summaryLower, "데이터") ||
		strings.Contains(titleLower, "data") || strings.Contains(summaryLower, "data") ||
		strings.Contains(titleLower, "분석") || strings.Contains(summaryLower, "분석") ||
		strings.Contains(titleLower, "analytics") || strings.Contains(summaryLower, "analytics") ||
		strings.Contains(titleLower, "빅데이터") || strings.Contains(summaryLower, "빅데이터") ||
		strings.Contains(titleLower, "datahub") || strings.Contains(summaryLower, "datahub") ||
		strings.Contains(titleLower, "airflow") || strings.Contains(summaryLower, "airflow") {
		return "데이터"
	}

	// 검색 관련
	if strings.Contains(titleLower, "검색") || strings.Contains(summaryLower, "검색") ||
		strings.Contains(titleLower, "search") || strings.Contains(summaryLower, "search") ||
		strings.Contains(titleLower, "indexing") || strings.Contains(summaryLower, "indexing") ||
		strings.Contains(titleLower, "형태소") || strings.Contains(summaryLower, "형태소") ||
		strings.Contains(titleLower, "seo") || strings.Contains(summaryLower, "seo") ||
		strings.Contains(titleLower, "elasticsearch") || strings.Contains(summaryLower, "elasticsearch") {
		return "검색"
	}

	// 엔지니어링 관련
	if strings.Contains(titleLower, "개발") || strings.Contains(summaryLower, "개발") ||
		strings.Contains(titleLower, "프로그래밍") || strings.Contains(summaryLower, "프로그래밍") ||
		strings.Contains(titleLower, "코딩") || strings.Contains(summaryLower, "코딩") ||
		strings.Contains(titleLower, "프론트엔드") || strings.Contains(summaryLower, "프론트엔드") ||
		strings.Contains(titleLower, "백엔드") || strings.Contains(summaryLower, "백엔드") ||
		strings.Contains(titleLower, "웹") || strings.Contains(summaryLower, "웹") ||
		strings.Contains(titleLower, "앱") || strings.Contains(summaryLower, "앱") ||
		strings.Contains(titleLower, "서버") || strings.Contains(summaryLower, "서버") ||
		strings.Contains(titleLower, "클라우드") || strings.Contains(summaryLower, "클라우드") ||
		strings.Contains(titleLower, "docker") || strings.Contains(summaryLower, "docker") ||
		strings.Contains(titleLower, "kubernetes") || strings.Contains(summaryLower, "kubernetes") ||
		strings.Contains(titleLower, "microservice") || strings.Contains(summaryLower, "microservice") {
		return "엔지니어링"
	}

	// IT스타트업 관련 (협업, 팀워크, 업무 방식 등)
	if strings.Contains(titleLower, "협업") || strings.Contains(summaryLower, "협업") ||
		strings.Contains(titleLower, "팀워크") || strings.Contains(summaryLower, "팀워크") ||
		strings.Contains(titleLower, "업무") || strings.Contains(summaryLower, "업무") ||
		strings.Contains(titleLower, "문화") || strings.Contains(summaryLower, "문화") ||
		strings.Contains(titleLower, "조직") || strings.Contains(summaryLower, "조직") ||
		strings.Contains(titleLower, "리더") || strings.Contains(summaryLower, "리더") ||
		strings.Contains(titleLower, "인터뷰") || strings.Contains(summaryLower, "인터뷰") ||
		strings.Contains(titleLower, "소개") || strings.Contains(summaryLower, "소개") ||
		strings.Contains(titleLower, "성장") || strings.Contains(summaryLower, "성장") ||
		strings.Contains(titleLower, "스타트업") || strings.Contains(summaryLower, "스타트업") {
		return "IT스타트업"
	}

	// 기본값
	return "엔지니어링"
}

// parseDate는 다양한 날짜 형식을 파싱합니다.
func (c *NaverCrawler) parseDate(dateStr string) (time.Time, error) {
	// 다양한 날짜 형식 시도
	formats := []string{
		"2006-01-02",
		"2006.01.02",
		"2006년 01월 02일",
		"Jan 02, 2006",
		"January 02, 2006",
		"02 Jan 2006",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Now(), fmt.Errorf("날짜 형식을 파싱할 수 없습니다: %s", dateStr)
}

// removeDuplicates는 중복된 포스트를 제거합니다.
func (c *NaverCrawler) removeDuplicates(posts []models.BlogPost) []models.BlogPost {
	seen := make(map[string]bool)
	var uniquePosts []models.BlogPost

	for _, post := range posts {
		if !seen[post.URL] {
			seen[post.URL] = true
			uniquePosts = append(uniquePosts, post)
		}
	}

	return uniquePosts
}

// sortByDate는 포스트를 최신순으로 정렬합니다.
func (c *NaverCrawler) sortByDate(posts []models.BlogPost) {
	// 내림차순 정렬 (최신순) - Go의 sort.Slice 사용
	for i := 0; i < len(posts)-1; i++ {
		for j := i + 1; j < len(posts); j++ {
			if posts[i].PublishedAt.Before(posts[j].PublishedAt) {
				posts[i], posts[j] = posts[j], posts[i]
			}
		}
	}

	log.Printf("포스트 정렬 완료: 최신 포스트는 '%s' (%v)", posts[0].Title, posts[0].PublishedAt)
}

// crawlPostDetail은 포스트 상세 페이지에서 관련 포스트나 추가 정보를 가져옵니다.
func (c *NaverCrawler) crawlPostDetail(url string) ([]models.BlogPost, error) {
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("상세 페이지 요청 실패: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("상세 페이지 응답 오류: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("상세 페이지 HTML 파싱 실패: %w", err)
	}

	var relatedPosts []models.BlogPost

	// 관련 포스트 링크 찾기
	doc.Find("a[href*='/helloworld/'], a[href*='/news/']").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		// 절대 URL로 변환
		if strings.HasPrefix(href, "/") {
			href = "https://d2.naver.com" + href
		}

		// 이미 수집된 포스트인지 확인
		for _, existingPost := range relatedPosts {
			if existingPost.URL == href {
				return
			}
		}

		title := strings.TrimSpace(s.Text())
		if title == "" {
			title = "네이버 D2 포스트"
		}

		// 간단한 포스트 정보 생성
		post := models.BlogPost{
			Title:       title,
			URL:         href,
			Author:      "네이버 D2팀",
			PublishedAt: time.Now(), // 정확한 날짜는 알 수 없으므로 현재 시간으로 설정
			Summary:     "네이버 D2 기술 블로그 포스트",
			Source:      "네이버 D2",
			Category:    "엔지니어링",
			Image:       "",
		}

		relatedPosts = append(relatedPosts, post)
	})

	return relatedPosts, nil
}

// extractDateFromPage는 실제 블로그 페이지에서 날짜를 추출합니다.
func (c *NaverCrawler) extractDateFromPage(url string) time.Time {
	resp, err := c.client.Get(url)
	if err != nil {
		log.Printf("페이지 날짜 추출 실패 (요청): %v", err)
		return time.Time{}
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Printf("페이지 날짜 추출 실패 (파싱): %v", err)
		return time.Time{}
	}

	// 다양한 날짜 선택자 시도
	dateSelectors := []string{
		".date", ".published", ".post-date", ".article-date", ".publish-date",
		".created", ".created-date", ".time", ".timestamp",
		"[class*='date']", "[class*='time']", "[class*='published']",
		"time[datetime]", "time", "span[datetime]",
		".article-info .date", ".post-info .date", ".meta .date",
		".entry-date", ".post-meta .date", ".article-meta .date",
	}

	for _, selector := range dateSelectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			// datetime 속성 확인
			if datetime, exists := s.Attr("datetime"); exists && datetime != "" {
				if t, err := c.parseAtomTime(datetime); err == nil {
					log.Printf("datetime 속성에서 날짜 발견: %v", t)
					return
				}
			}

			// 텍스트에서 날짜 추출
			text := strings.TrimSpace(s.Text())
			if text != "" {
				// 한국어 날짜 패턴 (예: "2025년 6월 25일", "2025.06.25")
				datePatterns := []*regexp.Regexp{
					regexp.MustCompile(`(\d{4})년\s*(\d{1,2})월\s*(\d{1,2})일`),
					regexp.MustCompile(`(\d{4})\.(\d{1,2})\.(\d{1,2})`),
					regexp.MustCompile(`(\d{4})-(\d{1,2})-(\d{1,2})`),
					regexp.MustCompile(`(\d{4})/(\d{1,2})/(\d{1,2})`),
				}

				for _, pattern := range datePatterns {
					if matches := pattern.FindStringSubmatch(text); len(matches) == 4 {
						dateStr := fmt.Sprintf("%s-%s-%s", matches[1], matches[2], matches[3])
						if t, err := time.Parse("2006-1-2", dateStr); err == nil {
							log.Printf("텍스트에서 날짜 발견: %s -> %v", text, t)
							return
						}
					}
				}
			}
		})
	}

	// 메타 태그에서 날짜 확인
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		if prop, _ := s.Attr("property"); prop == "article:published_time" {
			if content, ok := s.Attr("content"); ok {
				if t, err := c.parseAtomTime(content); err == nil {
					log.Printf("메타 태그에서 날짜 발견: %v", t)
					return
				}
			}
		}
	})

	return time.Time{}
}
