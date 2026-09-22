package config

import "testing"

func TestLoadConfig(t *testing.T) {
	t.Setenv("MONGO_URI", "mongodb://localhost:27017")
	t.Setenv("DB_NAME", "leads")
	t.Setenv("DB_Name", "legacy")
	t.Setenv("PORT", "")
	t.Setenv("MONGO_COLLECTION", "")
	t.Setenv("DNS_SERVER", "")
	t.Setenv("SEARCH_CACHE_TTL", "30s")
	t.Setenv("REDIS_URL", "")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DBName != "leads" || cfg.Port != "3000" || cfg.Collection != "listings" || cfg.DNSServer != "" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	t.Setenv("DB_NAME", "")
	cfg, err = LoadConfig()
	if err != nil || cfg.DBName != "legacy" {
		t.Fatal("legacy DB_Name was not accepted")
	}
	for _, tt := range []struct{ key, value string }{
		{"SEARCH_CACHE_TTL", "0s"}, {"SEARCH_CACHE_TTL", "bad"}, {"MONGO_URI", ""}, {"PORT", "0"}, {"PORT", "bad"}, {"DNS_SERVER", "invalid"}, {"DNS_SERVER", "8.8.8.8:99999"},
	} {
		t.Run(tt.key+tt.value, func(t *testing.T) {
			t.Setenv(tt.key, tt.value)
			if _, err := LoadConfig(); err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}
