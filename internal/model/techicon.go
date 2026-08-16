package model

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TechIcon 技术栈图标缓存（按归一化技术名缓存图标字节）
// 图标按需从指纹库上游拉取后落库，仅首次需要外网，之后完全本地化（离线部署友好）
type TechIcon struct {
	Id          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string             `bson:"name" json:"name"`                // 归一化技术名（小写、去版本/来源后缀）
	DisplayName string             `bson:"display_name" json:"displayName"` // 原始技术名（保留大小写）
	ContentType string             `bson:"content_type" json:"contentType"` // 图标 MIME 类型
	Data        []byte             `bson:"data" json:"data"`                // 图标字节
	Source      string             `bson:"source" json:"source"`            // 图标来源 URL
	CreateTime  time.Time          `bson:"create_time" json:"createTime"`
	UpdateTime  time.Time          `bson:"update_time" json:"updateTime"`
}

// TechIconModel 技术栈图标模型
type TechIconModel struct {
	coll *mongo.Collection
}

func NewTechIconModel(db *mongo.Database) *TechIconModel {
	coll := db.Collection("tech_icon")
	coll.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{Keys: bson.D{{Key: "name", Value: 1}}, Options: options.Index().SetUnique(true)},
	})
	return &TechIconModel{coll: coll}
}

func (m *TechIconModel) FindByName(ctx context.Context, name string) (*TechIcon, error) {
	var doc TechIcon
	err := m.coll.FindOne(ctx, bson.M{"name": name}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &doc, nil
}

func (m *TechIconModel) Upsert(ctx context.Context, doc *TechIcon) error {
	if doc.Id.IsZero() {
		doc.Id = primitive.NewObjectID()
	}
	now := time.Now()
	doc.UpdateTime = now
	if doc.CreateTime.IsZero() {
		doc.CreateTime = now
	}
	filter := bson.M{"name": doc.Name}
	update := bson.M{
		"$set": bson.M{
			"display_name": doc.DisplayName,
			"content_type": doc.ContentType,
			"data":         doc.Data,
			"source":       doc.Source,
			"update_time":  doc.UpdateTime,
		},
		"$setOnInsert": bson.M{"_id": doc.Id, "create_time": doc.CreateTime},
	}
	opts := options.Update().SetUpsert(true)
	_, err := m.coll.UpdateOne(ctx, filter, update, opts)
	return err
}

func (m *TechIconModel) Count(ctx context.Context) (int64, error) {
	return m.coll.CountDocuments(ctx, bson.M{})
}
