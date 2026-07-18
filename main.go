package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/smfh110994/chirpy/internal/database"
)

// 1. Create a struct to hold the state (in-memory counter)
type apiConfig struct {
	fileserverHits atomic.Int32
	// DB holds our SQLC generated type-safe database queries interface
	DB *database.Queries
}

// 2. Middleware that increments fileserverHits on every request
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1) // Safely increment the counter across goroutines
		next.ServeHTTP(w, r)      // Pass control to the next handler in the chain
	})
}

// 4. Handler method that returns the current hit count as plain text
func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	// 2. Generate and write the HTML response using fmt.Sprintf
	htmlContent := fmt.Sprintf(`<html>
<body>
	<h1>Welcome, Chirpy Admin</h1>
	<p>Chirpy has been visited %d times!</p>
</body>
</html>`, cfg.fileserverHits.Load())

	w.Write([]byte(htmlContent))
}

// Serve the app's landing page at the root path
func (cfg *apiConfig) handlerRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	http.ServeFile(w, r, "index.html")
}

// 6. Handler method that resets the hit count back to 0
func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hits reset to 0"))
}

func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// --- NEW JSON HELPERS ---

// respondWithError wraps respondWithJSON to send a standard error JSON response
func respondWithError(w http.ResponseWriter, code int, msg string) {
	type errResponse struct {
		Error string `json:"error"`
	}
	respondWithJSON(w, code, errResponse{
		Error: msg,
	})
}

// respondWithJSON marshals a payload structure to JSON and writes it with the correct status code
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}

// --- CHIRP VALIDATION HANDLER ---

func handlerChirpsValidate(w http.ResponseWriter, r *http.Request) {
	// 1. Define the incoming request struct shape
	type parameters struct {
		Body string `json:"body"`
	}

	// 2. Decode the JSON body safely
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Something went wrong")
		return
	}

	// 3. Perform business logic validation (Chirps must be <= 140 characters)
	if len(params.Body) > 140 {
		respondWithError(w, http.StatusBadRequest, "Chirp is too long")
		return
	}

	cleanedBody := getCleanedBody(params.Body)

	// 4. Define and send success response
	type returnVals struct {
		CleanedBody string `json:"cleaned_body"`
	}
	respondWithJSON(w, http.StatusOK, returnVals{
		CleanedBody: cleanedBody,
	})
}

// Helper function to handle word-by-word censorship
func getCleanedBody(body string) string {
	badWords := map[string]struct{}{
		"kerfuffle": {},
		"sharbert":  {},
		"fornax":    {},
	}

	// Split text into an array of separate words by white spaces
	words := strings.Split(body, " ")

	for i, word := range words {
		// Convert the word to lowercase to catch mixed-cased variants (e.g., ShArBeRt)
		lowercasedWord := strings.ToLower(word)

		// If the lowercase version exists in our bad words map, censor it
		if _, ok := badWords[lowercasedWord]; ok {
			words[i] = "****"
		}
	}

	// Rejoin the slice back into a complete sentence string
	return strings.Join(words, " ")
}

func main() {
	// 1. Load the .env configuration file into memory
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	// 2. Lookup the database connection string environment variable
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("DB_URL environment variable is not set")
	}

	// 3. Open a connection pool handle using the postgres driver
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Error opening database connection: %v", err)
	}
	defer db.Close()

	// 4. Create a new SQLC database query instance wrapped around our connection pool
	dbQueries := database.New(db)

	mux := http.NewServeMux()

	// Initialize our stateful configuration struct
	apiCfg := &apiConfig{}

	// Let's assume you have a file server handling static files under /app/
	// (Adjust directory path as needed for your project structure)
	fileServerHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))

	// 3. Wrap the file server handler with our metrics middleware
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(fileServerHandler))

	// 5. Register the root, /metrics, and /reset endpoints
	mux.HandleFunc("/", apiCfg.handlerRoot)
	mux.HandleFunc("GET /api/healthz", handlerReadiness)
	mux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.handlerReset)
	mux.HandleFunc("POST /api/validate_chirp", handlerChirpsValidate)

	// Start the server (adjust port as necessary, default is usually :8080)
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	server.ListenAndServe()
}
