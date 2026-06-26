package main

import(
	"log"
	"net/http"
	"github.com/felixge/httpsnoop"
)




// =========================

// type ResponseWriterWrapper struct {
// 	http.ResponseWriter
// 	status int
// }

// func (w *ResponseWriterWrapper) WriteHeader(status int) {
// 	w.status = status
// 	w.ResponseWriter.WriteHeader(status)
// }

func mainMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m := httpsnoop.CaptureMetrics(next,w,r)
		// start := time.Now()
		// wrapper := &ResponseWriterWrapper{w, http.StatusOK}
		// next.ServeHTTP(wrapper, r)
		// duration := time.Since(start)

		code := m.Code
		durationSeconds := m.Duration.Seconds()

		HTTPRequests.WithLabelValues(r.Method, r.URL.Path, http.StatusText(code)).Inc()
		HTTPDuration.WithLabelValues(r.Method, r.URL.Path).Observe(durationSeconds)

		log.Printf("[%s] %s [%s] - %.4fs (status: %d)", r.Method, r.URL.Path, r.RemoteAddr, durationSeconds,code)
	})
}
