package main

import "github.com/ehrlich-b/ngn/engine"

func selectNetwork(e *engine.SearchEngine, path string) error {
	if path == "" {
		return nil
	}
	n, err := engine.LoadNGNN1(path)
	if err != nil {
		return err
	}
	return e.SelectNGNN1Evaluator(n)
}
