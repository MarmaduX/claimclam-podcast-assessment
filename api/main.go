package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/time/rate"
)

const PODCAST_SERVICE_URL = "https://601f1754b5a0e9001706a292.mockapi.io"

type ipEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	ipLimiters = map[string]*ipEntry{}
	mutex      sync.Mutex
)

func getClientIP(request *http.Request) string {
	if forwardedFor := request.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		return strings.TrimSpace(parts[0])
	}

	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		return request.RemoteAddr
	}
	return host
}

func getRateLimiter(clientIP string) *rate.Limiter {
	mutex.Lock()
	defer mutex.Unlock()

	if entry, exists := ipLimiters[clientIP]; exists {
		entry.lastSeen = time.Now()
		return entry.limiter
	}

	limiter := rate.NewLimiter(5, 10)

	ipLimiters[clientIP] = &ipEntry{
		limiter:  limiter,
		lastSeen: time.Now(),
	}

	return limiter
}

func cleanupLimitersLoop() {
	for {
		time.Sleep(1 * time.Minute)
		mutex.Lock()
		for ip, entry := range ipLimiters {
			if time.Since(entry.lastSeen) > 10*time.Minute {
				delete(ipLimiters, ip)
			}
		}
		mutex.Unlock()
	}
}

func rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {

		clientIP := getClientIP(request)
		limiter := getRateLimiter(clientIP)

		if !limiter.Allow() {
			http.Error(response, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(response, request)
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {

		response.Header().Set("Access-Control-Allow-Origin", "*")
		response.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		response.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if request.Method == "OPTIONS" {
			response.WriteHeader(200)
			return
		}

		next.ServeHTTP(response, request)
	})
}

func handlePodcasts(response http.ResponseWriter, request *http.Request) {
	targetURL, _ := url.Parse(PODCAST_SERVICE_URL + "/podcasts")

	targetURL.RawQuery = request.URL.RawQuery

	log.Println("Forwarding request to:", targetURL.String())

	apiResponse, err := http.Get(targetURL.String())
	if err != nil {
		http.Error(response, "failed to fetch podcasts", http.StatusInternalServerError)
		return
	}
	defer apiResponse.Body.Close()

	body, _ := io.ReadAll(apiResponse.Body)

	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(apiResponse.StatusCode)
	response.Write(body)
}

func handleHealth(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	payload := fmt.Sprintf(`{"status":"ok", "timestamp":"%s"}`, time.Now().UTC().Format(time.RFC3339))
	response.Write([]byte(payload))
}

func main() {
	go cleanupLimitersLoop()
	router := mux.NewRouter()

	router.Handle("/api/podcasts", rateLimitMiddleware(http.HandlerFunc(handlePodcasts))).Methods("GET")
	router.HandleFunc("/api/health", handleHealth).Methods("GET")
	router.Handle("/api/hello", rateLimitMiddleware(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		fmt.Fprintf(response, "Hello World!")
	}))).Methods("GET")

	fmt.Println("API Gateway in 8080...")
	log.Fatal(http.ListenAndServe(":8080", withCORS(router)))
}
