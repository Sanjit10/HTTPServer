package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	log "log"
	http "net/http"
	"os"
	"regexp"
	atomic "sync/atomic"
	"time"

	"github.com/Sanjit10/HTTPServer/internal/database"
	"github.com/google/uuid"
	godotenv "github.com/joho/godotenv"
	_ "github.com/lib/pq"
	// Removed unused import: "github.com/gogo/protobuf/test/data"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
	platform       string
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

// respondWithJSON is a helper to send JSON responses
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON response: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

// respondWithError is a helper to send JSON error responses
func respondWithError(w http.ResponseWriter, code int, msg string) {
	respondWithJSON(w, code, map[string]string{"error": msg})
}

func censor(input string, profaneWords []string) string {
	for _, w := range profaneWords {
		// Build a case-insensitive regexp for the word
		re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(w))
		input = re.ReplaceAllString(input, "****")
	}
	return input
}

func main() {

	godotenv.Load()

	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	dbQueries := database.New(db)
	cfg := &apiConfig{
		dbQueries: dbQueries,
		platform:  platform,
	}

	mux := http.NewServeMux()

	// 1) File server with metrics middleware
	fileHandler := http.StripPrefix("/app/", http.FileServer(http.Dir("./files")))
	mux.Handle("/app/", cfg.middlewareMetricsInc(fileHandler))

	// 2) Root health-check
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// 3) Metrics endpoint
	metricsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		val := cfg.fileserverHits.Load()
		int_val := int(val)
		str_val := fmt.Sprintf(`
			<html>
				<body>
					<h1>Welcome, Chirpy Admin</h1>
					<p>Chirpy has been visited %d times!</p>
				</body>
			</html>
		`, int_val)
		w.Write([]byte(str_val))
	})
	mux.Handle("GET /admin/metrics", metricsHandler)

	// 6) User Creation
	createUserHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		// Decode JSON body into struct
		decoder := json.NewDecoder(r.Body)
		type reqBody struct {
			Email string `json:"email"`
		}
		var decodedBody reqBody

		if err := decoder.Decode(&decodedBody); err != nil {
			log.Printf("Error decoding parameters: %s", err)
			respondWithError(w, http.StatusBadRequest, "Invalid JSON body")
			return
		}

		// Now you can access decodedBody.Email
		dbuser, err := cfg.dbQueries.CreateUser(r.Context(), decodedBody.Email)
		if err != nil {
			log.Printf("Error creating user: %s", err)
			respondWithError(w, http.StatusInternalServerError, "Could not create user")
			return
		}
		user := User{
			ID:        dbuser.ID,
			CreatedAt: dbuser.CreatedAt,
			UpdatedAt: dbuser.UpdatedAt,
			Email:     dbuser.Email,
		}

		respondWithJSON(w, http.StatusCreated, user)
	})
	mux.Handle("POST /api/users", createUserHandler)

	//7) Delete all users in db
	delete_all_user := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cfg.platform != "dev" {
			log.Printf("Error deleteing users : Invalid env request ")
			respondWithError(w, http.StatusForbidden, "Could not delete user")
			return
		}

		if err := cfg.dbQueries.DeleteAllUsers(r.Context()); err != nil {
			log.Printf("Error deleteing users : %s", err)
			respondWithError(w, http.StatusInternalServerError, "Could not delete user")
			return
		}
		respondWithJSON(w, 200, nil)
	})
	mux.Handle("POST /admin/reset", delete_all_user)

	//8) add chirps
	post_chirp := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		type reqBody struct {
			UserID uuid.UUID `json:"user_id"`
			Body   string    `json:"body"`
		}
		type Chirp struct {
			ID        uuid.UUID `json:"id"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
			Body      string    `json:"body"`
			UserID    uuid.UUID `json:"user_id"`
		}

		// Decode body
		decoder := json.NewDecoder(r.Body)
		var decodedBody reqBody

		if err := decoder.Decode(&decodedBody); err != nil {
			log.Printf("Error decoding parameters: %s", err)
			respondWithError(w, http.StatusBadRequest, "Invalid JSON body") // Use helper, 400 status
			return
		}

		// Body Too large error
		if len(decodedBody.Body) >= 140 {
			respondWithError(w, http.StatusBadRequest, "Chirp is too long") // Use helper, 400 status
			return
		}
		profane_words := [3]string{"kerfuffle", "sharbert", "fornax"}
		new_sentance := censor(decodedBody.Body, profane_words[:])

		newChirp, err := cfg.dbQueries.CreateChirps(r.Context(), database.CreateChirpsParams{
			Body:   new_sentance,
			UserID: decodedBody.UserID,
		})

		if err != nil {
			log.Printf("Error adding Chirps to database: %s", err)
			respondWithError(w, http.StatusBadRequest, "Error adding chirps to db")
			return
		}

		resp := Chirp{
			ID:        newChirp.ID,
			CreatedAt: newChirp.CreatedAt,
			UpdatedAt: newChirp.UpdatedAt,
			Body:      newChirp.Body,
			UserID:    newChirp.UserID,
		}
		respondWithJSON(w, 201, resp)
	})
	mux.Handle("POST /api/chirps", post_chirp)

	// 9) Get all chirps
	getAllChirps := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		type Chirp struct {
			ID        uuid.UUID `json:"id"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
			Body      string    `json:"body"`
			UserID    uuid.UUID `json:"user_id"`
		}

		dbChirps, err := cfg.dbQueries.GetAllChirps(r.Context())
		if err != nil {
			log.Printf("Error retrieving chirps: %s", err)
			respondWithError(w, http.StatusInternalServerError, "Could not fetch chirps")
			return
		}

		chirps := make([]Chirp, 0, len(dbChirps))
		for _, c := range dbChirps {
			chirps = append(chirps, Chirp{
				ID:        c.ID,
				CreatedAt: c.CreatedAt,
				UpdatedAt: c.UpdatedAt,
				Body:      c.Body,
				UserID:    c.UserID,
			})
		}

		respondWithJSON(w, http.StatusOK, chirps)
	})
	mux.Handle("GET /api/chirps", getAllChirps)

	// 10) Get a chirp
	getOneChirps := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chirp_idStr := r.PathValue("chirp_id")
		chirp_idUUID, err := uuid.Parse(chirp_idStr)
		if err != nil {
			respondWithError(w, 404, "Invalid UUID format")
			return
		}
		type Chirp struct {
			ID        uuid.UUID `json:"id"`
			CreatedAt time.Time `json:"created_at"`
			UpdatedAt time.Time `json:"updated_at"`
			Body      string    `json:"body"`
			UserID    uuid.UUID `json:"user_id"`
		}

		dbChirps, err := cfg.dbQueries.GetChirp(r.Context(), chirp_idUUID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				// Not found
				respondWithError(w, http.StatusNotFound, "Chirp not found")
				return
			}
			// Other DB error
			respondWithError(w, http.StatusNotFound, "Database error")
			return
		}

		chirp := Chirp{
			ID:        dbChirps.ID,
			CreatedAt: dbChirps.CreatedAt,
			UpdatedAt: dbChirps.UpdatedAt,
			Body:      dbChirps.Body,
			UserID:    dbChirps.UserID,
		}

		respondWithJSON(w, http.StatusOK, chirp)
	})
	mux.Handle("GET /api/chirps/{chirp_id}", getOneChirps)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Printf("listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
