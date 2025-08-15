package generators

import (
	"fmt"
	"html/template"
	"os"
	"sort"
	"time"

	"hello-go/internal/models"
)

// HTMLGenerator는 블로그 포스트 데이터를 HTML로 변환하는 제너레이터입니다.
type HTMLGenerator struct {
	template *template.Template
}

// NewHTMLGenerator는 새로운 HTMLGenerator 인스턴스를 생성합니다.
func NewHTMLGenerator() *HTMLGenerator {
	return &HTMLGenerator{
		template: template.Must(template.New("blog").Parse(blogTemplate)),
	}
}

// GenerateHTML은 블로그 포스트 데이터를 HTML 파일로 생성합니다.
func (g *HTMLGenerator) GenerateHTML(posts []models.BlogPost, outputPath string) error {
	// 날짜순으로 정렬 (최신순)
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].PublishedAt.After(posts[j].PublishedAt)
	})

	// 카테고리별로 그룹화
	categories := g.groupByCategory(posts)

	// 소스별로 그룹화
	sources := g.groupBySource(posts)

	data := struct {
		Posts       []models.BlogPost
		Categories  map[string][]models.BlogPost
		Sources     map[string][]models.BlogPost
		TotalCount  int
		GeneratedAt time.Time
	}{
		Posts:       posts,
		Categories:  categories,
		Sources:     sources,
		TotalCount:  len(posts),
		GeneratedAt: time.Now(),
	}

	// HTML 파일 생성
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("HTML 파일 생성 실패: %w", err)
	}
	defer file.Close()

	if err := g.template.Execute(file, data); err != nil {
		return fmt.Errorf("HTML 템플릿 실행 실패: %w", err)
	}

	return nil
}

// groupByCategory는 포스트를 카테고리별로 그룹화합니다.
func (g *HTMLGenerator) groupByCategory(posts []models.BlogPost) map[string][]models.BlogPost {
	categories := make(map[string][]models.BlogPost)

	for _, post := range posts {
		categories[post.Category] = append(categories[post.Category], post)
	}

	return categories
}

// groupBySource는 포스트를 소스별로 그룹화합니다.
func (g *HTMLGenerator) groupBySource(posts []models.BlogPost) map[string][]models.BlogPost {
	sources := make(map[string][]models.BlogPost)

	for _, post := range posts {
		sources[post.Source] = append(sources[post.Source], post)
	}

	return sources
}

// blogTemplate은 블로그 SPA 사이트의 HTML 템플릿입니다.
const blogTemplate = `<!DOCTYPE html>
<html lang="ko">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>최신 기술 모음집</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }
        
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            line-height: 1.6;
            color: #333;
            background-color: #f8f9fa;
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
            border-radius: 12px;
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
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 20px;
            margin-bottom: 40px;
        }
        
        .stat-card {
            background: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
            text-align: center;
        }
        
        .stat-number {
            font-size: 2rem;
            font-weight: 700;
            color: #667eea;
            margin-bottom: 5px;
        }
        
        .stat-label {
            color: #666;
            font-size: 0.9rem;
        }
        
        .filters {
            display: flex;
            gap: 10px;
            margin-bottom: 30px;
            flex-wrap: wrap;
            justify-content: center;
        }
        
        .filter-btn {
            padding: 8px 16px;
            border: 2px solid #667eea;
            background: white;
            color: #667eea;
            border-radius: 20px;
            cursor: pointer;
            transition: all 0.3s ease;
            font-size: 0.9rem;
        }
        
        .filter-btn:hover, .filter-btn.active {
            background: #667eea;
            color: white;
        }
        
        .posts-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(350px, 1fr));
            gap: 20px;
            margin-bottom: 40px;
        }
        
        .post-card {
            background: white;
            border-radius: 12px;
            overflow: hidden;
            box-shadow: 0 4px 15px rgba(0,0,0,0.1);
            transition: transform 0.3s ease, box-shadow 0.3s ease;
            cursor: pointer;
            position: relative;
        }
        
        .post-card:hover {
            transform: translateY(-5px);
            box-shadow: 0 8px 25px rgba(0,0,0,0.15);
        }
        
        .post-card::before {
            content: '';
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: rgba(102, 126, 234, 0.05);
            opacity: 0;
            transition: opacity 0.3s ease;
            pointer-events: none;
        }
        
        .post-card:hover::before {
            opacity: 1;
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
            padding: 8px 16px;
            border-radius: 20px;
            font-size: 14px;
            font-weight: 500;
            box-shadow: 0 4px 15px rgba(102, 126, 234, 0.3);
        }
        
        .post-header {
            padding: 20px;
            border-bottom: 1px solid #eee;
        }
        
        .post-title {
            font-size: 1.2rem;
            font-weight: 600;
            margin-bottom: 10px;
            line-height: 1.4;
        }
        
        .post-title a {
            color: #333;
            text-decoration: none;
        }
        
        .post-title a:hover {
            color: #667eea;
        }
        
        .post-meta {
            display: flex;
            justify-content: space-between;
            align-items: center;
            font-size: 0.85rem;
            color: #666;
        }
        
        .post-source {
            background: #667eea;
            color: white;
            padding: 4px 8px;
            border-radius: 12px;
            font-size: 0.75rem;
            font-weight: 500;
        }
        
        .post-category {
            background: #f0f0f0;
            color: #666;
            padding: 4px 8px;
            border-radius: 12px;
            font-size: 0.75rem;
        }
        
        .post-body {
            padding: 20px;
        }
        
        .post-summary {
            color: #666;
            line-height: 1.6;
            margin-bottom: 15px;
        }
        
        .post-date {
            font-size: 0.85rem;
            color: #999;
        }
        
        .footer {
            text-align: center;
            padding: 40px 0;
            color: #666;
            border-top: 1px solid #eee;
            margin-top: 40px;
        }
        
        .hidden {
            display: none;
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
            
            .filters {
                justify-content: flex-start;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🚀 최신 기술 모음집</h1>
            <p>토스, 당근마켓, 네이버, 카카오의 최신 기술 포스트를 한눈에</p>
        </div>
        
        <div class="stats">
            <div class="stat-card">
                <div class="stat-number">{{.TotalCount}}</div>
                <div class="stat-label">총 포스트</div>
            </div>
            <div class="stat-card">
                <div class="stat-number">{{len .Categories}}</div>
                <div class="stat-label">카테고리</div>
            </div>
            <div class="stat-card">
                <div class="stat-number">{{len .Sources}}</div>
                <div class="stat-label">블로그 소스</div>
            </div>
            <div class="stat-card">
                <div class="stat-number">{{.GeneratedAt.Format "15:04"}}</div>
                <div class="stat-label">최근 업데이트</div>
            </div>
        </div>
        
        <div class="filters">
            <button class="filter-btn active" data-filter="all">전체</button>
            {{range $source, $posts := .Sources}}
            <button class="filter-btn" data-filter="source-{{$source}}">{{$source}}</button>
            {{end}}
            {{range $category, $posts := .Categories}}
            <button class="filter-btn" data-filter="category-{{$category}}">{{$category}}</button>
            {{end}}
        </div>
        
        <div class="posts-grid">
            {{range .Posts}}
            <div class="post-card" data-source="{{.Source}}" data-category="{{.Category}}" data-url="{{.URL}}" onclick="openPost('{{.URL}}')">
                <div class="post-header">
                    <h3 class="post-title">
                        {{.Title}}
                    </h3>
                    <div class="post-meta">
                        <span class="post-source">{{.Source}}</span>
                        <span class="post-category">{{.Category}}</span>
                    </div>
                </div>
                <div class="post-body">
                    <p class="post-summary">{{.Summary}}</p>
                    <div class="post-date">{{.PublishedAt.Format "2006년 1월 2일"}}</div>
                </div>
                <div class="post-overlay">
                    <span class="read-more">읽어보기 →</span>
                </div>
            </div>
            {{end}}
        </div>
        
        <div class="footer">
            <p>생성일: {{.GeneratedAt.Format "2006년 1월 2일 15:04:05"}}</p>
            <p>개발 기술 관련 컨텐츠만 필터링하여 표시됩니다.</p>
        </div>
    </div>
    
    <script>
        // 포스트 열기 함수
        function openPost(url) {
            if (url) {
                window.open(url, '_blank', 'noopener,noreferrer');
            }
        }
        
        // 필터링 기능
        document.addEventListener('DOMContentLoaded', function() {
            const filterBtns = document.querySelectorAll('.filter-btn');
            const postCards = document.querySelectorAll('.post-card');
            
            filterBtns.forEach(btn => {
                btn.addEventListener('click', function() {
                    const filter = this.dataset.filter;
                    
                    // 활성 버튼 표시
                    filterBtns.forEach(b => b.classList.remove('active'));
                    this.classList.add('active');
                    
                    // 포스트 필터링
                    postCards.forEach(card => {
                        if (filter === 'all') {
                            card.classList.remove('hidden');
                        } else if (filter.startsWith('source-')) {
                            const source = filter.replace('source-', '');
                            if (card.dataset.source === source) {
                                card.classList.remove('hidden');
                            } else {
                                card.classList.add('hidden');
                            }
                        } else if (filter.startsWith('category-')) {
                            const category = filter.replace('category-', '');
                            if (card.dataset.category === category) {
                                card.classList.remove('hidden');
                            } else {
                                card.classList.add('hidden');
                            }
                        }
                    });
                });
            });
        });
    </script>
</body>
</html>`
