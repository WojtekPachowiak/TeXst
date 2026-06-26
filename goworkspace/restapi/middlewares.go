package main

import(
	"log"
	"net/http"
	"time"
)




// =========================

type ResponseWriterWrapper struct {
	http.ResponseWriter
	status int
}

func (w *ResponseWriterWrapper) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func mainMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapper := &ResponseWriterWrapper{w, http.StatusOK}
		next.ServeHTTP(wrapper, r)
		duration := time.Since(start)

		HTTPRequests.WithLabelValues(r.Method, r.URL.Path, http.StatusText(wrapper.status)).Inc()
		HTTPDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration.Seconds())

		log.Printf("%s %s %s - %.4fs (status: %d)", r.Method, r.URL.Path, r.RemoteAddr, duration.Seconds(), wrapper.status)
	})
}
