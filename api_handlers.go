package main

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
)

func contains(arr []string, str string) bool {
	for _, a := range arr {
		if a == str {
			return true
		}
	}
	return false
}

type SR map[string]interface{}

var unknownMethodResponse, _ = json.Marshal(map[string]string{
	"error": "unknown method",
})

var badMethodResponse, _ = json.Marshal(map[string]string{
	"error": "bad method",
})

var unauthorizedResponse, _ = json.Marshal(map[string]string{
	"error": "unauthorized",
})

var emptyLoginResponse, _ = json.Marshal(map[string]string{
	"error": "login must me not empty",
})

var minLoginResponse, _ = json.Marshal(map[string]string{
	"error": "login len must be >= 10",
})

var intAgeResponse, _ = json.Marshal(map[string]string{
	"error": "age must be int",
})

var minAgeResponse, _ = json.Marshal(map[string]string{
	"error": "age must be >= 0",
})

var maxAgeResponse, _ = json.Marshal(map[string]string{
	"error": "age must be <= 128",
})

var enumStatusResponse, _ = json.Marshal(map[string]string{
	"error": "status must be one of [user, moderator, admin]",
})

func (srv *MyApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/user/create":
		srv.handlerCreate(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
		w.Write(unknownMethodResponse)
	}
}

func (srv *MyApi) handlerCreate(w http.ResponseWriter, r *http.Request) {
	login := r.FormValue("login")
	int_age, err_age := strconv.ParseInt(r.FormValue("age"), 10, 64)
	status := r.FormValue("status")
	if status == "" {
		status = "user"
	}
	userName := r.FormValue("user_name")

	if r.Method != "POST" {
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write(badMethodResponse)
	} else if r.Header.Get("X-Auth") != "100500" {
		w.WriteHeader(http.StatusForbidden)
		w.Write(unauthorizedResponse)
	} else if login == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write(emptyLoginResponse)
	} else if len([]rune(login)) < 10 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write(minLoginResponse)
	} else if err_age != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write(intAgeResponse)
	} else if int_age < 0 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write(minAgeResponse)
	} else if int_age > 128 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write(maxAgeResponse)
	}  else if !contains([]string{"user", "moderator", "admin"}, status) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write(enumStatusResponse)
	} else {
		params := CreateParams{
			Login:  login,
			Name:   userName,
			Status: status,
			Age:    int(int_age),
		}
		ctx := r.Context()
		newUser, err := srv.Create(ctx, params)
		// проверить на специфичную ошибку
		if err != nil {
			if reflect.TypeOf(err).String() != "main.ApiError" {
				w.WriteHeader(http.StatusInternalServerError)
				errJson, _ := json.Marshal(SR{
					"error": err.Error(),
				})
				w.Write(errJson)
			} else {
				errAPI := err.(ApiError)
				w.WriteHeader(errAPI.HTTPStatus)
				errJson, _ := json.Marshal(SR{
					"error": errAPI.Err.Error(),
				})
				w.Write(errJson)
			}
		} else {
			newUserJson, _ := json.Marshal(SR{
				"error": "",
				"response":newUser,
			})
			w.WriteHeader(http.StatusOK)
			w.Write(newUserJson)
		}
	}

}

func (srv *OtherApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {

	default:
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	}
}
