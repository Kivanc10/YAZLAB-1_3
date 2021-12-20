package db

import (
	"YAZLAB3MONGO/student"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/ledongthuc/pdf"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"

	"golang.org/x/crypto/bcrypt"
)

var mySigningKey = []byte("captainjacksparrowsayshi")

func CreateToken(name, lastname, number string) (string, error) {
	var err error
	//Creating Access Token
	os.Setenv("ACCESS_SECRET", string(mySigningKey)) //this should be in an env file
	atClaims := jwt.MapClaims{}
	atClaims["authorized"] = true
	//atClaims["user_id"] = userId
	atClaims["user_name"] = name
	atClaims["last_name"] = lastname
	atClaims["number"] = number
	atClaims["exp"] = time.Now().Add(time.Minute * 1500).Unix()
	at := jwt.NewWithClaims(jwt.SigningMethodHS256, atClaims)
	token, err := at.SignedString([]byte(os.Getenv("ACCESS_SECRET")))
	if err != nil {
		return "", errors.New("an error occured during the create token")
	}
	fmt.Println("jwt map --> ", atClaims)
	return token, nil
}

func ConnectToMongoDb() *mongo.Client {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://mongo:27017/")) //write 127.0.0.1 insted mongo for localhost
	if err != nil {
		panic(err)
	}
	err = client.Ping(ctx, nil) // to check there is an error caused by interrupting

	if err != nil {
		panic(err)
	}
	return client
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func checkPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func isAlreadyExist(name string, client *mongo.Client) (student.Student, bool) {
	collection := client.Database("User").Collection("token")
	temp := bson.M{"name": bson.M{"$eq": name}}
	result := student.Student{}
	err := collection.FindOne(context.Background(), temp).Decode(&result)
	if err != nil {
		return student.Student{}, false
	} else {
		return result, true
	}

}

func insertInto(client *mongo.Client, stndt student.Student) error {
	collection := client.Database("User").Collection("token")
	hash, err := hashPassword(stndt.Password)
	if err != nil {
		log.Fatal("an error occured during hashed the password")
	}
	//tempPswrd := stndt.Password
	stndt.Password = hash // --

	addTokenToPerson(&stndt)
	//stndt.Password = tempPswrd
	toSave, err := bson.Marshal(stndt)
	if err != nil {
		log.Fatal("an error occured during the marshalling")
		return err
	}
	if _, result := isAlreadyExist(stndt.Name, client); !result {
		res, err := collection.InsertOne(context.Background(), toSave)
		if err != nil {
			panic(err)
		}
		id := res.InsertedID
		fmt.Println("id --> ", id)
		return nil
	} else {
		return errors.New("The user is already exist")
	}
}

func addTokenToPerson(stdnt *student.Student) string {
	token, err := CreateToken(stdnt.Name, stdnt.Lastname, stdnt.Number)
	if err != nil {
		log.Fatal("An error occured during the produce token ", err)
	}
	temp := student.Token{Context: token}
	stdnt.Tokens = append(stdnt.Tokens, temp)
	os.Setenv("Token", stdnt.Tokens[0].Context)
	os.Setenv("userName", stdnt.Name)
	return token
}

func AddUser(username, lastname, password, number, schoolType string, client *mongo.Client) (*student.Student, error) {
	var stndt student.Student
	stndt.Name = username
	stndt.Lastname = lastname
	stndt.Password = password
	stndt.Number = number
	stndt.Type = schoolType
	addTokenToPerson(&stndt)
	if err := insertInto(client, stndt); err != nil {
		log.Println("an error occured during the inserting ", err)
		return &student.Student{}, err
	}
	return &stndt, nil
}

func Login(password, number string, client *mongo.Client) (*student.Student, error) {
	collection := client.Database("User").Collection("token")
	filter := bson.M{"number": bson.M{"$eq": number}}
	var stndt student.Student
	err := collection.FindOne(context.Background(), filter).Decode(&stndt)
	if err != nil {
		return &student.Student{}, errors.New("Önce kayıt ol")
	} else {
		if !checkPassword(password, stndt.Password) {
			return &student.Student{}, errors.New("Parolan YANLIŞ")
		}
		addTokenToPerson(&stndt)
		err = addTokenForLogin(client, &stndt)
		if err != nil {
			return &student.Student{}, errors.New("Error occured when updating token")
		}
		return &stndt, nil
	}
}

func addTokenForLogin(client *mongo.Client, stdnt *student.Student) error {
	collection := client.Database("User").Collection("token")
	filter := bson.M{"name": stdnt.Name}
	update := bson.M{"$set": bson.M{
		"tokens": stdnt.Tokens}}
	res := collection.FindOneAndUpdate(context.Background(), filter, update)
	resDecoded := student.Student{}
	err := res.Decode(&resDecoded)
	return err
}

func SignInForAdmin(username, password string) *student.Admin {
	var admin student.Admin

	admin.Name = username
	admin.Password = password
	return &admin
}

func GetAllUsers(client *mongo.Client) []bson.M {
	collection := client.Database("User").Collection("token")

	cursor, err := collection.Find(context.Background(), bson.M{})

	if err != nil {
		log.Fatal(err)
	}

	var students_b []bson.M

	if err = cursor.All(context.Background(), &students_b); err != nil {
		log.Fatal(err)
	}
	return students_b
}

func DeleteUser(client *mongo.Client, name string) error {
	collection := client.Database("User").Collection("token")
	filter := bson.M{"name": name}
	res := collection.FindOneAndDelete(context.Background(), filter)
	resDecoded := student.Student{}
	err := res.Decode(&resDecoded)
	if err != nil {
		log.Printf("an error occured during the delete user")
		return err
	}
	return nil

}

func DeleteUsersAllDocsWithUserName(client *mongo.Client, givenName string) {
	collection := client.Database("User").Collection("token")
	filter1 := bson.M{"name": givenName}
	res := collection.FindOne(context.Background(), filter1)
	var user student.Student
	err := res.Decode(&user)
	if err != nil {
		log.Fatal(err)
	}
	collection = client.Database("User").Collection("fs.files")
	cursor, err := collection.Find(context.Background(), bson.M{})
	if err != nil {
		log.Fatal(err)
	}
	var docs []bson.M
	if err = cursor.All(context.Background(), &docs); err != nil {
		log.Fatal(err)
	}
	for _, i := range docs {
		var temp student.Doc
		bsonBytes, _ := bson.Marshal(i)
		bson.Unmarshal(bsonBytes, &temp)
		if temp.MetaData.OwnerName == user.Name {
			//tempDoc = append(tempDoc, temp)
			fmt.Println("doc filename --> ", temp.FileName)
			fmt.Println("evet query is active")
			collection.FindOneAndDelete(context.Background(), bson.M{"filename": temp.FileName})
		}
	}

}

func UpdateUserByItself(client *mongo.Client, givenName, username, lastname, number, typeOf string) *student.Student {
	collection := client.Database("User").Collection("token")
	filter := bson.M{"name": bson.M{"$eq": givenName}}
	res := collection.FindOne(context.Background(), filter)
	var currentUser student.Student
	if err := res.Decode(&currentUser); err != nil {
		log.Fatal(err)
		return &student.Student{}
	}
	//fmt.Println("got username --> ", username)
	newStdnt := student.Student{}
	newStdnt.Name = username
	newStdnt.Lastname = lastname
	newStdnt.Number = number
	newStdnt.Type = typeOf
	newStdnt.Tokens = currentUser.Tokens
	newStdnt.Password = currentUser.Password
	fmt.Println("intermediate stdnt -> ", newStdnt)
	res = collection.FindOneAndUpdate(context.Background(), filter, bson.M{
		"$set": bson.M{
			"name":     newStdnt.Name,
			"lastname": newStdnt.Lastname,
			"number":   newStdnt.Number,
			"type":     newStdnt.Type,
		}})
	resDecoded := student.Student{}
	err := res.Decode(&resDecoded)
	if err != nil {
		panic(err)
	}
	//	os.Setenv("userName", newStdnt.Name)
	//	os.Setenv("userName",)
	// change docs name
	// -------------------------------------------------

	collection = client.Database("User").Collection("fs.files")
	var allDocs []bson.M
	cursor, err := collection.Find(context.Background(), bson.M{})
	if err != nil {
		log.Fatal(err)
	}
	if err = cursor.All(context.Background(), &allDocs); err != nil {
		log.Fatal(err)
	}
	var toReturn []student.Doc
	fmt.Println("allDocs --> ", allDocs)
	//bsonBytes, _ := bson.Marshal(allDocs)
	//bson.Unmarshal(bsonBytes, &toReturn)

	fmt.Println("toReturn data -> ", toReturn)
	for _, i := range allDocs {
		var temp student.Doc
		bsonBytes, _ := bson.Marshal(i)
		bson.Unmarshal(bsonBytes, &temp)
		fmt.Println("temp student ----> ", temp)
		if temp.MetaData.OwnerName == os.Getenv("userName") {
			filter := bson.M{"filename": temp.FileName}
			collection.FindOneAndUpdate(context.Background(), filter, bson.M{
				"$set": bson.M{
					"length":     temp.Length,
					"chunkSize":  temp.ChunkSize,
					"uploadDate": temp.UploadDate,
					"fileName":   temp.FileName,
					"metadata": bson.M{
						"owner_name":   newStdnt.Name,
						"deploy_date":  temp.MetaData.DeployDate,
						"project_name": temp.MetaData.ProjectName,
						"lesson_name":  temp.MetaData.Lesson,
						"summary":      temp.MetaData.Summary,
						"keywords":     temp.MetaData.KeyWords,
						"juri":         temp.MetaData.Juri,
						"No":           temp.MetaData.No,
						"typeOfDoc":    temp.MetaData.Type,
					}}})
			// "$set": bson.M{
			// 	"metadata": bson.M{"owner_name": os.Getenv("userName"), "deploy_date": temp.MetaData.DeployDate},
			// }})
		}
	}
	// -------------------------------------------------
	fmt.Println("new stdnt ", resDecoded)
	os.Setenv("userName", newStdnt.Name)
	fmt.Println("saved set env name --> ", os.Getenv("userName"))
	return &resDecoded

}

func UpdateUserByAdmin(client *mongo.Client, givenId, username, lastname, number, typeOf string) *student.Student {
	collection := client.Database("User").Collection("token")
	hexByte, err := primitive.ObjectIDFromHex(givenId)
	if err != nil {
		panic(err)
	}

	filter1 := bson.M{"_id": bson.M{"$eq": hexByte}}
	res := collection.FindOne(context.Background(), filter1)
	var stdnt student.Student
	if err := res.Decode(&stdnt); err != nil {
		log.Fatal(err)
		return &student.Student{}
	}
	fmt.Println("founded user ", stdnt)
	newStdnt := student.Student{}
	newStdnt.Name = username
	newStdnt.Lastname = lastname
	newStdnt.Number = number
	newStdnt.Type = typeOf
	newStdnt.Tokens = stdnt.Tokens
	newStdnt.Password = stdnt.Password
	res = collection.FindOneAndUpdate(context.Background(), filter1, bson.M{
		"$set": bson.M{
			"name":     newStdnt.Name,
			"lastname": newStdnt.Lastname,
			"number":   newStdnt.Number,
			"type":     newStdnt.Type,
		}})

	resDecoded := student.Student{}
	err = res.Decode(&resDecoded)
	if err != nil {
		panic(err)
	}
	fmt.Println("new stdnt ", resDecoded)
	// to update users docs
	collection = client.Database("User").Collection("fs.files")
	var allDocs []bson.M
	cursor, err := collection.Find(context.Background(), bson.M{})
	if err != nil {
		log.Fatal(err)
	}
	if err = cursor.All(context.Background(), &allDocs); err != nil {
		log.Fatal(err)
	}
	//var toReturn []student.Doc
	for _, i := range allDocs {
		var temp student.Doc
		bsonBytes, _ := bson.Marshal(i)
		bson.Unmarshal(bsonBytes, &temp)
		if temp.MetaData.OwnerName == stdnt.Name {
			fmt.Println("EVET BIR TANE VARRRR")
			filter := bson.M{"filename": temp.FileName}
			collection.FindOneAndUpdate(context.Background(), filter, bson.M{
				"$set": bson.M{
					"length":     temp.Length,
					"chunkSize":  temp.ChunkSize,
					"uploadDate": temp.UploadDate,
					"filename":   temp.FileName,
					"metadata": bson.M{
						"owner_name":   newStdnt.Name,
						"deploy_date":  temp.MetaData.DeployDate,
						"project_name": temp.MetaData.ProjectName,
						"lesson_name":  temp.MetaData.Lesson,
						"summary":      temp.MetaData.Summary,
						"keywords":     temp.MetaData.KeyWords,
						"juri":         temp.MetaData.Juri,
						"No":           temp.MetaData.No,
					}}})
		}
	}
	return &resDecoded
}

var deploy_date string

func UploadFile(client *mongo.Client, file multipart.File, filename, owner string) { //a multipart.File
	//data, err := ioutil.ReadFile(file)
	data, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatal(err)
	}
	result := downloadItForTemp(data)
	//fmt.Println("data ---> ", data)
	//collection := client.Database("User").Collection("pdfs")
	bucket, err := gridfs.NewBucket(
		client.Database("User"),
	)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	//deploy_date = "2018_2019_BAHAR" // --
	// deploy_date = "2018-2019 GÜZ" // --
	// lesson_name := "programming"
	// summary := "sample summary"
	// typeOfDoc := "araştırma" // it is extracted from pdf
	//result["keywords"]
	var keywords []student.KeyWord
	if _, ok := result["keywords"].([]string); ok {
		keywords = resToKeyword(result["keywords"].([]string))
	}

	uploadOpts := options.GridFSUpload().
		//SetMetadata(bson.D{{"owner_name", owner}, {"deploy_date", deploy_date}}) //, {"deploy_date", deploy_date}
		SetMetadata(bson.M{
			"owner_name":   owner,
			"deploy_date":  result["deploy_date"],  // --
			"project_name": result["project_name"], // --
			"keywords":     keywords,
			"lesson_name":  result["project_name"],
			"summary":      result["summary"],
			"typeOfDoc":    result["type"],
			"juri":         result["juri"],
			"No":           result["No"],
		})
	uploadStream, err := bucket.OpenUploadStream(
		filename,
		uploadOpts,
	)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer uploadStream.Close()

	fileSize, err := uploadStream.Write(data)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	log.Printf("Write file to DB was successful. File size: %d M\n", fileSize)

}

func DownloadDocs(client *mongo.Client, filename string) { //, filename string
	collection := client.Database("User").Collection("fs.files")
	//var results bson.M
	cursor, err := collection.Find(context.Background(), bson.M{})
	if err != nil {
		log.Fatal(err)
	}
	var docs []bson.M

	if err = cursor.All(context.Background(), &docs); err != nil {
		log.Fatal(err)
	}
	fmt.Println("results --> ", docs)
	bucket, _ := gridfs.NewBucket(
		client.Database("User"),
	)
	var buf bytes.Buffer
	dStream, err := bucket.DownloadToStreamByName(filename, &buf)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("File size to download: %v\n", dStream)
	ioutil.WriteFile("./static/images/"+filename, buf.Bytes(), 0600)

}
func DisplayImageById(client *mongo.Client, givenId string) string { // filename
	collection := client.Database("User").Collection("fs.files")
	hexByte, err := primitive.ObjectIDFromHex(givenId)
	if err != nil {
		panic(err)
	}
	filter1 := bson.M{"_id": bson.M{"$eq": hexByte}}
	res := collection.FindOne(context.Background(), filter1)
	var doc student.Doc
	if err := res.Decode(&doc); err != nil { // to get doc object
		log.Fatal(err)
	}
	bucket, _ := gridfs.NewBucket(
		client.Database("User"),
	)
	var buf bytes.Buffer
	dStream, err := bucket.DownloadToStreamByName(doc.FileName, &buf)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("File size to download: %v\n", dStream)
	ioutil.WriteFile("./static/images/"+doc.FileName, buf.Bytes(), 0600)
	return doc.FileName
}

func GetAllDocsByAdmin(client *mongo.Client, myUrl string) []bson.M {
	collection := client.Database("User").Collection("fs.files")
	u, err := url.Parse(myUrl)
	if err != nil {
		log.Fatal(err)
	}
	var docs []bson.M
	var toReturn []student.Doc
	m, _ := url.ParseQuery(u.RawQuery)
	if len(m) == 0 { // there is no query
		cursor, err := collection.Find(context.Background(), bson.M{})
		if err != nil {
			log.Fatal(err)
		}
		if err = cursor.All(context.Background(), &docs); err != nil {
			log.Fatal(err)
		}
		var tempDoc []student.Doc
		for _, i := range docs {
			var temp student.Doc
			bsonBytes, _ := bson.Marshal(i)
			bson.Unmarshal(bsonBytes, &temp)
			tempDoc = append(tempDoc, temp)
		}
		if tempDoc == nil {
			docs = nil
		} else {
			docs = nil
			for _, i := range tempDoc {
				var temp bson.M
				data, err := bson.Marshal(i)
				if err != nil {
					log.Fatal(err)
				}
				err = bson.Unmarshal(data, &temp)
				if err != nil {
					log.Fatal(err)
				}
				docs = append(docs, temp)
			}
		}

	} else if len(m) == 1 { // there is least one query
		fmt.Println("one query--------------")
		for k := range m {
			var allDocs []bson.M
			cursor, err := collection.Find(context.Background(), bson.M{})
			if err != nil {
				log.Fatal(err)
			}
			if err = cursor.All(context.Background(), &allDocs); err != nil {
				log.Fatal(err)
			}
			//var toReturn []student.Doc
			for _, i := range allDocs {
				var temp student.Doc
				bsonBytes, _ := bson.Marshal(i)
				bson.Unmarshal(bsonBytes, &temp)
				if (k == "deploy_date") && (temp.MetaData.DeployDate == m[k][0]) {
					//filter := bson.M{"filename": temp.FileName}
					toReturn = append(toReturn, temp)
				}
				if (k == "project_name") && temp.MetaData.ProjectName == m[k][0] {
					toReturn = append(toReturn, temp)
				}
				if (k == "keywords") && student.IsInsideOfKey(temp.MetaData.KeyWords, m[k][0]) {
					toReturn = append(toReturn, temp)
				}
				if (k == "lesson_name") && temp.MetaData.Lesson == m[k][0] {
					toReturn = append(toReturn, temp)
				}
				if (k == "owner_name") && temp.MetaData.OwnerName == m[k][0] {
					toReturn = append(toReturn, temp)
				}
				if (k == "typeOfDoc") && temp.MetaData.Type == m[k][0] {
					toReturn = append(toReturn, temp)
				}

			}
			//
			// for _, i := range toReturn {
			// 	var temp bson.M
			// 	data, err := bson.Marshal(i)
			// 	if err != nil {
			// 		log.Fatal(err)
			// 	}
			// 	err = bson.Unmarshal(data, &temp)
			// 	if err != nil {
			// 		log.Fatal(err)
			// 	}
			// 	docs = append(docs, temp)
			// }
			//
		}
	} else if len(m) == 2 {
		fmt.Println("two queries--------------")
		for k := range m {
			var allDocs []bson.M
			cursor, err := collection.Find(context.Background(), bson.M{})
			if err != nil {
				log.Fatal(err)
			}
			if err = cursor.All(context.Background(), &allDocs); err != nil {
				log.Fatal(err)
			}
			fmt.Println("k ---> ", k)

			for _, i := range allDocs {
				var temp student.Doc
				bsonBytes, _ := bson.Marshal(i)
				bson.Unmarshal(bsonBytes, &temp)
				if m.Has("owner_name") {
					if (temp.MetaData.OwnerName == m["owner_name"][0]) && (m.Has("deploy_date")) {
						if m["deploy_date"][0] == temp.MetaData.DeployDate {
							if !IsExist(toReturn, temp) {
								toReturn = append(toReturn, temp)
							}
						}
					}
					if (temp.MetaData.OwnerName == m["owner_name"][0]) && (m.Has("lesson_name")) {
						if m["lesson_name"][0] == temp.MetaData.Lesson {
							if !IsExist(toReturn, temp) {
								toReturn = append(toReturn, temp)
							}
						}
					}

					if (temp.MetaData.OwnerName == m["owner_name"][0]) && (m.Has("keywords")) {
						if student.IsInsideOfKey(temp.MetaData.KeyWords, m["keywords"][0]) { // student.IsInsideOfKey(temp.MetaData.KeyWords, m[k][0])
							if !IsExist(toReturn, temp) {
								toReturn = append(toReturn, temp)
							}
						}
					}

					if (temp.MetaData.OwnerName == m["owner_name"][0]) && (m.Has("juri")) {
						if isJuriThere(temp.MetaData.Juri, m["juri"][0]) { // isJuriThere
							if !IsExist(toReturn, temp) {
								toReturn = append(toReturn, temp)
							}
						}
					}

					if (temp.MetaData.OwnerName == m["owner_name"][0]) && (m.Has("No")) {
						if m["No"][0] == temp.MetaData.No {
							if !IsExist(toReturn, temp) {
								toReturn = append(toReturn, temp)
							}
						}
					}
				}
				// ------------
				if m.Has("deploy_date") {
					if (temp.MetaData.DeployDate == m["deploy_date"][0]) && m.Has("typeOfDoc") {
						if m["typeOfDoc"][0] == temp.MetaData.Type {
							if !IsExist(toReturn, temp) {
								toReturn = append(toReturn, temp)
							}
						}
					}
					if (temp.MetaData.DeployDate == m["deploy_date"][0]) && m.Has("lesson_name") {
						if m["lesson_name"][0] == temp.MetaData.Lesson {
							if !IsExist(toReturn, temp) {
								toReturn = append(toReturn, temp)
							}
						}
					}
					if (temp.MetaData.DeployDate == m["deploy_date"][0]) && m.Has("juri") {
						if isJuriThere(temp.MetaData.Juri, m["juri"][0]) {
							if !IsExist(toReturn, temp) {
								toReturn = append(toReturn, temp)
							}
						}
					}
				}
				if m.Has("typeOfDoc") {
					if (temp.MetaData.Type == m["typeOfDoc"][0]) && m.Has("deploy_date") {
						if m["deploy_date"][0] == temp.MetaData.DeployDate {
							if !IsExist(toReturn, temp) {
								toReturn = append(toReturn, temp)
							}
						}
					}
					if (temp.MetaData.Type == m["typeOfDoc"][0]) && m.Has("owner_name") {
						if m["owner_name"][0] == temp.MetaData.OwnerName {
							if !IsExist(toReturn, temp) {
								toReturn = append(toReturn, temp)
							}
						}
					}
					if (temp.MetaData.Type == m["typeOfDoc"][0]) && m.Has("lesson_name") {
						if m["lesson_name"][0] == temp.MetaData.Lesson {
							if !IsExist(toReturn, temp) {
								toReturn = append(toReturn, temp)
							}
						}
					}
					if (temp.MetaData.Type == m["typeOfDoc"][0]) && m.Has("juri") {
						if isJuriThere(temp.MetaData.Juri, m["juri"][0]) {
							if !IsExist(toReturn, temp) {
								toReturn = append(toReturn, temp)
							}
						}
					}
				}
			}
		}
	} else if len(m) == 3 { // olduu
		//fmt.Println("theee queries--------------")
		for k := range m {
			var allDocs []bson.M
			cursor, err := collection.Find(context.Background(), bson.M{})
			if err != nil {
				log.Fatal(err)
			}
			if err = cursor.All(context.Background(), &allDocs); err != nil {
				log.Fatal(err)
			}
			fmt.Println("k ---> ", k)
			for _, i := range allDocs {
				var temp student.Doc
				bsonBytes, _ := bson.Marshal(i)
				bson.Unmarshal(bsonBytes, &temp)
				if temp.MetaData.OwnerName == m["owner_name"][0] && (m.Has("deploy_date") && m.Has("typeOfDoc")) {
					if (m["deploy_date"][0] == temp.MetaData.DeployDate) && (m["typeOfDoc"][0] == temp.MetaData.Type) {
						if !IsExist(toReturn, temp) {
							toReturn = append(toReturn, temp)
						}
						//toReturn = append(toReturn, temp)
					}
				}
				if (temp.MetaData.OwnerName == m["owner_name"][0]) && (m.Has("lesson_name") && m.Has("typeOfDoc")) {
					if (m["lesson_name"][0] == temp.MetaData.Lesson) && (m["typeOfDoc"][0] == temp.MetaData.Type) {
						if !IsExist(toReturn, temp) {
							toReturn = append(toReturn, temp)
						}
					}
				}
				if temp.MetaData.OwnerName == m["owner_name"][0] && (m.Has("lesson_name") && m.Has("deploy_date")) {
					if (m["lesson_name"][0] == temp.MetaData.Lesson) && (m["deploy_date"][0] == temp.MetaData.DeployDate) {
						if !IsExist(toReturn, temp) {
							toReturn = append(toReturn, temp)
						}
						//toReturn = append(toReturn, temp)
					}
				}
			}
		}
	}
	for _, i := range toReturn {
		var temp bson.M
		data, err := bson.Marshal(i)
		if err != nil {
			log.Fatal(err)
		}
		err = bson.Unmarshal(data, &temp)
		if err != nil {
			log.Fatal(err)
		}
		docs = append(docs, temp)
	}
	return docs
}

func DeleteUsersAllDocsById(client *mongo.Client, givenId string) {
	collection := client.Database("User").Collection("token")
	hexByte, err := primitive.ObjectIDFromHex(givenId)
	if err != nil {
		log.Fatal(err)
	}
	res := collection.FindOne(context.Background(), bson.M{"_id": hexByte})

	var user student.Student
	err = res.Decode(&user)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("given user name --> ", user.Name, " ", user.Lastname)

	// if err = cursor.All(context.Background(), &user); err != nil {
	// 	log.Fatal(err)
	// }
	collection = client.Database("User").Collection("fs.files")
	cursor, err := collection.Find(context.Background(), bson.M{})
	if err != nil {
		log.Fatal(err)
	}
	var docs []bson.M
	if err = cursor.All(context.Background(), &docs); err != nil {
		log.Fatal(err)
	}
	//var tempDoc []student.Doc
	for _, i := range docs {
		var temp student.Doc
		bsonBytes, _ := bson.Marshal(i)
		bson.Unmarshal(bsonBytes, &temp)
		if temp.MetaData.OwnerName == user.Name {
			//tempDoc = append(tempDoc, temp)
			fmt.Println("doc filename --> ", temp.FileName)
			fmt.Println("evet query is active")
			collection.FindOneAndDelete(context.Background(), bson.M{"filename": temp.FileName})
		}
	}

}

func AccessAllDocs(client *mongo.Client, myUrl string) []bson.M {
	collection := client.Database("User").Collection("fs.files")
	fmt.Println("ownername --> ", os.Getenv("userName"))
	u, err := url.Parse(myUrl)
	if err != nil {
		log.Fatal(err)
	}
	var docs []bson.M
	var toReturn []student.Doc
	m, _ := url.ParseQuery(u.RawQuery)

	if len(m) == 0 {
		cursor, err := collection.Find(context.Background(), bson.M{}) //bson.M{"metadata": bson.M{}}
		if err != nil {
			log.Fatal(err)
		}
		//var docs []student.Doc
		if err = cursor.All(context.Background(), &docs); err != nil {
			log.Fatal(err)
		}
		var tempDoc []student.Doc
		for _, i := range docs {
			var temp student.Doc
			bsonBytes, _ := bson.Marshal(i)
			bson.Unmarshal(bsonBytes, &temp)
			if temp.MetaData.OwnerName == os.Getenv("userName") {
				fmt.Println("hereeeeeeeeeeeeeeeeeeeeeee")
				tempDoc = append(tempDoc, temp)
			}
		}
		if tempDoc == nil {
			docs = nil
		} else {
			docs = nil
			fmt.Println("tempdoc --> ", tempDoc)
			for _, i := range tempDoc {
				var temp bson.M
				data, err := bson.Marshal(i)
				if err != nil {
					log.Fatal(err)
				}
				err = bson.Unmarshal(data, &temp)
				if err != nil {
					log.Fatal(err)
				}
				docs = append(docs, temp)
			}
		}
		fmt.Println("new docs --> ", docs)
		//return docs
	} else if len(m) == 1 {
		fmt.Println("query is active")
		fmt.Println("m -->", m)
		for k := range m {
			fmt.Println("I'm in")
			fmt.Println("k -->", k)
			var allDocs []bson.M
			cursor, err := collection.Find(context.Background(), bson.M{})
			if err != nil {
				log.Fatal(err)
			}
			if err = cursor.All(context.Background(), &allDocs); err != nil {
				log.Fatal(err)
			}

			for _, i := range allDocs {
				var temp student.Doc
				bsonBytes, _ := bson.Marshal(i)
				bson.Unmarshal(bsonBytes, &temp)

				if (temp.MetaData.OwnerName == os.Getenv("userName")) && (k == "deploy_date") && temp.MetaData.DeployDate == m[k][0] {
					toReturn = append(toReturn, temp)
				}
				if (temp.MetaData.OwnerName == os.Getenv("userName")) && (k == "project_name") && temp.MetaData.ProjectName == m[k][0] {
					toReturn = append(toReturn, temp)
				}
				if (temp.MetaData.OwnerName == os.Getenv("userName")) && (k == "keywords") && student.IsInsideOfKey(temp.MetaData.KeyWords, m[k][0]) {
					toReturn = append(toReturn, temp)
				}
				if (temp.MetaData.OwnerName == os.Getenv("userName")) && (k == "lesson_name") && temp.MetaData.Lesson == m[k][0] {
					toReturn = append(toReturn, temp)
				}
				if (temp.MetaData.OwnerName == os.Getenv("userName")) && (k == "owner_name") && temp.MetaData.OwnerName == m[k][0] {
					toReturn = append(toReturn, temp)
				}
				if (temp.MetaData.OwnerName == os.Getenv("userName")) && (k == "typeOfDoc") && temp.MetaData.Type == m[k][0] {
					toReturn = append(toReturn, temp)
				}
			}
			//fmt.Println(toReturn)
			// doc to bson

			//doc to bson
		}
		//return nil
	} else if len(m) == 2 {
		for k := range m {
			var allDocs []bson.M
			cursor, err := collection.Find(context.Background(), bson.M{})
			if err != nil {
				log.Fatal(err)
			}
			if err = cursor.All(context.Background(), &allDocs); err != nil {
				log.Fatal(err)
			}
			fmt.Println("k ---> ", k)
			for _, i := range allDocs {
				var temp student.Doc
				bsonBytes, _ := bson.Marshal(i)
				bson.Unmarshal(bsonBytes, &temp)
				if (temp.MetaData.OwnerName == os.Getenv("userName")) && (m.Has("deploy_date") && m.Has("typeOfDoc")) {
					//filter := bson.M{"filename": temp.FileName}
					//toReturn = append(toReturn, temp)
					if (m["deploy_date"][0] == temp.MetaData.DeployDate) && (m["typeOfDoc"][0] == temp.MetaData.Type) {
						if !IsExist(toReturn, temp) {
							toReturn = append(toReturn, temp)
						}
						//toReturn = append(toReturn, temp)
					}
				}
				if (temp.MetaData.OwnerName == os.Getenv("userName")) && (m.Has("lesson_name") && m.Has("typeOfDoc")) {
					if (m["lesson_name"][0] == temp.MetaData.Lesson) && (m["typeOfDoc"][0] == temp.MetaData.Type) {
						if !IsExist(toReturn, temp) {
							toReturn = append(toReturn, temp)
						}
						//toReturn = append(toReturn, temp)
					}
				}
				if (temp.MetaData.OwnerName == os.Getenv("userName")) && (m.Has("lesson_name") && m.Has("deploy_date")) {
					if (m["lesson_name"][0] == temp.MetaData.Lesson) && (m["deploy_date"][0] == temp.MetaData.DeployDate) {
						if !IsExist(toReturn, temp) {
							toReturn = append(toReturn, temp)
						}
						//toReturn = append(toReturn, temp)
					}
				}
			}
		}

	}
	for _, i := range toReturn {
		var temp bson.M
		data, err := bson.Marshal(i)
		if err != nil {
			log.Fatal(err)
		}
		err = bson.Unmarshal(data, &temp)
		if err != nil {
			log.Fatal(err)
		}
		docs = append(docs, temp)
	}
	return docs
}
func IsExist(doc []student.Doc, temp student.Doc) bool {
	for _, i := range doc {
		if (i.FileName == temp.FileName) && i.Id == temp.Id {
			return true
		}
	}
	return false
}

//---------------------------------------------------------------------
func DeleteWithAdmin(client *mongo.Client, givenId string) {
	collection := client.Database("User").Collection("token")
	hexByte, err := primitive.ObjectIDFromHex(givenId)
	if err != nil {
		panic(err)
	}

	filter1 := bson.M{"_id": bson.M{"$eq": hexByte}}
	res := collection.FindOne(context.Background(), filter1)

	var stdnt student.Student
	if err := res.Decode(&stdnt); err != nil {
		log.Fatal(err)
		//return &student.Student{}
	}

	res = collection.FindOneAndDelete(context.Background(), filter1)

}

func DeleteDocByAdminClick(client *mongo.Client, givenId string) {
	collection := client.Database("User").Collection("fs.files")
	hexByte, err := primitive.ObjectIDFromHex(givenId)
	if err != nil {
		panic(err)
	}
	filter1 := bson.M{"_id": bson.M{"$eq": hexByte}}
	res := collection.FindOne(context.Background(), filter1)
	var doc student.Doc
	if err := res.Decode(&doc); err != nil {
		log.Fatal(err)
		//return &student.Student{}
	}
	res = collection.FindOneAndDelete(context.Background(), filter1)

}

func DeleteDocByUser(client *mongo.Client, givenId string) {
	collection := client.Database("User").Collection("fs.files")
	hexByte, err := primitive.ObjectIDFromHex(givenId)
	if err != nil {
		panic(err)
	}
	filter1 := bson.M{"_id": bson.M{"$eq": hexByte}}
	res := collection.FindOne(context.Background(), filter1)
	var doc student.Doc
	if err := res.Decode(&doc); err != nil {
		log.Fatal(err)
		//return &student.Student{}
	}
	res = collection.FindOneAndDelete(context.Background(), filter1)
}

func downloadItForTemp(data []byte) map[string]interface{} {
	tmpFile, err := ioutil.TempFile(os.TempDir(), "prefix-*.pdf")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	//fmt.Println("Created File: " + tmpFile.Name())
	if _, err = tmpFile.Write(data); err != nil {
		log.Fatal("failed to write bro")
	}
	f, err := os.Open(tmpFile.Name())
	if err != nil {
		panic(err)
	}
	defer f.Close()

	//tmpFile.Seek(0, 0)
	// s := bufio.NewScanner(tmpFile)
	// fmt.Println("content ---> ")
	// for s.Scan() {
	// 	fmt.Println(s.Text())
	// }
	if err := tmpFile.Close(); err != nil {
		log.Fatal(err)
	}
	pdf.DebugOn = true
	content, err := readPdf(tmpFile.Name()) // Read local pdf file
	if err != nil {
		panic(err)
	}
	result := map[string]interface{}{}
	//fmt.Println(content[0:400])
	if strings.Contains(content, "BİTİRME") {
		//fmt.Println("Bitirme projesi")
		result["type"] = "bitirme"
	}
	if strings.Contains(content, strings.ToUpper("Araştırma")) {
		//	fmt.Println("Araştırma projesi")
		result["type"] = "araştırma"
	}
	// juri

	var counselours []string
	if result["type"] == "bitirme" {
		if strings.Contains(content, "BİTİRME") {
			startIndex := strings.Index(content, "BİTİRME")
			if strings.Contains(content, "Tezin Savunulduğu") {
				endIndex := strings.Index(content, "Tezin Savunulduğu")
				//fmt.Println(content[startIndex:endIndex])
				field := content[startIndex:endIndex]
				for {
					if strings.Contains(field, "....") {
						field = strings.Replace(field, "..", "", 1)
					} else {
						break
					}
				}
				if strings.Contains(field, "Prof") {
					profIndex := strings.Index(field, "Prof")
					//fmt.Println("prof -->")
					//startIndex := field[profIndex:]
					//fmt.Println(field[profIndex:])
					newDoc := field[profIndex:]
					//fmt.Println("//////")
					//fmt.Println(newDoc)
					if strings.Contains(newDoc, "Danışman") {
						endIndex := strings.Index(newDoc, "Danışman")
						//	fmt.Println(newDoc[:endIndex])
						if !isExist(counselours, strings.Trim(newDoc[:endIndex], " ")) && !strings.Contains(strings.Trim(newDoc[:endIndex], " "), "Jüri") {
							counselours = append(counselours, strings.Trim(newDoc[:endIndex], " "))
						}

					}
					if strings.Contains(newDoc, "Jüri") {
						endIndex := strings.Index(newDoc, "Jüri")
						//fmt.Println(newDoc[:endIndex])
						if !isExist(counselours, strings.Trim(newDoc[:endIndex], " ")) && !strings.Contains(strings.Trim(newDoc[:endIndex], " "), "Danışman") {
							counselours = append(counselours, strings.Trim(newDoc[:endIndex], " "))

						}
					}

				}
				if strings.Contains(field, "Doç") {
					fmt.Println("doç --_>")
					docIndex := strings.Index(field, "Doç")
					//fmt.Println(field[docIndex:])
					newDoc := field[docIndex:]

					if strings.Contains(newDoc, "Jüri") {
						fmt.Println("until juri doç")
						juriIndex := strings.Index(newDoc, "Jüri")
						//	fmt.Println(newDoc[:juriIndex])
						if !isExist(counselours, strings.Trim(newDoc[:juriIndex], " ")) && !strings.Contains(strings.Trim(newDoc[:juriIndex], " "), "Danışman") {
							counselours = append(counselours, strings.Trim(newDoc[:juriIndex], " "))

						}

					} else if strings.Contains(newDoc, "Danışman") {
						//fmt.Println("until danışman doç")
						counselorIndex := strings.Index(newDoc, "Danışman")
						//fmt.Println(newDoc[:counselorIndex])
						if !isExist(counselours, strings.Trim(newDoc[:counselorIndex], " ")) && !strings.Contains(strings.Trim(newDoc[:counselorIndex], " "), "Jüri") {
							counselours = append(counselours, strings.Trim(newDoc[:counselorIndex], " "))
						}

					}

				}
				if strings.Contains(field, "Dr. Öğr") || strings.Contains(field, "Dr.Öğr") {
					//fmt.Println("dr ---> ")
					var drIndex int
					if strings.Contains(field, "Dr. Öğr") {
						drIndex = strings.Index(field, "Dr. Öğr")
					} else if strings.Contains(field, "Dr.Öğr") {
						drIndex = strings.Index(field, "Dr.Öğr")
					}
					newDoc := field[drIndex:]
					//	fmt.Println(newDoc)
					if strings.Contains(newDoc, "Jüri") {
						endIndex := strings.Index(newDoc, "Jüri")
						//fmt.Println(newDoc[:endIndex])
						if !isExist(counselours, strings.Trim(newDoc[:endIndex], " ")) && !strings.Contains(strings.Trim(newDoc[:endIndex], " "), "Danışman") {
							counselours = append(counselours, strings.Trim(newDoc[:endIndex], " "))
						}
					} else if strings.Contains(newDoc, "Danışman") {
						endIndex := strings.Index(newDoc, "Danışman")
						//fmt.Println(newDoc[:endIndex])
						if !isExist(counselours, strings.Trim(newDoc[:endIndex], " ")) && !strings.Contains(strings.Trim(newDoc[:endIndex], " "), "Jüri") {
							counselours = append(counselours, strings.Trim(newDoc[:endIndex], " "))
						}
					}

				}
				//fmt.Println(field)
			}

		}

	} else if result["type"] == "araştırma" {
		fmt.Println("fds")
		if strings.Contains(content, "ARAŞTIRMA") {
			startIndex := strings.Index(content, "ARAŞTIRMA")
			if strings.Contains(content, "Tezin Savunulduğu") {
				endIndex := strings.Index(content, "Tezin Savunulduğu")
				field := content[startIndex:endIndex]
				for {
					if strings.Contains(field, "....") {
						field = strings.Replace(field, "..", "", 1)
					} else {
						break
					}
				}
				if strings.Contains(field, "Prof") {
					profIndex := strings.Index(field, "Prof")
					//fmt.Println("prof -->")
					//startIndex := field[profIndex:]
					//fmt.Println(field[profIndex:])
					newDoc := field[profIndex:]
					//fmt.Println("//////")
					//fmt.Println(newDoc)
					if strings.Contains(newDoc, "Danışman") {
						endIndex := strings.Index(newDoc, "Danışman")
						//	fmt.Println(newDoc[:endIndex])
						if !isExist(counselours, strings.Trim(newDoc[:endIndex], " ")) {
							counselours = append(counselours, strings.Trim(newDoc[:endIndex], " "))
						}
						//counselours = append(counselours, strings.Trim(newDoc[:endIndex], " "))

					} else if strings.Contains(newDoc, "Jüri") {
						endIndex := strings.Index(newDoc, "Jüri")
						//fmt.Println(newDoc[:endIndex])
						if !isExist(counselours, strings.Trim(newDoc[:endIndex], " ")) {
							counselours = append(counselours, strings.Trim(newDoc[:endIndex], " "))
						}
						//counselours = append(counselours, strings.Trim(newDoc[:endIndex], " "))
					}

				}
				//--
				if strings.Contains(field, "Doç") {
					fmt.Println("doç --_>")
					docIndex := strings.Index(field, "Doç")
					//fmt.Println(field[docIndex:])
					newDoc := field[docIndex:]

					if strings.Contains(newDoc, "Jüri") {
						fmt.Println("until juri doç")
						juriIndex := strings.Index(newDoc, "Jüri")
						//	fmt.Println(newDoc[:juriIndex])
						if !isExist(counselours, strings.Trim(newDoc[:juriIndex], " ")) && !strings.Contains(newDoc[:juriIndex], "Danışman") {
							counselours = append(counselours, strings.Trim(newDoc[:juriIndex], " "))
						}

					}
					if strings.Contains(newDoc, "Danışman") {
						//fmt.Println("until danışman doç")
						counselorIndex := strings.Index(newDoc, "Danışman")
						//fmt.Println("doç danışman")
						//fmt.Println(newDoc[:counselorIndex])
						if !isExist(counselours, strings.Trim(newDoc[:counselorIndex], " ")) && !strings.Contains(newDoc[:counselorIndex], "Jüri") {
							counselours = append(counselours, strings.Trim(newDoc[:counselorIndex], " "))
						}

					}

				}
				//---
				if strings.Contains(field, "Dr. Öğr") || strings.Contains(field, "Dr.Öğr") {
					//fmt.Println("dr ---> ")
					var drIndex int
					if strings.Contains(field, "Dr. Öğr") {
						drIndex = strings.Index(field, "Dr. Öğr")
					} else if strings.Contains(field, "Dr.Öğr") {
						drIndex = strings.Index(field, "Dr.Öğr")
					}
					newDoc := field[drIndex:]
					//	fmt.Println(newDoc)
					if strings.Contains(newDoc, "Jüri") {
						endIndex := strings.Index(newDoc, "Jüri")
						//fmt.Println(newDoc[:endIndex])
						if !isExist(counselours, strings.Trim(newDoc[:endIndex], " ")) {
							counselours = append(counselours, strings.Trim(newDoc[:endIndex], " "))
						}

					} else if strings.Contains(newDoc, "Danışman") {
						endIndex := strings.Index(newDoc, "Danışman")
						//fmt.Println(newDoc[:endIndex])
						if !isExist(counselours, strings.Trim(newDoc[:endIndex], " ")) {
							counselours = append(counselours, strings.Trim(newDoc[:endIndex], " "))
						}

					}

				}
			}
		}
	}
	result["juri"] = counselours
	fmt.Println(result["juri"])

	// anahtar kelimeler

	if strings.Contains(content, "Anahtar") {
		if strings.Contains(content, "Anahtar kelimeler") {
			index := strings.Index(content, "Anahtar kelimeler")
			newStr := strings.Replace(strings.Trim(content[index:index+300], " "), "Anahtar kelimeler", "", 1)
			if strings.Contains(newStr, ":") {
				//index = strings.Index(newStr, ":")
				newStr = strings.Replace(newStr, ":", "", 1)
			}
			// take keys until encounter with .
			dotIndex := strings.Index(newStr, ".")
			newStr = newStr[:dotIndex]
			//fmt.Println(newStr)
			keywords := strings.Split(newStr, ",")
			//fmt.Println(keywords)
			for i, k := range keywords { // to trim string
				keywords[i] = strings.Trim(k, " ")
			}
			result["keywords"] = keywords
		}
	}
	// nums and name
	nums := "0123456789"
	var name string
	if strings.Contains(content, "Öğrenci No:") {
		index := strings.Index(content, "Öğrenci No:")
		field := content[index : index+80]
		field = strings.Replace(field, "Öğrenci No:", "", 1)
		field = strings.Trim(field, " ")
		var temp string
		for _, i := range field {
			if strings.Contains(nums, string(i)) {
				temp += string(i)
			} else {
				break
			}
		}
		result["No"] = temp
		// get name and surname
		index = strings.Index(field, "Adı Soyadı:")
		field = strings.Replace(field, "Adı Soyadı:", "", 1)
		field = field[index:]
		field = strings.Trim(field, " ")
		signIndex := strings.Index(field, "İmza")
		name = strings.Trim(field[:signIndex], " ")
		//fmt.Println(field)
		result["name"] = name
	}

	// project name
	if result["type"] == "bitirme" {
		bitirmeTitle := strings.Index(content, "BİTİRME")
		fmt.Println("bitirme title")
		newDoc := content[bitirmeTitle:]
		//fmt.Println("name --> ", strings.ToUpper(string(r)))
		newName := toTurkishCharacter(strings.ToUpper(name), name)
		//fmt.Println(newName)
		//fmt.Println(content[bitirmeTitle : bitirmeTitle+50])
		//fmt.Println("name --> ", strings.ToUpper(name))
		endIndex := strings.Index(newDoc, newName)

		fmt.Println(newDoc[:endIndex])
		newDoc = newDoc[:endIndex]
		removeText := "BİTİRME PROJESİ"
		project_name := strings.Trim(strings.Replace(newDoc, removeText, "", 1), " ")
		fmt.Println(project_name)
		result["project_name"] = project_name

	} else { // araştırma
		arastirma := strings.Index(content, "ARAŞTIRMA")
		fmt.Println("arastirma title")
		newDoc := content[arastirma:]
		//fmt.Println("name --> ", strings.ToUpper(string(r)))
		newName := toTurkishCharacter(strings.ToUpper(name), name)
		//fmt.Println(newName)
		//fmt.Println(content[bitirmeTitle : bitirmeTitle+50])
		//fmt.Println("name --> ", strings.ToUpper(name))
		endIndex := strings.Index(newDoc, newName)

		fmt.Println(newDoc[:endIndex])
		newDoc = newDoc[:endIndex]
		removeText := "ARAŞTIRMA PROBLEMLERİ"
		project_name := strings.Trim(strings.Replace(newDoc, removeText, "", 1), " ")
		fmt.Println(project_name)
		result["project_name"] = project_name
	}

	// period
	guz := "Eylül,Ekim,Kasım,Aralık"
	bahar := "Şubat,MartNisan,Mayıs,Haziran"

	if strings.Contains(content, "KOCAELİ 2") {
		fmt.Println("*****************************")
		index := strings.Index(content, "KOCAELİ 2")
		period := content[index+len("KOCAELİ 2")-1:]
		//fmt.Println(period[:10])
		var temp string
		for _, i := range period[:12] {
			if strings.Contains(nums, string(i)) {
				temp += string(i)
			} else {
				break
			}
		}
		for _, i := range content {
			if strings.Contains(guz, string(i)) && !strings.Contains(bahar, string(i)) {
				result["deploy_date"] = fmt.Sprintf("%s Guz", temp)
			} else {
				result["deploy_date"] = fmt.Sprintf("%s Bahar", temp)
			}
		}
		//result["deploy_date"] = temp
	}

	// summary
	if strings.Contains(content, "Anahtar kelimeler:") {
		index := strings.Index(content, "Anahtar kelimeler:")
		//fmt.Println(content[index-50 : index])
		increase := 200
		for {
			if strings.Contains(content[index-increase:index], "ÖZET") {
				content = content[index-increase : index]
				break
			} else {
				increase += 150
			}

		}

		findOzet := strings.Index(content, "ÖZET")
		content = strings.Trim(content[findOzet+5:], " ")
		//fmt.Println(content)
		result["summary"] = content
	}

	fmt.Println("result -------------------> ", result)
	return result
}

func readPdf(path string) (string, error) {
	f, r, err := pdf.Open(path)
	// remember close file

	if err != nil {
		return "", err
	}
	defer f.Close()
	var buf bytes.Buffer
	b, err := r.GetPlainText()
	if err != nil {
		return "", err
	}
	buf.ReadFrom(b)
	return buf.String(), nil
}

func toTurkishCharacter(origin string, name string) string {
	if strings.Contains(origin, "I") && strings.Contains(name, "i") && strings.Index(origin, "I") == strings.Index(name, "i") {
		origin = strings.Replace(origin, "I", "İ", 1)
	}
	return origin
}

func isExist(arr []string, element string) bool {
	for _, i := range arr {
		if i == element {
			return true
		}
	}
	return false
}

func resToKeyword(keywords []string) []student.KeyWord {
	var keys []student.KeyWord
	for _, i := range keywords {
		keys = append(keys, student.KeyWord{Word: i})
	}
	return keys
}

func isJuriThere(juri []string, element string) bool {
	for _, i := range juri {
		if i == element {
			return true
		}
	}
	return false
}
