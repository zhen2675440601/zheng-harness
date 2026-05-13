package server

import "net/http"

type authRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (a *API) HandleRegister(w http.ResponseWriter, r *http.Request) error {
	var req authRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	result, err := a.authService().Register(r.Context(), req.Username, req.Password)
	if err != nil {
		return a.mapServiceError(err, "register user")
	}
	writeJSON(w, http.StatusCreated, result)
	return nil
}

func (a *API) HandleLogin(w http.ResponseWriter, r *http.Request) error {
	var req authRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	result, err := a.authService().Login(r.Context(), req.Username, req.Password)
	if err != nil {
		return a.mapServiceError(err, "login user")
	}
	writeJSON(w, http.StatusOK, result)
	return nil
}
