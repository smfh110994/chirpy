package main

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/smfh110994/chirpy/internal/auth"
)

func (cfg *apiConfig) handlerChirpsDelete(w http.ResponseWriter, r *http.Request) {
	// 1. Extract and validate chirpID from path parameter
	chirpIDString := r.PathValue("chirpID")
	chirpID, err := uuid.Parse(chirpIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid chirp ID")
		return
	}

	// 2. Validate JWT token header
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT")
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	// 3. Retrieve the chirp to check existence and authorship
	chirp, err := cfg.DB.GetChirp(r.Context(), chirpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Chirp not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Couldn't get chirp")
		return
	}

	// 4. Authorize: Ensure the authenticated user owns the chirp
	if chirp.UserID != userID {
		respondWithError(w, http.StatusForbidden, "You can't delete this chirp")
		return
	}

	// 5. Delete chirp from database
	err = cfg.DB.DeleteChirp(r.Context(), chirp.ID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't delete chirp")
		return
	}

	// 6. Return 204 No Content
	w.WriteHeader(http.StatusNoContent)
}
