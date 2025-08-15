package main

import (
	"html/template"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"hello-go/internal/crawlers"
	"hello-go/internal/models"
)

// 전역 상수: 이 날짜 이후의 포스트만 수집
const FILTER_DATE = "2025-01-01"

func main() {
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

	// HTML 파일 생성
	if err := generateHTML(uniquePosts, blogStats); err != nil {
		log.Fatalf("HTML 생성 실패: %v", err)
	}

	duration := time.Since(start)
	log.Printf("🎉 완료! 총 소요시간: %v", duration)
	log.Printf("📊 총 포스트 수: %d개", len(allPosts))
	log.Printf("📁 생성된 파일: index.html")
}

func generateHTML(posts []models.BlogPost, blogStats map[string]int) error {
	// 포스트를 최신순으로 정렬 (내림차순)
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].PublishedAt.After(posts[j].PublishedAt)
	})

	log.Printf("HTML 생성: 최신 포스트는 '%s' (%v)", posts[0].Title, posts[0].PublishedAt)

	// 카테고리와 블로그 목록 추출 및 이미지 처리
	categories := make(map[string]bool)
	blogs := make(map[string]bool)
	defaultImage := "data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMzUwIiBoZWlnaHQ9IjIwMCIgdmlld0JveD0iMCAwIDM1MCAyMDAiIGZpbGw9Im5vbmUiIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyI+CjxyZWN0IHdpZHRoPSIzNTAiIGhlaWdodD0iMjAwIiBmaWxsPSJ1cmwoI2dyYWRpZW50KSIvPgo8ZGVmcz4KPGxpbmVhckdyYWRpZW50IGlkPSJncmFkaWVudCIgeDE9IjAiIHkxPSIwIiB4Mj0iMzUwIiB5Mj0iMjAwIiBncmFkaWVudFVuaXRzPSJ1c2VyU3BhY2VPblVzZSI+CjxzdG9wIHN0b3AtY29sb3I9IiM2NjdFRUEiLz4KPHN0b3Agb2Zmc2V0PSIxIiBzdG9wLWNvbG9yPSIjNzY0YmE2Ii8+CjwvbGluZWFyR3JhZGllbnQ+CjwvZGVmcz4KPHN2ZyB4PSIxNzUiIHk9IjEwMCIgd2lkdGg9IjgwIiBoZWlnaHQ9IjgwIiB2aWV3Qm94PSIwIDAgMjQgMjQiIGZpbGw9IndoaXRlIiB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciPgo8cGF0aCBkPSJNMTkgM0g1Yy0xLjEgMC0yIC45LTIgMnYxNGMwIDEuMS45IDIgMiAyaDE0YzEuMSAwIDItLjkgMi0yVjVjMC0xLjEtLjktMi0yLTJ6bTAgMTZINVY1aDE0djE0eiIvPgo8cGF0aCBkPSJNMTQgMTJoLTJ2LTJoMnYyem0tMiAyaDJ2LTJoLTJ2MnoiLz4KPC9zdmc+Cjwvc3ZnPgo="

	for i := range posts {
		// 이미지가 없는 경우 기본 이미지 설정
		if posts[i].Image == "" {
			posts[i].Image = defaultImage
		}

		if posts[i].Category != "" {
			categories[posts[i].Category] = true
		}
		blogs[posts[i].Source] = true
	}

	// 카테고리와 블로그 목록을 슬라이스로 변환
	var categoryList []string
	var blogList []string
	for category := range categories {
		categoryList = append(categoryList, category)
	}
	for blog := range blogs {
		blogList = append(blogList, blog)
	}
	sort.Strings(categoryList)
	sort.Strings(blogList)

	const htmlTemplate = `<!DOCTYPE html>
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
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 20px;
        }

        .container {
            max-width: 1200px;
            margin: 0 auto;
        }

        .header {
            text-align: center;
            color: white;
            margin-bottom: 40px;
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
            background: rgba(255, 255, 255, 0.1);
            backdrop-filter: blur(10px);
            border-radius: 15px;
            padding: 20px;
            margin-bottom: 30px;
            color: white;
            text-align: center;
        }

        .stats h2 {
            font-size: 1.5rem;
            margin-bottom: 15px;
        }

        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 15px;
            margin-top: 15px;
        }

        .stat-item {
            background: rgba(255, 255, 255, 0.1);
            padding: 15px;
            border-radius: 10px;
            text-align: center;
        }

        .stat-number {
            font-size: 2rem;
            font-weight: bold;
            margin-bottom: 5px;
        }

        .stat-label {
            font-size: 0.9rem;
            opacity: 0.8;
        }

        .filters {
            background: rgba(255, 255, 255, 0.1);
            backdrop-filter: blur(10px);
            border-radius: 15px;
            padding: 25px;
            margin-bottom: 30px;
            color: white;
        }

        .filters h3 {
            font-size: 1.3rem;
            margin-bottom: 15px;
            text-align: center;
        }

        .filter-section {
            margin-bottom: 20px;
        }

        .filter-section h4 {
            font-size: 1.1rem;
            margin-bottom: 10px;
            color: #f0f0f0;
        }

        .filter-options {
            display: flex;
            flex-wrap: wrap;
            gap: 10px;
        }

        .filter-option {
            background: rgba(255, 255, 255, 0.1);
            border: 2px solid rgba(255, 255, 255, 0.2);
            border-radius: 25px;
            padding: 10px 20px;
            cursor: pointer;
            transition: all 0.3s ease;
            font-size: 0.9rem;
            color: white;
            user-select: none;
            display: inline-block;
            margin: 5px;
        }

        .filter-option:hover {
            background: rgba(255, 255, 255, 0.2);
            border-color: rgba(255, 255, 255, 0.4);
            transform: translateY(-2px);
        }

        .filter-option.active {
            background: rgba(255, 255, 255, 0.3);
            border-color: #667eea;
            box-shadow: 0 4px 15px rgba(102, 126, 234, 0.3);
            transform: translateY(-2px);
        }

        .filter-option.active:hover {
            border-color: #5a6fd8;
            box-shadow: 0 6px 20px rgba(102, 126, 234, 0.4);
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
            box-shadow: 0 10px 30px rgba(0, 0, 0, 0.1);
            transition: all 0.3s ease;
            cursor: pointer;
            position: relative;
            display: flex;
            flex-direction: column;
            height: 450px;
        }

        .post-card:hover {
            transform: translateY(-5px);
            box-shadow: 0 20px 40px rgba(0, 0, 0, 0.15);
        }

        .post-image {
            width: 100%;
            height: 200px;
            background-size: cover;
            background-position: center;
            background-repeat: no-repeat;
            background-color: #f8f9fa;
            flex-shrink: 0;
        }

        .post-content {
            display: flex;
            flex-direction: column;
            flex: 1;
            padding: 20px 25px;
        }

        .post-header {
            margin-bottom: 15px;
        }

        .post-title {
            font-size: 1.1rem;
            font-weight: 600;
            color: #333;
            line-height: 1.3;
            margin-bottom: 8px;
            display: -webkit-box;
            -webkit-line-clamp: 2;
            -webkit-box-orient: vertical;
            overflow: hidden;
            max-height: 2.6em;
        }

        .post-meta {
            display: flex;
            gap: 8px;
            flex-wrap: wrap;
            margin-bottom: 12px;
        }

        .post-source {
            background: #667eea;
            color: white;
            padding: 3px 10px;
            border-radius: 15px;
            font-size: 0.75rem;
            font-weight: 500;
        }

        .post-category {
            background: #f8f9fa;
            color: #666;
            padding: 3px 10px;
            border-radius: 15px;
            font-size: 0.75rem;
            font-weight: 500;
        }

        .post-summary {
            color: #666;
            line-height: 1.5;
            display: -webkit-box;
            -webkit-line-clamp: 2;
            -webkit-box-orient: vertical;
            overflow: hidden;
            flex: 1;
            margin-bottom: 12px;
            font-size: 0.9rem;
            max-height: 3em;
        }

        .post-footer {
            display: flex;
            justify-content: space-between;
            align-items: center;
            font-size: 0.8rem;
            color: #999;
            padding-top: 12px;
            border-top: 1px solid #f0f0f0;
            margin-top: auto;
            min-height: 20px;
        }

        .post-author {
            font-weight: 500;
            max-width: 60%;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }

        .post-date {
            color: #bbb;
            flex-shrink: 0;
        }

        .post-overlay {
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: rgba(102, 126, 234, 0.1);
            display: flex;
            align-items: center;
            justify-content: center;
            opacity: 0;
            transition: opacity 0.3s ease;
            pointer-events: none;
        }

        .post-card:hover .post-overlay {
            opacity: 1;
        }

        .read-more {
            background: #667eea;
            color: white;
            padding: 10px 20px;
            border-radius: 25px;
            font-size: 0.9rem;
            font-weight: 500;
            box-shadow: 0 5px 15px rgba(102, 126, 234, 0.3);
        }

        .filters {
            background: rgba(255, 255, 255, 0.1);
            backdrop-filter: blur(10px);
            border-radius: 15px;
            padding: 20px;
            margin-bottom: 30px;
            color: white;
        }

        .filters h3 {
            margin-bottom: 15px;
            font-size: 1.2rem;
        }

        .filter-buttons {
            display: flex;
            gap: 10px;
            flex-wrap: wrap;
        }

        .filter-btn {
            background: rgba(255, 255, 255, 0.2);
            color: white;
            border: none;
            padding: 8px 16px;
            border-radius: 20px;
            cursor: pointer;
            transition: all 0.3s ease;
            font-size: 0.9rem;
        }

        .filter-btn:hover,
        .filter-btn.active {
            background: rgba(255, 255, 255, 0.3);
            transform: translateY(-2px);
        }

        .no-image {
            position: absolute;
            top: 50%;
            left: 50%;
            transform: translate(-50%, -50%);
            text-align: center;
            color: white;
            opacity: 0.9;
            z-index: 1;
        }
        
        .no-image-icon {
            font-size: 2.5rem;
            margin-bottom: 8px;
            filter: drop-shadow(0 2px 4px rgba(0,0,0,0.3));
            display: block;
        }
        
        .no-image-text {
            font-size: 0.9rem;
            font-weight: 500;
            text-shadow: 0 1px 2px rgba(0,0,0,0.5);
            display: block;
        }

        .hidden {
            display: none !important;
        }

        @media (max-width: 768px) {
            .posts-grid {
                grid-template-columns: 1fr;
            }
            
            .header h1 {
                font-size: 2rem;
            }
            
            .container {
                padding: 10px;
            }

            .filter-options {
                flex-direction: column;
            }

            .stats-grid {
                grid-template-columns: 1fr;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>개발자들의 이야기 모음집</h1>
            <p>최신 기술 블로그 포스트들을 한 곳에서 확인하세요</p>
        </div>

        <div class="stats">
            <h2>📊 집계 결과</h2>
            <div class="stats-grid">
                <div class="stat-item">
                    <div class="stat-number">{{len .Posts}}</div>
                    <div class="stat-label">총 포스트</div>
                </div>
                {{range $blog, $count := .BlogStats}}
                <div class="stat-item">
                    <div class="stat-number">{{$count}}</div>
                    <div class="stat-label">{{$blog}}</div>
                </div>
                {{end}}
            </div>
        </div>

        <div class="filters">
            <h3>🔍 필터</h3>
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

	file, err := os.Create("index.html")
	if err != nil {
		return err
	}
	defer file.Close()

	return tmpl.Execute(file, data)
}
