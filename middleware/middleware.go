package middleware

import (
	"fmt"
	"net/http"
	"os"

	"github.com/dgrijalva/jwt-go"
)

var mySigningKey = []byte("captainjacksparrowsayshi")

func IsAuth(next func(http.ResponseWriter, *http.Request)) http.Handler {
	userToken := os.Getenv("Token")
	fmt.Println("middleware token ", userToken)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header["Token"] != nil || userToken != "" { // if there is a token exist
			if r.Header["Token"] != nil {
				token, err := jwt.Parse(r.Header["Token"][0], func(token *jwt.Token) (interface{}, error) {
					if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, fmt.Errorf("There was an error")
					}
					return mySigningKey, nil
				})
				if err != nil {
					fmt.Fprintf(w, err.Error())
				}
				if token.Valid {
					next(w, r)
				}
			} else if userToken != "" {
				token, err := jwt.Parse(userToken, func(token *jwt.Token) (interface{}, error) {
					if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, fmt.Errorf("There was an error")
					}
					return mySigningKey, nil
				})
				if err != nil {
					fmt.Fprintf(w, err.Error())
				}
				if token.Valid {
					next(w, r)
				}
			}

		} else {
			fmt.Fprintf(w, "Giriş Yapılmadı !")
		}
	})
}

func IsAdminLoggedIn(next func(http.ResponseWriter, *http.Request)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header["Admin"] != nil {
			next(w, r)
		} else {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, "Admin girişi yapılmadı")
		}
	})
}
