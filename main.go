package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

type chirpRequest struct {
	Body string `json:"body"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type cleanedResponse struct {
	CleanedBody string `json:"cleaned_body"`
}

func main() {
	const filepathRoot = "."
	const port = "8080"

	apiCfg := &apiConfig{}

	mux := http.NewServeMux()
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(filepathRoot)))))
	mux.HandleFunc("GET /api/healthz", handlerReadiness)
	mux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.handlerReset)
	mux.HandleFunc("POST /api/validate_chirp", handlerValidateChirp)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Printf("Serving files from %s on port: %s\n", filepathRoot, port)
	log.Fatal(srv.ListenAndServe())
}

// profane words list
var profaneWords = []string{"kerfuffle", "sharbert", "fornax"}

// status check
func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(http.StatusText(http.StatusOK)))
}

// increment counter
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

// display count for /admin/metrics
func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, r *http.Request) {
	count := cfg.fileserverHits.Load()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	htmlContent := fmt.Sprintf(`
		<html>
			<body>
				<h1>Welcome, Chirpy Admin</h1>
				<p>Chirpy has been visited %d times!</p>
			</body>
		</html>`, count)
	w.Write([]byte(htmlContent))
}

// reset counter
func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Counter reset to 0"))
}

// validate chirp
func handlerValidateChirp(w http.ResponseWriter, r *http.Request) {
	var chirp chirpRequest

	//parse JSON
	if err := json.NewDecoder(r.Body).Decode(&chirp); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errorResponse{Error: "Invalid JSON"})
		return
	}

	//validate chirp length
	if len(chirp.Body) > 140 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errorResponse{Error: "Chirp is too long"})
		return
	}

	//replace profane words
	cleanedBody := cleanProfanity(chirp.Body)

	//valid and cleaned
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(cleanedResponse{CleanedBody: cleanedBody})
}

// replace profane words
func cleanProfanity(body string) string {
	for _, word := range profaneWords {
		regex := regexp.MustCompile(`\b(?i)` + word + `\b`)
		body = regex.ReplaceAllString(body, "****")
	}
	return body
}
