package engine

// The accepted H1 policy is the production policy: completed quiet winners
// train history, and LMR consumes the normalized history term.
const (
	h1NewHistoryProducer    = true
	h1NormalizedLMRConsumer = true
)

type h1CompletedQuietBuffer [256]Move

func h1AppendCompletedQuiet(buffer *h1CompletedQuietBuffer, count *int, move Move) {
	if *count < len(buffer) {
		buffer[*count] = move
		*count = *count + 1
	}
}

func h1CompletedQuiets(buffer *h1CompletedQuietBuffer, count int) []Move {
	return buffer[:count]
}
