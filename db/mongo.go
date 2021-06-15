package db

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Mongo struct {
	endpoint string
	client   *mongo.Client
}

func NewMgo(endpoint string) *Mongo {
	return &Mongo{
		endpoint: endpoint,
	}
}

func (m *Mongo) Connect() error {
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(m.endpoint))
	if err != nil {
		return err
	}
	err = client.Ping(context.TODO(), nil)
	if err != nil {
		return err
	}
	m.client = client
	return nil
}

func (m *Mongo) DisConnect() error {
	m.client.Disconnect(context.TODO())
	return nil
}

func (m *Mongo) InsertOne(mgoDBName, mgoCollectionName string, document interface{}) error {
	err := m.client.Ping(context.TODO(), nil)
	if err != nil {
		err = m.Connect()
		if err != nil {
			return err
		}
	}
	collection := m.client.Database(mgoDBName).Collection(mgoCollectionName)
	_, err = collection.InsertOne(context.TODO(), document)
	if err != nil {
		return err
	}
	return nil
}

func (m *Mongo) FindOne(mgoDBName, mgoCollectionName string, filter, result interface{}) error {
	err := m.client.Ping(context.TODO(), nil)
	if err != nil {
		err = m.Connect()
		if err != nil {
			return err
		}
	}
	collection := m.client.Database(mgoDBName).Collection(mgoCollectionName)
	err = collection.FindOne(context.TODO(), filter).Decode(result)
	if err != nil {
		return err
	}
	return nil
}

func (m *Mongo) UpdateOne(mgoDBName, mgoCollectionName string, filter, update interface{}) (*mongo.UpdateResult, error) {
	err := m.client.Ping(context.TODO(), nil)
	if err != nil {
		err = m.Connect()
		if err != nil {
			return nil, err
		}
	}
	collection := m.client.Database(mgoDBName).Collection(mgoCollectionName)
	res, err := collection.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return nil, err
	}
	return res, nil
}
