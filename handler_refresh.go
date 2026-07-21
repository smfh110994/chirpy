package main

import (
	"net/http"
	"time"

	"github.com/smfh110994/chirpy/internal/auth"
)

func (cfg *apiConfig) handlerRefresh(w http.ResponseWriter, r *http.Request) {
	type response struct {
		Token string `json:"token"`
	}

	refreshToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't find refresh token")
		return
	}

	user, err := cfg.DB.GetUserFromRefreshToken(r.Context(), refreshToken)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid, expired, or revoked refresh token")
		return
	}

	// Generate a new 1-hour access token for the user
	accessToken, err := auth.MakeJWT(user.ID, cfg.jwtSecret, time.Hour)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create access token")
		return
	}

	respondWithJSON(w, http.StatusOK, response{
		Token: accessToken,
	})
}
