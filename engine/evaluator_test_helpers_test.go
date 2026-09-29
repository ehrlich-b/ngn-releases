package engine

func testHCEWorkerEvaluator(evaluator *hceEvaluator) *workerEvaluator {
	if evaluator == nil {
		evaluator = &hceEvaluator{}
	}
	worker, err := hceEvaluatorModel(evaluator.seenGeneration).newWorker(evaluator)
	if err != nil {
		panic(err)
	}
	return worker
}
