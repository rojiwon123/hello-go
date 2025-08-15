package crawlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"

	"hello-go/internal/models"
)

// TossCrawler는 토스 기술 블로그를 크롤링합니다.
type TossCrawler struct {
	client *http.Client
}

// Toss API 응답 구조체
type TossPost struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	Subtitle      string `json:"subtitle"`
	Key           string `json:"key"`
	CreatedTime   string `json:"createdTime"`
	PublishedTime string `json:"publishedTime"`
	Category      string `json:"category"`
	Categories    []struct {
		Name string `json:"name"`
	} `json:"categories"`
	Editor struct {
		Name string `json:"name"`
	} `json:"editor"`
	ShortDescription string `json:"shortDescription"`
	FullDescription  string `json:"fullDescription"`
	Thumbnail        string `json:"thumbnail"`
	CoverImage       string `json:"coverImage"`
	Image            string `json:"image"`
}

type TossAPIResponse struct {
	Page    int        `json:"page"`
	Results []TossPost `json:"results"`
	Total   int        `json:"total"`
}

// NewTossCrawler는 새로운 TossCrawler 인스턴스를 생성합니다.
func NewTossCrawler() *TossCrawler {
	return &TossCrawler{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// GetSource는 토스 블로그 소스 정보를 반환합니다.
func (t *TossCrawler) GetSource() models.BlogSource {
	return models.BlogSource{
		Name: "토스",
		URL:  "https://toss.tech/",
	}
}

// Crawl은 토스 기술 블로그를 크롤링합니다.
func (t *TossCrawler) Crawl() ([]models.BlogPost, error) {
	log.Printf("토스 블로그 크롤링 시작 (병렬 페이지네이션)")

	// 병렬 크롤링을 위한 구조체
	type pageResult struct {
		page  int
		posts []models.BlogPost
		err   error
	}

	// 결과를 저장할 채널
	resultChan := make(chan pageResult, 50)
	var wg sync.WaitGroup

	// 동시 요청 수 제한 (서버 부하 방지)
	maxConcurrent := 5
	semaphore := make(chan struct{}, maxConcurrent)

	// 첫 번째 페이지로 총 페이지 수 확인
	firstPagePosts, err := t.crawlPage("https://toss.tech/?page=1")
	if err != nil {
		return nil, fmt.Errorf("첫 페이지 크롤링 실패: %v", err)
	}

	// 첫 번째 페이지 결과 추가
	resultChan <- pageResult{page: 1, posts: firstPagePosts, err: nil}

	// 나머지 페이지들을 병렬로 크롤링
	maxPages := 50 // 최대 50페이지까지 시도
	for page := 2; page <= maxPages; page++ {
		wg.Add(1)
		go func(pageNum int) {
			defer wg.Done()

			// 세마포어로 동시 요청 수 제한
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			url := fmt.Sprintf("https://toss.tech/?page=%d", pageNum)
			log.Printf("토스 블로그 페이지 %d 크롤링: %s", pageNum, url)

			pagePosts, err := t.crawlPage(url)
			if err != nil {
				log.Printf("페이지 %d 크롤링 실패: %v", pageNum, err)
				resultChan <- pageResult{page: pageNum, posts: nil, err: err}
				return
			}

			if len(pagePosts) == 0 {
				log.Printf("페이지 %d에서 포스트를 찾을 수 없음", pageNum)
				resultChan <- pageResult{page: pageNum, posts: nil, err: nil}
				return
			}

			log.Printf("페이지 %d 완료: %d개 포스트", pageNum, len(pagePosts))
			resultChan <- pageResult{page: pageNum, posts: pagePosts, err: nil}
		}(page)

		// 서버 부하 방지를 위한 짧은 대기
		time.Sleep(100 * time.Millisecond)
	}

	// 모든 고루틴이 완료될 때까지 대기
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 결과 수집 및 정렬
	var allPosts []models.BlogPost
	pageResults := make(map[int][]models.BlogPost)

	for result := range resultChan {
		if result.err != nil {
			continue
		}
		if len(result.posts) > 0 {
			pageResults[result.page] = result.posts
		}
	}

	// 페이지 순서대로 결과 정렬
	for page := 1; page <= maxPages; page++ {
		if posts, exists := pageResults[page]; exists {
			allPosts = append(allPosts, posts...)
		} else if page > 1 {
			// 빈 페이지가 나오면 크롤링 종료
			break
		}
	}

	// 이미지 추출을 병렬로 처리
	t.extractImagesParallel(&allPosts)

	log.Printf("토스 블로그 크롤링 완료: 총 %d개 포스트 발견", len(allPosts))
	return allPosts, nil
}

// crawlPage는 특정 페이지를 크롤링합니다.
func (t *TossCrawler) crawlPage(url string) ([]models.BlogPost, error) {
	resp, err := t.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("토스 블로그 페이지 로드 실패: %v", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("토스 블로그 HTML 파싱 실패: %v", err)
	}

	// JavaScript 데이터에서 포스트 추출 시도
	posts, err := t.extractFromJavaScript(doc)
	if err != nil {
		log.Printf("JavaScript 데이터 추출 실패: %v", err)
		// HTML에서 포스트 추출 시도
		posts, err = t.extractFromHTML(doc)
		if err != nil {
			return nil, fmt.Errorf("HTML 파싱도 실패: %v", err)
		}
	}

	return posts, nil
}

func (t *TossCrawler) extractFromJavaScript(doc *goquery.Document) ([]models.BlogPost, error) {
	var posts []models.BlogPost

	log.Printf("JavaScript 데이터 추출 시작...")

	// Next.js 데이터 스크립트 태그 찾기
	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		scriptContent := s.Text()
		log.Printf("스크립트 %d 내용 길이: %d", i, len(scriptContent))

		// 큰 스크립트의 경우 일부만 로그로 출력
		if len(scriptContent) > 1000 {
			log.Printf("스크립트 %d 내용 (처음 500자): %s", i, scriptContent[:500])
		}

		// Next.js 데이터가 포함된 스크립트 찾기
		if len(scriptContent) > 1000 && strings.Contains(scriptContent, "dehydratedState") {
			log.Printf("Next.js 데이터 스크립트 발견")

			// JSON 데이터 추출
			jsonStart := strings.Index(scriptContent, "{")
			if jsonStart == -1 {
				return
			}

			jsonData := scriptContent[jsonStart:]
			log.Printf("JSON 데이터 길이: %d", len(jsonData))

			var nextData map[string]interface{}
			if err := json.Unmarshal([]byte(jsonData), &nextData); err != nil {
				log.Printf("JSON 파싱 실패: %v", err)
				return
			}

			log.Printf("JSON 파싱 성공")

			// props > pageProps > prefetchResult > dehydratedState > queries에서 포스트 데이터 찾기
			if props, ok := nextData["props"].(map[string]interface{}); ok {
				log.Printf("props 발견")
				log.Printf("props 키들: %v", getKeys(props))
				if pageProps, ok := props["pageProps"].(map[string]interface{}); ok {
					log.Printf("pageProps 발견")
					log.Printf("pageProps 키들: %v", getKeys(pageProps))
					if prefetchResult, ok := pageProps["prefetchResult"].(map[string]interface{}); ok {
						log.Printf("prefetchResult 발견")
						log.Printf("prefetchResult 키들: %v", getKeys(prefetchResult))
						if dehydratedState, ok := prefetchResult["dehydratedState"].(map[string]interface{}); ok {
							log.Printf("dehydratedState 발견")
							if queries, ok := dehydratedState["queries"].([]interface{}); ok {
								log.Printf("queries 발견, 개수: %d", len(queries))
								for _, query := range queries {
									if queryMap, ok := query.(map[string]interface{}); ok {
										if state, ok := queryMap["state"].(map[string]interface{}); ok {
											if data, ok := state["data"].(string); ok {
												log.Printf("API 데이터 발견, 길이: %d", len(data))
												// API 응답 파싱
												var apiResponse TossAPIResponse
												if err := json.Unmarshal([]byte(data), &apiResponse); err == nil {
													log.Printf("API 응답 파싱 성공, 포스트 수: %d", len(apiResponse.Results))
													for _, post := range apiResponse.Results {
														// publishedTime을 우선적으로 사용, 없으면 createdTime 사용
														var publishedAt time.Time
														if post.PublishedTime != "" {
															publishedAt, _ = t.parseDate(post.PublishedTime)
														} else if post.CreatedTime != "" {
															publishedAt, _ = t.parseDate(post.CreatedTime)
														} else {
															publishedAt = time.Now()
														}

														// 카테고리 결정
														category := "개발"
														for _, cat := range post.Categories {
															if cat.Name == "개발" || cat.Name == "데이터/ML" {
																category = cat.Name
																break
															}
														}

														// 이미지 URL 결정 (우선순위: thumbnail > coverImage > image)
														imageURL := ""
														if post.Thumbnail != "" {
															imageURL = post.Thumbnail
														} else if post.CoverImage != "" {
															imageURL = post.CoverImage
														} else if post.Image != "" {
															imageURL = post.Image
														}
														// API에서 이미지가 없으면 나중에 병렬로 처리

														postURL := fmt.Sprintf("https://toss.tech/article/%s", post.Key)

														blogPost := models.BlogPost{
															Title:       post.Title,
															URL:         postURL,
															Author:      post.Editor.Name,
															PublishedAt: publishedAt,
															Summary:     post.ShortDescription,
															Source:      "토스",
															Category:    category,
															Image:       imageURL,
														}
														posts = append(posts, blogPost)
													}
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	})

	return posts, nil
}

// getKeys는 map의 키들을 문자열 슬라이스로 반환합니다.
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func (t *TossCrawler) extractFromHTML(doc *goquery.Document) ([]models.BlogPost, error) {
	var posts []models.BlogPost

	// 다양한 선택자로 포스트 찾기
	selectors := []string{
		"article", ".post-item", ".blog-item", ".content-item",
		"[class*='post']", "[class*='article']", "[class*='card']",
	}

	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			title := t.extractTitle(s)
			url := t.extractURL(s)
			date := t.extractDate(s)
			summary := t.extractSummary(s)

			if title != "" && url != "" {
				publishedAt, _ := t.parseDate(date)

				post := models.BlogPost{
					Title:       title,
					URL:         url,
					Author:      "토스",
					PublishedAt: publishedAt,
					Summary:     summary,
					Source:      "토스",
					Category:    "개발",
				}
				posts = append(posts, post)
			}
		})
	}

	return posts, nil
}

func (t *TossCrawler) extractTitle(s *goquery.Selection) string {
	// 다양한 제목 선택자
	titleSelectors := []string{
		"h1", "h2", "h3", "h4", ".title", ".post-title", ".article-title",
		"[class*='title']", "[class*='heading']",
	}

	for _, selector := range titleSelectors {
		if title := s.Find(selector).First().Text(); title != "" {
			return strings.TrimSpace(title)
		}
	}

	// 부모 요소에서 제목 찾기
	if parent := s.Parent(); parent.Length() > 0 {
		for _, selector := range titleSelectors {
			if title := parent.Find(selector).First().Text(); title != "" {
				return strings.TrimSpace(title)
			}
		}
	}

	return ""
}

func (t *TossCrawler) extractURL(s *goquery.Selection) string {
	// 직접 링크 찾기
	if url, exists := s.Find("a").First().Attr("href"); exists && url != "" {
		if strings.HasPrefix(url, "/") {
			return "https://toss.tech" + url
		}
		return url
	}

	// 부모 요소에서 링크 찾기
	if parent := s.Parent(); parent.Length() > 0 {
		if url, exists := parent.Find("a").First().Attr("href"); exists && url != "" {
			if strings.HasPrefix(url, "/") {
				return "https://toss.tech" + url
			}
			return url
		}
	}

	return ""
}

func (t *TossCrawler) extractDate(s *goquery.Selection) string {
	// 다양한 날짜 선택자
	dateSelectors := []string{
		"time", ".date", ".published", ".post-date", ".article-date",
		"[class*='date']", "[class*='time']", "[datetime]",
	}

	for _, selector := range dateSelectors {
		if date := s.Find(selector).First().Text(); date != "" {
			return strings.TrimSpace(date)
		}
		// datetime 속성 확인
		if datetime, exists := s.Find(selector).First().Attr("datetime"); exists && datetime != "" {
			return strings.TrimSpace(datetime)
		}
	}

	// 부모 요소에서 날짜 찾기
	if parent := s.Parent(); parent.Length() > 0 {
		for _, selector := range dateSelectors {
			if date := parent.Find(selector).First().Text(); date != "" {
				return strings.TrimSpace(date)
			}
			if datetime, exists := parent.Find(selector).First().Attr("datetime"); exists && datetime != "" {
				return strings.TrimSpace(datetime)
			}
		}
	}

	return ""
}

func (t *TossCrawler) extractSummary(s *goquery.Selection) string {
	// 다양한 요약 선택자
	summarySelectors := []string{
		"p", ".excerpt", ".summary", ".description", ".post-excerpt",
		"[class*='excerpt']", "[class*='summary']", "[class*='description']",
	}

	for _, selector := range summarySelectors {
		if summary := s.Find(selector).First().Text(); summary != "" {
			summary = strings.TrimSpace(summary)
			if len(summary) > 10 && len(summary) < 200 {
				return summary
			}
		}
	}

	// 일반적인 텍스트 요소에서 요약 찾기
	textElements := s.Find("div, span").FilterFunction(func(i int, s *goquery.Selection) bool {
		text := strings.TrimSpace(s.Text())
		return len(text) > 20 && len(text) < 300
	})

	if textElements.Length() > 0 {
		return strings.TrimSpace(textElements.First().Text())
	}

	return ""
}

func (t *TossCrawler) parseDate(dateStr string) (time.Time, error) {
	// 다양한 날짜 형식 시도
	formats := []string{
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05+09:00",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"2006년 1월 2일",
		"January 2, 2006",
		"Jan 2, 2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	// 정규식으로 날짜 추출 시도
	dateRegex := regexp.MustCompile(`(\d{4})[-/년](\d{1,2})[-/월](\d{1,2})`)
	if matches := dateRegex.FindStringSubmatch(dateStr); len(matches) == 4 {
		dateStr = fmt.Sprintf("%s-%s-%s", matches[1], matches[2], matches[3])
		if t, err := time.Parse("2006-1-2", dateStr); err == nil {
			return t, nil
		}
	}

	return time.Now(), fmt.Errorf("날짜 파싱 실패: %s", dateStr)
}

// resolveThumbnail는 포스트 상세 페이지에서 og:image를 추출합니다.
func (t *TossCrawler) resolveThumbnail(postURL string) string {
	resp, err := t.client.Get(postURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return ""
	}

	var imageURL string
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		if prop, _ := s.Attr("property"); prop == "og:image" {
			if content, ok := s.Attr("content"); ok {
				imageURL = strings.TrimSpace(content)
			}
		}
	})

	return imageURL
}

// extractDateFromPage는 실제 블로그 페이지에서 날짜를 추출합니다.
func (t *TossCrawler) extractDateFromPage(url string) time.Time {
	resp, err := t.client.Get(url)
	if err != nil {
		log.Printf("토스 페이지 날짜 추출 실패 (요청): %v", err)
		return time.Time{}
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Printf("토스 페이지 날짜 추출 실패 (파싱): %v", err)
		return time.Time{}
	}

	// 메타 태그에서 날짜 확인 (우선순위 1)
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		if prop, _ := s.Attr("property"); prop == "article:published_time" {
			if content, ok := s.Attr("content"); ok {
				if parsedTime, err := t.parseDate(content); err == nil {
					log.Printf("토스 메타 태그에서 날짜 발견: %v", parsedTime)
					return
				}
			}
		}
	})

	// 다양한 날짜 선택자 시도 (우선순위 2)
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
				if parsedTime, err := t.parseDate(datetime); err == nil {
					log.Printf("토스 datetime 속성에서 날짜 발견: %v", parsedTime)
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
						if parsedTime, err := time.Parse("2006-1-2", dateStr); err == nil {
							log.Printf("토스 텍스트에서 날짜 발견: %s -> %v", text, parsedTime)
							return
						}
					}
				}
			}
		})
	}

	return time.Time{}
}

// extractThumbnailFromPage는 포스트 페이지에서 썸네일 이미지를 추출합니다.
func (t *TossCrawler) extractThumbnailFromPage(postURL string) string {
	resp, err := t.client.Get(postURL)
	if err != nil {
		log.Printf("포스트 페이지 로드 실패: %v", err)
		return ""
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Printf("포스트 페이지 HTML 파싱 실패: %v", err)
		return ""
	}

	// og:image 메타 태그에서 이미지 추출
	if ogImage := doc.Find("meta[property='og:image']").AttrOr("content", ""); ogImage != "" {
		return ogImage
	}

	// twitter:image 메타 태그에서 이미지 추출
	if twitterImage := doc.Find("meta[name='twitter:image']").AttrOr("content", ""); twitterImage != "" {
		return twitterImage
	}

	// 첫 번째 이미지 태그에서 이미지 추출
	if firstImage := doc.Find("img").First().AttrOr("src", ""); firstImage != "" {
		return firstImage
	}

	return ""
}

// extractImagesParallel은 포스트들의 이미지를 병렬로 추출합니다.
func (t *TossCrawler) extractImagesParallel(posts *[]models.BlogPost) {
	var wg sync.WaitGroup
	imageChan := make(chan struct {
		index int
		image string
	}, len(*posts))

	// 동시 이미지 추출 수 제한
	maxConcurrent := 10
	semaphore := make(chan struct{}, maxConcurrent)

	for i, post := range *posts {
		if post.Image == "" { // 이미지가 없는 포스트만 처리
			wg.Add(1)
			go func(index int, postURL string, title string) {
				defer wg.Done()

				// 세마포어로 동시 요청 수 제한
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				imageURL := t.extractThumbnailFromPage(postURL)
				if imageURL != "" {
					imageChan <- struct {
						index int
						image string
					}{index: index, image: imageURL}
				}
			}(i, post.URL, post.Title)
		}
	}

	// 모든 고루틴이 완료될 때까지 대기
	go func() {
		wg.Wait()
		close(imageChan)
	}()

	// 이미지 URL 업데이트
	for result := range imageChan {
		(*posts)[result.index].Image = result.image
	}
}
