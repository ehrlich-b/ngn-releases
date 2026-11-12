package engine

// Defaults are generated from the strength ledger, separately from the search
// implementation. An empty ledger preserves every experimental option at zero.
func init() {
	for i := range TunableSearchParams {
		p := &TunableSearchParams[i]
		if value, ok := acceptedOptionDefaults[p.Name]; ok {
			p.Def = value
			*p.Ptr = value
		}
	}
}
