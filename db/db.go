package db

import (
	"MirrorBotGo/utils"
	"context"
	"log"
	"time"

	"github.com/PaulSonOfLars/gotgbot/ext"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var dbClient *mongo.Client = getDbClient()
var AuthorizedUsers []int
var AuthorizedChats []int

func getDbClient() *mongo.Client {
	log.Printf("[DB] Connection: %s\n", utils.GetDbUri())
	client, err := mongo.NewClient(options.Client().ApplyURI(utils.GetDbUri()))
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	err = client.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
	return client
}

func IsUserAuthorized(userId int) bool {
	for _, i := range AuthorizedUsers {
		if i == userId {
			return true
		}
	}
	return false
}

func IsChatAuthorized(chatId int) bool {
	for _, i := range AuthorizedChats {
		if i == chatId {
			return true
		}
	}
	return false
}

func GetUserIndex(userId int) int {
	for i := 0; i <= len(AuthorizedUsers); i++ {
		if AuthorizedUsers[i] == userId {
			return i
		}
	}
	return -1
}

func GetChatIndex(chatId int) int {
	for i := 0; i <= len(AuthorizedChats); i++ {
		if AuthorizedChats[i] == chatId {
			return i
		}
	}
	return -1
}

func AuthorizeUserLocal(userId int) bool {
	if IsUserAuthorized(userId) {
		return false
	}
	AuthorizedUsers = append(AuthorizedUsers, userId)
	return true
}

func UnAuthorizeUserLocal(userId int) bool {
	if !IsUserAuthorized(userId) {
		return false
	}
	index := GetUserIndex(userId)
	if index != -1 {
		AuthorizedUsers[index] = AuthorizedUsers[len(AuthorizedUsers)-1]
		AuthorizedUsers[len(AuthorizedUsers)-1] = 0
		AuthorizedUsers = AuthorizedUsers[:len(AuthorizedUsers)-1]
	}
	return true
}

func AuthorizeChatLocal(chatId int) bool {
	if IsChatAuthorized(chatId) {
		return false
	}
	AuthorizedChats = append(AuthorizedChats, chatId)
	return true
}

func UnAuthorizeChatLocal(chatId int) bool {
	if !IsChatAuthorized(chatId) {
		return false
	}
	index := GetChatIndex(chatId)
	if index != -1 {
		AuthorizedChats[index] = AuthorizedChats[len(AuthorizedChats)-1]
		AuthorizedChats[len(AuthorizedChats)-1] = 0
		AuthorizedChats = AuthorizedChats[:len(AuthorizedChats)-1]
	}
	return true
}

func AuthorizeChatDb(chatId int) bool {
	Ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	collection := dbClient.Database("mirrorBot").Collection("AUTHORIZEDCHATS")
	res, err := collection.InsertOne(Ctx, bson.M{
		"chatId": chatId,
	})
	if err != nil {
		log.Println(err)
		return false
	}
	log.Println(res)
	AuthorizeChatLocal(chatId)
	return true
}

func UnAuthorizeChatDb(chatId int) bool {
	Ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	collection := dbClient.Database("mirrorBot").Collection("AUTHORIZEDCHATS")
	res, err := collection.DeleteOne(Ctx, bson.M{
		"chatId": chatId,
	})
	if err != nil {
		log.Println(err)
		return false
	}
	log.Println(res)
	UnAuthorizeChatLocal(chatId)
	return true
}

func AuthorizeUserDb(userId int) bool {
	Ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	collection := dbClient.Database("mirrorBot").Collection("AUTHORIZEDUSERS")
	res, err := collection.InsertOne(Ctx, bson.M{
		"userId": userId,
	})
	if err != nil {
		log.Println(err)
		return false
	}
	log.Println(res)
	AuthorizeUserLocal(userId)
	return true
}

func UnAuthorizeUserDb(userId int) bool {
	Ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	collection := dbClient.Database("mirrorBot").Collection("AUTHORIZEDUSERS")
	res, err := collection.DeleteOne(Ctx, bson.M{
		"userId": userId,
	})
	if err != nil {
		log.Println(err)
		return false
	}
	log.Println(res)
	UnAuthorizeUserLocal(userId)
	return true
}

func Init() {
	InitChats()
	InitUsers()
	for _, i := range utils.GetSudoUsers() {
		AuthorizeUserLocal(i)
	}
	for _, i := range utils.GetAuthorizedChats() {
		AuthorizeChatLocal(i)
	}
}

func InitChats() bool {
	Ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	collection := dbClient.Database("mirrorBot").Collection("AUTHORIZEDCHATS")
	cur, err := collection.Find(Ctx, bson.D{})
	if err != nil {
		log.Println(err)
		return false
	}
	defer cur.Close(Ctx)
	for cur.Next(Ctx) {
		var result bson.M
		err := cur.Decode(&result)
		if err != nil {
			log.Println(err)
		} else {
			if result["userId"] != nil {
				chatId := utils.ParseInterfaceToInt(result["chatId"])
				AuthorizeUserLocal(chatId)
			}
		}
	}
	return true
}

func InitUsers() bool {
	Ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	collection := dbClient.Database("mirrorBot").Collection("AUTHORIZEDUSERS")
	cur, err := collection.Find(Ctx, bson.D{})
	if err != nil {
		log.Println(err)
		return false
	}
	defer cur.Close(Ctx)
	for cur.Next(Ctx) {
		var result bson.M
		err := cur.Decode(&result)
		if err != nil {
			log.Println(err)
		} else {
			if result["userId"] != nil {
				userId := utils.ParseInterfaceToInt(result["userId"])
				AuthorizeUserLocal(userId)
			}
		}
	}
	return true
}

func IsAuthorized(message *ext.Message) bool {
	if IsUserAuthorized(message.From.Id) || IsChatAuthorized(message.Chat.Id) {
		return true
	}
	return false
}
