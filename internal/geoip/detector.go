package geoip

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/oschwald/geoip2-golang"
)

// GeoResult holds the result of a GeoIP lookup.
type GeoResult struct {
	CountryCode string
	CountryName string
	Continent   string
	IsVPN       bool
	DetectedAt  time.Time
}

type cacheEntry struct {
	result    *GeoResult
	expiresAt time.Time
}

// Detector performs GeoIP lookups with in-memory caching.
type Detector struct {
	db     *geoip2.Reader
	cache  sync.Map
	ttl    time.Duration
	stopCh chan struct{}
}

// NewDetector opens the MaxMind database and starts a background eviction loop.
func NewDetector(dbPath string, cacheTTLMinutes int) (*Detector, error) {
	db, err := geoip2.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening GeoIP database: %w", err)
	}

	if cacheTTLMinutes <= 0 {
		cacheTTLMinutes = 10
	}

	d := &Detector{
		db:     db,
		ttl:    time.Duration(cacheTTLMinutes) * time.Minute,
		stopCh: make(chan struct{}),
	}

	go d.evictLoop()
	return d, nil
}

// Detect looks up the GeoIP information for the given IP string.
func (d *Detector) Detect(ipStr string) (*GeoResult, error) {
	if v, ok := d.cache.Load(ipStr); ok {
		entry := v.(*cacheEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.result, nil
		}
		d.cache.Delete(ipStr)
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ipStr)
	}

	record, err := d.db.Country(ip)
	if err != nil {
		return nil, fmt.Errorf("GeoIP lookup failed: %w", err)
	}

	result := &GeoResult{
		CountryCode: record.Country.IsoCode,
		CountryName: record.Country.Names["en"],
		Continent:   record.Continent.Code,
		IsVPN:       false,
		DetectedAt:  time.Now(),
	}

	d.cache.Store(ipStr, &cacheEntry{
		result:    result,
		expiresAt: time.Now().Add(d.ttl),
	})

	return result, nil
}

// Close releases the database resources and stops the eviction loop.
func (d *Detector) Close() error {
	close(d.stopCh)
	return d.db.Close()
}

// evictLoop runs every 5 minutes and removes expired cache entries.
func (d *Detector) evictLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			d.cache.Range(func(k, v interface{}) bool {
				if now.After(v.(*cacheEntry).expiresAt) {
					d.cache.Delete(k)
				}
				return true
			})
		case <-d.stopCh:
			return
		}
	}
}
