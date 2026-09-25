package response

import (
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ListingSummary limits the public response to the listing card field.
func ListingSummary(listing bson.M) bson.M {
	details := nestedDocument(listing["listing_details"])
	address := nestedDocument(listing["listing_address"])
	commercial := nestedDocument(listing["commercial_details"])
	currency := stringValue(listing["currency"])
	if currency == "" {
		currency = "INR"
	}
	return bson.M{
		"_id": listing["_id"], "listing_id": stringValue(listing["listing_id"]),
		"title":         stringValue(details["listing_name"]),
		"listing_type":  strings.ToUpper(stringValue(listing["listing_type"])),
		"coverImageKey": stringValue(listing["coverImageKey"]),
		"price":         commercial["property_price"], "currency": strings.ToUpper(currency),
		"status": stringValue(details["listing_status"]),
		"lat":    address["lat"], "lng": address["lng"],
		"locality": stringValue(address["locality"]), "city": stringValue(address["city"]),
		"h3_res6":  stringValue(listing["h3_res6"]),
		"h3_res7":  stringValue(listing["h3_res7"]),
		"h3_res8":  stringValue(listing["h3_res8"]),
		"h3_res9":  stringValue(listing["h3_res9"]),
		"h3_res10": stringValue(listing["h3_res10"]),
		"h3_res11": stringValue(listing["h3_res11"]),
		"bhk":      stringValue(details["bhk_type"]), "area": details["area"],
		"area_unit":  strings.ToUpper(stringValue(details["area_unit_type"])),
		"furnishing": strings.ToUpper(stringValue(details["furnishing"])),
	}
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

// MongoDB and the JSON cache can decode nested documents into different types.
func nestedDocument(value any) bson.M {
	switch document := value.(type) {
	case bson.M:
		return document
	case map[string]any:
		return bson.M(document)
	case bson.D:
		result := bson.M{}
		for _, field := range document {
			result[field.Key] = field.Value
		}
		return result
	default:
		return nil
	}
}
