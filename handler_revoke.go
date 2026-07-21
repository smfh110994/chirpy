package main

import (
	"net/http"

	"github.com/smfh110994/chirpy/internal/auth"
)

func (cfg *apiConfig) handlerRevoke(w http.ResponseWriter, r *http.Request) {
	refreshToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't find refresh token")
		return
	}

	err = cfg.DB.RevokeRefreshToken(r.Context(), refreshToken)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't revoke token")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
