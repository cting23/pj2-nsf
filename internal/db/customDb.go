package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const mongoTimeout = 5 * time.Second

// MongoDB 文件格式：{_id: key, value: value}
type mongoRecord struct {
	Key   string `bson:"_id"`
	Value string `bson:"value"`
}

// MongoDB 實作 DbIf 介面
type customDb struct {
	client     *mongo.Client
	collection *mongo.Collection
}

var _ DbIf = (*customDb)(nil)

// 建立 MongoDB 連線
func newCustomDb() (DbIf, error) {
	uri := os.Getenv("NSF_MONGO_URI")
	if uri == "" {
		uri = "mongodb://127.0.0.1:27019"
	}

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("connect MongoDB: %w", err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(), mongoTimeout)
	defer cancel()

	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("ping MongoDB: %w", err)
	}

	return &customDb{
		client: client,
		collection: client.Database("pj2_nsf").
			Collection("services"),
	}, nil
}

// 關閉資料庫連線
func (d *customDb) Release() error {
	ctx, cancel := context.WithTimeout(
		context.Background(), mongoTimeout)
	defer cancel()
	return d.client.Disconnect(ctx)
}

// 儲存單筆資料，相同 key 則覆蓋
func (d *customDb) Save(key, value string) error {
	ctx, cancel := context.WithTimeout(
		context.Background(), mongoTimeout)
	defer cancel()

	_, err := d.collection.ReplaceOne(
		ctx,
		bson.M{"_id": key},
		mongoRecord{Key: key, Value: value},
		options.Replace().SetUpsert(true),
	)
	return err
}

// 儲存多筆資料
func (d *customDb) SaveAll(data map[string]string) error {
	for key, value := range data {
		if err := d.Save(key, value); err != nil {
			return fmt.Errorf("save %s: %w", key, err)
		}
	}
	return nil
}

// 確認 key 是否存在
func (d *customDb) Exist(key string) (bool, error) {
	ctx, cancel := context.WithTimeout(
		context.Background(), mongoTimeout)
	defer cancel()

	count, err := d.collection.CountDocuments(
		ctx, bson.M{"_id": key})
	return count > 0, err
}

// 讀取單筆資料
func (d *customDb) Load(key string) (string, error) {
	ctx, cancel := context.WithTimeout(
		context.Background(), mongoTimeout)
	defer cancel()

	var record mongoRecord
	err := d.collection.FindOne(
		ctx, bson.M{"_id": key}).Decode(&record)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", fmt.Errorf("key %s not found: %w", key, err)
	}
	if err != nil {
		return "", err
	}
	return record.Value, nil
}

// 讀取所有資料
func (d *customDb) LoadAll() (map[string]string, error) {
	ctx, cancel := context.WithTimeout(
		context.Background(), mongoTimeout)
	defer cancel()

	cursor, err := d.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			fmt.Printf("Failed to close MongoDB cursor: %v\n", err)
		}
	}()

	result := make(map[string]string)
	for cursor.Next(ctx) {
		var record mongoRecord
		if err := cursor.Decode(&record); err != nil {
			return nil, err
		}
		result[record.Key] = record.Value
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// 刪除單筆資料
func (d *customDb) Remove(key string) error {
	ctx, cancel := context.WithTimeout(
		context.Background(), mongoTimeout)
	defer cancel()

	_, err := d.collection.DeleteOne(
		ctx, bson.M{"_id": key})
	return err
}

// 刪除所有資料
func (d *customDb) RemoveAll() error {
	ctx, cancel := context.WithTimeout(
		context.Background(), mongoTimeout)
	defer cancel()

	_, err := d.collection.DeleteMany(ctx, bson.M{})
	return err
}
