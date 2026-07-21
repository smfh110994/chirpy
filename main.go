package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/smfh110994/chirpy/internal/auth"
	"github.com/smfh110994/chirpy/internal/database"
)

func (cfg *apiConfig) handlerChirpsGet(w http.ResponseWriter, r *http.Request) {
	// 1. Extract the path value from URL
	chirpIDString := r.PathValue("chirpID")

	// 2. Parse the string into a UUID
	chirpID, err := uuid.Parse(chirpIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid chirp ID")
		return
	}

	// 3. Retrieve the chirp from the database
	dbChirp, err := cfg.DB.GetChirp(r.Context(), chirpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Chirp not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Couldn't get chirp")
		return
	}

	// 4. Respond with 200 OK and the formatted chirp
	respondWithJSON(w, http.StatusOK, Chirp{
		ID:        dbChirp.ID,
		CreatedAt: dbChirp.CreatedAt,
		UpdatedAt: dbChirp.UpdatedAt,
		Body:      dbChirp.Body,
		UserID:    dbChirp.UserID,
	})
}

func (cfg *apiConfig) handlerChirpsRetrieve(w http.ResponseWriter, r *http.Request) {
	// 1. Fetch chirps from the database
	dbChirps, err := cfg.DB.GetChirps(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't retrieve chirps")
		return
	}

	// 2. Map database model structs to response JSON structs
	chirps := []Chirp{}
	for _, dbChirp := range dbChirps {
		chirps = append(chirps, Chirp{
			ID:        dbChirp.ID,
			CreatedAt: dbChirp.CreatedAt,
			UpdatedAt: dbChirp.UpdatedAt,
			Body:      dbChirp.Body,
			UserID:    dbChirp.UserID,
		})
	}

	// 3. Respond with HTTP 200 OK and the list of chirps
	respondWithJSON(w, http.StatusOK, chirps)
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

func (cfg *apiConfig) handlerChirpsCreate(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body   string    `json:"body"`
		UserID uuid.UUID `json:"user_id"`
	}

	// 1. Extract Bearer token from header
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT")
		return
	}

	// 2. Validate token & parse userID
	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't decode parameters")
		return
	}

	// Port validation rules: clean and length checks
	const maxChirpLength = 140
	if len(params.Body) > maxChirpLength {
		respondWithError(w, http.StatusBadRequest, "Chirp is too long")
		return
	}

	// Clean profane words if your previous step required it
	cleanedBody := getCleanedBody(params.Body)

	// Save to the database
	chirp, err := cfg.DB.CreateChirp(r.Context(), database.CreateChirpParams{
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		Body:      cleanedBody,
		UserID:    userID,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create chirp")
		return
	}

	// Respond with 201 Created and the row layout
	respondWithJSON(w, http.StatusCreated, Chirp{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	})
}

// 1. Create a struct to hold the state (in-memory counter)
type apiConfig struct {
	fileserverHits atomic.Int32
	// DB holds our SQLC generated type-safe database queries interface
	DB        *database.Queries
	DBConn    *sql.DB
	platform  string
	jwtSecret string
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
	// Block this endpoint completely unless we are running locally in dev mode
	if cfg.platform != "dev" {
		respondWithError(w, http.StatusForbidden, "Forbidden: Only allowed in dev environment")
		return
	}

	// Clean out all user data records using SQLC
	err := cfg.DB.ResetUsers(r.Context())
	if err != nil {
		log.Printf("ERROR: ResetUsers failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Could not reset users table")
		return
	}

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

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

func (cfg *apiConfig) getUserPasswordColumn(ctx context.Context) (string, error) {
	var columnName string
	err := cfg.DBConn.QueryRowContext(ctx, `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = 'public'
		AND table_name = 'users'
		AND column_name IN ('hashed_password', 'password_hash')
		ORDER BY CASE column_name WHEN 'hashed_password' THEN 1 WHEN 'password_hash' THEN 2 ELSE 3 END
		LIMIT 1
	`).Scan(&columnName)
	if err != nil {
		return "", err
	}
	return columnName, nil
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

	if err := goose.Up(db, "sql/schema"); err != nil {
		log.Fatalf("Error applying migrations: %v", err)
	}

	if _, err := db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			email TEXT NOT NULL UNIQUE
		);
		ALTER TABLE users ADD COLUMN IF NOT EXISTS hashed_password TEXT;
		ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT;
	`); err != nil {
		log.Fatalf("Error ensuring user password columns: %v", err)
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is not set")
	}

	dbQueries := database.New(db)

	apiCfg := &apiConfig{
		DB:        dbQueries,
		DBConn:    db,
		platform:  os.Getenv("PLATFORM"),
		jwtSecret: jwtSecret,
	}
	// 4. Create a new SQLC database query instance wrapped around our connection pool

	mux := http.NewServeMux()

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
	mux.HandleFunc("POST /api/chirps", apiCfg.handlerChirpsCreate)
	mux.HandleFunc("POST /api/users", apiCfg.handlerUsersCreate)
	mux.HandleFunc("GET /api/chirps", apiCfg.handlerChirpsRetrieve)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.handlerChirpsGet)
	mux.HandleFunc("POST /api/login", apiCfg.handlerLogin)
	mux.HandleFunc("POST /api/refresh", apiCfg.handlerRefresh)
	mux.HandleFunc("POST /api/revoke", apiCfg.handlerRevoke)
	mux.HandleFunc("PUT /api/users", apiCfg.handlerUsersUpdate)

	// Start the server (adjust port as necessary, default is usually :8080)
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	server.ListenAndServe()
}
