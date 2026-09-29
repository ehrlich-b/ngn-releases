package engine

// defaultTTForDirectTest supplies the table explicitly to tests that invoke
// alphaBetaPV/quiescence without entering a SearchEngine session. It deliberately
// avoids a production fallback to package-global state.
func defaultTTForDirectTest() *Cache {
	if defaultSearchEngine.tt == nil {
		defaultSearchEngine.tt = NewCache(defaultSearchEngine.HashSize())
	}
	return defaultSearchEngine.tt
}
