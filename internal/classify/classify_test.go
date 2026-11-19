package classify

import "testing"

func TestClassify(t *testing.T) {
	wildcard := Baseline{Established: true, StatusCode: 200, BodyHash: "same"}

	tests := []struct {
		name     string
		status   int
		hash     string
		baseline Baseline
		want     Verdict
	}{
		{"404 is not found", 404, "h", Baseline{}, NotFound},
		{"500 is not a candidate", 500, "h", Baseline{}, NotFound},
		{"200 without a baseline is a candidate", 200, "h", Baseline{}, Candidate},
		{"204 is a candidate", 204, "h", Baseline{}, Candidate},
		{"301 is a candidate", 301, "h", Baseline{}, Candidate},
		{"302 is a candidate", 302, "h", Baseline{}, Candidate},
		{"307 is a candidate", 307, "h", Baseline{}, Candidate},
		{"401 is a candidate", 401, "h", Baseline{}, Candidate},
		{"403 is a candidate", 403, "h", Baseline{}, Candidate},
		{"405 is a candidate", 405, "h", Baseline{}, Candidate},
		{"a wildcard match is filtered out", 200, "same", wildcard, NotFound},
		{"same status but different body is a candidate", 200, "different", wildcard, Candidate},
		{"different status is a candidate", 302, "same", wildcard, Candidate},
	}
	for _, tc := range tests {
		if got := Classify(tc.status, tc.hash, tc.baseline); got != tc.want {
			t.Errorf("%s: Classify(%d) = %v, want %v", tc.name, tc.status, got, tc.want)
		}
	}
}
