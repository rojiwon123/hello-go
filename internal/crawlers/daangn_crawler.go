package crawlers

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"hello-go/internal/models"
)

// RSS 구조체 정의
type RSS struct {
	Channel Channel `xml:"channel"`
}

type Channel struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Link        string `xml:"link"`
	Items       []Item `xml:"item"`
}

type Item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	Category    string `xml:"category"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Author      string `xml:"dc:creator"`
	Content     string `xml:"content:encoded"`
	// 네임스페이스 처리를 위한 추가 필드
	ContentEncoded string `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
}

// DaangnCrawler는 당근마켓 기술 블로그를 크롤링합니다.
type DaangnCrawler struct {
	client *http.Client
}

// NewDaangnCrawler는 새로운 DaangnCrawler 인스턴스를 생성합니다.
func NewDaangnCrawler() *DaangnCrawler {
	return &DaangnCrawler{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// GetSource는 블로그 소스 정보를 반환합니다.
func (c *DaangnCrawler) GetSource() models.BlogSource {
	return models.BlogSource{
		Name: "당근마켓",
		URL:  "https://medium.com/daangn",
	}
}

// Crawl은 당근마켓 기술 블로그를 크롤링합니다.
func (c *DaangnCrawler) Crawl() ([]models.BlogPost, error) {
	var allPosts []models.BlogPost

	// 1. 메인 RSS 피드에서 포스트 가져오기 (가장 정확한 날짜 정보)
	log.Printf("메인 RSS 피드 크롤링 시작")
	mainPosts, err := c.crawlMainRSS()
	if err != nil {
		log.Printf("메인 RSS 크롤링 실패: %v", err)
	} else {
		allPosts = append(allPosts, mainPosts...)
		log.Printf("메인 RSS에서 %d개 포스트 가져옴", len(mainPosts))
	}

	log.Printf("당근마켓 블로그 크롤링 완료: 총 %d개 포스트 발견", len(allPosts))
	return allPosts, nil
}

// crawlArchivePage는 아카이브 페이지에서 포스트를 크롤링합니다.
func (c *DaangnCrawler) crawlArchivePage() ([]models.BlogPost, error) {
	// 아카이브 페이지 URL들 시도
	archiveURLs := []string{
		"https://medium.com/daangn/archive",
		"https://medium.com/daangn/archive/2025",
		"https://medium.com/daangn/archive/2025/01",
		"https://medium.com/daangn/archive/2025/02",
		"https://medium.com/daangn/archive/2025/03",
		"https://medium.com/daangn/archive/2025/04",
		"https://medium.com/daangn/archive/2025/05",
		"https://medium.com/daangn/archive/2025/06",
		"https://medium.com/daangn/archive/2025/07",
		"https://medium.com/daangn/archive/2025/08",
	}

	var allArchivePosts []models.BlogPost

	for _, url := range archiveURLs {
		log.Printf("아카이브 페이지 크롤링: %s", url)
		posts, err := c.crawlSingleArchivePage(url)
		if err != nil {
			log.Printf("아카이브 페이지 %s 크롤링 실패: %v", url, err)
			continue
		}
		allArchivePosts = append(allArchivePosts, posts...)
	}

	return allArchivePosts, nil
}

// crawlSingleArchivePage는 단일 아카이브 페이지를 크롤링합니다.
func (c *DaangnCrawler) crawlSingleArchivePage(url string) ([]models.BlogPost, error) {
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("아카이브 페이지 요청 실패: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("아카이브 페이지 응답 오류: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("HTML 파싱 실패: %w", err)
	}

	var posts []models.BlogPost

	// 다양한 선택자로 포스트 링크 찾기
	selectors := []string{
		"a[href*='/daangn/']",
		"article a",
		".post-item a",
		"[data-testid='post-preview'] a",
		"h1 a, h2 a, h3 a, h4 a",
		".post-title a",
		".title a",
	}

	// 카테고리 메뉴 항목들 (제외할 항목들)
	excludeTitles := []string{
		"엔지니어링", "머신러닝", "데이터", "검색", "IT스타트업", "AI", "아카이브", "채용공고",
		"Engineering", "Machine Learning", "Data", "Search", "IT Startup", "Archive", "Jobs",
	}

	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if !exists {
				return
			}

			// 포스트 링크인지 확인 (글자 수가 긴 링크)
			if !strings.Contains(href, "/daangn/") || strings.Count(href, "/") < 3 {
				return
			}

			// 제목 추출
			title := strings.TrimSpace(s.Text())
			if title == "" || len(title) < 10 {
				return
			}

			// 카테고리 메뉴 항목 제외
			for _, excludeTitle := range excludeTitles {
				if strings.TrimSpace(title) == excludeTitle {
					return
				}
			}

			// 링크 정리
			if !strings.HasPrefix(href, "http") {
				href = "https://medium.com" + href
			}

			// 중복 제거
			for _, existingPost := range posts {
				if existingPost.URL == href {
					return
				}
			}

			// 요약 추출
			summary := ""
			if parent := s.Parent(); parent.Length() > 0 {
				summary = strings.TrimSpace(parent.Text())
				if len(summary) > 200 {
					summary = summary[:200] + "..."
				}
			}
			if summary == "" {
				summary = "당근마켓 기술 블로그 포스트"
			}

			// 이미지 추출
			imageURL := ""
			if img := s.Find("img").First(); img.Length() > 0 {
				if src, exists := img.Attr("src"); exists {
					imageURL = src
				}
			}

			post := models.BlogPost{
				Title:       title,
				URL:         href,
				Author:      "당근마켓팀",
				PublishedAt: time.Now(), // 아카이브에서는 정확한 날짜를 파싱하기 어려우므로 현재 시간 사용
				Summary:     summary,
				Source:      "당근마켓",
				Category:    "개발", // 기본 카테고리
				Image:       imageURL,
			}

			posts = append(posts, post)
			log.Printf("아카이브 포스트 발견: %s", title)
		})
	}

	return posts, nil
}

// getPostDetails는 포스트 ID로 상세 정보를 가져옵니다.
func (c *DaangnCrawler) getPostDetails(postID, postURL string) (models.BlogPost, error) {
	// 포스트 페이지에서 정보 추출
	resp, err := c.client.Get(postURL)
	if err != nil {
		return models.BlogPost{}, fmt.Errorf("포스트 페이지 요청 실패: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return models.BlogPost{}, fmt.Errorf("포스트 HTML 파싱 실패: %w", err)
	}

	// 제목 추출
	title := ""
	if titleElem := doc.Find("h1").First(); titleElem.Length() > 0 {
		title = strings.TrimSpace(titleElem.Text())
	}
	if title == "" {
		if titleElem := doc.Find("title").First(); titleElem.Length() > 0 {
			title = strings.TrimSpace(titleElem.Text())
		}
	}

	if title == "" {
		return models.BlogPost{}, fmt.Errorf("제목을 찾을 수 없습니다")
	}

	// 요약 추출
	summary := ""
	if metaDesc := doc.Find("meta[name='description']").First(); metaDesc.Length() > 0 {
		if desc, exists := metaDesc.Attr("content"); exists {
			summary = strings.TrimSpace(desc)
		}
	}
	if summary == "" {
		summary = "당근마켓 기술 블로그 포스트"
	}

	// 이미지 추출 (더 적극적으로)
	imageURL := ""

	// 1. og:image 메타 태그
	if ogImage := doc.Find("meta[property='og:image']").First(); ogImage.Length() > 0 {
		if img, exists := ogImage.Attr("content"); exists {
			imageURL = strings.TrimSpace(img)
		}
	}

	// 2. twitter:image 메타 태그
	if imageURL == "" {
		if twitterImage := doc.Find("meta[name='twitter:image']").First(); twitterImage.Length() > 0 {
			if img, exists := twitterImage.Attr("content"); exists {
				imageURL = strings.TrimSpace(img)
			}
		}
	}

	// 3. 첫 번째 이미지 태그
	if imageURL == "" {
		if img := doc.Find("img").First(); img.Length() > 0 {
			if src, exists := img.Attr("src"); exists {
				imageURL = strings.TrimSpace(src)
			}
		}
	}

	// 4. 배경 이미지가 있는 요소
	if imageURL == "" {
		doc.Find("[style*='background-image']").Each(func(i int, s *goquery.Selection) {
			if imageURL != "" {
				return
			}
			if style, exists := s.Attr("style"); exists {
				re := regexp.MustCompile(`background-image:\s*url\(['"]?([^'"]+)['"]?\)`)
				matches := re.FindStringSubmatch(style)
				if len(matches) > 1 {
					imageURL = strings.TrimSpace(matches[1])
				}
			}
		})
	}

	// 날짜 추출 (더 정확한 방법)
	publishedAt := time.Now()

	log.Printf("Apollo State 포스트 상세 페이지에서 날짜 추출 시도: %s", postURL)

	// 1. time 태그의 datetime 속성
	if timeElem := doc.Find("time").First(); timeElem.Length() > 0 {
		if datetime, exists := timeElem.Attr("datetime"); exists {
			log.Printf("time 태그에서 datetime 발견: %s", datetime)
			if t, err := time.Parse(time.RFC3339, datetime); err == nil {
				publishedAt = t
				log.Printf("time 태그 날짜 파싱 성공: %s", publishedAt.Format("2006-01-02 15:04:05"))
			} else {
				log.Printf("time 태그 날짜 파싱 실패: %v", err)
			}
		}
	}

	// 2. meta 태그에서 날짜 찾기
	if publishedAt.Equal(time.Now()) {
		if metaDate := doc.Find("meta[property='article:published_time']").First(); metaDate.Length() > 0 {
			if datetime, exists := metaDate.Attr("content"); exists {
				log.Printf("article:published_time 메타 태그에서 날짜 발견: %s", datetime)
				if t, err := time.Parse(time.RFC3339, datetime); err == nil {
					publishedAt = t
					log.Printf("메타 태그 날짜 파싱 성공: %s", publishedAt.Format("2006-01-02 15:04:05"))
				} else {
					log.Printf("메타 태그 날짜 파싱 실패: %v", err)
				}
			}
		}
	}

	// 3. 다른 날짜 관련 메타 태그들
	if publishedAt.Equal(time.Now()) {
		dateSelectors := []string{
			"meta[name='publish_date']",
			"meta[name='date']",
			"meta[property='og:updated_time']",
		}
		for _, selector := range dateSelectors {
			if metaDate := doc.Find(selector).First(); metaDate.Length() > 0 {
				if datetime, exists := metaDate.Attr("content"); exists {
					log.Printf("%s 메타 태그에서 날짜 발견: %s", selector, datetime)
					if t, err := time.Parse(time.RFC3339, datetime); err == nil {
						publishedAt = t
						log.Printf("메타 태그 날짜 파싱 성공: %s", publishedAt.Format("2006-01-02 15:04:05"))
						break
					} else {
						log.Printf("메타 태그 날짜 파싱 실패: %v", err)
					}
				}
			}
		}
	}

	if publishedAt.Equal(time.Now()) {
		log.Printf("모든 날짜 추출 방법 실패, 현재 시간 사용: %s", publishedAt.Format("2006-01-02 15:04:05"))
	}

	// 카테고리 결정
	category := c.determineCategory(title, summary, postURL)

	post := models.BlogPost{
		Title:       title,
		URL:         postURL,
		Author:      "당근마켓팀",
		PublishedAt: publishedAt,
		Summary:     summary,
		Source:      "당근마켓",
		Category:    category,
		Image:       imageURL,
	}

	return post, nil
}

// determineCategory는 제목, 요약, URL을 분석해서 정확한 카테고리를 결정합니다.
func (c *DaangnCrawler) determineCategory(title, summary, url string) string {
	titleLower := strings.ToLower(title)
	summaryLower := strings.ToLower(summary)

	// AI/머신러닝 관련
	if strings.Contains(titleLower, "ai") || strings.Contains(summaryLower, "ai") ||
		strings.Contains(titleLower, "머신러닝") || strings.Contains(summaryLower, "머신러닝") ||
		strings.Contains(titleLower, "llm") || strings.Contains(summaryLower, "llm") ||
		strings.Contains(titleLower, "챗봇") || strings.Contains(summaryLower, "챗봇") ||
		strings.Contains(titleLower, "시맨틱") || strings.Contains(summaryLower, "시맨틱") ||
		strings.Contains(titleLower, "rag") || strings.Contains(summaryLower, "rag") ||
		strings.Contains(titleLower, "ai show & tell") || strings.Contains(summaryLower, "ai show & tell") {
		return "AI"
	}

	// 데이터 관련
	if strings.Contains(titleLower, "데이터") || strings.Contains(summaryLower, "데이터") ||
		strings.Contains(titleLower, "datahub") || strings.Contains(summaryLower, "datahub") ||
		strings.Contains(titleLower, "airflow") || strings.Contains(summaryLower, "airflow") ||
		strings.Contains(titleLower, "n8n") || strings.Contains(summaryLower, "n8n") ||
		strings.Contains(titleLower, "dbt") || strings.Contains(summaryLower, "dbt") ||
		strings.Contains(titleLower, "feature store") || strings.Contains(summaryLower, "feature store") ||
		strings.Contains(titleLower, "karrotmetrics") || strings.Contains(summaryLower, "karrotmetrics") {
		return "데이터"
	}

	// 검색 관련
	if strings.Contains(titleLower, "검색") || strings.Contains(summaryLower, "검색") ||
		strings.Contains(titleLower, "search") || strings.Contains(summaryLower, "search") ||
		strings.Contains(titleLower, "indexing") || strings.Contains(summaryLower, "indexing") ||
		strings.Contains(titleLower, "형태소") || strings.Contains(summaryLower, "형태소") ||
		strings.Contains(titleLower, "seo") || strings.Contains(summaryLower, "seo") {
		return "검색"
	}

	// 엔지니어링 관련
	if strings.Contains(titleLower, "에디터") || strings.Contains(summaryLower, "에디터") ||
		strings.Contains(titleLower, "로깅") || strings.Contains(summaryLower, "로깅") ||
		strings.Contains(titleLower, "웹앱") || strings.Contains(summaryLower, "웹앱") ||
		strings.Contains(titleLower, "프론트엔드") || strings.Contains(summaryLower, "프론트엔드") ||
		strings.Contains(titleLower, "개발") || strings.Contains(summaryLower, "개발") ||
		strings.Contains(titleLower, "swift") || strings.Contains(summaryLower, "swift") ||
		strings.Contains(titleLower, "macro") || strings.Contains(summaryLower, "macro") ||
		strings.Contains(titleLower, "streaming ssr") || strings.Contains(summaryLower, "streaming ssr") ||
		strings.Contains(titleLower, "백엔드") || strings.Contains(summaryLower, "백엔드") ||
		strings.Contains(titleLower, "feed") || strings.Contains(summaryLower, "feed") ||
		strings.Contains(titleLower, "entity") || strings.Contains(summaryLower, "entity") ||
		strings.Contains(titleLower, "의존성") || strings.Contains(summaryLower, "의존성") ||
		strings.Contains(titleLower, "프로젝트") || strings.Contains(summaryLower, "프로젝트") {
		return "엔지니어링"
	}

	// IT스타트업 관련 (협업, 팀워크, 업무 방식 등)
	if strings.Contains(titleLower, "협업") || strings.Contains(summaryLower, "협업") ||
		strings.Contains(titleLower, "팀워크") || strings.Contains(summaryLower, "팀워크") ||
		strings.Contains(titleLower, "업무") || strings.Contains(summaryLower, "업무") ||
		strings.Contains(titleLower, "리더") || strings.Contains(summaryLower, "리더") ||
		strings.Contains(titleLower, "인터뷰") || strings.Contains(summaryLower, "인터뷰") ||
		strings.Contains(titleLower, "프로덕트 디자이너") || strings.Contains(summaryLower, "프로덕트 디자이너") ||
		strings.Contains(titleLower, "소개") || strings.Contains(summaryLower, "소개") ||
		strings.Contains(titleLower, "입사") || strings.Contains(summaryLower, "입사") ||
		strings.Contains(titleLower, "온보딩") || strings.Contains(summaryLower, "온보딩") ||
		strings.Contains(titleLower, "mvp") || strings.Contains(summaryLower, "mvp") ||
		strings.Contains(titleLower, "성장") || strings.Contains(summaryLower, "성장") {
		return "IT스타트업"
	}

	// 기본값
	return "엔지니어링"
}

// crawlMainRSS는 메인 RSS 피드를 크롤링합니다.
func (c *DaangnCrawler) crawlMainRSS() ([]models.BlogPost, error) {
	url := "https://medium.com/feed/daangn"
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("RSS 피드 요청 실패: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RSS 피드 응답 오류: %d", resp.StatusCode)
	}

	var rss RSS
	if err := xml.NewDecoder(resp.Body).Decode(&rss); err != nil {
		return nil, fmt.Errorf("RSS 피드 파싱 실패: %w", err)
	}

	var posts []models.BlogPost

	for _, item := range rss.Channel.Items {
		// 제목에서 CDATA 제거
		title := strings.TrimSpace(strings.ReplaceAll(item.Title, "<![CDATA[", ""))
		title = strings.TrimSpace(strings.ReplaceAll(title, "]]>", ""))

		if title == "" {
			continue
		}

		// 링크 정리
		link := strings.TrimSpace(item.Link)
		if link == "" {
			continue
		}

		// 날짜 파싱
		log.Printf("RSS 날짜 파싱 시도: %s", item.PubDate)
		publishedAt, err := c.parseRSSTime(item.PubDate)
		if err != nil {
			log.Printf("RSS 날짜 파싱 실패: %v, 현재 시간 사용", err)
			publishedAt = time.Now()
		} else {
			log.Printf("RSS 날짜 파싱 성공: %s -> %s", item.PubDate, publishedAt.Format("2006-01-02 15:04:05"))
		}

		// 제목과 날짜 정보 출력
		log.Printf("당근 포스트: %s | 날짜: %s", title, publishedAt.Format("2006-01-02 15:04:05"))

		// 요약 정리
		summary := strings.TrimSpace(strings.ReplaceAll(item.Description, "<![CDATA[", ""))
		summary = strings.TrimSpace(strings.ReplaceAll(summary, "]]>", ""))
		if summary == "" {
			summary = "당근마켓 기술 블로그 포스트"
		}

		// 작성자
		author := strings.TrimSpace(item.Author)
		if author == "" {
			author = "당근마켓팀"
		}

		// RSS 필드 디버깅
		log.Printf("RSS 필드 디버깅 - Title: %s, Content 길이: %d, ContentEncoded 길이: %d, Description 길이: %d", title, len(item.Content), len(item.ContentEncoded), len(item.Description))

		// 썸네일 이미지 추출 (두 필드 모두 시도)
		content := item.Content
		if content == "" {
			content = item.ContentEncoded
		}
		imageURL := c.extractThumbnail(content, item.Description)
		log.Printf("이미지 추출 결과: %s -> %s", title, imageURL)

		// 카테고리 결정
		category := c.determineCategory(title, summary, link)

		post := models.BlogPost{
			Title:       title,
			URL:         link,
			Author:      author,
			PublishedAt: publishedAt,
			Summary:     summary,
			Source:      "당근마켓",
			Category:    category,
			Image:       imageURL,
		}

		posts = append(posts, post)
		log.Printf("RSS 포스트 발견: %s (카테고리: %s)", title, category)
	}

	return posts, nil
}

// crawlApolloState는 Medium의 Apollo State에서 포스트 정보를 추출합니다.
func (c *DaangnCrawler) crawlApolloState() ([]models.BlogPost, error) {
	resp, err := c.client.Get("https://medium.com/daangn")
	if err != nil {
		return nil, fmt.Errorf("메인 페이지 요청 실패: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("메인 페이지 응답 오류: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("HTML 파싱 실패: %w", err)
	}

	// Apollo State 스크립트 찾기
	var apolloStateStr string
	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		scriptContent := s.Text()
		if strings.Contains(scriptContent, "window.__APOLLO_STATE__") {
			// window.__APOLLO_STATE__ = {...} 형태에서 JSON 부분 추출
			re := regexp.MustCompile(`window\.__APOLLO_STATE__\s*=\s*({.*})`)
			matches := re.FindStringSubmatch(scriptContent)
			if len(matches) > 1 {
				apolloStateStr = matches[1]
			}
		}
	})

	if apolloStateStr == "" {
		return nil, fmt.Errorf("Apollo State를 찾을 수 없습니다")
	}

	// JSON 파싱
	var apolloState map[string]interface{}
	if err := json.Unmarshal([]byte(apolloStateStr), &apolloState); err != nil {
		return nil, fmt.Errorf("Apollo State JSON 파싱 실패: %w", err)
	}

	var posts []models.BlogPost

	// Post 객체들 찾기
	for key := range apolloState {
		if strings.HasPrefix(key, "Post:") {
			postID := strings.TrimPrefix(key, "Post:")
			postURL := fmt.Sprintf("https://medium.com/daangn/%s", postID)

			// 포스트 상세 정보 가져오기
			post, err := c.getPostDetails(postID, postURL)
			if err != nil {
				log.Printf("포스트 %s 상세 정보 가져오기 실패: %v", postID, err)
				continue
			}

			if post.Title != "" {
				posts = append(posts, post)
				log.Printf("Apollo State 포스트 발견: %s", post.Title)
			}
		}
	}

	return posts, nil
}

// extractThumbnail은 콘텐츠에서 썸네일 이미지를 추출합니다.
func (c *DaangnCrawler) extractThumbnail(content, description string) string {
	log.Printf("이미지 추출 시작 - content 길이: %d, description 길이: %d", len(content), len(description))

	// 1. content:encoded에서 이미지 찾기
	if content != "" {
		// img 태그에서 src 추출 (더 정교한 정규식)
		imgRe := regexp.MustCompile(`<img[^>]+src=["']([^"']+)["']`)
		matches := imgRe.FindStringSubmatch(content)
		if len(matches) > 1 {
			imageURL := strings.TrimSpace(matches[1])
			if imageURL != "" && !strings.HasPrefix(imageURL, "data:") {
				log.Printf("content에서 이미지 발견: %s", imageURL)
				return imageURL
			}
		}

		// figure 태그 내의 img 찾기
		figureRe := regexp.MustCompile(`<figure[^>]*>.*?<img[^>]+src=["']([^"']+)["']`)
		matches = figureRe.FindStringSubmatch(content)
		if len(matches) > 1 {
			imageURL := strings.TrimSpace(matches[1])
			if imageURL != "" && !strings.HasPrefix(imageURL, "data:") {
				log.Printf("figure에서 이미지 발견: %s", imageURL)
				return imageURL
			}
		}
	}

	// 2. description에서 이미지 찾기
	if description != "" {
		imgRe := regexp.MustCompile(`<img[^>]+src=["']([^"']+)["']`)
		matches := imgRe.FindStringSubmatch(description)
		if len(matches) > 1 {
			imageURL := strings.TrimSpace(matches[1])
			if imageURL != "" && !strings.HasPrefix(imageURL, "data:") {
				log.Printf("description에서 이미지 발견: %s", imageURL)
				return imageURL
			}
		}
	}

	log.Printf("이미지를 찾을 수 없음")
	return ""
}

// parseRSSTime은 RSS 날짜 형식을 파싱합니다.
func (c *DaangnCrawler) parseRSSTime(dateStr string) (time.Time, error) {
	// RSS 표준 형식들 (Medium에서 사용하는 형식 포함)
	formats := []string{
		"Mon, 02 Jan 2006 15:04:05 MST",
		"Mon, 02 Jan 2006 15:04:05 GMT",
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 +0000",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05+00:00",
		"2006-01-02 15:04:05",
		"Jan 02, 2006 15:04:05",
		"January 02, 2006 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	// 디버깅을 위한 로그 추가
	log.Printf("RSS 날짜 파싱 시도: %s", dateStr)

	// 파싱 실패 시 현재 시간 반환
	return time.Now(), fmt.Errorf("RSS 날짜 파싱 실패: %s", dateStr)
}
