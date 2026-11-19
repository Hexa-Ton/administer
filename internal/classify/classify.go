package classify

// Verdict represents how a single path's response was classified.
type Verdict int

const (
	NotFound Verdict = iota
	Candidate
)

func (v Verdict) String() string {
	if v == Candidate {
		return "candidate"
	}
	return "not-found"
}

// Baseline is the wildcard fingerprint captured before the real scan
// begins: the response the target gives for a path that almost
// certainly does not exist. A later response matching this
// fingerprint is treated as a wildcard/catch-all response, not a
// real discovery.
type Baseline struct {
	Established bool
	StatusCode  int
	BodyHash    string
}

var candidateStatuses = map[int]bool{
	200: true, 204: true, 301: true, 302: true, 307: true, 401: true, 403: true, 405: true,
}

// Classify returns Candidate only for a status this tool treats as
// meaningful AND a response that does not match the wildcard
// baseline. Soft-404 body inspection and the login-panel heuristic
// are Phase 3 refinements on top of this.
func Classify(statusCode int, bodyHash string, baseline Baseline) Verdict {
	if !candidateStatuses[statusCode] {
		return NotFound
	}
	if baseline.Established && statusCode == baseline.StatusCode && bodyHash == baseline.BodyHash {
		return NotFound
	}
	return Candidate
}
