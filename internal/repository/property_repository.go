package repository

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type PropertyRepository struct {
	collection *mongo.Collection
}

func NewPropertyRepository(collection *mongo.Collection) *PropertyRepository {
	return &PropertyRepository{collection: collection}
}

func (r *PropertyRepository) SearchByHexagons(ctx context.Context, hexagons []string, resolution int, page, limit int64) ([]bson.M, int64, error) {
	if resolution < 7 || resolution > 9 {
		return nil, 0, fmt.Errorf("unsupported H3 resolution: %d", resolution)
	}
	listings := []bson.M{}
	if len(hexagons) == 0 {
		return listings, 0, nil
	}
	filter := bson.M{fmt.Sprintf("h3_res%d", resolution): bson.M{"$in": hexagons}}
	if page < 1 || limit < 1 || limit > 100 || page-1 > (1<<63-1)/limit {
		return nil, 0, fmt.Errorf("invalid pagination")
	}
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetSkip((page - 1) * limit).SetLimit(limit)
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)
	if err := cursor.All(ctx, &listings); err != nil {
		return nil, 0, err
	}
	return listings, total, nil
}
