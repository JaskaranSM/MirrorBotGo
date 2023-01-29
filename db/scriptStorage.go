package db

import (
	"MirrorBotGo/engine"
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"regexp"
	"time"
)

func UpdateExtractor(regex string, script string) error {
	engine.L().Infof("updating extractor code %s : %s", regex, script)
	Ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	collection := dbClient.Database("mirrorBot").Collection("EXTRACTORS")
	opts := options.Update().SetUpsert(true)
	filter := bson.M{
		"regex": regex,
	}
	_, err := collection.UpdateOne(Ctx, filter, bson.D{{
		"$set", bson.M{
			"regex":  regex,
			"script": script,
		},
	}}, opts)
	return err
}

func RemoveExtractor(regex string) error {
	engine.L().Infof("deleting extractor code %s", regex)
	Ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	collection := dbClient.Database("mirrorBot").Collection("EXTRACTORS")
	filter := bson.M{
		"regex": regex,
	}
	_, err := collection.DeleteOne(Ctx, filter)
	return err
}

func GetExtractors() (map[string]string, error) {
	engine.L().Infof("getting extractors")
	Ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	collection := dbClient.Database("mirrorBot").Collection("EXTRACTORS")
	cur, err := collection.Find(Ctx, bson.D{})
	if err != nil {
		return nil, err
	}
	var rtr = make(map[string]string)
	defer func(cur *mongo.Cursor, ctx context.Context) {
		err := cur.Close(ctx)
		if err != nil {
			engine.L().Errorf("getExtractors: failed to close cursor: %v", err)
		}
	}(cur, Ctx)
	for cur.Next(Ctx) {
		var result bson.M
		err := cur.Decode(&result)
		if err != nil {
			engine.L().Error(err)
		} else {
			if result["regex"] != nil && result["script"] != nil {
				rtr[fmt.Sprint(result["regex"])] = fmt.Sprint(result["script"])
			}
		}
	}
	return rtr, nil
}

func IsExtractable(link string) bool {
	extractors, err := GetExtractors()
	if err != nil {
		engine.L().Error(err)
		return false
	}
	for regex, _ := range extractors {
		matched, err := regexp.MatchString(regex, link)
		if err != nil {
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

func UpdateSecret(key string, value string) error {
	engine.L().Infof("updating secret %s : %s", key, value)
	Ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	collection := dbClient.Database("mirrorBot").Collection("SCRIPT-SECRETS")
	opts := options.Update().SetUpsert(true)
	filter := bson.M{
		"key": key,
	}
	_, err := collection.UpdateOne(Ctx, filter, bson.D{{
		"$set", bson.M{
			"key":   key,
			"value": value,
		},
	}}, opts)
	return err
}

func RemoveSecret(key string) error {
	engine.L().Infof("deleting secret %s", key)
	Ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	collection := dbClient.Database("mirrorBot").Collection("SCRIPT-SECRETS")
	filter := bson.M{
		"key": key,
	}
	_, err := collection.DeleteOne(Ctx, filter)
	return err
}

func GetSecrets() (map[string]string, error) {
	engine.L().Infof("getting secrets")
	Ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	collection := dbClient.Database("mirrorBot").Collection("SCRIPT-SECRETS")
	cur, err := collection.Find(Ctx, bson.D{})
	if err != nil {
		return nil, err
	}
	var rtr = make(map[string]string)
	defer func(cur *mongo.Cursor, ctx context.Context) {
		err := cur.Close(ctx)
		if err != nil {
			engine.L().Errorf("getSecrets: failed to close cursor: %v", err)
		}
	}(cur, Ctx)
	for cur.Next(Ctx) {
		var result bson.M
		err := cur.Decode(&result)
		if err != nil {
			engine.L().Error(err)
		} else {
			if result["key"] != nil && result["value"] != nil {
				rtr[fmt.Sprint(result["key"])] = fmt.Sprint(result["value"])
			}
		}
	}
	return rtr, nil
}
