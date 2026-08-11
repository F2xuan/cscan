package model

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EnsureUserIndexes 创建 user 集合所需的索引（含 partial unique index 防止重复 superadmin）
func EnsureUserIndexes(ctx context.Context, db *mongo.Database) error {
	coll := db.Collection("user")
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "username", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "role", Value: 1}},
			Options: options.Index().SetUnique(true).
				SetPartialFilterExpression(bson.M{"role": "superadmin"}),
		},
	}
	_, err := coll.Indexes().CreateMany(ctx, indexes)
	return err
}
