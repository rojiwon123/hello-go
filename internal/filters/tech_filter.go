package filters

import (
	"strings"
	"time"

	"hello-go/internal/models"
)

// TechFilter는 개발 기술 관련 컨텐츠와 30일 이내 작성된 포스트만 필터링합니다.
type TechFilter struct {
	techKeywords []string
	daysLimit    int
}

// NewTechFilter는 새로운 TechFilter 인스턴스를 생성합니다.
func NewTechFilter() *TechFilter {
	return &TechFilter{
		techKeywords: []string{
			"개발", "프로그래밍", "코딩", "소프트웨어", "엔지니어링",
			"프론트엔드", "백엔드", "풀스택", "데이터베이스", "API",
			"클라우드", "DevOps", "CI/CD", "테스트", "리팩토링",
			"아키텍처", "마이크로서비스", "모니터링", "로깅", "보안",
			"성능", "최적화", "스케일링", "컨테이너", "쿠버네티스",
			"머신러닝", "AI", "데이터", "분석", "알고리즘",
			"자료구조", "디자인패턴", "클린코드", "리팩토링", "TDD",
			"BDD", "DDD", "SOLID", "DRY", "KISS",
			"React", "Vue", "Angular", "Node.js", "Go",
			"Python", "Java", "JavaScript", "TypeScript", "Rust",
			"Kotlin", "Swift", "Docker", "AWS", "GCP", "Azure",
		},
		daysLimit: 365, // 30일에서 1년(365일)으로 변경
	}
}

// Filter는 개발 기술 관련 컨텐츠와 30일 이내 작성된 포스트만 필터링합니다.
func (f *TechFilter) Filter(posts []models.BlogPost) []models.BlogPost {
	var filtered []models.BlogPost
	cutoffDate := time.Now().AddDate(0, 0, -f.daysLimit)

	for _, post := range posts {
		// 30일 이내 작성된 포스트인지 확인
		if post.PublishedAt.Before(cutoffDate) {
			continue
		}

		// 개발 기술 관련 컨텐츠인지 확인
		if f.isTechRelated(post) {
			filtered = append(filtered, post)
		}
	}

	return filtered
}

// isTechRelated는 포스트가 개발 기술 관련 컨텐츠인지 확인합니다.
func (f *TechFilter) isTechRelated(post models.BlogPost) bool {
	// 제목에서 키워드 확인
	title := strings.ToLower(post.Title)
	for _, keyword := range f.techKeywords {
		if strings.Contains(title, strings.ToLower(keyword)) {
			return true
		}
	}

	// 카테고리에서 키워드 확인
	category := strings.ToLower(post.Category)
	for _, keyword := range f.techKeywords {
		if strings.Contains(category, strings.ToLower(keyword)) {
			return true
		}
	}

	// 요약에서 키워드 확인
	summary := strings.ToLower(post.Summary)
	for _, keyword := range f.techKeywords {
		if strings.Contains(summary, strings.ToLower(keyword)) {
			return true
		}
	}

	return false
}
