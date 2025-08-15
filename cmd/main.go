package main

import (
	"bytes"
	"context"
	"html/template"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/service/lambda"

	"hello-go/internal/crawlers"
	"hello-go/internal/models"
)

// 전역 상수: 이 날짜 이후의 포스트만 수집
const FILTER_DATE = "2025-01-01"

// S3 설정
const S3_KEY_NAME = "index.html"

// getS3BucketName은 환경변수에서 S3 버킷 이름을 가져옵니다.
func getS3BucketName() string {
	bucketName := os.Getenv("S3_BUCKET")
	if bucketName == "" {
		log.Fatal("S3_BUCKET 환경변수가 설정되지 않았습니다.")
	}
	return bucketName
}

// uploadToS3는 HTML 파일을 S3에 업로드합니다.
func uploadToS3(htmlContent string) error {
	// AWS 설정 로드
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return err
	}

	// S3 클라이언트 생성
	s3Client := s3.NewFromConfig(cfg)

	// HTML 내용을 바이트로 변환
	htmlBytes := []byte(htmlContent)

	// S3 버킷 이름 가져오기
	bucketName := getS3BucketName()

	// S3에 업로드
	_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(S3_KEY_NAME),
		Body:        bytes.NewReader(htmlBytes),
		ContentType: aws.String("text/html"),
		ACL:         "public-read", // 공개 읽기 권한
	})

	if err != nil {
		return err
	}

	log.Printf("✅ HTML 파일이 S3에 업로드되었습니다: s3://%s/%s", bucketName, S3_KEY_NAME)
	return nil
}

// generateHTML은 HTML을 생성하고 S3에 업로드합니다.
func generateHTML(posts []models.BlogPost, blogStats map[string]int) error {
	// 포스트를 최신순으로 정렬 (내림차순)
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].PublishedAt.After(posts[j].PublishedAt)
	})

	// 카테고리 목록 추출
	categorySet := make(map[string]bool)
	blogSet := make(map[string]bool)
	for _, post := range posts {
		categorySet[post.Category] = true
		blogSet[post.Source] = true
	}

	var categoryList []string
	for category := range categorySet {
		categoryList = append(categoryList, category)
	}
	sort.Strings(categoryList)

	var blogList []string
	for blog := range blogSet {
		blogList = append(blogList, blog)
	}
	sort.Strings(blogList)

	// 기본 이미지 (SVG)
	defaultImage := `data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMzAwIiBoZWlnaHQ9IjIwMCIgdmlld0JveD0iMCAwIDMwMCAyMDAiIGZpbGw9Im5vbmUiIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyI+CjxyZWN0IHdpZHRoPSIzMDAiIGhlaWdodD0iMjAwIiBmaWxsPSIjRjNGNEY2Ii8+CjxwYXRoIGQ9Ik0xNTAgMTAwTDE3MCAxMjBMMTUwIDE0MEwxMzAgMTIwTDE1MCAxMDBaIiBmaWxsPSIjN0MzQTVGIi8+Cjx0ZXh0IHg9IjE1MCIgeT0iMTgwIiB0ZXh0LWFuY2hvcj0ibWlkZGxlIiBmaWxsPSIjN0MzQTVGIiBmb250LWZhbWlseT0iQXJpYWwiIGZvbnQtc2l6ZT0iMTQiPuiJvuacn+WbvueJhzwvdGV4dD4KPC9zdmc+`

	// 이미지가 없는 포스트에 기본 이미지 설정
	for i := range posts {
		if posts[i].Image == "" {
			posts[i].Image = defaultImage
		}
	}

	// 최신 포스트 정보 로깅
	if len(posts) > 0 {
		log.Printf("HTML 생성: 최신 포스트는 '%s' (%v)", posts[0].Title, posts[0].PublishedAt)
	}

	// HTML 템플릿
	htmlTemplate := `<!DOCTYPE html>
<html lang="ko">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>개발자들의 이야기 모음집</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background-color: #f8f9fa;
            color: #333;
            line-height: 1.6;
        }

        .container {
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
        }

        .header {
            text-align: center;
            margin-bottom: 40px;
            padding: 40px 0;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            border-radius: 15px;
            box-shadow: 0 10px 30px rgba(0,0,0,0.1);
        }

        .header h1 {
            font-size: 2.5rem;
            margin-bottom: 10px;
            font-weight: 700;
        }

        .header p {
            font-size: 1.1rem;
            opacity: 0.9;
        }

        .stats {
            display: flex;
            justify-content: center;
            gap: 30px;
            margin-bottom: 40px;
            flex-wrap: wrap;
        }

        .stat-item {
            background: white;
            padding: 20px;
            border-radius: 10px;
            box-shadow: 0 5px 15px rgba(0,0,0,0.08);
            text-align: center;
            min-width: 120px;
        }

        .stat-number {
            font-size: 2rem;
            font-weight: bold;
            color: #667eea;
            display: block;
        }

        .stat-label {
            color: #666;
            font-size: 0.9rem;
            margin-top: 5px;
        }

        .filters {
            background: white;
            padding: 30px;
            border-radius: 15px;
            margin-bottom: 40px;
            box-shadow: 0 5px 15px rgba(0,0,0,0.08);
        }

        .filter-section {
            margin-bottom: 25px;
        }

        .filter-section:last-child {
            margin-bottom: 0;
        }

        .filter-section h4 {
            margin-bottom: 15px;
            color: #333;
            font-size: 1.1rem;
        }

        .filter-options {
            display: flex;
            flex-wrap: wrap;
            gap: 10px;
        }

        .filter-option {
            padding: 8px 16px;
            border: 2px solid #e1e5e9;
            border-radius: 25px;
            cursor: pointer;
            transition: all 0.3s ease;
            font-size: 0.9rem;
            background: white;
        }

        .filter-option:hover {
            border-color: #667eea;
            color: #667eea;
        }

        .filter-option.active {
            background: #667eea;
            color: white;
            border-color: #667eea;
        }

        .posts-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(350px, 1fr));
            gap: 25px;
        }

        .post-card {
            background: white;
            border-radius: 15px;
            overflow: hidden;
            box-shadow: 0 5px 15px rgba(0,0,0,0.08);
            transition: all 0.3s ease;
            cursor: pointer;
            position: relative;
        }

        .post-card:hover {
            transform: translateY(-5px);
            box-shadow: 0 15px 35px rgba(0,0,0,0.15);
        }

        .post-card.hidden {
            display: none;
        }

        .post-image {
            width: 100%;
            height: 200px;
            background-size: cover;
            background-position: center;
            background-repeat: no-repeat;
            position: relative;
        }

        .post-content {
            padding: 20px;
            display: flex;
            flex-direction: column;
            height: 200px;
        }

        .post-header {
            margin-bottom: 15px;
        }

        .post-title {
            font-size: 1.1rem;
            font-weight: 600;
            margin-bottom: 10px;
            color: #333;
            line-height: 1.4;
            display: -webkit-box;
            -webkit-line-clamp: 2;
            -webkit-box-orient: vertical;
            overflow: hidden;
        }

        .post-meta {
            display: flex;
            gap: 10px;
            font-size: 0.8rem;
        }

        .post-source {
            background: #667eea;
            color: white;
            padding: 3px 8px;
            border-radius: 12px;
            font-weight: 500;
        }

        .post-category {
            background: #f1f3f4;
            color: #666;
            padding: 3px 8px;
            border-radius: 12px;
        }

        .post-summary {
            color: #666;
            font-size: 0.9rem;
            line-height: 1.5;
            flex-grow: 1;
            display: -webkit-box;
            -webkit-line-clamp: 3;
            -webkit-box-orient: vertical;
            overflow: hidden;
        }

        .post-footer {
            margin-top: auto;
            display: flex;
            justify-content: space-between;
            align-items: center;
            font-size: 0.8rem;
            color: #999;
            padding-top: 15px;
            border-top: 1px solid #f0f0f0;
        }

        .post-author {
            font-weight: 500;
        }

        .post-date {
            color: #999;
        }

        .post-overlay {
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: rgba(102, 126, 234, 0.9);
            display: flex;
            align-items: center;
            justify-content: center;
            opacity: 0;
            transition: opacity 0.3s ease;
        }

        .post-card:hover .post-overlay {
            opacity: 1;
        }

        .read-more {
            color: white;
            font-weight: 600;
            font-size: 1.1rem;
        }

        @media (max-width: 768px) {
            .container {
                padding: 15px;
            }

            .header h1 {
                font-size: 2rem;
            }

            .posts-grid {
                grid-template-columns: 1fr;
            }

            .stats {
                gap: 15px;
            }

            .stat-item {
                min-width: 100px;
                padding: 15px;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>개발자들의 이야기 모음집</h1>
            <p>최신 기술 블로그 포스트들을 한 곳에서 만나보세요</p>
        </div>

        <div class="stats">
            {{range $blog, $count := .BlogStats}}
            <div class="stat-item">
                <span class="stat-number">{{$count}}</span>
                <span class="stat-label">{{$blog}}</span>
            </div>
            {{end}}
        </div>

        <div class="filters">
            <div class="filter-section">
                <h4>블로그</h4>
                <div class="filter-options">
                    {{range .BlogList}}
                    <div class="filter-option" data-type="blog" data-value="{{.}}" onclick="toggleFilter(this)">
                        {{.}}
                    </div>
                    {{end}}
                </div>
            </div>

            <div class="filter-section">
                <h4>카테고리</h4>
                <div class="filter-options">
                    {{range .CategoryList}}
                    <div class="filter-option" data-type="category" data-value="{{.}}" onclick="toggleFilter(this)">
                        {{.}}
                    </div>
                    {{end}}
                </div>
            </div>
        </div>

        <div class="posts-grid">
            {{range .Posts}}
            <div class="post-card" data-source="{{.Source}}" data-category="{{.Category}}" onclick="window.open('{{.URL}}', '_blank')">
                <div class="post-image" style="background-image: url('{{.Image}}')">
                </div>
                <div class="post-content">
                    <div class="post-header">
                        <h3 class="post-title">{{.Title}}</h3>
                        <div class="post-meta">
                            <span class="post-source">{{.Source}}</span>
                            <span class="post-category">{{.Category}}</span>
                        </div>
                    </div>
                    <p class="post-summary">{{.Summary}}</p>
                    
                    <div class="post-footer">
                        <span class="post-author">{{.Author}}</span>
                        <span class="post-date">{{.PublishedAt.Format "2006년 1월 2일"}}</span>
                    </div>
                </div>
                <div class="post-overlay">
                    <div class="read-more">읽어보기</div>
                </div>
            </div>
            {{end}}
        </div>
    </div>

    <script>
        function toggleFilter(element) {
            element.classList.toggle('active');
            updateFilters();
        }

        function updateFilters() {
            const selectedBlogs = Array.from(document.querySelectorAll('.filter-option[data-type="blog"].active'))
                .map(el => el.dataset.value);
            
            const selectedCategories = Array.from(document.querySelectorAll('.filter-option[data-type="category"].active'))
                .map(el => el.dataset.value);

            const posts = document.querySelectorAll('.post-card');
            
            posts.forEach(post => {
                const postSource = post.dataset.source;
                const postCategory = post.dataset.category;
                
                const blogMatch = selectedBlogs.length === 0 || selectedBlogs.includes(postSource);
                const categoryMatch = selectedCategories.length === 0 || selectedCategories.includes(postCategory);
                
                if (blogMatch && categoryMatch) {
                    post.classList.remove('hidden');
                } else {
                    post.classList.add('hidden');
                }
            });
        }
    </script>
</body>
</html>`

	// 템플릿 데이터 구조 생성
	data := struct {
		Posts        []models.BlogPost
		BlogStats    map[string]int
		CategoryList []string
		BlogList     []string
	}{
		Posts:        posts,
		BlogStats:    blogStats,
		CategoryList: categoryList,
		BlogList:     blogList,
	}

	tmpl, err := template.New("blog").Parse(htmlTemplate)
	if err != nil {
		return err
	}

	// HTML 내용을 문자열로 생성
	var htmlBuffer bytes.Buffer
	if err := tmpl.Execute(&htmlBuffer, data); err != nil {
		return err
	}

	htmlContent := htmlBuffer.String()

	// 로컬 파일로도 저장 (디버깅용)
	file, err := os.Create("index.html")
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.WriteString(htmlContent); err != nil {
		return err
	}

	// S3에 업로드
	if err := uploadToS3(htmlContent); err != nil {
		return err
	}

	return nil
}

func handler(ctx context.Context, event events.CloudWatchEvent) {
	log.Println("🚀 개발자들의 이야기 모음집 시작")
	start := time.Now()

	// 모든 크롤러 생성
	tossCrawler := crawlers.NewTossCrawler()
	daangnCrawler := crawlers.NewDaangnCrawler()
	naverCrawler := crawlers.NewNaverCrawler()
	danminCrawler := crawlers.NewDanminCrawler()

	// 병렬 크롤링을 위한 구조체
	type crawlerResult struct {
		posts []models.BlogPost
		err   error
		name  string
	}

	// 결과를 저장할 채널
	resultChan := make(chan crawlerResult, 4)
	var wg sync.WaitGroup

	// Toss 블로그 크롤링 (고루틴)
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("📡 Toss 블로그 크롤링 시작...")
		posts, err := tossCrawler.Crawl()
		resultChan <- crawlerResult{posts: posts, err: err, name: "Toss"}
	}()

	// Daangn 블로그 크롤링 (고루틴)
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("📡 Daangn 블로그 크롤링 시작...")
		posts, err := daangnCrawler.Crawl()
		resultChan <- crawlerResult{posts: posts, err: err, name: "Daangn"}
	}()

	// Naver D2 블로그 크롤링 (고루틴)
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("📡 Naver D2 블로그 크롤링 시작...")
		posts, err := naverCrawler.Crawl()
		resultChan <- crawlerResult{posts: posts, err: err, name: "Naver D2"}
	}()

	// 단민 블로그 크롤링 (고루틴)
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("📡 단민 블로그 크롤링 시작...")
		posts, err := danminCrawler.Crawl()
		resultChan <- crawlerResult{posts: posts, err: err, name: "단민"}
	}()

	// 모든 고루틴이 완료될 때까지 대기
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 결과 수집
	var allPosts []models.BlogPost
	for result := range resultChan {
		if result.err != nil {
			log.Printf("%s 크롤링 실패: %v", result.name, result.err)
		} else {
			allPosts = append(allPosts, result.posts...)
			log.Printf("✅ %s 크롤링 완료: %d개 포스트", result.name, len(result.posts))
		}
	}

	if len(allPosts) == 0 {
		log.Println("⚠️  크롤링된 포스트가 없습니다.")
		return
	}

	// 블로그별 통계 계산
	blogStats := make(map[string]int)
	for _, post := range allPosts {
		blogStats[post.Source]++
	}

	// 필터 날짜 파싱
	filterTime, err := time.Parse("2006-01-02", FILTER_DATE)
	if err != nil {
		log.Fatalf("필터 날짜 파싱 실패: %v", err)
	}

	// 지정된 날짜 이후의 포스트만 필터링
	var filteredPosts []models.BlogPost
	for _, post := range allPosts {
		if post.PublishedAt.After(filterTime) || post.PublishedAt.Equal(filterTime) {
			filteredPosts = append(filteredPosts, post)
		}
	}

	// 중복 제거 (제목 기준)
	var uniquePosts []models.BlogPost
	seenTitles := make(map[string]bool)
	duplicateCount := 0

	for _, post := range filteredPosts {
		title := strings.TrimSpace(post.Title)
		if !seenTitles[title] {
			seenTitles[title] = true
			uniquePosts = append(uniquePosts, post)
		} else {
			duplicateCount++
			log.Printf("중복 제거: %s", title)
		}
	}

	log.Printf("중복 제거 완료: %d개 중복 제거됨 (필터링 후: %d개 -> 중복 제거 후: %d개)",
		duplicateCount, len(filteredPosts), len(uniquePosts))

	// 중복 제거된 포스트로 통계 재계산
	blogStats = make(map[string]int)
	for _, post := range uniquePosts {
		blogStats[post.Source]++
	}

	log.Printf("✅ 전체 크롤링 완료: %d개 포스트 (필터링 후: %d개, 중복 제거 후: %d개)",
		len(allPosts), len(filteredPosts), len(uniquePosts))
	log.Printf("📅 필터 기준: %s 이후", FILTER_DATE)
	for blog, count := range blogStats {
		log.Printf("📊 %s: %d개", blog, count)
	}

	// HTML 파일 생성 및 S3 업로드
	if err := generateHTML(uniquePosts, blogStats); err != nil {
		log.Fatalf("HTML 생성 및 S3 업로드 실패: %v", err)
	}

	duration := time.Since(start)
	log.Printf("🎉 완료! 총 소요시간: %v", duration)
	log.Printf("📊 총 포스트 수: %d개", len(allPosts))
	log.Printf("📁 생성된 파일: index.html")
}

func main() {
	lambda.Start(handler)
}
