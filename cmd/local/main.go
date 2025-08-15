package main

import (
	"log"
	"os"
	"time"

	"hello-go/internal"
	"hello-go/internal/crawlers"
)

func writeFile(html string) {
	// 로컬 파일로도 저장 (디버깅용)
	file, err := os.Create("index.html")
	if err != nil {
		log.Fatalf("파일 생성 실패: %v", err)
	}
	defer file.Close()
	if _, err := file.WriteString(html); err != nil {
		log.Fatalf("파일 쓰기 실패: %v", err)
	}
}

func main() {
	filterDate, err := time.Parse("2006-01-02", "2025-01-01")
	if err != nil {
		log.Fatalf("날짜 파싱 실패: %v", err)
	}

	internal.Crawl(
		"2025-01-01",
		writeFile,
		crawlers.NewTossCrawler(filterDate),
		crawlers.NewDaangnCrawler(),
		crawlers.NewDanminCrawler(),
		crawlers.NewNaverCrawler(),
	)
}
