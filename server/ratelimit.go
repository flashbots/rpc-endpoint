package server

// import (
// 	"log"
// 	"net"
// 	"net/http"
// 	"os"
// 	"sync"
// 	"time"

// 	"golang.org/x/time/rate"
// )

// const maxRequestsPerSecAvg = 10
// const maxRequestsPerSecBurst = 20

// var logMissingForwardedForHeader = os.Getenv("LOG_MISSING_FORWARDED_FOR") == "1"

// // Create a custom visitor struct which holds the rate limiter for each
// // visitor and the last time that the visitor was seen.
// type visitor struct {
// 	limiter  *rate.Limiter
// 	lastSeen time.Time
// }

// // Visitor by IP map
// var visitors = make(map[string]*visitor)
// var mu sync.Mutex

// func init() {
// 	// Run a background goroutine to remove old entries from the visitors map.
// 	go cleanupVisitors()
// }

// func getVisitor(ip string) *rate.Limiter {
// 	mu.Lock()
// 	defer mu.Unlock()

// 	v, exists := visitors[ip]
// 	if !exists {
// 		limiter := rate.NewLimiter(maxRequestsPerSecAvg, maxRequestsPerSecBurst)
// 		// Include the current time when creating a new visitor.
// 		visitors[ip] = &visitor{limiter, time.Now()}
// 		return limiter
// 	}

// 	// Update the last seen time for the visitor.
// 	v.lastSeen = time.Now()
// 	return v.limiter
// }

// // Every minute check the map for visitors that haven't been seen for
// // more than 3 minutes and delete the entries.
// func cleanupVisitors() {
// 	for {
// 		time.Sleep(time.Minute)

// 		mu.Lock()
// 		for ip, v := range visitors {
// 			if time.Since(v.lastSeen) > 3*time.Minute {
// 				delete(visitors, ip)
// 			}
// 		}
// 		mu.Unlock()
// 	}
// }

// func limit(next http.Handler) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		// Get sender IP
// 		ip, _, err := net.SplitHostPort(r.RemoteAddr)
// 		if err != nil {
// 			log.Println(err.Error())
// 			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
// 			return
// 		}

// 		// If exists, use IP forwarded from Cloudflare
// 		cfForwardedFor := r.Header["X-Forwarded-For"]
// 		if len(cfForwardedFor) > 0 {
// 			ip = cfForwardedFor[0]
// 		} else if logMissingForwardedForHeader {
// 			log.Println("Error: No X-Forwarded-For header from", ip)
// 		}

// 		// Limit by IP
// 		limiter := getVisitor(ip)
// 		if !limiter.Allow() {
// 			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
// 			return
// 		}

// 		next.ServeHTTP(w, r)
// 	})
// }
