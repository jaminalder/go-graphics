package limits

import "testing"

func TestProfilesKeepCapacityBoundedAndProductionDefault(t *testing.T) {
	p, e := Parse("", "")
	if e != nil || p.Profile != "production" || p.Outstanding != 16 {
		t.Fatalf("%+v %v", p, e)
	}
	l, e := Parse("local-capacity", "")
	if e != nil || l.Outstanding != 64 || l.VisitorJobs != p.VisitorJobs || l.ImageBufferMiB != p.ImageBufferMiB {
		t.Fatalf("%+v %v", l, e)
	}
	if l.GenerateIP.PerMinute <= p.GenerateIP.PerMinute {
		t.Fatal("capacity profile must permit greater offered load")
	}
}

func TestOverridesFailClosedAndDoNotMutateDefaults(t *testing.T) {
	for _, raw := range []string{`null`, `[]`, `{"unknown":1}`, `{"outstanding_jobs":0}`, `{"read_ip":{"burst":-1}}`, `{"image_buffer_mib":10000}`, `{"profile":"local-capacity"}`, `{} {}`, `{"queue_age_seconds":5}`} {
		if _, e := Parse("production", raw); e == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	p, e := Parse("production", `{"outstanding_jobs":24,"start_ip":{"burst":20}}`)
	if e != nil || p.StartIP.PerMinute != 60 || p.StartIP.Burst != 20 || p.Outstanding != 24 {
		t.Fatalf("%+v %v", p, e)
	}
	original, _ := Defaults("production")
	if original.Outstanding != 16 {
		t.Fatal("defaults mutated")
	}
	if _, e = Parse("typo", ""); e == nil {
		t.Fatal("accepted unknown profile")
	}
}
