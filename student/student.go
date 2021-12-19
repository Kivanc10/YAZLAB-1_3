package student

import (
	"encoding/json"
	"time"

	"github.com/globalsign/mgo/bson"
)

type Student struct {
	Name     string  `bson:"name"`
	Lastname string  `bson:"lastname"`
	Password string  `bson:"password"`
	Number   string  `bson:"number"`
	Type     string  `bson:"type"`
	Tokens   []Token `bson:"Token"`
}

type Admin struct {
	Name     string `bson:"name"`
	Password string `bson:"password"`
}

type Token struct {
	Context string `bson:"context"`
}

type Doc struct {
	Id         bson.ObjectId `bson:"_id"`
	Length     int64         `bson:"length"`
	ChunkSize  int32         `bson:"chunkSize"`
	UploadDate time.Time     `bson:"uploadDate"`
	FileName   string        `bson:"filename"`
	MetaData   MetaData      `bson:"metadata"` //MetaData   MetaData  `json:"metadata,omitempty"`
}

type MetaData struct {
	OwnerName   string    `bson:"owner_name"` //OwnerName string `bson:"owner_name,omitempty"
	DeployDate  string    `bson:"deploy_date"`
	ProjectName string    `bson:"project_name"`
	Lesson      string    `bson:"lesson_name"`
	Summary     string    `bson:"summary"`
	KeyWords    []KeyWord `bson:"keywords"`
	Type        string    `bson:"typeOfDoc"` // graduation or searching problems doc
	Juri        []string  `bson:"juri"`
	No          string    `bson:"No"`
}

type KeyWord struct {
	Word string `bson:"keyword"`
}

func ProcessToJson(body []byte) (*Student, error) {
	var stndt Student
	if err := json.Unmarshal(body, &stndt); err != nil {
		return &Student{}, err
	}
	return &stndt, nil
}

func ProcessJSONforAdmin(body []byte) (*Admin, error) {
	var admin Admin
	if err := json.Unmarshal(body, &admin); err != nil {
		return &Admin{}, err
	}
	return &admin, nil
}

func IsInsideOfKey(arr []KeyWord, keyword string) bool {
	for _, i := range arr {
		if i.Word == keyword {
			return true
		}
	}
	return false
}
