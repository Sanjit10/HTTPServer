package main

import (
	"context"
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

	"github.com/Sanjit10/HTTPServer/internal/auth"
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
	secret         string
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type contextKey string

const userContextKey = contextKey("user")
const tokenContextKey = contextKey("refresh_token")

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func getUserFromContext(ctx context.Context) (database.User, bool) {
	user, ok := ctx.Value(userContextKey).(database.User)
	return user, ok
}

func (cfg *apiConfig) middlewareJWTtokenValidator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := auth.GetBearerToken(r.Header)
		if err != nil {
			log.Printf("Error decoding headers: %s", err)
			respondWithError(w, http.StatusUnauthorized, "Invalid header")
			return
		}
		user_uuid, err := auth.ValidateJWT(token, cfg.secret)
		if err != nil {
			log.Printf("Error validating: %s", err)
			respondWithError(w, http.StatusUnauthorized, "Invalid token")
			return
		}
		user, db_err := cfg.dbQueries.GetUserById(r.Context(), user_uuid)
		if db_err != nil {
			log.Printf("Error fetching user: %s", err)
			respondWithError(w, http.StatusUnauthorized, "Invalid uuid")
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (cfg *apiConfig) middlewareRefreshTokenValidation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshToken, err := auth.GetBearerToken(r.Header)
		if err != nil {
			log.Printf("Error decoding refresh token header: %s", err)
			respondWithError(w, http.StatusUnauthorized, "Invalid refresh token header")
			return
		}
		ctx := context.WithValue(r.Context(), tokenContextKey, refreshToken)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getTokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(tokenContextKey).(string)
	return token, ok
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
	secret := os.Getenv("SECRET")

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	dbQueries := database.New(db)
	cfg := &apiConfig{
		dbQueries: dbQueries,
		platform:  platform,
		secret:    secret,
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
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		var decodedBody reqBody

		if err := decoder.Decode(&decodedBody); err != nil {
			log.Printf("Error decoding parameters: %s", err)
			respondWithError(w, http.StatusBadRequest, "Invalid JSON body")
			return
		}

		// Get the hashed password and set the password in db
		hashedPassword, error := auth.HashPassword(decodedBody.Password)
		if error != nil {
			log.Printf("Error hashing password: %s", err)
			respondWithError(w, http.StatusInternalServerError, "Could not create user")
			return
		}

		// Now you can access decodedBody.Email
		dbuser, err := cfg.dbQueries.CreateUser(r.Context(), decodedBody.Email)
		if err != nil {
			log.Printf("Error creating user: %s", err)
			respondWithError(w, http.StatusInternalServerError, "Could not create user")
			return
		}

		err = cfg.dbQueries.SetUserPassword(
			r.Context(),
			database.SetUserPasswordParams{
				HashedPassword: hashedPassword,
				ID:             dbuser.ID,
			})

		if err != nil {
			log.Printf("Error setting user password: %s", err)
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

		user, ok := getUserFromContext(r.Context())
		if !ok {
			respondWithError(w, http.StatusUnauthorized, "No user in context")
			return
		}

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
		decodedBody.UserID = user.ID

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
	mux.Handle("POST /api/chirps", cfg.middlewareJWTtokenValidator(post_chirp))

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

	// 11) Login Endpoint
	handelLogin := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Decode body
		defer r.Body.Close()
		type reqBody struct {
			Password string `json:"password"`
			Email    string `json:"email"`
		}
		decoder := json.NewDecoder(r.Body)
		var decodedBody reqBody
		if err := decoder.Decode(&decodedBody); err != nil {
			log.Printf("Error decoding parameters: %s", err)
			respondWithError(w, http.StatusBadRequest, "Invalid JSON body") // Use helper, 400 status
			return
		}

		// Get UserByEmail
		user, err := cfg.dbQueries.GetUserByEmail(r.Context(), decodedBody.Email)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				// Not found
				respondWithError(w, http.StatusNotFound, "USER not found")
				return
			}
			// Other DB error
			respondWithError(w, http.StatusInternalServerError, "Database error")
			return
		}

		// Check UserHashPassword
		authorized, err_auth := auth.VerifyPassword(
			user.HashedPassword,
			decodedBody.Password,
		)

		if err_auth != nil || !authorized {
			log.Printf("Unauthorized User: %s", err_auth)
			respondWithError(w, http.StatusUnauthorized, "Unauthorized User")
			return
		}
		type RespBody struct {
			ID           uuid.UUID `json:"id"`
			CreatedAt    time.Time `json:"created_at"`
			UpdatedAt    time.Time `json:"updated_at"`
			Email        string    `json:"email"`
			Token        string    `json:"token"`
			RefreshToken string    `json:"refresh_token"`
		}
		expires_in := 60 * 60 //seconds
		token, err_token := auth.MakeJWT(
			user.ID,
			cfg.secret,
			time.Duration(expires_in)*time.Second,
		)
		if err_token != nil {
			log.Printf("Token err: %s", err_token)
			respondWithError(w, http.StatusUnauthorized, "Token err")
			return
		}

		refresh_token, err_refresh := auth.MakeRefreshToken()
		if err_refresh != nil {
			log.Printf("Refresh Token err: %s", err_refresh)
			respondWithError(w, http.StatusUnauthorized, "Refresh Token err")
			return
		}
		duration := 60 * 60 * 24 * 60
		refresh_token_expiry := time.Now().Add(
			time.Duration(duration) * time.Second)
		_, db_err := cfg.dbQueries.CreateRefreshToken(
			r.Context(),
			database.CreateRefreshTokenParams{
				Token:     refresh_token,
				UserID:    user.ID,
				ExpiresAt: refresh_token_expiry,
			},
		)
		if db_err != nil {
			log.Printf("Refresh Token err: %s", db_err)
			respondWithError(w, http.StatusUnauthorized, "Refresh Token err")
			return
		}

		respBody := RespBody{
			ID:           user.ID,
			CreatedAt:    user.CreatedAt,
			UpdatedAt:    user.UpdatedAt,
			Email:        user.Email,
			Token:        token,
			RefreshToken: refresh_token,
		}
		respondWithJSON(w, 200, respBody)

	})
	mux.Handle("POST /api/login", handelLogin)

	handelRefresh := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshToken, ok := getTokenFromContext(r.Context())
		if !ok || refreshToken == "" {
			respondWithError(w, http.StatusUnauthorized, "No refresh token in context")
			return
		}

		// Validate refresh token in DB
		dbToken, err := cfg.dbQueries.GetValidToken(r.Context(), refreshToken)
		if err != nil {
			log.Printf("Invalid or expired refresh token: %s", err)
			respondWithError(w, http.StatusUnauthorized, "Invalid or expired refresh token")
			return
		}

		// Issue new access token
		accessToken, err := auth.MakeJWT(
			dbToken.UserID,
			cfg.secret,
			time.Hour, // 1 hour
		)
		if err != nil {
			log.Printf("Error creating new access token: %s", err)
			respondWithError(w, http.StatusInternalServerError, "Could not create access token")
			return
		}

		type RespBody struct {
			Token string `json:"token"`
		}
		respondWithJSON(w, http.StatusOK, RespBody{Token: accessToken})
	})
	mux.Handle("POST /api/refresh", cfg.middlewareRefreshTokenValidation(handelRefresh))

	handelRevoke := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := auth.GetBearerToken(r.Header)
		if err != nil {
			log.Printf("Error accessing token: %s", err)
			respondWithError(w, http.StatusUnauthorized, "Could not revoke refresh token")
			return
		}
		revoke_err := cfg.dbQueries.RevokeToken(r.Context(), token)
		if revoke_err != nil {
			log.Printf("Error revoking token: %s", revoke_err)
			respondWithError(w, http.StatusUnauthorized, "Could not revoke refresh token")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(204)
	})
	mux.Handle("POST /api/revoke", cfg.middlewareRefreshTokenValidation(handelRevoke))

	handelEmailUpdate := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		user, ok := getUserFromContext(r.Context())
		if !ok {
			respondWithError(w, http.StatusUnauthorized, "No user in context")
			return
		}
		type reqBody struct{
			Password string `json:"password"`
			Email	 string	`json:"email"`
		}
		reqBodyDecoder := json.NewDecoder(r.Body)
		var decodedReqBody reqBody
		if decode_err := reqBodyDecoder.Decode(&decodedReqBody); decode_err != nil {
			log.Printf("Error decoding parameters: %s", err)
			respondWithError(w, http.StatusBadRequest, "Invalid JSON body") // Use helper, 400 status
			return
		}

		hashedPassword, err := auth.HashPassword(decodedReqBody.Password)
		if err != nil {
			log.Printf("Error hashing password: %s", err)
			respondWithError(w, http.StatusBadRequest, "Error hashing password") // Use helper, 400 status
			return
		}

		db_err := cfg.dbQueries.UpdateUser(r.Context(), database.UpdateUserParams{
			Email: decodedReqBody.Email,
			HashedPassword: hashedPassword,
			ID: user.ID,
		})
		if db_err != nil {
			log.Printf("Error updating user: %s", err)
			respondWithError(w, http.StatusBadRequest, "Error updating user") // Use helper, 400 status
			return
		}

		updated_user, err := cfg.dbQueries.GetUserById(r.Context(), user.ID)
		if err != nil {
			log.Printf("Error updating user: %s", err)
			respondWithError(w, http.StatusBadRequest, "Error updating user") // Use helper, 400 status
			return
		}
		respUser := User{
			ID: updated_user.ID,
			CreatedAt: updated_user.CreatedAt,
			UpdatedAt: updated_user.UpdatedAt,
			Email: updated_user.Email,
		}
		// type respBody struct {
		// 	User User `json:"user"`
		// }
		respondWithJSON(w, 200, respUser)

	})
	mux.Handle("PUT /api/users", cfg.middlewareJWTtokenValidator(handelEmailUpdate))

	deleteChirpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	// Get user from context (auth middleware should have set this)
	user, ok := getUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "No user in context")
		return
	}

	// Extract chirpID from URL
	chirp_idStr := r.PathValue("chirp_id")
	chirpID, err := uuid.Parse(chirp_idStr)
	if err != nil {
		respondWithError(w, 404, "Invalid UUID format")
		return
	}
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid chirp ID")
		return
	}

	// Fetch chirp from DB
	chirp, err := cfg.dbQueries.GetChirp(r.Context(), chirpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Chirp not found")
			return
		}
		log.Printf("Error fetching chirp: %s", err)
		respondWithError(w, http.StatusInternalServerError, "Error fetching chirp")
		return
	}

	// Check ownership
	if chirp.UserID != user.ID {
		respondWithError(w, http.StatusForbidden, "Not authorized to delete this chirp")
		return
	}

	// Delete chirp
	err = cfg.dbQueries.DeleteChirp(r.Context(), chirpID)
	if err != nil {
		log.Printf("Error deleting chirp: %s", err)
		respondWithError(w, http.StatusInternalServerError, "Error deleting chirp")
		return
	}

	// Success, no content
	w.WriteHeader(http.StatusNoContent)
})

// Register route
mux.Handle("DELETE /api/chirps/{chirp_id}", cfg.middlewareJWTtokenValidator(deleteChirpHandler))

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Printf("listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
