// Package limits defines explicit, bounded operational policies independent of artwork code.
package limits

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Rate is a token bucket, replenished per minute with a finite immediate burst.
type Rate struct {
	PerMinute int `json:"per_minute"`
	Burst     int `json:"burst"`
}

// Policy is fixed at process startup and exposed only through private diagnostics.
type Policy struct {
	Profile         string `json:"profile"`
	ReadIP          Rate   `json:"read_ip"`
	AssetIP         Rate   `json:"asset_ip"`
	StartIP         Rate   `json:"start_ip"`
	GenerateIP      Rate   `json:"generate_ip"`
	GenerateVisitor Rate   `json:"generate_visitor"`
	Outstanding     int    `json:"outstanding_jobs"`
	VisitorJobs     int    `json:"visitor_jobs"`
	QueueAgeSeconds int    `json:"queue_age_seconds"`
	ImageReaders    int    `json:"image_readers"`
	ImageBufferMiB  int    `json:"image_buffer_mib"`
	ImageWaitMS     int    `json:"image_wait_ms"`
}

// Defaults returns named, finite policies. Production is never inferred from hostname.
func Defaults(profile string) (Policy, error) {
	if profile == "" {
		profile = "production"
	}
	p := Policy{Profile: profile, ReadIP: Rate{1200, 120}, AssetIP: Rate{3000, 300}, StartIP: Rate{60, 12}, GenerateIP: Rate{120, 24}, GenerateVisitor: Rate{30, 6}, Outstanding: 16, VisitorJobs: 4, QueueAgeSeconds: 120, ImageReaders: 32, ImageBufferMiB: 64, ImageWaitMS: 2000}
	switch profile {
	case "production":
	case "local-capacity":
		p.ReadIP = Rate{60000, 4000}
		p.AssetIP = Rate{60000, 4000}
		p.StartIP = Rate{12000, 400}
		p.GenerateIP = Rate{12000, 400}
		p.GenerateVisitor = Rate{600, 30}
		p.Outstanding = 64
	default:
		return Policy{}, fmt.Errorf("unknown limits profile %q", profile)
	}
	return p, nil
}

// FromEnv loads a profile and optional strictly checked partial JSON overrides.
func FromEnv() (Policy, error) {
	return Parse(os.Getenv("ART_LIMITS_PROFILE"), os.Getenv("ART_LIMITS_JSON"))
}

// Parse rejects unknown fields, unbounded values and inconsistent admission bounds.
func Parse(profile, overrides string) (Policy, error) {
	p, err := Defaults(profile)
	if err != nil {
		return p, err
	}
	if overrides != "" {
		if !strings.HasPrefix(strings.TrimSpace(overrides), "{") {
			return Policy{}, errors.New("ART_LIMITS_JSON must be an object")
		}
		originalProfile := p.Profile
		decoder := json.NewDecoder(bytes.NewBufferString(overrides))
		decoder.DisallowUnknownFields()
		if err = decoder.Decode(&p); err != nil {
			return Policy{}, fmt.Errorf("invalid ART_LIMITS_JSON: %w", err)
		}
		var extra any
		if err = decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			return Policy{}, errors.New("ART_LIMITS_JSON must contain one object")
		}
		if p.Profile != originalProfile {
			return Policy{}, errors.New("set profile with ART_LIMITS_PROFILE, not JSON")
		}
	}
	return p, p.Validate()
}

// Validate also protects direct construction in tests and callers.
func (p Policy) Validate() error {
	if p.Profile != "production" && p.Profile != "local-capacity" {
		return errors.New("invalid limits profile")
	}
	for _, r := range []Rate{p.ReadIP, p.AssetIP, p.StartIP, p.GenerateIP, p.GenerateVisitor} {
		if r.PerMinute < 1 || r.PerMinute > 120000 || r.Burst < 1 || r.Burst > 10000 {
			return errors.New("rate must be 1..120000/minute, burst 1..10000")
		}
	}
	if p.VisitorJobs < 4 || p.VisitorJobs > 32 || p.Outstanding < p.VisitorJobs || p.Outstanding > 256 {
		return errors.New("visitor jobs must be 4..32 and outstanding between visitor jobs and 256")
	}
	if p.QueueAgeSeconds < 30 || p.QueueAgeSeconds > 300 {
		return errors.New("queue age must be 30..300 seconds")
	}
	if p.ImageReaders < 1 || p.ImageReaders > 128 || p.ImageBufferMiB < 16 || p.ImageBufferMiB > 128 || p.ImageWaitMS < 0 || p.ImageWaitMS > 5000 {
		return errors.New("invalid image buffer/reader/wait bounds")
	}
	return nil
}
