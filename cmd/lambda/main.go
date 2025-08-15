package main

import (
	"bytes"
	"context"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"hello-go/internal"
	"hello-go/internal/crawlers"
)

// getS3BucketName은 환경변수에서 S3 버킷 이름을 가져옵니다.
func getS3BucketName() string {
	bucketName := os.Getenv("S3_BUCKET")
	if bucketName == "" {
		log.Fatal("S3_BUCKET 환경변수가 설정되지 않았습니다.")
	}
	return bucketName
}

func getFilterDate() string {
	filterDate := os.Getenv("FILTER_DATE")
	if filterDate == "" {
		return "2025-01-01"
	}
	return filterDate
}

// uploadToS3는 HTML 파일을 S3에 업로드합니다.
func uploadToS3(htmlContent string) {
	// AWS 설정 로드
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Printf("AWS 설정 로드 실패: %v", err)
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
		Key:         aws.String("index.html"),
		Body:        bytes.NewReader(htmlBytes),
		ContentType: aws.String("text/html"),
	})

	if err != nil {
		log.Printf("S3 업로드 실패: %v", err)
		return
	}

	log.Printf("✅ HTML 파일이 S3에 업로드되었습니다: s3://%s/%s", bucketName, "index.html")
}

func main() {
	filterDate, err := time.Parse("2006-01-02", getFilterDate())
	if err != nil {
		log.Fatalf("날짜 파싱 실패: %v", err)
	}

	lambda.Start(func() {
		internal.Crawl(
			getFilterDate(),
			uploadToS3,
			crawlers.NewTossCrawler(filterDate),
			crawlers.NewDaangnCrawler(),
			crawlers.NewDanminCrawler(),
			crawlers.NewNaverCrawler(),
		)
	})
}
