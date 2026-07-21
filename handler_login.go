package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/smfh110994/chirpy/internal/auth"
)

func (cfg *apiConfig) handlerLogin(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Password         string `json:"password"`
		Email            string `json:"email"`
		ExpiresInSeconds *int   `json:"expires_in_seconds"`
	}

	type response struct {
		User
		Token string `json:"token"` // 👈 Return the signed JWT
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't decode parameters")
		return
	}

	// 1. Fetch user by email
	user, err := cfg.DB.GetUserByEmail(r.Context(), params.Email)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}

	// 2. Compare password using Argon2id
	match, err := auth.CheckPasswordHash(params.Password, user.HashedPassword)
	if err != nil || !match {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}

	// Set default expiration duration (1 hour)
	defaultExpiration := time.Hour
	if params.ExpiresInSeconds != nil {
		requestedDuration := time.Duration(*params.ExpiresInSeconds) * time.Second
		if requestedDuration < defaultExpiration {
			defaultExpiration = requestedDuration
		}
	}

	token, err := auth.MakeJWT(user.ID, cfg.jwtSecret, defaultExpiration)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create JWT")
		return
	}

	// 3. Return 200 OK with User payload
	respondWithJSON(w, http.StatusOK, response{
		User: User{
			ID:        user.ID,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
			Email:     user.Email,
		},
		Token: token,
	})
}
