package response

import (
	"encoding/json"
	"go.mongodb.org/mongo-driver/v2/bson"
	"reflect"
	"testing"
)

func TestListingSummary(t *testing.T) {
	const source = `{"_id": "6ab3817d204b9a8820f4462e", "listing_type": "home", "listing_details": {"listing_status": "work_in_progress", "unit_no": "1001", "area": 1250, "listing_name": "Spacious 3 BHK Home in Mumbai", "area_unit_type": "sqft", "furnishing": "semi_furnished", "bhk_type": "3 BHK"}, "commercial_details": {"property_price": 10000000, "internal_notes": "Contact the owner before arranging a visit"}, "broker_and_agent": {"sub": "657eae983f9b3bd028f97d42", "firm_id": "6565ec75ea6c68622ab04574"}, "listing_address": {"line_1": "Sunshine Residency, Western Express Highway", "locality": "Andheri East", "city": "Mumbai", "lat": 77.35366, "lng": 128.765}, "coverImageKey": "", "listing_id": "LST2765230", "h3_res7": "870519080ffffff", "h3_res8": "880519081dfffff", "h3_res9": "890519081dbffff"}`
	const expected = `{"_id": "6ab3817d204b9a8820f4462e", "listing_id": "LST2765230", "title": "Spacious 3 BHK Home in Mumbai", "listing_type": "HOME", "coverImageKey": "", "price": 10000000, "currency": "INR", "status": "work_in_progress", "lat": 77.35366, "lng": 128.765, "locality": "Andheri East", "city": "Mumbai", "h3_res7": "870519080ffffff", "h3_res8": "880519081dfffff", "h3_res9": "890519081dbffff", "bhk": "3 BHK", "area": 1250, "area_unit": "SQFT", "furnishing": "SEMI_FURNISHED"}`
	var want map[string]any
	if err := json.Unmarshal([]byte(expected), &want); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"mongo", "cache"} {
		t.Run(mode, func(t *testing.T) {
			var listing bson.M
			if mode == "mongo" {
				if err := bson.UnmarshalExtJSON([]byte(source), false, &listing); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := json.Unmarshal([]byte(source), &listing); err != nil {
					t.Fatal(err)
				}
			}
			raw, err := json.Marshal(ListingSummary(listing))
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("unexpected summary: %s", raw)
			}
			listing["currency"] = "usd"
			if ListingSummary(listing)["currency"] != "USD" {
				t.Fatal("stored currency must be preserved")
			}
		})
	}
}
