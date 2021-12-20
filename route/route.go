package route

import (
	"YAZLAB3MONGO/db"
	"YAZLAB3MONGO/middleware"
	"YAZLAB3MONGO/student"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"text/template"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/mongo"
)

//client := db.ConnectToMongoDb()

var client *mongo.Client

func addUser(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil { // to parse the form element (HTML)
		fmt.Fprintf(w, "Parse error :%v\n", err)
		return
	}

	name := r.FormValue("name")         // get the input that has firstname tag
	lastname := r.FormValue("lastname") // get the input that has lastname tag
	password := r.FormValue("password")
	number := r.FormValue("number")
	typeOfStudent := r.FormValue("myAction")
	if name == "" || lastname == "" || password == "" || typeOfStudent == "" { // via postman (without html from elements)
		rBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, "An error occured during the get credentials")
		}
		if stdnt, err := student.ProcessToJson(rBody); err != nil {
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, "An error occured during the get person struct")
		} else {
			if stdnt.Name != "" && stdnt.Lastname != "" && stdnt.Number != "" && stdnt.Type != "" {
				//r.Header
				stdnt, err = db.AddUser(stdnt.Name, stdnt.Lastname, stdnt.Password, stdnt.Number, stdnt.Type, client)
				if err != nil {
					w.WriteHeader(http.StatusNotAcceptable)
					fmt.Fprintf(w, err.Error()) //This username is already used by someone
				} else {
					fmt.Println(stdnt)
					http.Redirect(w, r, "/signIn", http.StatusFound)
				}
			} else {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, "Please provide the correct credentials to sign up")
			}
		}
	} else { // via form from html
		if stdnt, err := db.AddUser(name, lastname, password, number, typeOfStudent, client); err != nil {
			fmt.Fprintf(w, "This username is already used by someone")
		} else {
			//json.NewEncoder(w).Encode(stdnt)
			fmt.Println(stdnt)
			os.Setenv("Token", stdnt.Tokens[0].Context)

			http.Redirect(w, r, "/signIn", http.StatusFound)
		}
	}

}

func LoginUser(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil { // to parse the form element (HTML)
		fmt.Fprintf(w, "Parse error :%v\n", err)
		return
	}
	password := r.FormValue("password")
	number := r.FormValue("number")

	if password == "" || number == "" { // via postman , &&
		rBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, "An error occured during the get credentials")
		}
		if stdnt, err := student.ProcessToJson(rBody); err != nil {
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, "An error occured during the get student struct")
		} else {
			if stdnt.Password != "" && stdnt.Number != "" {
				stdnt, err := db.Login(stdnt.Password, stdnt.Number, client)
				if err != nil {
					w.WriteHeader(http.StatusNotAcceptable)
					fmt.Fprintf(w, err.Error())
				} else {
					os.Setenv("Token", stdnt.Tokens[0].Context)
					os.Setenv("userName", stdnt.Name)
					fmt.Println(stdnt)
					json.NewEncoder(w).Encode(stdnt)

				}
			} else {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, "Please provide the correct credentials to login")
			}
		}
	} else { // via html
		if stdnt, err := db.Login(password, number, client); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, "Giriş yapamazsın %q", err)
		} else {
			fmt.Printf("you logged in %s\n", stdnt.Name)
			os.Setenv("Token", stdnt.Tokens[0].Context)
			os.Setenv("userName", stdnt.Name)
			json.NewEncoder(w).Encode(stdnt)
		}

	}

}

func deleteUser(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("userName") != "" && os.Getenv("Token") != "" {
		if err := db.DeleteUser(client, os.Getenv("userName")); err != nil {
			panic(err)
		} else {
			os.Setenv("Token", "")
			os.Setenv("userName", "")
			http.Redirect(w, r, "/", http.StatusFound)
		}
	}
}

func logOutForAdmin(w http.ResponseWriter, r *http.Request) {
	os.Setenv("admin", "")
	http.Redirect(w, r, "/admin", http.StatusFound)
}

func insidePage(w http.ResponseWriter, r *http.Request) {
	myVars := map[string]interface{}{"userName": os.Getenv("userName"), "authToken": os.Getenv("Token")}
	fmt.Println("vars ---> = ", myVars)
	outputHTML(w, "./static/inside/index.html", myVars)

}

func adminPage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./static/admin/index.html")
}

func middlewareOne(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("Executing middlewareOne")
		if os.Getenv("Token") != "" {
			r.Header.Set("Token", os.Getenv("Token"))
		}
		next.ServeHTTP(w, r)
		log.Println("Executing middlewareOne again")
	})
}

func middlewareForAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("Executing middleware admin")
		fmt.Println("middleware for admin (env) --> ", os.Getenv("admin"))
		if os.Getenv("admin") != "" {
			r.Header.Set("Admin", os.Getenv("admin"))
		}
		next.ServeHTTP(w, r)
		log.Println("Executing middleware admin")
	})
}

func insideOfAdmin(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./static/adminInside/index.html")
}

func userUpdatePageByAdmin(w http.ResponseWriter, r *http.Request) {
	//http.ServeFile(w, r, "./static/adminUpdate/index.html")
	vars := mux.Vars(r)
	key := vars["id"]
	data := map[string]interface{}{"_id": key}
	outputHTML(w, "./static/adminUpdate/index.html", data)
}

func userUpdateByItself(w http.ResponseWriter, r *http.Request) {
	// vars := mux.Vars(r)
	// key := vars["name"]
	if os.Getenv("userName") != "" {
		data := map[string]interface{}{"userName": os.Getenv("userName")}
		outputHTML(w, "./static/userUpdate/index.html", data)
	}

}

func outputHTML(w http.ResponseWriter, filename string, data interface{}) {
	t, err := template.ParseFiles(filename)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := t.Execute(w, data); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

}

func loginForAdmin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil { // to parse the form element (HTML)
		fmt.Fprintf(w, "Parse error :%v\n", err)
		return
	}
	name := r.FormValue("name")
	password := r.FormValue("password")
	if password == "" || name == "" { // via postman
		rBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, "An error occured during the get credentials")
		}
		if admin, err := student.ProcessJSONforAdmin(rBody); err != nil {
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, "An error occured during the get admin struct")
		} else {
			if admin.Name != "" || admin.Password != "" {
				//admin = db.SignInForAdmin(name, password)
				fmt.Println("admin --> ", admin)
				if admin.Name != "adminKvnc" || admin.Password != "adminKvnc" {
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprintf(w, "Girilen bilgiler yanlışş")
				} else {
					os.Setenv("admin", admin.Name)
					fmt.Println("os admin --> ", os.Getenv("admin"))
					json.NewEncoder(w).Encode(admin)
					//http.Redirect(w, r, "/admin/inside", http.StatusFound)
				}
			}

		}
	} else { // via html
		admin := db.SignInForAdmin(name, password)
		if admin.Name != "adminKvnc" || admin.Password != "adminKvnc" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Girilen bilgiler yanlışş")
		} else {
			os.Setenv("admin", admin.Name)
			fmt.Println("os admin --> ", os.Getenv("admin"))
			json.NewEncoder(w).Encode(admin)
		}
	}
}

func getAllUsers(w http.ResponseWriter, r *http.Request) {
	studentsData := db.GetAllUsers(client)

	json.NewEncoder(w).Encode(studentsData)
}

func updateUserWithAdmin(w http.ResponseWriter, r *http.Request) {
	// use vars id
	vars := mux.Vars(r)
	key := vars["id"]
	if err := r.ParseForm(); err != nil { // to parse the form element (HTML)
		fmt.Fprintf(w, "Parse error :%v\n", err)
		return
	}
	name := r.FormValue("name")
	lastname := r.FormValue("lastname")
	number := r.FormValue("number")
	typeOf := "2"
	if name == "" || lastname == "" || number == "" || typeOf == "" { // via postmn
		fmt.Println("postman dude")
		rBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, "An error occured during the get credentials")
		}
		if stdnt, err := student.ProcessToJson(rBody); err != nil {
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, "An error occured during the get student struct")
		} else {
			fmt.Println("stdnt --> ", stdnt)
			// get tokens from previous user
			if stdnt.Name != "" && stdnt.Lastname != "" && stdnt.Type != "" {
				newStdnt := db.UpdateUserByAdmin(client, key, stdnt.Name, stdnt.Lastname, stdnt.Number, stdnt.Type)
				//json.NewEncoder(w).Encode(newStdnt)
				fmt.Println(newStdnt)
				os.Setenv("userName", stdnt.Name)
				http.Redirect(w, r, "/admin/inside", http.StatusFound)
			}
		}

	} else { // via html
		stdnt := db.UpdateUserByAdmin(client, key, name, lastname, number, typeOf)
		//json.NewEncoder(w).Encode(stdnt)
		fmt.Println(stdnt)
		os.Setenv("userName", name)
		http.Redirect(w, r, "/admin/inside", http.StatusFound)
	}

}

func addDocumentFile(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./static/addDocument/index.html")
}

func addDocumentForUser(w http.ResponseWriter, r *http.Request) { //---
	fmt.Println("File Upload Endpoint Hit")
	r.ParseMultipartForm(10 << 30)
	file, handler, err := r.FormFile("myFile")
	if err != nil {
		fmt.Println("Error Retrieving the File")
		fmt.Println(err)
		return
	}
	defer file.Close()
	fmt.Printf("Uploaded File: %+v\n", handler.Filename)
	fmt.Printf("File Size: %+v\n", handler.Size)
	fmt.Printf("MIME Header: %+v\n", handler.Header)

	db.UploadFile(client, file, handler.Filename, os.Getenv("userName"))
	// db.DownloadDocs(client, handler.Filename)
	// http.ServeFile(w, r, "./static/images/"+handler.Filename)
	http.Redirect(w, r, "/inside", http.StatusFound)
}

func UpdateUserByItselfRoute(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["name"]
	if err := r.ParseForm(); err != nil { // to parse the form element (HTML)
		fmt.Fprintf(w, "Parse error :%v\n", err)
		return
	}
	name := r.FormValue("name")
	lastname := r.FormValue("lastname")
	number := r.FormValue("number")
	typeOf := r.FormValue("myAction")
	if name == "" || lastname == "" || number == "" || typeOf == "" { // // via postmn
		rBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, "An error occured during the get credentials")
		}
		if stdnt, err := student.ProcessToJson(rBody); err != nil {
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, "An error occured during the get student struct")
		} else {
			fmt.Println("stdnt --> ", stdnt)
			// get tokens from previous user
			if stdnt.Name != "" && stdnt.Lastname != "" && stdnt.Type != "" {
				newStdnt := db.UpdateUserByItself(client, key, stdnt.Name, stdnt.Lastname, stdnt.Number, stdnt.Type)
				//json.NewEncoder(w).Encode(newStdnt)
				fmt.Println(newStdnt)
				//os.Setenv("userName", newStdnt.Name)
				http.Redirect(w, r, "/inside", http.StatusFound)
			}
		}
	} else {
		stdnt := db.UpdateUserByItself(client, key, name, lastname, number, typeOf)
		fmt.Println(stdnt)
		//os.Setenv("userName", stdnt.Name)
		http.Redirect(w, r, "/inside", http.StatusFound)
	}
}

func displayDocById(w http.ResponseWriter, r *http.Request) {
	//displayImageById
	vars := mux.Vars(r)
	key := vars["id"]
	filename := db.DisplayImageById(client, key)
	http.ServeFile(w, r, "./static/images/"+filename)
}

func displayDocByIdAdmin(w http.ResponseWriter, r *http.Request) {
	//displayImageById
	vars := mux.Vars(r)
	key := vars["id"]
	filename := db.DisplayImageById(client, key)
	http.ServeFile(w, r, "./static/images/"+filename)
}

func deleteWithAdmin(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["id"]
	db.DeleteWithAdmin(client, key)
	os.Setenv("Token", "")
}

func AccessMyDocs(w http.ResponseWriter, r *http.Request) {
	docs := db.AccessAllDocs(client, r.URL.String())
	json.NewEncoder(w).Encode(docs)
}

func grabAllDocsWithAdmin(w http.ResponseWriter, r *http.Request) {
	allDocs := db.GetAllDocsByAdmin(client, r.URL.String())
	json.NewEncoder(w).Encode(allDocs)
}

func deleteUsersAllDocsWithAdmin(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["id"]
	db.DeleteUsersAllDocsById(client, key)
}

func deleteUsersAllDocsByUserName(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["name"]
	db.DeleteUsersAllDocsWithUserName(client, key)
}

func deleteDocByAdminClick(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["id"]
	db.DeleteDocByAdminClick(client, key)
}

func deleteDocByUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["id"]
	db.DeleteDocByUser(client, key)
}

func HandleRequest() {
	client = db.ConnectToMongoDb()
	myRouter := NewRouter() // NewRouter()
	myRouter.HandleFunc("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		outputHTML(w, "./static/index.html", "")
	}))
	myRouter.HandleFunc("/signIn", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/signIn/index.html")
	})).Methods("GET")
	myRouter.HandleFunc("/signIn", LoginUser).Methods("POST")
	myRouter.HandleFunc("/signUp", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		outputHTML(w, "./static/signUp/index.html", "")
	})).Methods("GET")
	myRouter.Handle("/inside", middlewareOne(middleware.IsAuth(insidePage))).Methods("GET")
	myRouter.HandleFunc("/admin", adminPage) //middlewareForAdmin(middleware.IsAdminLoggedIn(adminPage))
	myRouter.HandleFunc("/adminLogin", loginForAdmin).Methods("POST")
	myRouter.Handle("/admin/inside", middlewareForAdmin(middleware.IsAdminLoggedIn(insideOfAdmin))).Methods("GET")
	myRouter.Handle("/user/update/{id}", middlewareForAdmin(middleware.IsAdminLoggedIn(updateUserWithAdmin))).Methods("POST")
	myRouter.Handle("/user/me/update/{name}", middlewareOne(middleware.IsAuth(UpdateUserByItselfRoute)))
	myRouter.HandleFunc("/admin/logOut", logOutForAdmin).Methods("POST")
	myRouter.Handle("/admin/update/{id}", middlewareForAdmin(middleware.IsAdminLoggedIn(userUpdatePageByAdmin)))
	myRouter.Handle("/user/me/update", middlewareOne(middleware.IsAuth(userUpdateByItself))).Methods("GET")
	myRouter.Handle("/users", middlewareForAdmin(middleware.IsAdminLoggedIn(getAllUsers)))
	myRouter.Handle("/delete", middlewareOne(middleware.IsAuth(deleteUser))).Methods("DELETE")
	//myRouter.Handle("/delete/admin/{id}", middlewareForAdmin(middleware.IsAdminLoggedIn(deleteWithAdmin)))
	myRouter.HandleFunc("/delete/admin/{id}", deleteWithAdmin)
	myRouter.Handle("/user/document", middlewareOne(middleware.IsAuth(addDocumentFile)))
	myRouter.Handle("/user/upload/document", middlewareOne(middleware.IsAuth(addDocumentForUser)))
	myRouter.Handle("/mydocs/all", middlewareOne(middleware.IsAuth(AccessMyDocs)))
	myRouter.Handle("/display/doc/{id}", middlewareOne(middleware.IsAuth(displayDocById)))
	myRouter.Handle("/admin/display/doc/{id}", middlewareForAdmin(middleware.IsAdminLoggedIn(displayDocByIdAdmin)))
	myRouter.Handle("/admin/allDocs", middlewareForAdmin(middleware.IsAdminLoggedIn(grabAllDocsWithAdmin)))
	myRouter.Handle("/admin/delete/allDocs/{id}", middlewareForAdmin(middleware.IsAdminLoggedIn(deleteUsersAllDocsWithAdmin)))
	myRouter.Handle("/user/delete/allDocs/{name}", middlewareOne(middleware.IsAuth(deleteUsersAllDocsByUserName)))
	myRouter.HandleFunc("/signUp", addUser).Methods("POST")
	myRouter.Handle("/delete/admin/doc/{id}", middlewareForAdmin(middleware.IsAdminLoggedIn(deleteDocByAdminClick)))
	myRouter.Handle("/delete/user/doc/{id}", middlewareOne(middleware.IsAuth(deleteDocByUser))).Methods("DELETE")
	log.Fatal(http.ListenAndServe(":8080", myRouter))
}

//
func NewRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)

	// Choose the folder to serve
	staticDir := "/static/" ///static/

	// Create the route
	router.
		PathPrefix("/static").
		Handler(http.StripPrefix(staticDir, http.FileServer(http.Dir("."+staticDir))))

	return router
}
