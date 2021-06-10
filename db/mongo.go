package db

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Mongo struct {
	mgoEndpoint string
	mgoClient   *mongo.Client
	mgoCancel   context.CancelFunc
	ctx         context.Context
}

func NewMgo(mgoEndpoint string) *Mongo {
	return &Mongo{
		mgoEndpoint: mgoEndpoint,
	}
}

func (m *Mongo) Connect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(m.mgoEndpoint))
	if err != nil {
		return err
	}
	m.mgoClient = client
	m.mgoCancel = cancel
	m.ctx = ctx
	return nil
}

func (m *Mongo) DisConnect() error {
	m.mgoClient.Disconnect(m.ctx)
	m.mgoCancel()
	return nil
}

func (m *Mongo) InsertOne(mgoDBName, mgoCollectionName string, document interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := m.mgoClient.Database(mgoDBName).Collection(mgoCollectionName)
	_, err := collection.InsertOne(ctx, document)
	if err != nil {
		m.Connect()
		_, err = collection.InsertOne(ctx, document)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Mongo) FindOne(mgoDBName, mgoCollectionName string, filter, result interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := m.mgoClient.Database(mgoDBName).Collection(mgoCollectionName)
	err := collection.FindOne(ctx, filter).Decode(result)
	if err != nil {
		m.Connect()
		err = collection.FindOne(ctx, filter).Decode(result)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Mongo) UpdateOne(mgoDBName, mgoCollectionName string, filter, update interface{}) (*mongo.UpdateResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := m.mgoClient.Database(mgoDBName).Collection(mgoCollectionName)
	res, err := collection.UpdateOne(ctx, filter, update)
	if err != nil {
		m.Connect()
		res, err = collection.UpdateOne(ctx, filter, update)
		if err != nil {
			return nil, err
		}
	}
	return res, nil
}
