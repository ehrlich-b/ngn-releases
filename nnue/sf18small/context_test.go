package sf18small

import (
    "errors"
    "reflect"
    "sync"
    "testing"

    base "github.com/ehrlich-b/ngn/nnue"
)

var (
    contextModelOnce sync.Once
    contextModelValue *Model
)

func incrementalContextModel() *Model {
    contextModelOnce.Do(func() {
        model := blankEvaluationModel()
        for lane := range model.featureBias { model.featureBias[lane] = int16(lane - 64) }
        for index := range model.featureWeights { model.featureWeights[index] = int16((index*17)%101 - 50) }
        for index := range model.psqtWeights { model.psqtWeights[index] = int32((index*29)%2001 - 1000) }
        for bucket := range model.stacks {
            stack := &model.stacks[bucket]
            for index := range stack.fc0Bias { stack.fc0Bias[index] = int32(bucket*97 + index*13 - 300) }
            for index := range stack.fc0Weight { stack.fc0Weight[index] = int8((index+bucket*11)%15 - 7) }
            for index := range stack.fc1Bias { stack.fc1Bias[index] = int32(bucket*61 - index*9) }
            for index := range stack.fc1Weight { stack.fc1Weight[index] = int8((index*3+bucket)%13 - 6) }
            stack.fc2Bias[0] = int32(bucket*211 - 700)
            for index := range stack.fc2Weight { stack.fc2Weight[index] = int8((index*5+bucket)%17 - 8) }
        }
        contextModelValue = model
    })
    return contextModelValue
}

func contextPosition(side base.Color, pieces ...base.PieceOnSquare) base.Position {
    return base.Position{SideToMove: side, Pieces: pieces}
}

func contextDelta(t *testing.T, kind base.MoveKind, before, after base.Position, removed, added []base.PieceOnSquare) base.Delta {
    t.Helper()
    _, beforeFacts, err := contextBoardAndFacts(before)
    if err != nil { t.Fatal(err) }
    _, afterFacts, err := contextBoardAndFacts(after)
    if err != nil { t.Fatal(err) }
    var delta base.Delta
    delta.Kind = kind
    delta.RemovedCount = uint8(len(removed))
    delta.AddedCount = uint8(len(added))
    copy(delta.Removed[:], removed)
    copy(delta.Added[:], added)
    delta.Before = beforeFacts
    delta.After = afterFacts
    return delta
}

type frozenContext struct { board [64]boardPiece; frames []contextFrame }

func freezeContext(context *Context) frozenContext {
    return frozenContext{board: context.board, frames: append([]contextFrame(nil), context.frames...)}
}

func requireContextMatchesRefresh(t *testing.T, context *Context, position base.Position) {
    t.Helper()
    full, err := context.model.evaluateAllTrace(position)
    if err != nil { t.Fatal(err) }
    if len(context.frames) == 0 { t.Fatal("context has no frame") }
    frame := context.frames[len(context.frames)-1]
    for perspective := 0; perspective < 2; perspective++ {
        for lane := 0; lane < transformerLanes; lane++ {
            if got, want := frame.values[perspective][lane], full.accumulator.values[perspective][lane]; got != want {
                t.Fatalf("accumulator[%d][%d] = %d, full refresh %d", perspective, lane, got, want)
            }
        }
        for bucket := 0; bucket < psqtBuckets; bucket++ {
            if got, want := frame.psqt[perspective][bucket], full.accumulator.psqt[perspective][bucket]; got != want {
                t.Fatalf("psqt[%d][%d] = %d, full refresh %d", perspective, bucket, got, want)
            }
        }
    }
    got, err := context.EvaluateAll()
    if err != nil { t.Fatal(err) }
    if got.CorrectBucket != full.public.CorrectBucket { t.Fatalf("correct bucket = %d, full refresh %d", got.CorrectBucket, full.public.CorrectBucket) }
    for bucket := 0; bucket < layerStacks; bucket++ {
        if got.Buckets[bucket] != full.public.Buckets[bucket] { t.Fatalf("output stack %d = %+v, full refresh %+v", bucket, got.Buckets[bucket], full.public.Buckets[bucket]) }
    }
}

type realContextCase struct {
    name string
    kind base.MoveKind
    before base.Position
    after base.Position
    removed []base.PieceOnSquare
    added []base.PieceOnSquare
}

func contextRealCases() []realContextCase {
    whiteKing := sfPiece(base.King, base.White, 4)
    blackKing := sfPiece(base.King, base.Black, 60)
    return []realContextCase{
        {name: "quiet", kind: base.MoveNormal,
            before: contextPosition(base.White, whiteKing, blackKing, sfPiece(base.Knight, base.White, 1)),
            after: contextPosition(base.Black, whiteKing, blackKing, sfPiece(base.Knight, base.White, 18)),
            removed: []base.PieceOnSquare{sfPiece(base.Knight, base.White, 1)}, added: []base.PieceOnSquare{sfPiece(base.Knight, base.White, 18)}},
        {name: "capture", kind: base.MoveCapture,
            before: contextPosition(base.White, whiteKing, blackKing, sfPiece(base.Rook, base.White, 0), sfPiece(base.Rook, base.Black, 56)),
            after: contextPosition(base.Black, whiteKing, blackKing, sfPiece(base.Rook, base.White, 56)),
            removed: []base.PieceOnSquare{sfPiece(base.Rook, base.White, 0), sfPiece(base.Rook, base.Black, 56)}, added: []base.PieceOnSquare{sfPiece(base.Rook, base.White, 56)}},
        {name: "en-passant", kind: base.MoveEnPassant,
            before: contextPosition(base.White, whiteKing, blackKing, sfPiece(base.Pawn, base.White, 36), sfPiece(base.Pawn, base.Black, 35)),
            after: contextPosition(base.Black, whiteKing, blackKing, sfPiece(base.Pawn, base.White, 43)),
            removed: []base.PieceOnSquare{sfPiece(base.Pawn, base.White, 36), sfPiece(base.Pawn, base.Black, 35)}, added: []base.PieceOnSquare{sfPiece(base.Pawn, base.White, 43)}},
        {name: "castle", kind: base.MoveCastle,
            before: contextPosition(base.White, whiteKing, blackKing, sfPiece(base.Rook, base.White, 7)),
            after: contextPosition(base.Black, sfPiece(base.King, base.White, 6), blackKing, sfPiece(base.Rook, base.White, 5)),
            removed: []base.PieceOnSquare{whiteKing, sfPiece(base.Rook, base.White, 7)}, added: []base.PieceOnSquare{sfPiece(base.King, base.White, 6), sfPiece(base.Rook, base.White, 5)}},
        {name: "promotion", kind: base.MovePromotion,
            before: contextPosition(base.White, whiteKing, sfPiece(base.King, base.Black, 63), sfPiece(base.Pawn, base.White, 48)),
            after: contextPosition(base.Black, whiteKing, sfPiece(base.King, base.Black, 63), sfPiece(base.Queen, base.White, 56)),
            removed: []base.PieceOnSquare{sfPiece(base.Pawn, base.White, 48)}, added: []base.PieceOnSquare{sfPiece(base.Queen, base.White, 56)}},
        {name: "promotion-capture", kind: base.MovePromotionCapture,
            before: contextPosition(base.White, whiteKing, sfPiece(base.King, base.Black, 63), sfPiece(base.Pawn, base.White, 49), sfPiece(base.Rook, base.Black, 56)),
            after: contextPosition(base.Black, whiteKing, sfPiece(base.King, base.Black, 63), sfPiece(base.Knight, base.White, 56)),
            removed: []base.PieceOnSquare{sfPiece(base.Pawn, base.White, 49), sfPiece(base.Rook, base.Black, 56)}, added: []base.PieceOnSquare{sfPiece(base.Knight, base.White, 56)}},
    }
}

func TestContextRealTransitionsAllLanesAndPSQT(t *testing.T) {
    model := incrementalContextModel()
    for _, test := range contextRealCases() {
        t.Run(test.name, func(t *testing.T) {
            delta := contextDelta(t, test.kind, test.before, test.after, test.removed, test.added)
            for _, mode := range []struct { name string; push func(*Context) error }{
                {"full-position", func(context *Context) error { return context.Push(delta, test.after) }},
                {"compact-delta", func(context *Context) error { return context.PushDelta(delta) }},
            } {
                t.Run(mode.name, func(t *testing.T) {
                    context, err := NewContext(model)
                    if err != nil { t.Fatal(err) }
                    if err := context.Reset(test.before); err != nil { t.Fatal(err) }
                    root := freezeContext(context)
                    if err := mode.push(context); err != nil { t.Fatal(err) }
                    requireContextMatchesRefresh(t, context, test.after)
                    if err := context.Pop(); err != nil { t.Fatal(err) }
                    if got := freezeContext(context); !reflect.DeepEqual(got, root) { t.Fatal("pop did not restore exact root context") }
                })
            }
        })
    }
}

func TestContextMixedRealNullUnwind(t *testing.T) {
    model := incrementalContextModel()
    whiteKing := sfPiece(base.King, base.White, 4)
    blackKing := sfPiece(base.King, base.Black, 60)
    whiteKnightFrom := sfPiece(base.Knight, base.White, 1)
    whiteKnightTo := sfPiece(base.Knight, base.White, 18)
    blackKnightFrom := sfPiece(base.Knight, base.Black, 57)
    blackKnightTo := sfPiece(base.Knight, base.Black, 42)
    whitePawnFrom := sfPiece(base.Pawn, base.White, 8)
    whitePawnTo := sfPiece(base.Pawn, base.White, 16)
    blackPawn := sfPiece(base.Pawn, base.Black, 55)
    root := contextPosition(base.White, whiteKing, blackKing, whiteKnightFrom, blackKnightFrom, whitePawnFrom, blackPawn)
    steps := []struct {
        kind base.MoveKind
        after base.Position
        removed []base.PieceOnSquare
        added []base.PieceOnSquare
    }{
        {base.MoveNormal, contextPosition(base.Black, whiteKing, blackKing, whiteKnightTo, blackKnightFrom, whitePawnFrom, blackPawn), []base.PieceOnSquare{whiteKnightFrom}, []base.PieceOnSquare{whiteKnightTo}},
        {base.MoveNormal, contextPosition(base.White, whiteKing, blackKing, whiteKnightTo, blackKnightTo, whitePawnFrom, blackPawn), []base.PieceOnSquare{blackKnightFrom}, []base.PieceOnSquare{blackKnightTo}},
        {base.MoveNormal, contextPosition(base.Black, whiteKing, blackKing, whiteKnightTo, blackKnightTo, whitePawnTo, blackPawn), []base.PieceOnSquare{whitePawnFrom}, []base.PieceOnSquare{whitePawnTo}},
    }
    for _, mode := range []struct {
        name string
        push func(*Context, base.Delta, base.Position) error
    }{
        {"full-position", func(context *Context, delta base.Delta, after base.Position) error { return context.Push(delta, after) }},
        {"compact-delta", func(context *Context, delta base.Delta, _ base.Position) error { return context.PushDelta(delta) }},
    } {
        t.Run(mode.name, func(t *testing.T) {
            context, err := NewContext(model)
            if err != nil { t.Fatal(err) }
            if err := context.Reset(root); err != nil { t.Fatal(err) }
            rootSnapshot := freezeContext(context)
            positions := []base.Position{root}
            for index, step := range steps {
                before := positions[len(positions)-1]
                delta := contextDelta(t, step.kind, before, step.after, step.removed, step.added)
                if err := mode.push(context, delta, step.after); err != nil { t.Fatalf("real push %d: %v", index, err) }
                positions = append(positions, step.after)
                requireContextMatchesRefresh(t, context, step.after)
                if index == 1 {
                    nullAfter := step.after
                    nullAfter.SideToMove ^= 1
                    if err := context.PushNull(nullAfter); err != nil { t.Fatal(err) }
                    requireContextMatchesRefresh(t, context, nullAfter)
                    if err := context.Pop(); err != nil { t.Fatal(err) }
                    requireContextMatchesRefresh(t, context, step.after)
                }
            }
            for index := len(positions) - 2; index >= 0; index-- {
                if err := context.Pop(); err != nil { t.Fatalf("reverse pop %d: %v", index, err) }
                requireContextMatchesRefresh(t, context, positions[index])
            }
            if context.Depth() != 0 { t.Fatalf("final depth = %d", context.Depth()) }
            if got := freezeContext(context); !reflect.DeepEqual(got, rootSnapshot) { t.Fatal("mixed unwind did not restore exact root") }
        })
    }
}

func TestContextKingRefreshByPerspectiveBucketAndMirror(t *testing.T) {
    tests := []realContextCase{
        {name: "white-mirror-same-bucket", kind: base.MoveNormal,
            before: contextPosition(base.White, sfPiece(base.King, base.White, 3), sfPiece(base.King, base.Black, 60), sfPiece(base.Pawn, base.White, 8), sfPiece(base.Bishop, base.Black, 42)),
            after: contextPosition(base.Black, sfPiece(base.King, base.White, 4), sfPiece(base.King, base.Black, 60), sfPiece(base.Pawn, base.White, 8), sfPiece(base.Bishop, base.Black, 42)),
            removed: []base.PieceOnSquare{sfPiece(base.King, base.White, 3)}, added: []base.PieceOnSquare{sfPiece(base.King, base.White, 4)}},
        {name: "white-bucket-change", kind: base.MoveNormal,
            before: contextPosition(base.White, sfPiece(base.King, base.White, 4), sfPiece(base.King, base.Black, 60), sfPiece(base.Rook, base.White, 0), sfPiece(base.Pawn, base.Black, 48)),
            after: contextPosition(base.Black, sfPiece(base.King, base.White, 12), sfPiece(base.King, base.Black, 60), sfPiece(base.Rook, base.White, 0), sfPiece(base.Pawn, base.Black, 48)),
            removed: []base.PieceOnSquare{sfPiece(base.King, base.White, 4)}, added: []base.PieceOnSquare{sfPiece(base.King, base.White, 12)}},
        {name: "black-mirror-same-bucket", kind: base.MoveNormal,
            before: contextPosition(base.Black, sfPiece(base.King, base.White, 4), sfPiece(base.King, base.Black, 59), sfPiece(base.Knight, base.White, 21), sfPiece(base.Pawn, base.Black, 48)),
            after: contextPosition(base.White, sfPiece(base.King, base.White, 4), sfPiece(base.King, base.Black, 60), sfPiece(base.Knight, base.White, 21), sfPiece(base.Pawn, base.Black, 48)),
            removed: []base.PieceOnSquare{sfPiece(base.King, base.Black, 59)}, added: []base.PieceOnSquare{sfPiece(base.King, base.Black, 60)}},
        {name: "black-bucket-change", kind: base.MoveNormal,
            before: contextPosition(base.Black, sfPiece(base.King, base.White, 4), sfPiece(base.King, base.Black, 60), sfPiece(base.Bishop, base.White, 18), sfPiece(base.Rook, base.Black, 63)),
            after: contextPosition(base.White, sfPiece(base.King, base.White, 4), sfPiece(base.King, base.Black, 52), sfPiece(base.Bishop, base.White, 18), sfPiece(base.Rook, base.Black, 63)),
            removed: []base.PieceOnSquare{sfPiece(base.King, base.Black, 60)}, added: []base.PieceOnSquare{sfPiece(base.King, base.Black, 52)}},
    }
    model := incrementalContextModel()
    for _, test := range tests {
        t.Run(test.name, func(t *testing.T) {
            context, _ := NewContext(model)
            if err := context.Reset(test.before); err != nil { t.Fatal(err) }
            beforeFrame := context.frames[0]
            delta := contextDelta(t, test.kind, test.before, test.after, test.removed, test.added)
            if err := context.PushDelta(delta); err != nil { t.Fatal(err) }
            requireContextMatchesRefresh(t, context, test.after)
            other := int(test.removed[0].Color) ^ 1
            expectedValues := beforeFrame.values[other]
            expectedPSQT := beforeFrame.psqt[other]
            king := beforeFrame.facts.KingSquare[other]
            for _, piece := range test.removed { model.updatePerspective(&expectedValues, &expectedPSQT, piece, king, base.Color(other), false) }
            for _, piece := range test.added { model.updatePerspective(&expectedValues, &expectedPSQT, piece, king, base.Color(other), true) }
            got := context.frames[1]
            if got.values[other] != expectedValues || got.psqt[other] != expectedPSQT { t.Fatal("nonmoving perspective did not use exact incremental update") }
        })
    }
}

func TestContextRejectsCapturedKingTransactionally(t *testing.T) {
    model := incrementalContextModel()
    whiteKing := sfPiece(base.King, base.White, 4)
    blackKing := sfPiece(base.King, base.Black, 60)
    tests := []struct {
        name string
        kind base.MoveKind
        moverBefore base.PieceOnSquare
        moverAfter base.PieceOnSquare
    }{
        {"capture", base.MoveCapture, sfPiece(base.Rook, base.White, 52), sfPiece(base.Rook, base.White, 60)},
        {"promotion-capture", base.MovePromotionCapture, sfPiece(base.Pawn, base.White, 51), sfPiece(base.Queen, base.White, 60)},
    }
    for _, test := range tests {
        t.Run(test.name, func(t *testing.T) {
            before := contextPosition(base.White, whiteKing, blackKing, test.moverBefore)
            malformedAfter := contextPosition(base.Black, whiteKing, test.moverAfter)
            _, beforeFacts, err := contextBoardAndFacts(before)
            if err != nil { t.Fatal(err) }
            afterFacts := base.PositionFacts{
                Occupied: uint64(1)<<whiteKing.Square | uint64(1)<<test.moverAfter.Square,
                KingSquare: [2]base.Square{whiteKing.Square, base.NoSquare},
            }
            badDelta := base.Delta{
                Kind: test.kind,
                RemovedCount: 2,
                AddedCount: 1,
                Removed: [base.MaxDeltaPieces]base.PieceOnSquare{test.moverBefore, blackKing},
                Added: [base.MaxDeltaPieces]base.PieceOnSquare{test.moverAfter},
                Before: beforeFacts,
                After: afterFacts,
            }
            validAfter := contextPosition(base.Black, sfPiece(base.King, base.White, 5), blackKing, test.moverBefore)
            validDelta := contextDelta(t, base.MoveNormal, before, validAfter, []base.PieceOnSquare{whiteKing}, []base.PieceOnSquare{sfPiece(base.King, base.White, 5)})
            for _, mode := range []struct {
                name string
                reject func(*Context) error
                succeed func(*Context) error
            }{
                {"full-position", func(context *Context) error { return context.Push(badDelta, malformedAfter) }, func(context *Context) error { return context.Push(validDelta, validAfter) }},
                {"compact-delta", func(context *Context) error { return context.PushDelta(badDelta) }, func(context *Context) error { return context.PushDelta(validDelta) }},
            } {
                t.Run(mode.name, func(t *testing.T) {
                    context, err := NewContext(model)
                    if err != nil { t.Fatal(err) }
                    if err := context.Reset(before); err != nil { t.Fatal(err) }
                    frozen := freezeContext(context)
                    beforeEvaluation, err := context.EvaluateAll()
                    if err != nil { t.Fatal(err) }
                    beforeDepth := context.Depth()
                    if err := mode.reject(context); !errors.Is(err, ErrContext) { t.Fatalf("captured king error = %v", err) }
                    if got := freezeContext(context); !reflect.DeepEqual(got, frozen) { t.Fatal("captured-king rejection changed context") }
                    if got := context.Depth(); got != beforeDepth { t.Fatalf("depth after rejection = %d, want %d", got, beforeDepth) }
                    afterEvaluation, err := context.EvaluateAll()
                    if err != nil { t.Fatal(err) }
                    if afterEvaluation != beforeEvaluation { t.Fatal("evaluation changed after captured-king rejection") }
                    if err := mode.succeed(context); err != nil { t.Fatalf("valid operation after rejection: %v", err) }
                    requireContextMatchesRefresh(t, context, validAfter)
                })
            }
        })
    }
}

func TestContextNullPopResetAndTransactionality(t *testing.T) {
    model := incrementalContextModel()
    if _, err := NewContext(nil); !errors.Is(err, ErrContext) { t.Fatalf("nil NewContext error = %v", err) }
    if _, err := NewContext(&Model{}); !errors.Is(err, ErrContext) { t.Fatalf("zero NewContext error = %v", err) }
    root := contextRealCases()[0].before
    context, err := NewContext(model)
    if err != nil { t.Fatal(err) }
    if _, err := context.EvaluateAll(); !errors.Is(err, ErrContext) { t.Fatalf("EvaluateAll before Reset error = %v", err) }
    if err := context.Pop(); !errors.Is(err, ErrContext) { t.Fatalf("Pop before Reset error = %v", err) }
    if err := context.Reset(root); err != nil { t.Fatal(err) }
    rootSnapshot := freezeContext(context)
    nullAfter := root
    nullAfter.SideToMove ^= 1
    if err := context.PushNull(nullAfter); err != nil { t.Fatal(err) }
    if context.frames[1].values != context.frames[0].values || context.frames[1].psqt != context.frames[0].psqt { t.Fatal("null changed accumulators") }
    requireContextMatchesRefresh(t, context, nullAfter)
    if err := context.Pop(); err != nil { t.Fatal(err) }
    if got := freezeContext(context); !reflect.DeepEqual(got, rootSnapshot) { t.Fatal("null pop did not restore root") }
    changed := nullAfter
    changed.Pieces = append([]base.PieceOnSquare(nil), nullAfter.Pieces...)
    for index := range changed.Pieces { if changed.Pieces[index].Piece == base.Knight { changed.Pieces[index].Piece = base.Bishop } }
    if err := context.PushNull(changed); !errors.Is(err, ErrContext) { t.Fatalf("changed-board null error = %v", err) }
    if got := freezeContext(context); !reflect.DeepEqual(got, rootSnapshot) { t.Fatal("failed null changed context") }
    real := contextRealCases()[4]
    bad := contextDelta(t, real.kind, real.before, real.after, real.removed, real.added)
    bad.Added[0].Piece = base.Rook
    if err := context.Reset(real.before); err != nil { t.Fatal(err) }
    beforeBad := freezeContext(context)
    if err := context.Push(bad, real.after); !errors.Is(err, ErrContext) { t.Fatalf("wrong promotion error = %v", err) }
    if got := freezeContext(context); !reflect.DeepEqual(got, beforeBad) { t.Fatal("failed real move changed context") }
    badCompact := contextDelta(t, real.kind, real.before, real.after, real.removed, real.added)
    badCompact.After.Occupied ^= uint64(1) << 9
    if err := context.PushDelta(badCompact); !errors.Is(err, ErrContext) { t.Fatalf("bad compact facts error = %v", err) }
    if got := freezeContext(context); !reflect.DeepEqual(got, beforeBad) { t.Fatal("failed compact delta changed context") }
    invalid := base.Position{SideToMove: base.White, Pieces: []base.PieceOnSquare{sfPiece(base.King, base.White, 4)}}
    if err := context.Reset(invalid); err == nil { t.Fatal("invalid Reset succeeded") }
    if got := freezeContext(context); !reflect.DeepEqual(got, beforeBad) { t.Fatal("failed Reset changed context") }
    if err := context.Reset(root); err != nil { t.Fatal(err) }
    if context.Depth() != 0 { t.Fatalf("reset depth = %d", context.Depth()) }
    requireContextMatchesRefresh(t, context, root)
}

func TestContextFrameGrowthAndCheckedArithmetic(t *testing.T) {
    model := incrementalContextModel()
    root := contextRealCases()[0].before
    context, _ := NewContext(model)
    if err := context.Reset(root); err != nil { t.Fatal(err) }
    rootFrame := context.frames[0]
    position := root
    pushes := initialContextFrameCapacity + 17
    for index := 0; index < pushes; index++ {
        position.SideToMove ^= 1
        if err := context.PushNull(position); err != nil { t.Fatalf("PushNull %d: %v", index, err) }
    }
    if context.Depth() != pushes || cap(context.frames) <= initialContextFrameCapacity { t.Fatalf("grown stack depth/capacity = %d/%d", context.Depth(), cap(context.frames)) }
    if context.frames[0] != rootFrame { t.Fatal("growth changed the root frame") }
    requireContextMatchesRefresh(t, context, position)
    for index := 0; index < pushes; index++ { if err := context.Pop(); err != nil { t.Fatal(err) } }
    if context.Depth() != 0 || context.frames[0] != rootFrame { t.Fatal("full growth unwind did not restore root") }
    maximum := maxContextFrameCount()
    if _, err := checkedNextContextLength(maximum); !errors.Is(err, ErrContext) { t.Fatalf("length overflow error = %v", err) }
    if _, err := nextContextCapacity(maximum, maximum); err != nil { t.Fatalf("maximum stable capacity error = %v", err) }
    if capacity, err := nextContextCapacity(1, initialContextFrameCapacity+1); err != nil || capacity < initialContextFrameCapacity+1 { t.Fatalf("capacity growth = %d, %v", capacity, err) }
}

func TestContextWorkerIsolationAndExistingCapacityAllocations(t *testing.T) {
    model := incrementalContextModel()
    type preparedCase struct {
        test realContextCase
        delta base.Delta
        want Trace
        values [2][transformerLanes]int16
        psqt [2][psqtBuckets]int32
    }
    prepared := make([]preparedCase, 0, 2)
    for _, test := range contextRealCases()[:2] {
        delta := contextDelta(t, test.kind, test.before, test.after, test.removed, test.added)
        full, err := model.evaluateAllTrace(test.after)
        if err != nil { t.Fatal(err) }
        prepared = append(prepared, preparedCase{test: test, delta: delta, want: full.public, values: full.accumulator.values, psqt: full.accumulator.psqt})
    }
    var wait sync.WaitGroup
    for _, prepared := range prepared {
        prepared := prepared
        wait.Add(1)
        go func() {
            defer wait.Done()
            context, err := NewContext(model)
            if err != nil { t.Errorf("%s NewContext: %v", prepared.test.name, err); return }
            if err := context.Reset(prepared.test.before); err != nil { t.Errorf("%s Reset: %v", prepared.test.name, err); return }
            for index := 0; index < 32; index++ {
                if err := context.PushDelta(prepared.delta); err != nil { t.Errorf("%s PushDelta: %v", prepared.test.name, err); return }
                got, err := context.EvaluateAll()
                if err != nil { t.Errorf("%s EvaluateAll: %v", prepared.test.name, err); return }
                frame := context.frames[len(context.frames)-1]
                if got != prepared.want || frame.values != prepared.values || frame.psqt != prepared.psqt {
                    t.Errorf("%s worker state differs from full refresh", prepared.test.name)
                    return
                }
                if err := context.Pop(); err != nil { t.Errorf("%s Pop: %v", prepared.test.name, err); return }
            }
        }()
    }
    wait.Wait()
    test := contextRealCases()[0]
    delta := contextDelta(t, test.kind, test.before, test.after, test.removed, test.added)
    context, _ := NewContext(model)
    if err := context.Reset(test.before); err != nil { t.Fatal(err) }
    if allocations := testing.AllocsPerRun(100, func() {
        if err := context.PushDelta(delta); err != nil { panic(err) }
        if err := context.Pop(); err != nil { panic(err) }
    }); allocations != 0 { t.Fatalf("PushDelta+Pop allocations below capacity = %g", allocations) }
    nullAfter := test.before
    nullAfter.SideToMove ^= 1
    if allocations := testing.AllocsPerRun(100, func() {
        if err := context.PushNull(nullAfter); err != nil { panic(err) }
        if err := context.Pop(); err != nil { panic(err) }
    }); allocations != 0 { t.Fatalf("PushNull+Pop allocations below capacity = %g", allocations) }
}
