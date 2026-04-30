package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	// 설정
	port := "8097"
	rootDir := "/Volumes"
	
	// 로그 기록 설정
	logFile := "/tmp/mistvault_storage.log"
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	log.SetOutput(f)

	// 정적 파일 서버 (기본적인 디렉토리 리스팅 제공)
	fileServer := http.FileServer(http.Dir(rootDir))

	// 미들웨어: 로깅 및 헤더 설정
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// 기본 보안/편의 헤더
		w.Header().Set("X-Powered-By", "MistVault-M4")
		
		// 요청 처리
		fileServer.ServeHTTP(w, r)
		
		log.Printf("[%s] %s %s %s", r.RemoteAddr, r.Method, r.URL.Path, time.Since(start))
	})

	fmt.Printf("👻 MistVault Storage Server starting on port %s...\n", port)
	fmt.Printf("📂 Serving: %s\n", rootDir)
	fmt.Println("🚀 M4 optimized native build.")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
