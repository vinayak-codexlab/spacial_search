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

func (r *PropertyRepository) SearchByHexagons(ctx context.Context, hexagons []string, resolution int) ([]bson.M, error) {
	if resolution < 6 || resolution > 11 {
		return nil, fmt.Errorf("unsupported H3 resolution: %d", resolution)
	}
	listings := []bson.M{}
	if len(hexagons) == 0 {
		return listings, nil
	}
	filter := bson.M{fmt.Sprintf("h3_res%d", resolution): bson.M{"$in": hexagons}}
	opts := options.Find().SetProjection(bson.M{
		"_id": 1, "listing_id": 1, "listing_type": 1, "coverImageKey": 1, "currency": 1,
		"h3_res7": 1, "h3_res8": 1, "h3_res9": 1,
		"listing_details.listing_name": 1, "listing_details.listing_status": 1,
		"listing_details.bhk_type": 1, "listing_details.area": 1,
		"listing_details.area_unit_type": 1, "listing_details.furnishing": 1,
		"commercial_details.property_price": 1,
		"listing_address.lat":               1, "listing_address.lng": 1,
		"listing_address.locality": 1, "listing_address.city": 1,
	})
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("find listings in %s.%s: %w", r.collection.Database().Name(), r.collection.Name(), err)
	}
	defer cursor.Close(ctx)
	if err := cursor.All(ctx, &listings); err != nil {
		return nil, fmt.Errorf("decode listings in %s.%s: %w", r.collection.Database().Name(), r.collection.Name(), err)
	}
	return listings, nil
}
