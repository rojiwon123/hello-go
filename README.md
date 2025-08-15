# 🚀 최신 기술 모음집

토스, 당근마켓, 네이버, 카카오의 기술 블로그를 병렬로 크롤링하여 하나의 SPA 사이트로 만들어주는 Go 프로그램입니다.

## ✨ 주요 기능

- **병렬 크롤링**: 고루틴을 사용하여 4개 블로그를 동시에 크롤링
- **스마트 필터링**: 개발 기술 관련 컨텐츠만 자동 필터링
- **최신성 보장**: 1년 이내 작성된 포스트만 수집
- **아름다운 UI**: 모던하고 반응형인 SPA 인터페이스
- **실시간 필터링**: 소스별, 카테고리별 실시간 포스트 필터링

## 🎯 지원 블로그

- **토스**: https://toss.tech/tech
- **당근마켓**: https://about.daangn.com/blog/category/career/
- **네이버**: https://d2.naver.com/home
- **카카오**: https://tech.kakao.com/blog

## 🛠️ 기술 스택

- **Backend**: Go 1.24+
- **HTML Parsing**: goquery
- **Concurrency**: Goroutines & Channels
- **Frontend**: Vanilla JavaScript + CSS Grid
- **Responsive Design**: Mobile-first 접근법

## 📦 설치 및 실행

### 1. 의존성 설치
```bash
go mod tidy
```

### 2. 프로그램 빌드
```bash
go build -o blog-aggregator cmd/main.go
```

### 3. 실행
```bash
# 기본 실행 (blog.html 파일 생성)
./blog-aggregator

# 커스텀 출력 파일명으로 실행
./blog-aggregator -output my-blog.html
```

## 🔧 사용법

### 명령행 옵션
- `-output`: 생성할 HTML 파일 경로 (기본값: blog.html)

### 실행 예시
```bash
# 기본 실행
./blog-aggregator

# 커스텀 파일명으로 실행
./blog-aggregator -output tech-blogs.html

# 도움말 보기
./blog-aggregator -h
```

## 📊 출력 결과

프로그램 실행 후 생성되는 HTML 파일은 다음과 같은 기능을 제공합니다:

- **통계 대시보드**: 총 포스트 수, 카테고리 수, 소스 수, 최근 업데이트 시간
- **필터링**: 블로그 소스별, 카테고리별 실시간 필터링
- **반응형 그리드**: 모바일과 데스크톱에 최적화된 카드 레이아웃
- **직접 링크**: 각 포스트의 원본 블로그로 바로 이동

## 🏗️ 프로젝트 구조

```
hello-go/
├── cmd/
│   └── main.go              # 메인 프로그램 진입점
├── internal/
│   ├── models/              # 데이터 모델 정의
│   │   └── blog.go
│   ├── crawlers/            # 블로그별 크롤러
│   │   ├── manager.go       # 크롤러 매니저
│   │   ├── toss_crawler.go  # 토스 크롤러
│   │   ├── daangn_crawler.go # 당근마켓 크롤러
│   │   ├── naver_crawler.go # 네이버 크롤러
│   │   └── kakao_crawler.go # 카카오 크롤러
│   ├── filters/             # 컨텐츠 필터링
│   │   └── tech_filter.go   # 기술 컨텐츠 필터
│   └── generators/          # HTML 생성
│       └── html_generator.go # HTML 제너레이터
├── go.mod                   # Go 모듈 정의
├── go.sum                   # 의존성 체크섬
└── README.md                # 프로젝트 문서
```

## 🔍 크롤링 로직

### 1. 병렬 크롤링
- 각 블로그 크롤러가 독립적인 고루틴에서 실행
- `sync.WaitGroup`을 사용하여 모든 크롤러 완료 대기
- `sync.Mutex`로 스레드 안전한 결과 수집

### 2. 스마트 필터링
- **기술 키워드**: 개발, 프로그래밍, 코딩, 소프트웨어, 엔지니어링 등
- **날짜 필터**: 1년 이내 작성된 포스트만 수집
- **컨텐츠 분석**: 제목, 카테고리, 요약에서 키워드 매칭

### 3. 에러 처리
- 개별 크롤러 실패 시에도 다른 크롤러는 계속 실행
- 상세한 로깅으로 디버깅 정보 제공
- 타임아웃 설정으로 무한 대기 방지

## 🎨 UI/UX 특징

- **모던 디자인**: 그라디언트 헤더와 그림자 효과
- **반응형 레이아웃**: CSS Grid를 활용한 적응형 카드 배치
- **인터랙티브 필터**: 클릭으로 실시간 포스트 필터링
- **호버 효과**: 카드 호버 시 부드러운 애니메이션
- **모바일 최적화**: 터치 친화적인 인터페이스

## 🚀 성능 최적화

- **병렬 처리**: 4개 블로그 동시 크롤링으로 시간 단축
- **메모리 효율성**: 스트리밍 방식의 HTML 파싱
- **타임아웃 관리**: 30초 타임아웃으로 응답 없는 블로그 처리
- **에러 격리**: 개별 크롤러 실패가 전체 프로세스에 영향 없음

## 🔒 주의사항

- **크롤링 정책**: 각 블로그의 robots.txt 및 이용약관 준수
- **요청 빈도**: 과도한 요청으로 서버에 부하를 주지 않도록 주의
- **데이터 사용**: 개인정보나 저작권이 있는 컨텐츠는 수집하지 않음
- **에러 처리**: 네트워크 오류나 HTML 구조 변경 시 크롤링 실패 가능

## 🤝 기여하기

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## 📄 라이선스

이 프로젝트는 MIT 라이선스 하에 배포됩니다.

## 🙏 감사의 말

- [goquery](https://github.com/PuerkitoBio/goquery) - HTML 파싱 라이브러리
- 각 기술 블로그 팀들의 훌륭한 컨텐츠
- Go 언어의 강력한 동시성 기능

---

**Made with ❤️ in Go**
